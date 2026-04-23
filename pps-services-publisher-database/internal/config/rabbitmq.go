package config

import (
	"context"
	"fmt"
	"pps-services-publisher-database/internal/pkg/logger"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v5"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

// RabbitMQConnection wraps an AMQP connection with a channel pool and auto-reconnect.
type RabbitMQConnection struct {
	conn           *amqp.Connection
	channels       chan *amqp.Channel
	config         *viper.Viper
	log            zerolog.Logger
	mu             sync.RWMutex
	closed         int32
	reconnecting   int32
	verifiedQueues sync.Map
	notifyReconn   []chan struct{}
	notifyMu       sync.Mutex
}

func NewRabbitMQ(config *viper.Viper, log zerolog.Logger) *RabbitMQConnection {
	poolSize := config.GetInt("rabbitmq.channel_pool_size")
	if poolSize <= 0 {
		poolSize = 5
	}

	rmq := &RabbitMQConnection{
		config:   config,
		log:      log,
		channels: make(chan *amqp.Channel, poolSize),
	}

	if err := rmq.connect(); err != nil {
		log.Fatal().Msgf("Failed initial RabbitMQ connect: %v", err)
	}

	if err := rmq.setupExchange(); err != nil {
		log.Fatal().Msgf("Failed to setup exchange: %v", err)
	}

	rmq.declareStartupQueues()

	go rmq.connectionWatcher()

	return rmq
}

func (r *RabbitMQConnection) connect() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	host := r.config.GetString("rabbitmq.host")
	port := r.config.GetInt("rabbitmq.port")
	username := r.config.GetString("rabbitmq.username")
	password := r.config.GetString("rabbitmq.password")
	vhost := r.config.GetString("rabbitmq.vhost")

	url := fmt.Sprintf("amqp://%s:%s@%s:%d/%s", username, password, host, port, vhost)

	conn, err := amqp.Dial(url)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	r.conn = conn

	r.drainPool()

	// Pool channels without QoS — publisher only. Consumers create their own channels.
	for i := 0; i < cap(r.channels); i++ {
		ch, err := conn.Channel()
		if err != nil {
			conn.Close()
			return fmt.Errorf("create channel[%d] failed: %w", i, err)
		}
		r.channels <- ch
	}

	r.log.Info().Msgf("RabbitMQ connected at %s:%d (pool=%d)", host, port, cap(r.channels))
	return nil
}

func (r *RabbitMQConnection) drainPool() {
	for {
		select {
		case ch := <-r.channels:
			ch.Close()
		default:
			return
		}
	}
}

func (r *RabbitMQConnection) setupExchange() error {
	ch, err := r.conn.Channel()
	if err != nil {
		return fmt.Errorf("channel for exchange failed: %w", err)
	}
	defer ch.Close()

	name := r.config.GetString("rabbitmq.exchange.name")
	kind := r.config.GetString("rabbitmq.exchange.type")
	if err := ch.ExchangeDeclare(name, kind, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %s failed: %w", name, err)
	}
	r.log.Info().Msgf("Exchange '%s' (%s) ready", name, kind)

	dlxEnabled := r.config.GetBool("rabbitmq.dead_letter.enabled")
	dlxName := r.config.GetString("rabbitmq.dead_letter.exchange")
	if dlxEnabled && dlxName != "" {
		if err := ch.ExchangeDeclare(dlxName, kind, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare DLX %s failed: %w", dlxName, err)
		}
		r.log.Info().Msgf("Dead letter exchange '%s' ready", dlxName)
	}

	return nil
}

func (r *RabbitMQConnection) declareStartupQueues() {
	mode := r.config.GetString("rabbitmq.queue_declare_mode")
	if mode != "startup" {
		return
	}

	queues := r.config.GetStringSlice("rabbitmq.startup_queues")
	if len(queues) == 0 {
		r.log.Warn().Msg("queue_declare_mode is 'startup' but startup_queues is empty")
		return
	}

	for _, qn := range queues {
		if err := r.EnsureQueue(qn); err != nil {
			r.log.Error().Err(err).Msgf("Failed to declare startup queue: %s", qn)
		} else {
			r.log.Info().Msgf("Startup queue declared: %s", qn)
		}
	}
}

