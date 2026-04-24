package mqpublisher

import (
	"context"
	"fmt"
	"strings"
	"time"

	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Compile-time interface compliance check.
var _ contractsvc.MQPublisher = (*AMQPPublisher)(nil)

// AMQPPublisher mengimplementasikan MQPublisher menggunakan amqp091-go.
type AMQPPublisher struct {
	logger contractsvc.Logger
}

// NewAMQPPublisher membuat instance baru AMQPPublisher.
func NewAMQPPublisher(logger contractsvc.Logger) *AMQPPublisher {
	return &AMQPPublisher{logger: logger}
}

// Publish membuka koneksi baru ke mqTransactionURL, publish body ke queueName.
// Koneksi dan channel ditutup setelah publish selesai.
func (p *AMQPPublisher) Publish(ctx context.Context, mqTransactionURL string, queueName string, body []byte) error {
	if strings.TrimSpace(mqTransactionURL) == "" {
		return fmt.Errorf("mq_transaction URL is empty")
	}
	if strings.TrimSpace(queueName) == "" {
		return fmt.Errorf("queue name is empty")
	}

	conn, err := amqp.Dial(mqTransactionURL)
	if err != nil {
		return fmt.Errorf("dial rabbitmq: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer ch.Close()

	// Do not declare the queue here.
	// Queues are expected to be provisioned by infrastructure/consumer.
	returns := ch.NotifyReturn(make(chan amqp.Return, 1))

	err = ch.PublishWithContext(ctx, "", queueName, true, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
	if err != nil {
		return fmt.Errorf("publish to %s: %w", queueName, err)
	}

	select {
	case ret := <-returns:
		return fmt.Errorf(
			"message unroutable: reply_code=%d reply_text=%s exchange=%s routing_key=%s",
			ret.ReplyCode, ret.ReplyText, ret.Exchange, ret.RoutingKey,
		)
	case <-time.After(500 * time.Millisecond):
		return nil
	}
}
