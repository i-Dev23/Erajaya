package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// RabbitMQService defines the interface for RabbitMQ operations.
type RabbitMQService interface {
	Publish(ctx context.Context, exchange, routingKey string, body []byte, headers amqp091.Table) error
	Consume(ctx context.Context, queueName string) (<-chan amqp091.Delivery, error)
	Ping(ctx context.Context) error
	Close() error
}

// rabbitMQService implements RabbitMQService with connection pooling and channel reuse.
type rabbitMQService struct {
	conn    *amqp091.Connection
	channel *amqp091.Channel
	mu      sync.Mutex
}

// NewRabbitMQService initializes a singleton RabbitMQ connection and channel.
// Reads configuration from environment variables:
//
//	RABBITMQ_URL, RABBITMQ_PUBLISH_TIMEOUT (seconds, optional)
func NewRabbitMQService() (RabbitMQService, error) {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		return nil, fmt.Errorf("RABBITMQ_URL environment variable is required")
	}

	conn, err := amqp091.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	return &rabbitMQService{
		conn:    conn,
		channel: ch,
	}, nil
}

// isConnClosedErr returns true if the error indicates a closed channel/connection
func isConnClosedErr(err error) bool {
	if err == nil {
		return false
	}
	e := strings.ToLower(err.Error())
	return strings.Contains(e, "not open") || strings.Contains(e, "closed") || strings.Contains(e, "channel/connection")
}

// reconnect closes existing connection/channel (if any) and re-dials
func (r *rabbitMQService) reconnect() error {
	// assumes r.mu is already held by caller
	if r.channel != nil {
		_ = r.channel.Close()
		r.channel = nil
	}
	if r.conn != nil {
		_ = r.conn.Close()
		r.conn = nil
	}

	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		return fmt.Errorf("RABBITMQ_URL environment variable is required")
	}

	conn, err := amqp091.Dial(url)
	if err != nil {
		return fmt.Errorf("failed to reconnect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open channel on reconnect: %w", err)
	}

	r.conn = conn
	r.channel = ch
	return nil
}

// Publish sends a message directly to a queue using the default exchange.
// Uses context for timeout/cancellation.
// The queue name is taken from the environment variable "RABBITMQ_QUEUE_NAME" and used as the routingKey.
func (r *rabbitMQService) Publish(ctx context.Context, exchange, routingKey string, body []byte, headers amqp091.Table) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if connection/channel is nil or closed (service not initialized or closed)
	if r.conn == nil || r.channel == nil || r.conn.IsClosed() {
		// Attempt to reconnect
		if err := r.reconnect(); err != nil {
			return fmt.Errorf("rabbitmq connection is nil/closed, reconnect failed: %w", err)
		}
	}

	// Get queue name from environment variable, fallback to provided routingKey if not set
	queueName := os.Getenv("RABBITMQ_QUEUE_NAME")
	if queueName == "" {
		queueName = routingKey
	}

	// Declare queue (idempotent operation - safe to call multiple times)
	// This ensures the queue exists before publishing
	_, err := r.channel.QueueDeclare(
		queueName, // queue name
		true,      // durable - survive broker restart
		false,     // auto-delete - don't delete when unused
		false,     // exclusive - not exclusive to this connection
		false,     // no-wait - don't wait for server confirmation
		nil,       // arguments
	)
	if err != nil {
		if isConnClosedErr(err) {
			if recErr := r.reconnect(); recErr != nil {
				return fmt.Errorf("failed to declare queue '%s' and reconnect: %v; original: %w", queueName, recErr, err)
			}
			// retry declare once
			if _, err2 := r.channel.QueueDeclare(queueName, true, false, false, false, nil); err2 != nil {
				return fmt.Errorf("failed to declare queue '%s' after reconnect: %w", queueName, err2)
			}
		} else {
			return fmt.Errorf("failed to declare queue '%s': %w", queueName, err)
		}
	}

	done := make(chan error, 1)
	go func() {
		err := r.channel.PublishWithContext(
			ctx,
			"", // use default exchange (direct routing to queue)
			queueName,
			true,  // mandatory - return error if message cannot be routed
			false, // immediate - deprecated, always use false
			amqp091.Publishing{
				Headers:      headers,
				ContentType:  "text/plain",
				Body:         body,
				Timestamp:    time.Now(),
				DeliveryMode: amqp091.Persistent, // persistent delivery (2) for durable messages
			},
		)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("publish timeout: %w", ctx.Err())
	case err := <-done:
		if err != nil {
			if isConnClosedErr(err) {
				// attempt reconnect and retry once
				if recErr := r.reconnect(); recErr != nil {
					return fmt.Errorf("failed to publish and reconnect: %v; original: %w", recErr, err)
				}
				// retry publish
				if err2 := r.channel.PublishWithContext(
					ctx,
					"",
					queueName,
					true,
					false,
					amqp091.Publishing{
						Headers:      headers,
						ContentType:  "text/plain",
						Body:         body,
						Timestamp:    time.Now(),
						DeliveryMode: amqp091.Persistent,
					},
				); err2 != nil {
					return fmt.Errorf("failed to publish after reconnect to queue '%s': %w", queueName, err2)
				}
				return nil
			}
			return fmt.Errorf("failed to publish message to queue '%s': %w", queueName, err)
		}
		return nil
	}
}

