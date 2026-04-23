package messaging

import (
	"context"
	"database/sql"
	"fmt"
	"pps-services-consumer-database/internal/model"
	"pps-services-consumer-database/internal/pkg/logger"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/sony/gobreaker"
	"github.com/spf13/viper"
)

// Consumer reads messages from RabbitMQ queues and processes them through a circuit breaker.
type Consumer struct {
	rmq      RabbitMQProvider
	handler  MessageHandler
	config   *viper.Viper
	log      zerolog.Logger
	oracleDB *sql.DB
	cb       *gobreaker.CircuitBreaker
	wg       sync.WaitGroup
}

func NewConsumer(rmq RabbitMQProvider, handler MessageHandler, cfg *viper.Viper, log zerolog.Logger, oracleDB *sql.DB) *Consumer {
	cbSettings := gobreaker.Settings{
		Name:        "oracle-cb",
		MaxRequests: uint32(cfg.GetInt("circuit_breaker.max_requests")),
		Interval:    time.Duration(cfg.GetInt("circuit_breaker.interval")) * time.Second,
		Timeout:     time.Duration(cfg.GetInt("circuit_breaker.timeout")) * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			t := cfg.GetInt("circuit_breaker.failure_threshold")
			if t <= 0 {
				t = 3
			}
			return int(counts.ConsecutiveFailures) >= t
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Warn().Msgf("CB '%s': %s -> %s", name, from, to)
			if to == gobreaker.StateOpen {
				logger.SendTelegramLog(cfg, log, "🔴 Circuit breaker OPEN")
			} else if to == gobreaker.StateClosed {
				logger.SendTelegramLog(cfg, log, "🟢 Circuit breaker CLOSED")
			}
		},
	}

	return &Consumer{
		rmq: rmq, handler: handler, config: cfg,
		log: log, oracleDB: oracleDB,
		cb: gobreaker.NewCircuitBreaker(cbSettings),
	}
}

// Start spawns worker goroutines for each queue.
func (c *Consumer) Start(ctx context.Context, queueNames []string) {
	workers := c.config.GetInt("rabbitmq.consumer.workers")
	if workers <= 0 {
		workers = 3
	}
	for _, qn := range queueNames {
		if err := c.rmq.EnsureQueue(qn); err != nil {
			c.log.Error().Err(err).Msgf("EnsureQueue %s failed", qn)
			continue
		}
		for i := 0; i < workers; i++ {
			c.wg.Add(1)
			go c.workerLoop(ctx, qn, i)
		}
		c.log.Info().Msgf("Started %d workers for queue: %s", workers, qn)
	}
}

// workerLoop restarts the consume session when the channel dies, with a configurable delay.
func (c *Consumer) workerLoop(ctx context.Context, queue string, id int) {
	defer c.wg.Done()
	le := c.log.With().Str("queue", queue).Int("worker", id).Logger()

	for {
		select {
		case <-ctx.Done():
			le.Info().Msg("Worker stopped (context cancelled)")
			return
		default:
		}

		err := c.consumeOnce(ctx, queue, id, le)
		if err != nil {
			le.Warn().Err(err).Msg("Worker consume session ended, restarting...")
		}

		restartDelay := c.config.GetInt("rabbitmq.consumer.worker_restart_delay")
		if restartDelay <= 0 {
			restartDelay = 3
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(restartDelay) * time.Second):
		}

		if err := c.rmq.EnsureQueue(queue); err != nil {
			le.Warn().Err(err).Msg("Re-ensure queue failed, will retry")
		}
	}
}

// consumeOnce opens a channel, starts consuming, and processes messages until the channel dies.
func (c *Consumer) consumeOnce(ctx context.Context, queue string, id int, le zerolog.Logger) error {
	conn := c.rmq.GetConnection()
	if conn == nil || conn.IsClosed() {
		return fmt.Errorf("connection not available")
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("create channel: %w", err)
	}
	defer ch.Close()

	prefetch := c.config.GetInt("rabbitmq.consumer.prefetch")
	if prefetch <= 0 {
		prefetch = 10
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return fmt.Errorf("set QoS: %w", err)
	}

	tag := fmt.Sprintf("%s-w%d", queue, id)
	msgs, err := ch.Consume(queue, tag, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start consume: %w", err)
	}

	le.Info().Msg("Consume session started")

	chanClose := ch.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case <-ctx.Done():
			return nil

		case amqpErr := <-chanClose:
			if amqpErr != nil {
				return fmt.Errorf("channel closed: %v", amqpErr)
			}
			return fmt.Errorf("channel closed gracefully")

		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("delivery channel closed")
			}
			c.processMessage(ctx, &msg)
		}
	}
}

// processMessage processes a single message through the circuit breaker.
// When CB is OPEN, the message is held (no ack/nack) to prevent requeue storms.
// RabbitMQ stops delivering new messages once prefetch limit is reached.
func (c *Consumer) processMessage(ctx context.Context, msg *amqp.Delivery) {
	// var event model.TransactionEvent
	// if err := json.Unmarshal(msg.Body, &event); err != nil {
	// 	c.log.Error().Err(err).Msg("Unmarshal failed, rejecting message")
	// 	msg.Reject(false)
	// 	return
	// }
	event := model.TransactionEvent{
		Id:        msg.MessageId,
		QueueName: msg.RoutingKey,
		Headers:   AMQPToHeaders(msg.Headers),
		Payload:   msg.Body,
	}

	cl := logger.ContextLogger(c.log, event.Id)

	for {
		_, err := c.cb.Execute(func() (any, error) {
			return nil, c.handler.Handle(ctx, &event)
		})

		if err == nil {
			msg.Ack(false)
			return
		}

		// CB open — hold message until CB transitions to closed/half-open
		if err == gobreaker.ErrOpenState || err == gobreaker.ErrTooManyRequests {
			cl.Warn().Msg("Circuit breaker open, holding message until CB closed...")

			cbPollInterval := c.config.GetInt("rabbitmq.consumer.cb_poll_interval")
			if cbPollInterval <= 0 {
				cbPollInterval = 2
			}
			for {
				select {
				case <-ctx.Done():
					msg.Nack(false, true)
					return
				case <-time.After(time.Duration(cbPollInterval) * time.Second):
				}

				if c.cb.State() != gobreaker.StateOpen {
					cl.Info().Msg("Circuit breaker no longer open, retrying message...")
					break
				}
			}
			continue
		}

		cl.Warn().Err(err).Msg("Process failed, nack with requeue")
		msg.Nack(false, true)
		return
	}
}

func AMQPToHeaders(table amqp.Table) map[string][]string {
	headers := make(map[string][]string)

	for k, v := range table {
		switch val := v.(type) {
		case string:
			headers[k] = strings.Split(val, ",")
		case []byte:
			headers[k] = strings.Split(string(val), ",")
		default:
			headers[k] = strings.Split(strings.TrimSpace(strings.ReplaceAll(fmt.Sprintf("%v", val), " ", "")), ",")
		}
	}

	return headers
}

// Wait blocks until all workers have finished (for graceful shutdown).
func (c *Consumer) Wait() {
	c.wg.Wait()
}
