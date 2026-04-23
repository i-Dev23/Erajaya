package telkomsel

import (
	"context"
	"errors"
)

// APICallLog represents a single API call log entry.
type APICallLog struct {
	Endpoint              string
	Method                string
	ExternalTransactionID string

	MSISDN    string
	MID       string
	QueueName string
	MsgID     string

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

// APILogger defines the contract for persisting API call logs.
// Implementations should not return errors to avoid disrupting the main flow.
type APILogger interface {
	// Log persists a single API call log entry asynchronously.
	Log(ctx context.Context, entry APICallLog)
}

// apiLoggerInstance adalah package-level variable yang di-set saat startup.
var apiLoggerInstance APILogger

// SetAPILogger menyuntikkan API logger saat aplikasi startup.
// Setelah di-set, semua API call via consume wrappers akan di-log ke database.
func SetAPILogger(l APILogger) {
	apiLoggerInstance = l
}

// logAPICall logs an API call if apiLogger is configured.
func (c *Client) logAPICall(ctx context.Context, entry APICallLog) {
	if c.apiLogger == nil {
		return
	}
	// Inject context fields from client.
	entry.MSISDN = c.logMSISDN
	entry.MID = c.logMID
	entry.QueueName = c.logQueueName
	entry.MsgID = c.logMsgID
	// Fire and forget — don't block the main flow.
	go c.apiLogger.Log(context.WithoutCancel(ctx), entry)
}

// buildLogEntry creates a base APICallLog from common request/response data.
func buildLogEntry(
	endpoint, method, url, externalTransactionID string,
	reqHeaders map[string]string,
	reqBody, respBody []byte,
	respStatusCode int,
	durationMs int,
	resultErr error,
) APICallLog {
	entry := APICallLog{
		Endpoint:              endpoint,
		Method:                method,
		ExternalTransactionID: externalTransactionID,
		RequestURL:            url,
		RequestHeaders:        reqHeaders,
		RequestBody:           reqBody,
		ResponseStatusCode:    respStatusCode,
		ResponseBody:          respBody,
		ResponseDurationMs:    durationMs,
	}
	entry.ErrorType, entry.ErrorMessage = classifyError(resultErr)
	return entry
}

// classifyError returns error_type and error_message from an error.
func classifyError(err error) (errorType, errorMessage string) {
	if err == nil {
		return "", ""
	}
	var bErr *BusinessError
	if errors.As(err, &bErr) {
		return "business", bErr.Error()
	}
	return "technical", err.Error()
}
