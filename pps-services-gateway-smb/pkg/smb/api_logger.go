package smb

import "context"

// APICallLog represents a single API call log entry.
type APICallLog struct {
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

// APILogger defines the interface for logging API calls to persistent storage.
type APILogger interface {
	Log(ctx context.Context, entry APICallLog)
}
