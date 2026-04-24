package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
)

// Compile-time interface compliance check.
var _ contractsvc.APILogRepository = (*APILogRepositoryImpl)(nil)

// apiLogMigrationStatements contains idempotent DDL for the log_smb schema.
var apiLogMigrationStatements = []string{
	`CREATE SCHEMA IF NOT EXISTS log_smb`,
	`CREATE TABLE IF NOT EXISTS log_smb.smb_api_logs (
		id                      BIGSERIAL       PRIMARY KEY,
		endpoint                VARCHAR(255)    NOT NULL,
		method                  VARCHAR(10)     NOT NULL,
		client_number           VARCHAR(50),
		mid                     VARCHAR(50),
		queue_name              VARCHAR(100),
		msg_id                  VARCHAR(100),
		request_url             TEXT            NOT NULL,
		request_headers         JSONB,
		request_body            JSONB,
		response_status_code    INT,
		response_body           JSONB,
		response_duration_ms    INT,
		status_code             VARCHAR(100),
		status_desc             TEXT,
		error_message           TEXT,
		error_type              VARCHAR(20),
		created_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_smb_api_logs_client_number ON log_smb.smb_api_logs (client_number)`,
	`CREATE INDEX IF NOT EXISTS idx_smb_api_logs_endpoint ON log_smb.smb_api_logs (endpoint)`,
	`CREATE INDEX IF NOT EXISTS idx_smb_api_logs_created_at ON log_smb.smb_api_logs (created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_smb_api_logs_mid ON log_smb.smb_api_logs (mid)`,
	`CREATE INDEX IF NOT EXISTS idx_smb_api_logs_msg_id ON log_smb.smb_api_logs (msg_id)`,
	`CREATE INDEX IF NOT EXISTS idx_smb_api_logs_status_code ON log_smb.smb_api_logs (status_code)`,
}

// APILogRepositoryImpl implements APILogRepository using PostgreSQL.
type APILogRepositoryImpl struct {
	db     *sql.DB
	logger contractsvc.Logger
}

// NewAPILogRepositoryImpl creates a new PostgreSQL API log repository.
func NewAPILogRepositoryImpl(db *sql.DB, logger contractsvc.Logger) *APILogRepositoryImpl {
	return &APILogRepositoryImpl{db: db, logger: logger}
}

const insertSMBAPILogSQL = `
INSERT INTO log_smb.smb_api_logs (
    endpoint,
    method,
    client_number,
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
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`

// Insert persists a single API log entry to PostgreSQL.
func (r *APILogRepositoryImpl) Insert(ctx context.Context, entry contractsvc.APILogEntry) error {
	headersJSON, err := toJSONB(entry.RequestHeaders)
	if err != nil {
		r.logger.Error("failed to marshal request headers", "error", err)
		headersJSON = nil
	}

	reqBodyJSON := toRawJSONB(entry.RequestBody)
	respBodyJSON := toRawJSONB(entry.ResponseBody)

	_, err = r.db.ExecContext(ctx, insertSMBAPILogSQL,
		entry.Endpoint,
		entry.Method,
		nullIfEmpty(entry.ClientNumber),
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
		return fmt.Errorf("insert smb api log: %w", err)
	}

	return nil
}

// RunMigration executes idempotent DDL for the log_smb schema.
func (r *APILogRepositoryImpl) RunMigration(ctx context.Context) error {
	for _, stmt := range apiLogMigrationStatements {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("api log migration failed: %w\nstatement: %s", err, stmt)
		}
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
