package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
)

// Compile-time interface compliance check.
var _ contractsvc.TransactionLogger = (*PostgresTransactionLogger)(nil)

// migrationStatements contains idempotent DDL statements executed during RunMigration.
var migrationStatements = []string{
	`CREATE SCHEMA IF NOT EXISTS transaction_smb`,
	`CREATE TABLE IF NOT EXISTS transaction_smb.smb_transaction (
		msg_id             VARCHAR     PRIMARY KEY,
		our_trx_id         VARCHAR     NOT NULL,
		client_number      VARCHAR     NOT NULL,
		mid                VARCHAR     NOT NULL,
		product_type       VARCHAR     NOT NULL,
		product_code       VARCHAR,
		amount             INTEGER     NOT NULL,
		queue_name         VARCHAR     NOT NULL,
		mq_transaction     VARCHAR,
		status             VARCHAR     NOT NULL DEFAULT 'PROCESSING',
		processing_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		success_at         TIMESTAMPTZ,
		failed_at          TIMESTAMPTZ,
		first_requested_at TIMESTAMPTZ,
		last_response_at   TIMESTAMPTZ,
		created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS transaction_smb.smb_transaction_response (
		id                  BIGSERIAL   PRIMARY KEY,
		msg_id              VARCHAR     NOT NULL,
		our_trx_id          VARCHAR     NOT NULL,
		smb_trx_id          VARCHAR,
		response_type       VARCHAR     NOT NULL,
		status_code         VARCHAR,
		status_desc         VARCHAR,
		request_payload     JSONB,
		raw_payload         JSONB,
		requested_at        TIMESTAMPTZ,
		response_latency_ms INTEGER,
		received_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_smb_transaction_response_msg_id
		ON transaction_smb.smb_transaction_response (msg_id)`,
	`CREATE INDEX IF NOT EXISTS idx_smb_transaction_our_trx_id
		ON transaction_smb.smb_transaction (our_trx_id)`,
}

// PostgresTransactionLogger mengimplementasikan contractsvc.TransactionLogger menggunakan pgx/v5 stdlib.
type PostgresTransactionLogger struct {
	db     *sql.DB
	logger contractsvc.Logger
}

// NewTransactionLogger membuat instance baru dengan connection pool pgx/v5.
func NewTransactionLogger(dsn string, logger contractsvc.Logger) (*PostgresTransactionLogger, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PostgresTransactionLogger{db: db, logger: logger}, nil
}

// Close menutup connection pool.
func (l *PostgresTransactionLogger) Close() {
	if l.db != nil {
		l.db.Close()
	}
}

// DB mengembalikan *sql.DB yang digunakan oleh PostgresTransactionLogger.
func (l *PostgresTransactionLogger) DB() *sql.DB {
	return l.db
}

// RunMigration menjalankan DDL idempoten untuk membuat tabel dan index.
func (l *PostgresTransactionLogger) RunMigration(ctx context.Context) error {
	for _, stmt := range migrationStatements {
		if _, err := l.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration failed: %w\nstatement: %s", err, stmt)
		}
	}
	return nil
}

// --- INSERT ---

const insertTransactionSQL = `
INSERT INTO transaction_smb.smb_transaction
    (msg_id, our_trx_id, client_number, mid, product_type, product_code, amount,
     queue_name, mq_transaction, status, first_requested_at)
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'PROCESSING', $10)
ON CONFLICT (msg_id) DO NOTHING`

// InsertTransaction menyisipkan baris PROCESSING ke smb_transaction.
func (l *PostgresTransactionLogger) InsertTransaction(ctx context.Context, rec contractsvc.TransactionRecord) error {
	_, err := l.db.ExecContext(ctx, insertTransactionSQL,
		rec.MsgID, rec.OurTrxID, rec.ClientNumber, rec.MID, rec.ProductType,
		rec.ProductCode, rec.Amount, rec.QueueName, rec.MQTransaction, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}
	return nil
}

// --- UPDATE STATUS ---

