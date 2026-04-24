package service

import "context"

type MQPublisher interface {
	Publish(ctx context.Context, mqTransactionURL string, queueName string, body []byte) error
}
