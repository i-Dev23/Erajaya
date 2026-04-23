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
	MSISDN        string
	MID           string
	ProductType   string
	ProductID     string
	Amount        int
	StockType     string
	QueueName     string
	MQTransaction string
}

// ResponseRecord merepresentasikan data respons dari Telkomsel API.
type ResponseRecord struct {
	MsgID             string
	OurTrxID          string
	TelkomselTrxID    string
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
	// InsertTransaction menyisipkan baris baru ke telkomsel_transaction dengan status PROCESSING.
	// Menggunakan INSERT ... ON CONFLICT (msg_id) DO NOTHING untuk idempotence.
	InsertTransaction(ctx context.Context, rec TransactionRecord) error

	// UpdateTransactionStatus memperbarui kolom status dan updated_at pada baris dengan msg_id yang sesuai.
	// status harus berupa "SUCCESS" atau "FAILED".
	UpdateTransactionStatus(ctx context.Context, msgID string, status string) error

	// InsertSyncResponse menyisipkan baris ke telkomsel_transaction_response dengan response_type = 'SYNC'.
	InsertSyncResponse(ctx context.Context, rec ResponseRecord) error

	// InsertCallbackResponse menyisipkan baris ke telkomsel_transaction_response dengan response_type = 'CALLBACK'.
	// Juga memperbarui status di telkomsel_transaction berdasarkan status_code.
	InsertCallbackResponse(ctx context.Context, rec ResponseRecord) error

	// GetResponsesByMsgID mengambil semua baris telkomsel_transaction_response untuk msg_id tertentu.
	GetResponsesByMsgID(ctx context.Context, msgID string) ([]ResponseRecord, error)

	// GetTransactionStatusByMsgID mengambil nilai kolom status dari telkomsel_transaction berdasarkan msg_id.
	// Mengembalikan status string ("PROCESSING", "SUCCESS", "FAILED") dan nil error jika ditemukan.
	// Mengembalikan error jika msg_id tidak ditemukan atau terjadi error database.
	GetTransactionStatusByMsgID(ctx context.Context, msgID string) (string, error)

	// GetTransactionByOurTrxID mengambil satu baris dari telkomsel_transaction berdasarkan our_trx_id.
	// Mengembalikan *TransactionRecord dan nil error jika ditemukan.
	// Mengembalikan error jika tidak ditemukan atau terjadi error database.
	GetTransactionByOurTrxID(ctx context.Context, ourTrxID string) (*TransactionRecord, error)

	// RunMigration menjalankan DDL untuk membuat tabel dan index jika belum ada.
	// Bersifat idempoten — aman dipanggil berulang kali.
	RunMigration(ctx context.Context) error

	// Close menutup connection pool Postgres.
	Close()
}
