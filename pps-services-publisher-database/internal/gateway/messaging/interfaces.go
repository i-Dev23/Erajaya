package messaging

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"

	"pps-services-publisher-database/internal/model"
)

// RabbitMQProvider abstracts RabbitMQ connection and channel pool operations.
type RabbitMQProvider interface {
	GetChannel() (*amqp.Channel, error)
	PutChannel(*amqp.Channel)
	GetConnection() *amqp.Connection
	EnsureQueue(queueName string) error
	IsReconnecting() bool
	Ping() error
}

// MessageHandler processes messages consumed from a queue.
type MessageHandler interface {
	Handle(ctx context.Context, event *model.CallbackEvent) error
}
