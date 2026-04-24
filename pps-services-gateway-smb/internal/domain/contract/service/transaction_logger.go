package service

import (
	"context"
	"encoding/json"
	"time"
)

// TransactionRecord merepresentasikan data transaksi yang akan dicatat saat pesan RabbitMQ diterima.
type TransactionRecord struct {
	MsgID         string
	OurTrxID      string
	ClientNumber  string
	MID           string
	ProductType   string
	ProductCode   string
	Amount        int
	QueueName     string
	MQTransaction string
}

// ResponseRecord merepresentasikan data respons dari SMB API.
type ResponseRecord struct {
	MsgID             string
	OurTrxID          string
	SMBTrxID          string
	ResponseType      string
	StatusCode        string
	StatusDesc        string
	RequestPayload    json.RawMessage
	RawPayload        json.RawMessage
	RequestedAt       time.Time
	ResponseLatencyMs int64
}

// TransactionLogger mendefinisikan kontrak untuk pencatatan transaksi ke Postgres.
// Semua method bersifat non-blocking terhadap alur utama Consumer:
// jika Postgres tidak tersedia, implementasi harus log error dan return nil.
type TransactionLogger interface {
	// InsertTransaction menyisipkan baris baru ke smb_transaction dengan status PROCESSING.
	// Menggunakan INSERT ... ON CONFLICT (msg_id) DO NOTHING untuk idempotence.
	InsertTransaction(ctx context.Context, rec TransactionRecord) error

	// UpdateTransactionStatus memperbarui kolom status dan updated_at.
	// status harus berupa "SUCCESS", "FAILED", atau "PROCESSING".
	UpdateTransactionStatus(ctx context.Context, msgID string, status string) error

	// InsertSyncResponse menyisipkan baris ke smb_transaction_response dengan response_type = 'SYNC'.
	InsertSyncResponse(ctx context.Context, rec ResponseRecord) error

	// InsertCallbackResponse menyisipkan baris ke smb_transaction_response dengan response_type = 'CALLBACK'.
	// Juga memperbarui status di smb_transaction berdasarkan status_code.
	InsertCallbackResponse(ctx context.Context, rec ResponseRecord) error

	// GetResponsesByMsgID mengambil semua baris smb_transaction_response untuk msg_id tertentu.
	GetResponsesByMsgID(ctx context.Context, msgID string) ([]ResponseRecord, error)

	// GetTransactionStatusByMsgID mengambil nilai kolom status dari smb_transaction berdasarkan msg_id.
	GetTransactionStatusByMsgID(ctx context.Context, msgID string) (string, error)

	// GetTransactionByOurTrxID mengambil satu baris dari smb_transaction berdasarkan our_trx_id.
	GetTransactionByOurTrxID(ctx context.Context, ourTrxID string) (*TransactionRecord, error)

	// RunMigration menjalankan DDL untuk membuat tabel dan index jika belum ada.
	RunMigration(ctx context.Context) error

	// Close menutup connection pool Postgres.
	Close()
}