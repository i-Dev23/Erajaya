package service

import "context"

// ConsumerService defines the contract for message queue consumer operations.
type ConsumerService interface {
	// Start begins consuming messages and blocks until context is canceled.
	Start(ctx context.Context) error
}