// connectionWatcher monitors the AMQP connection and triggers reconnect on failure.
func (r *RabbitMQConnection) connectionWatcher() {
	for {
		if atomic.LoadInt32(&r.closed) == 1 {
			return
		}

		r.mu.RLock()
		conn := r.conn
		r.mu.RUnlock()

		if conn == nil {
			time.Sleep(time.Second)
			continue
		}

		notifyClose := conn.NotifyClose(make(chan *amqp.Error, 1))
		amqpErr := <-notifyClose

		if atomic.LoadInt32(&r.closed) == 1 {
			return
		}

		if amqpErr != nil {
			r.log.Warn().Msgf("RabbitMQ connection lost: %v", amqpErr)
			logger.SendTelegramLog(r.config, r.log, fmt.Sprintf("🔴 RabbitMQ lost: %v", amqpErr))
		}

		r.reconnectWithBackoff()
	}
}

// reconnectWithBackoff uses exponential backoff. Only one goroutine reconnects at a time.
func (r *RabbitMQConnection) reconnectWithBackoff() {
	if !atomic.CompareAndSwapInt32(&r.reconnecting, 0, 1) {
		r.waitForReconnect()
		return
	}
	defer atomic.StoreInt32(&r.reconnecting, 0)

	r.log.Info().Msg("Starting RabbitMQ reconnect with exponential backoff...")

	r.verifiedQueues = sync.Map{}

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = time.Duration(r.config.GetInt("rabbitmq.reconnect.initial_interval")) * time.Second
	bo.MaxInterval = time.Duration(r.config.GetInt("rabbitmq.reconnect.max_interval")) * time.Second

	operation := func() (struct{}, error) {
		if atomic.LoadInt32(&r.closed) == 1 {
			return struct{}{}, nil
		}

		if err := r.connect(); err != nil {
			r.log.Warn().Msgf("Reconnect attempt failed: %v", err)
			return struct{}{}, err
		}

		if err := r.setupExchange(); err != nil {
			r.log.Warn().Msgf("Exchange setup after reconnect failed: %v", err)
			return struct{}{}, err
		}

		return struct{}{}, nil
	}

	_, err := backoff.Retry(context.Background(), operation,
		backoff.WithBackOff(bo),
		backoff.WithMaxElapsedTime(0),
	)
	if err != nil {
		r.log.Error().Msgf("Reconnect gave up: %v", err)
		return
	}

	r.log.Info().Msg("RabbitMQ reconnected successfully")
	logger.SendTelegramLog(r.config, r.log, "🟢 RabbitMQ reconnected")

	r.declareStartupQueues()

	r.notifyMu.Lock()
	for _, ch := range r.notifyReconn {
		close(ch)
	}
	r.notifyReconn = nil
	r.notifyMu.Unlock()
}

func (r *RabbitMQConnection) waitForReconnect() {
	ch := make(chan struct{})
	r.notifyMu.Lock()
	r.notifyReconn = append(r.notifyReconn, ch)
	r.notifyMu.Unlock()
	<-ch
}

func (r *RabbitMQConnection) IsReconnecting() bool {
	return atomic.LoadInt32(&r.reconnecting) == 1
}

// GetChannel retrieves a channel from the pool, waiting for reconnect if needed.
func (r *RabbitMQConnection) GetChannel() (*amqp.Channel, error) {
	if atomic.LoadInt32(&r.closed) == 1 {
		return nil, fmt.Errorf("connection closed")
	}

	if r.IsReconnecting() {
		r.waitForReconnect()
	}

	select {
	case ch := <-r.channels:
		if ch.IsClosed() {
			r.mu.RLock()
			conn := r.conn
			r.mu.RUnlock()
			newCh, err := conn.Channel()
			if err != nil {
				return nil, fmt.Errorf("create replacement channel: %w", err)
			}
			return newCh, nil
		}
		return ch, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("channel pool timeout (10s)")
	}
}