const updateStatusSuccessSQL = `
UPDATE transaction_smb.smb_transaction
SET status = 'SUCCESS', success_at = NOW(), last_response_at = NOW(), updated_at = NOW()
WHERE msg_id = $1`

const updateStatusFailedSQL = `
UPDATE transaction_smb.smb_transaction
SET status = 'FAILED', failed_at = NOW(), last_response_at = NOW(), updated_at = NOW()
WHERE msg_id = $1`

const updateStatusProcessingSQL = `
UPDATE transaction_smb.smb_transaction
SET status = 'PROCESSING', updated_at = NOW()
WHERE msg_id = $1`

// UpdateTransactionStatus memperbarui status dan updated_at.
func (l *PostgresTransactionLogger) UpdateTransactionStatus(ctx context.Context, msgID string, status string) error {
	var q string
	switch status {
	case "SUCCESS":
		q = updateStatusSuccessSQL
	case "FAILED":
		q = updateStatusFailedSQL
	case "PROCESSING":
		q = updateStatusProcessingSQL
	default:
		return fmt.Errorf("invalid status: %s", status)
	}
	_, err := l.db.ExecContext(ctx, q, msgID)
	if err != nil {
		return fmt.Errorf("update transaction status: %w", err)
	}
	return nil
}

// --- INSERT RESPONSE ---

const insertResponseSQL = `
INSERT INTO transaction_smb.smb_transaction_response
    (msg_id, our_trx_id, smb_trx_id, response_type, status_code, status_desc,
     request_payload, raw_payload, requested_at, response_latency_ms)
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

const updateLastResponseAtSQL = `
UPDATE transaction_smb.smb_transaction SET last_response_at = NOW() WHERE msg_id = $1`

// InsertSyncResponse menyisipkan baris SYNC ke smb_transaction_response.
func (l *PostgresTransactionLogger) InsertSyncResponse(ctx context.Context, rec contractsvc.ResponseRecord) error {
	var reqPayload, rawPayload interface{}
	if rec.RequestPayload != nil {
		reqPayload = []byte(rec.RequestPayload)
	}
	if rec.RawPayload != nil {
		rawPayload = []byte(rec.RawPayload)
	}

	_, err := l.db.ExecContext(ctx, insertResponseSQL,
		rec.MsgID, rec.OurTrxID, rec.SMBTrxID, "SYNC", rec.StatusCode, rec.StatusDesc,
		reqPayload, rawPayload, rec.RequestedAt, rec.ResponseLatencyMs,
	)
	if err != nil {
		return fmt.Errorf("insert sync response: %w", err)
	}

	_, err = l.db.ExecContext(ctx, updateLastResponseAtSQL, rec.MsgID)
	if err != nil {
		return fmt.Errorf("update last_response_at: %w", err)
	}
	return nil
}

// InsertCallbackResponse menyisipkan baris CALLBACK dan memperbarui status transaksi.
func (l *PostgresTransactionLogger) InsertCallbackResponse(ctx context.Context, rec contractsvc.ResponseRecord) error {
	var reqPayload, rawPayload interface{}
	if rec.RequestPayload != nil {
		reqPayload = []byte(rec.RequestPayload)
	}
	if rec.RawPayload != nil {
		rawPayload = []byte(rec.RawPayload)
	}

	_, err := l.db.ExecContext(ctx, insertResponseSQL,
		rec.MsgID, rec.OurTrxID, rec.SMBTrxID, "CALLBACK", rec.StatusCode, rec.StatusDesc,
		reqPayload, rawPayload, rec.RequestedAt, rec.ResponseLatencyMs,
	)
	if err != nil {
		return fmt.Errorf("insert callback response: %w", err)
	}

	// Tentukan status berdasarkan status_code dari SMB
	status := "FAILED"
	if rec.StatusCode == "00" {
		status = "SUCCESS"
	}
	return l.UpdateTransactionStatus(ctx, rec.MsgID, status)
}

// --- QUERY ---

const getTransactionStatusByMsgIDSQL = `
SELECT status FROM transaction_smb.smb_transaction WHERE msg_id = $1 LIMIT 1`

// GetTransactionStatusByMsgID mengambil nilai kolom status dari smb_transaction berdasarkan msg_id.
func (l *PostgresTransactionLogger) GetTransactionStatusByMsgID(ctx context.Context, msgID string) (string, error) {
	var status string
	err := l.db.QueryRowContext(ctx, getTransactionStatusByMsgIDSQL, msgID).Scan(&status)
	if err != nil {
		return "", fmt.Errorf("get transaction status by msg_id: %w", err)
	}
	return status, nil
}

const getTransactionByOurTrxIDSQL = `
SELECT msg_id, our_trx_id, client_number, mid, product_type, product_code, amount,
       queue_name, mq_transaction
