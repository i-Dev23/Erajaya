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
type TransactionLogger interface {
	InsertTransaction(ctx context.Context, rec TransactionRecord) error
	UpdateTransactionStatus(ctx context.Context, msgID string, status string) error
	InsertSyncResponse(ctx context.Context, rec ResponseRecord) error
	GetTransactionByOurTrxID(ctx context.Context, ourTrxID string) (*TransactionRecord, error)
	RunMigration(ctx context.Context) error
	Close()
}
