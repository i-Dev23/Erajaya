package service

import "context"

// APILogEntry represents a single API call log entry for SMB gateway.
type APILogEntry struct {
	Endpoint     string
	Method       string
	ClientNumber string
	MID          string
	QueueName    string
	MsgID        string

	RequestURL     string
	RequestHeaders map[string]string
	RequestBody    []byte

	ResponseStatusCode int
	ResponseBody       []byte
	ResponseDurationMs int

	StatusCode   string
	StatusDesc   string
	ErrorMessage string
	ErrorType    string
}

// APILogRepository defines the contract for persisting API call logs.
type APILogRepository interface {
	// Insert persists a single API log entry.
	Insert(ctx context.Context, entry APILogEntry) error

	// RunMigration executes idempotent DDL for the log_smb schema.
	RunMigration(ctx context.Context) error
}
