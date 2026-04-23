package repository

import (
	"context"
	"time"
)

// APILogEntry represents a single API request/response log entry.
type APILogEntry struct {
	Endpoint       string
	Method         string
	RequestURL     string
	RequestHeaders string
	RequestBody    string
	ResponseCode   int
	ResponseBody   string
	DurationMs     int64
	ErrorMessage   string
	CreatedAt      time.Time
}

// APILogRepository defines the interface for persisting API logs.
type APILogRepository interface {
	Insert(ctx context.Context, entry *APILogEntry) error
}