FROM transaction_smb.smb_transaction
WHERE our_trx_id = $1
LIMIT 1`

// GetTransactionByOurTrxID mengambil satu baris dari smb_transaction berdasarkan our_trx_id.
func (l *PostgresTransactionLogger) GetTransactionByOurTrxID(ctx context.Context, ourTrxID string) (*contractsvc.TransactionRecord, error) {
	var rec contractsvc.TransactionRecord
	var productCode, mqTransaction sql.NullString

	err := l.db.QueryRowContext(ctx, getTransactionByOurTrxIDSQL, ourTrxID).Scan(
		&rec.MsgID, &rec.OurTrxID, &rec.ClientNumber, &rec.MID,
		&rec.ProductType, &productCode, &rec.Amount,
		&rec.QueueName, &mqTransaction,
	)
	if err != nil {
		return nil, fmt.Errorf("get transaction by our_trx_id: %w", err)
	}

	rec.ProductCode = productCode.String
	rec.MQTransaction = mqTransaction.String

	return &rec, nil
}

const getResponsesByMsgIDSQL = `
SELECT msg_id, our_trx_id, smb_trx_id, response_type, status_code, status_desc,
       request_payload, raw_payload, requested_at, response_latency_ms
FROM transaction_smb.smb_transaction_response
WHERE msg_id = $1
ORDER BY received_at ASC`

// GetResponsesByMsgID mengambil semua response untuk msg_id tertentu.
func (l *PostgresTransactionLogger) GetResponsesByMsgID(ctx context.Context, msgID string) ([]contractsvc.ResponseRecord, error) {
	rows, err := l.db.QueryContext(ctx, getResponsesByMsgIDSQL, msgID)
	if err != nil {
		return nil, fmt.Errorf("query responses by msg_id: %w", err)
	}
	defer rows.Close()

	var results []contractsvc.ResponseRecord
	for rows.Next() {
		var (
			rec          contractsvc.ResponseRecord
			smbTrxID     sql.NullString
			responseType string
			statusCode   sql.NullString
			statusDesc   sql.NullString
			reqPayload   []byte
			rawPayload   []byte
			requestedAt  sql.NullTime
			latencyMs    sql.NullInt64
		)
		if err := rows.Scan(
			&rec.MsgID, &rec.OurTrxID, &smbTrxID, &responseType,
			&statusCode, &statusDesc, &reqPayload, &rawPayload,
			&requestedAt, &latencyMs,
		); err != nil {
			return nil, fmt.Errorf("scan response row: %w", err)
		}
		rec.SMBTrxID = smbTrxID.String
		rec.ResponseType = responseType
		rec.StatusCode = statusCode.String
		rec.StatusDesc = statusDesc.String
		if reqPayload != nil {
			rec.RequestPayload = reqPayload
		}
		if rawPayload != nil {
			rec.RawPayload = rawPayload
		}
		if requestedAt.Valid {
			rec.RequestedAt = requestedAt.Time
		}
		if latencyMs.Valid {
			rec.ResponseLatencyMs = latencyMs.Int64
		}
		results = append(results, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate response rows: %w", err)
	}

	if results == nil {
		results = []contractsvc.ResponseRecord{}
	}
	return results, nil
}
