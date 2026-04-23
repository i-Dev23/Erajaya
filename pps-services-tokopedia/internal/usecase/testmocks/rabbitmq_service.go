package testmocks

import (
	"context"
	"github.com/rabbitmq/amqp091-go"
)

type MockRabbitMQService struct {
	// Add methods as needed for tests
}

func (m *MockRabbitMQService) Close() error {
	return nil
}

func (m *MockRabbitMQService) Publish(ctx context.Context, exchange, routingKey string, body []byte, headers amqp091.Table) error {
	return nil
}

func (m *MockRabbitMQService) Consume(ctx context.Context, queueName string) (<-chan amqp091.Delivery, error) {
	ch := make(chan amqp091.Delivery)
	return ch, nil
}

func (m *MockRabbitMQService) Ping(ctx context.Context) error {
	return nil
}
