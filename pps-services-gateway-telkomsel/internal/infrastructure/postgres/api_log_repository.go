package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
)

// Compile-time interface compliance check.
var _ contractsvc.APILogRepository = (*APILogRepositoryImpl)(nil)

// APILogRepositoryImpl implements APILogRepository using PostgreSQL.
type APILogRepositoryImpl struct {
	db     *sql.DB
	logger contractsvc.Logger
}

// NewAPILogRepositoryImpl creates a new PostgreSQL API log repository.
func NewAPILogRepositoryImpl(db *sql.DB, logger contractsvc.Logger) *APILogRepositoryImpl {
	return &APILogRepositoryImpl{db: db, logger: logger}
}

const insertAPILogSQL = `
INSERT INTO log.telkomsel_api_logs (
    endpoint,
    method,
    external_transaction_id,
    msisdn,
    mid,
    queue_name,
    msg_id,
    request_url,
    request_headers,
    request_body,
    response_status_code,
    response_body,
    response_duration_ms,
    status_code,
    status_desc,
    error_message,
    error_type
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`

// Insert persists a single API log entry to PostgreSQL.
func (r *APILogRepositoryImpl) Insert(ctx context.Context, entry contractsvc.APILogEntry) error {
	headersJSON, err := toJSONB(entry.RequestHeaders)
	if err != nil {
		r.logger.Error("failed to marshal request headers", "error", err)
		headersJSON = nil
	}

	reqBodyJSON := toRawJSONB(entry.RequestBody)
	respBodyJSON := toRawJSONB(entry.ResponseBody)

	_, err = r.db.ExecContext(ctx, insertAPILogSQL,
		entry.Endpoint,
		entry.Method,
		nullIfEmpty(entry.ExternalTransactionID),
		nullIfEmpty(entry.MSISDN),
		nullIfEmpty(entry.MID),
		nullIfEmpty(entry.QueueName),
		nullIfEmpty(entry.MsgID),
		entry.RequestURL,
		headersJSON,
		reqBodyJSON,
		nullIfZero(entry.ResponseStatusCode),
		respBodyJSON,
		nullIfZero(entry.ResponseDurationMs),
		nullIfEmpty(entry.StatusCode),
		nullIfEmpty(entry.StatusDesc),
		nullIfEmpty(entry.ErrorMessage),
		nullIfEmpty(entry.ErrorType),
	)
	if err != nil {
		return fmt.Errorf("insert api log: %w", err)
	}

	return nil
}

func toJSONB(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func toRawJSONB(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	if !json.Valid(b) {
		// Wrap non-JSON as string
		wrapped, _ := json.Marshal(string(b))
		return wrapped
	}
	return b
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero(n int) any {
	if n == 0 {
		return nil
	}
	return n
}
