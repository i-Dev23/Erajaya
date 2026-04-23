package messaging

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"

	"pps-services-consumer-database/internal/model"
)

// RabbitMQProvider abstracts RabbitMQ connection management.
type RabbitMQProvider interface {
	GetConnection() *amqp.Connection
	EnsureQueue(queueName string) error
	IsReconnecting() bool
}

// MessageHandler processes a consumed message event.
type MessageHandler interface {
	Handle(ctx context.Context, event *model.TransactionEvent) error
}
