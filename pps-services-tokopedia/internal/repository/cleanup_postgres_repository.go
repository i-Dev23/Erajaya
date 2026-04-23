package repository

import (
	"context"
	"fmt"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
)

type cleanupPostgresRepository struct {
	postgresService service.PostgresService
	logger          service.Logger
}

// NewCleanupPostgresRepository creates a new cleanup repository
func NewCleanupPostgresRepository(postgresService service.PostgresService, logger service.Logger) domain.CleanupRepository {
	return &cleanupPostgresRepository{
		postgresService: postgresService,
		logger:          logger,
	}
}

func (r *cleanupPostgresRepository) CleanupOldHTTPLogs(ctx context.Context, daysToKeep int) (int, error) {
	return r.executeCleanup(ctx, "SELECT log.cleanup_old_http_logs($1)", daysToKeep)
}

func (r *cleanupPostgresRepository) CleanupOldCallbackLogs(ctx context.Context, daysToKeep int) (int, error) {
	return r.executeCleanup(ctx, "SELECT callback.cleanup_old_callback_logs($1)", daysToKeep)
}

func (r *cleanupPostgresRepository) CleanupOldInquiryLogs(ctx context.Context, daysToKeep int) (int, error) {
	return r.executeCleanup(ctx, "SELECT inquiry.cleanup_old_inquiry_logs($1)", daysToKeep)
}

func (r *cleanupPostgresRepository) CleanupOldPaymentLogs(ctx context.Context, daysToKeep int) (int, error) {
	return r.executeCleanup(ctx, "SELECT payment.cleanup_old_payment_logs($1)", daysToKeep)
}

func (r *cleanupPostgresRepository) executeCleanup(ctx context.Context, query string, daysToKeep int) (int, error) {
	rows, err := r.postgresService.Query(ctx, query, daysToKeep)
	if err != nil {
		r.logger.Error("Failed to execute cleanup query", "error", err, "query", query)
		return 0, fmt.Errorf("failed to execute cleanup query: %w", err)
	}
	defer rows.Close()

	var deletedCount int
	if rows.Next() {
		if err := rows.Scan(&deletedCount); err != nil {
			r.logger.Error("Failed to scan cleanup result", "error", err)
			return 0, fmt.Errorf("failed to scan cleanup result: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating cleanup rows", "error", err)
		return 0, fmt.Errorf("error iterating cleanup rows: %w", err)
	}

	return deletedCount, nil
}
