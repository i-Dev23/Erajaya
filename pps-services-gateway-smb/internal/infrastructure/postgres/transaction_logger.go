package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
)

// Compile-time interface compliance check.
var _ contractsvc.TransactionLogger = (*PostgresTransactionLogger)(nil)

var migrationStatements = []string{
	`CREATE SCHEMA IF NOT EXISTS transaction`,
	`CREATE TABLE IF NOT EXISTS transaction.smb_transaction (
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
		created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS transaction.smb_transaction_response (
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
		ON transaction.smb_transaction_response (msg_id)`,
	`CREATE INDEX IF NOT EXISTS idx_smb_transaction_our_trx_id
		ON transaction.smb_transaction (our_trx_id)`,
}

// PostgresTransactionLogger mengimplementasikan contractsvc.TransactionLogger.
type PostgresTransactionLogger struct {
	db     *sql.DB
	logger contractsvc.Logger
}

// NewTransactionLogger membuat instance baru dengan connection pool.
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

// DB mengembalikan *sql.DB untuk sharing connection pool.
func (l *PostgresTransactionLogger) DB() *sql.DB {
	return l.db
}

// RunMigration menjalankan DDL idempoten.
func (l *PostgresTransactionLogger) RunMigration(ctx context.Context) error {
	for _, stmt := range migrationStatements {
		if _, err := l.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration failed: %w\nstatement: %s", err, stmt)
		}
	}
	return nil
}

// InsertTransaction menyisipkan baris baru ke smb_transaction.
func (l *PostgresTransactionLogger) InsertTransaction(ctx context.Context, rec contractsvc.TransactionRecord) error {
	_, err := l.db.ExecContext(ctx,
		`INSERT INTO transaction.smb_transaction
			(msg_id, our_trx_id, client_number, mid, product_type, product_code, amount, queue_name, mq_transaction, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'PROCESSING')
		ON CONFLICT (msg_id) DO NOTHING`,
		rec.MsgID, rec.OurTrxID, rec.ClientNumber, rec.MID, rec.ProductType,
		rec.ProductCode, rec.Amount, rec.QueueName, rec.MQTransaction,
	)
	return err
}

// UpdateTransactionStatus memperbarui status transaksi.
func (l *PostgresTransactionLogger) UpdateTransactionStatus(ctx context.Context, msgID string, status string) error {
	var setExtra string
	switch status {
	case "SUCCESS":
		setExtra = ", success_at = NOW()"
	case "FAILED":
		setExtra = ", failed_at = NOW()"
	}

	_, err := l.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE transaction.smb_transaction SET status = $1, updated_at = NOW()%s WHERE msg_id = $2`, setExtra),
		status, msgID,
	)
	return err
}

// InsertSyncResponse menyisipkan baris ke smb_transaction_response.
func (l *PostgresTransactionLogger) InsertSyncResponse(ctx context.Context, rec contractsvc.ResponseRecord) error {
	_, err := l.db.ExecContext(ctx,
		`INSERT INTO transaction.smb_transaction_response
			(msg_id, our_trx_id, smb_trx_id, response_type, status_code, status_desc, request_payload, raw_payload, requested_at, response_latency_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		rec.MsgID, rec.OurTrxID, rec.SMBTrxID, rec.ResponseType,
		rec.StatusCode, rec.StatusDesc, rec.RequestPayload, rec.RawPayload,
		rec.RequestedAt, rec.ResponseLatencyMs,
	)
	return err
}

// GetTransactionByOurTrxID mengambil satu baris dari smb_transaction berdasarkan our_trx_id.
func (l *PostgresTransactionLogger) GetTransactionByOurTrxID(ctx context.Context, ourTrxID string) (*contractsvc.TransactionRecord, error) {
	var rec contractsvc.TransactionRecord
	err := l.db.QueryRowContext(ctx,
		`SELECT msg_id, our_trx_id, client_number, mid, product_type, product_code, amount, queue_name, mq_transaction
		FROM transaction.smb_transaction WHERE our_trx_id = $1`,
		ourTrxID,
	).Scan(&rec.MsgID, &rec.OurTrxID, &rec.ClientNumber, &rec.MID, &rec.ProductType,
		&rec.ProductCode, &rec.Amount, &rec.QueueName, &rec.MQTransaction)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}