// PutChannel returns a channel to the pool. Closed channels are discarded.
func (r *RabbitMQConnection) PutChannel(ch *amqp.Channel) {
	if ch == nil || ch.IsClosed() {
		return
	}
	select {
	case r.channels <- ch:
	default:
		ch.Close()
	}
}

func (r *RabbitMQConnection) GetConnection() *amqp.Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.conn
}

// Ping checks if the RabbitMQ connection is alive.
func (r *RabbitMQConnection) Ping() error {
	if atomic.LoadInt32(&r.closed) == 1 {
		return fmt.Errorf("connection closed")
	}
	if r.IsReconnecting() {
		return fmt.Errorf("reconnecting")
	}
	r.mu.RLock()
	conn := r.conn
	r.mu.RUnlock()
	if conn == nil || conn.IsClosed() {
		return fmt.Errorf("connection not available")
	}
	return nil
}

// EnsureQueue declares a quorum queue with DLX support and binds it to the exchange. Results are cached.
func (r *RabbitMQConnection) EnsureQueue(queueName string) error {
	if _, found := r.verifiedQueues.Load(queueName); found {
		return nil
	}

	r.mu.RLock()
	conn := r.conn
	r.mu.RUnlock()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("channel for queue declare: %w", err)
	}
	defer ch.Close()

	exchangeName := r.config.GetString("rabbitmq.exchange.name")
	dlxEnabled := r.config.GetBool("rabbitmq.dead_letter.enabled")
	dlxName := r.config.GetString("rabbitmq.dead_letter.exchange")
	dlqSuffix := r.config.GetString("rabbitmq.dead_letter.suffix")
	deliveryLimit := r.config.GetInt("rabbitmq.dead_letter.delivery_limit")

	args := amqp.Table{"x-queue-type": "quorum"}
	if dlxEnabled && dlxName != "" {
		dlqName := queueName + dlqSuffix
		args["x-dead-letter-exchange"] = dlxName
		args["x-dead-letter-routing-key"] = dlqName
		if deliveryLimit > 0 {
			args["x-delivery-limit"] = int32(deliveryLimit)
		}
	}

	if _, err = ch.QueueDeclare(queueName, true, false, false, false, args); err != nil {
		return fmt.Errorf("declare queue %s: %w", queueName, err)
	}
	if err = ch.QueueBind(queueName, queueName, exchangeName, false, nil); err != nil {
		return fmt.Errorf("bind queue %s: %w", queueName, err)
	}

	if dlxEnabled && dlxName != "" {
		dlqName := queueName + dlqSuffix
		if _, err = ch.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare DLQ %s: %w", dlqName, err)
		}
		if err = ch.QueueBind(dlqName, dlqName, dlxName, false, nil); err != nil {
			return fmt.Errorf("bind DLQ %s: %w", dlqName, err)
		}
		r.log.Debug().Msgf("DLQ '%s' ensured (delivery_limit=%d)", dlqName, deliveryLimit)
	}

	r.verifiedQueues.Store(queueName, true)
	r.log.Debug().Msgf("Queue '%s' ensured", queueName)
	return nil
}

func (r *RabbitMQConnection) Close() {
	if !atomic.CompareAndSwapInt32(&r.closed, 0, 1) {
		return
	}

	close(r.channels)
	for ch := range r.channels {
		ch.Close()
	}

	r.mu.Lock()
	if r.conn != nil {
		r.conn.Close()
	}
	r.mu.Unlock()

	r.log.Info().Msg("RabbitMQ pool closed")
}