// Consume starts consuming messages from the specified queue.
// It declares the queue if it does not exist and returns a delivery channel.
// Consumption is cancelled when the provided context is done.
func (r *rabbitMQService) Consume(ctx context.Context, queueName string) (<-chan amqp091.Delivery, error) {
	// Do minimal critical section work under the mutex
	r.mu.Lock()

	// Check if connection/channel is nil or closed
	if r.conn == nil || r.channel == nil || r.conn.IsClosed() {
		if err := r.reconnect(); err != nil {
			r.mu.Unlock()
			return nil, fmt.Errorf("rabbitmq connection is nil/closed, reconnect failed: %w", err)
		}
	}

	// Ensure queue exists (idempotent)
	_, err := r.channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		if isConnClosedErr(err) {
			if recErr := r.reconnect(); recErr != nil {
				r.mu.Unlock()
				return nil, fmt.Errorf("failed to declare queue and reconnect: %v; original: %w", recErr, err)
			}
			// retry declare once
			if _, err2 := r.channel.QueueDeclare(queueName, true, false, false, false, nil); err2 != nil {
				r.mu.Unlock()
				return nil, fmt.Errorf("failed to declare queue '%s' after reconnect: %w", queueName, err2)
			}
		} else {
			r.mu.Unlock()
			return nil, fmt.Errorf("failed to declare queue '%s': %w", queueName, err)
		}
	}

	// Reasonable QoS to avoid flooding a single consumer; prefetch 1
	if err := r.channel.Qos(1, 0, false); err != nil {
		r.mu.Unlock()
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	consumerTag := fmt.Sprintf("consumer-%d", time.Now().UnixNano())
	deliveries, err := r.channel.Consume(
		queueName,
		consumerTag,
		false, // auto-ack disabled; caller should Ack/Nack
		false, // exclusive
		false, // no-local (RabbitMQ ignores this flag)
		false, // no-wait
		nil,   // args
	)
	r.mu.Unlock()
	if err != nil {
		if isConnClosedErr(err) {
			// reconnect then try again
			r.mu.Lock()
			if recErr := r.reconnect(); recErr != nil {
				r.mu.Unlock()
				return nil, fmt.Errorf("failed to start consumer and reconnect: %v; original: %w", recErr, err)
			}
			deliveries, err2 := r.channel.Consume(queueName, consumerTag, false, false, false, false, nil)
			r.mu.Unlock()
			if err2 != nil {
				return nil, fmt.Errorf("failed to start consumer on queue '%s' after reconnect: %w", queueName, err2)
			}
			// Tie lifecycle to context: cancel consumer when ctx is done
			go func() {
				<-ctx.Done()
				_ = r.channel.Cancel(consumerTag, false)
			}()
			return deliveries, nil
		}
		return nil, fmt.Errorf("failed to start consumer on queue '%s': %w", queueName, err)
	}

	// Tie lifecycle to context: cancel consumer when ctx is done
	go func() {
		<-ctx.Done()
		_ = r.channel.Cancel(consumerTag, false)
	}()

	return deliveries, nil
}

// Ping checks if the RabbitMQ connection is healthy
func (r *rabbitMQService) Ping(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conn == nil || r.channel == nil {
		// Attempt to reconnect
		if err := r.reconnect(); err != nil {
			return fmt.Errorf("RabbitMQ connection not initialized, reconnect failed: %w", err)
		}
	}

	// Try to declare a temporary queue to test connectivity
	_, err := r.channel.QueueDeclare(
		"",    // name (empty = auto-generate)
		false, // durable
		true,  // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)

	if err != nil {
		if isConnClosedErr(err) {
			// try reconnect once and ping again
			if recErr := r.reconnect(); recErr != nil {
				return fmt.Errorf("RabbitMQ ping failed and reconnect error: %v; original: %w", recErr, err)
			}
			if _, err2 := r.channel.QueueDeclare("", false, true, true, false, nil); err2 != nil {
				return fmt.Errorf("RabbitMQ ping failed after reconnect: %w", err2)
			}
			return nil
		}
		return fmt.Errorf("RabbitMQ ping failed: %w", err)
	}

	return nil
}

// Close closes the RabbitMQ connection and channel safely.
func (r *rabbitMQService) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.channel != nil {
		_ = r.channel.Close()
		r.channel = nil
	}
	if r.conn != nil {
		_ = r.conn.Close()
		r.conn = nil
	}
	return nil
}
