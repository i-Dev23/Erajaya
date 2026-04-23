package messaging

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"pps-services-publisher-database/internal/model"
)

// Publisher sends events to RabbitMQ via a direct exchange with channel pooling and retry.
type Publisher struct {
	rmq    RabbitMQProvider
	log    zerolog.Logger
	config *viper.Viper
}

func NewPublisher(rmq RabbitMQProvider, log zerolog.Logger, config *viper.Viper) *Publisher {
	return &Publisher{rmq: rmq, log: log, config: config}
}

// PingRabbitMQ checks if the RabbitMQ connection is alive.
func (p *Publisher) PingRabbitMQ() error {
	return p.rmq.Ping()
}

// Publish sends a TransactionEvent to RabbitMQ with exponential backoff retry.
// headers is optional AMQP headers (e.g. X-Action) that are set on the message, not in the JSON body.
func (p *Publisher) Publish(event *model.CallbackEvent) error {
	if err := p.rmq.EnsureQueue(event.QueueName); err != nil {
		p.log.Error().Err(err).Msgf("EnsureQueue failed: %s", event.QueueName)
		return fmt.Errorf("ensure queue %s: %w", event.QueueName, err)
	}

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = time.Duration(p.config.GetInt("rabbitmq.publish.backoff_initial_interval_ms")) * time.Millisecond
	bo.MaxInterval = time.Duration(p.config.GetInt("rabbitmq.publish.backoff_max_interval")) * time.Second

	operation := func() (struct{}, error) {
		pubErr := p.tryPublish(event)
		if pubErr != nil {
			p.log.Warn().Err(pubErr).Msgf("Publish attempt failed for %s, retrying...", event.ID)
			return struct{}{}, pubErr
		}
		return struct{}{}, nil
	}

	_, err := backoff.Retry(context.Background(), operation,
		backoff.WithBackOff(bo),
		backoff.WithMaxElapsedTime(time.Duration(p.config.GetInt("rabbitmq.publish.backoff_max_elapsed_time"))*time.Second),
	)
	if err != nil {
		p.log.Error().Err(err).Msgf("Publish gave up for event %s after retries", event.ID)
		return fmt.Errorf("publish %s failed after retries: %w", event.ID, err)
	}

	p.log.Debug().Msgf("Event %s published to queue %s", event.ID, event.QueueName)
	return nil
}

func HeadersToAMQP(headers map[string][]string) amqp.Table {
	table := amqp.Table{}

	for key, values := range headers {
		if len(values) > 1 {
			table[key] = strings.Join(values, ",")
		} else if len(values) == 1 {
			table[key] = values[0]
		} else {
			table[key] = ""
		}
	}

	return table
}

func (p *Publisher) tryPublish(event *model.CallbackEvent) error {
	ch, err := p.rmq.GetChannel()
	if err != nil {
		return fmt.Errorf("get channel: %w", err)
	}
	defer p.rmq.PutChannel(ch)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.config.GetInt("rabbitmq.publish.timeout"))*time.Second)
	defer cancel()

	publishing := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         event.Payload,
		MessageId:    event.ID,
		Timestamp:    time.Now(),
		Headers:      HeadersToAMQP(event.Headers),
	}

	return ch.PublishWithContext(ctx,
		p.config.GetString("rabbitmq.exchange.name"),
		event.QueueName,
		false, false,
		publishing,
	)
}
