package postgres

import (
	"context"

	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
	"pps-services-gateway-smb/pkg/smb"
)

// Compile-time interface compliance check.
var _ smb.APILogger = (*APILoggerAdapter)(nil)

// APILoggerAdapter adapts APILogRepository to smb.APILogger interface.
type APILoggerAdapter struct {
	repo   contractsvc.APILogRepository
	logger contractsvc.Logger
}

// NewAPILoggerAdapter creates a new adapter.
func NewAPILoggerAdapter(repo contractsvc.APILogRepository, logger contractsvc.Logger) *APILoggerAdapter {
	return &APILoggerAdapter{repo: repo, logger: logger}
}

// Log persists an API call log entry. Errors are logged but not propagated.
func (a *APILoggerAdapter) Log(ctx context.Context, entry smb.APICallLog) {
	err := a.repo.Insert(ctx, contractsvc.APILogEntry{
		Endpoint:           entry.Endpoint,
		Method:             entry.Method,
		ClientNumber:       entry.ClientNumber,
		MID:                entry.MID,
		QueueName:          entry.QueueName,
		MsgID:              entry.MsgID,
		RequestURL:         entry.RequestURL,
		RequestHeaders:     entry.RequestHeaders,
		RequestBody:        entry.RequestBody,
		ResponseStatusCode: entry.ResponseStatusCode,
		ResponseBody:       entry.ResponseBody,
		ResponseDurationMs: entry.ResponseDurationMs,
		StatusCode:         entry.StatusCode,
		StatusDesc:         entry.StatusDesc,
		ErrorMessage:       entry.ErrorMessage,
		ErrorType:          entry.ErrorType,
	})
	if err != nil {
		a.logger.Error("failed to insert smb api log", "error", err, "endpoint", entry.Endpoint)
	}
}
