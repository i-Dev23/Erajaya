package domain

import "context"

// CleanupRepository defines data access for cleanup operations
type CleanupRepository interface {
	CleanupOldHTTPLogs(ctx context.Context, daysToKeep int) (int, error)
	CleanupOldCallbackLogs(ctx context.Context, daysToKeep int) (int, error)
	CleanupOldInquiryLogs(ctx context.Context, daysToKeep int) (int, error)
	CleanupOldPaymentLogs(ctx context.Context, daysToKeep int) (int, error)
}
