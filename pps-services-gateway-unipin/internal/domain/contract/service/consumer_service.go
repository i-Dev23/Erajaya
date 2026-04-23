package service

import "context"

// ConsumerService defines the interface for message consumption.
type ConsumerService interface {
	Start(ctx context.Context) error
}
