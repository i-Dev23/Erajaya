package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"pps-services-gateway-unipin/internal/domain/contract/repository"
)

var _ repository.APILogRepository = (*APILogRepositoryImpl)(nil)

// APILogRepositoryImpl implements APILogRepository using Postgres.
type APILogRepositoryImpl struct {
	db *sql.DB
}

// NewAPILogRepository creates a new APILogRepositoryImpl.
func NewAPILogRepository(db *sql.DB) *APILogRepositoryImpl {
	return &APILogRepositoryImpl{db: db}
}

// Insert persists an API log entry to log_unipin.api_log.
func (r *APILogRepositoryImpl) Insert(ctx context.Context, entry *repository.APILogEntry) error {
	const query = `INSERT INTO log_unipin.api_log
		(endpoint, method, request_url, request_headers, request_body,
		 response_code, response_body, duration_ms, error_message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.db.ExecContext(ctx, query,
		entry.Endpoint,
		entry.Method,
		entry.RequestURL,
		entry.RequestHeaders,
		entry.RequestBody,
		entry.ResponseCode,
		entry.ResponseBody,
		entry.DurationMs,
		entry.ErrorMessage,
		entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert api_log: %w", err)
	}

	return nil
}
