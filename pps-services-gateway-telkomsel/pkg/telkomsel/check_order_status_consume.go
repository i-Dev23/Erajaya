package telkomsel

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// CheckOrderStatusOnConsume wraps Telkomsel check order status call to be executed by a queue consumer.
//
// Params:
// - msisdn: target MSISDN (supports 0xxxxxxxxxxx / 62xxxxxxxxxxx / +62xxxxxxxxxxx)
// - mid: used to derive organization code from env mapping
// - queueName: source queue name (for logging)
// - msgID: upstream message id (used to derive transaction sequence)
// - originalTransactionID: the original transaction id to check status for
// - serialNumber: serial number from the original transaction (optional)
func CheckOrderStatusOnConsume(
	ctx context.Context,
	msisdn string,
	mid string,
	queueName string,
	msgID string,
	originalTransactionID string,
	serialNumber string,
) (*CheckOrderStatusResponse, error) {
	return CheckOrderStatusOnConsumeWithTransactionID(ctx, msisdn, mid, queueName, msgID, "", originalTransactionID, serialNumber)
}

// CheckOrderStatusOnConsumeWithTransactionID is the same as CheckOrderStatusOnConsume, but allows
// the caller to explicitly specify the transaction_id used in the request.
//
// When transactionID is empty, it will be generated using the same logic as the legacy wrapper.
func CheckOrderStatusOnConsumeWithTransactionID(
	ctx context.Context,
	msisdn string,
	mid string,
	queueName string,
	msgID string,
	transactionID string,
	originalTransactionID string,
	serialNumber string,
) (*CheckOrderStatusResponse, error) {
	baseURL := strings.TrimSpace(os.Getenv("BASE_URL"))
	channelID := strings.TrimSpace(os.Getenv("CHANNEL_ID"))
	secretKey := strings.TrimSpace(os.Getenv("SECRET_KEY"))
	apiKey := strings.TrimSpace(os.Getenv("API_KEY"))

	if baseURL == "" {
		return nil, fmt.Errorf("BASE_URL env is required")
	}
	if channelID == "" {
		return nil, fmt.Errorf("CHANNEL_ID env is required")
	}
	if secretKey == "" {
		return nil, fmt.Errorf("SECRET_KEY env is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API_KEY env is required")
	}
	if strings.TrimSpace(mid) == "" {
		return nil, fmt.Errorf("mid is required")
	}

	orgCode, _, err := organizationCodeAndPINFromMIDEnv(mid)
	if err != nil {
		return nil, err
	}

	msisdnNorm, err := normalizeMSISDN(msisdn)
	if err != nil {
		return nil, err
	}

	originalTransactionID = strings.TrimSpace(originalTransactionID)
	if originalTransactionID == "" {
		return nil, fmt.Errorf("original_transaction_id is required")
	}

	timeout := 30 * time.Second
	if raw := strings.TrimSpace(os.Getenv("TIMEOUT")); raw != "" {
		if sec, err := strconv.Atoi(raw); err == nil && sec > 0 {
			timeout = time.Duration(sec) * time.Second
		}
	}

	now := time.Now()
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		seq := deriveSequence(msgID)
		transactionID = buildTelkomselTransactionID(orgCode, now, seq)
	}

	logger := slog.Default()
	logger.Info("telkomsel check order status consume wrapper",
		"queue", queueName,
		"msisdn", msisdnNorm,
		"mid", mid,
		"msgid", msgID,
		"original_transaction_id", originalTransactionID,
		"serial_number", serialNumber,
		"transaction_id", transactionID,
	)

	client, err := NewClient(baseURL, channelID, secretKey, apiKey, timeout, logger, WithAPILogger(apiLoggerInstance), WithLogContext(msisdnNorm, mid, queueName, msgID))
	if err != nil {
		return nil, err
	}

	req := CheckOrderStatusRequest{
		TransactionID:         transactionID,
		OriginalTransactionID: originalTransactionID,
		SerialNumber:          strings.TrimSpace(serialNumber),
		ServiceID:             msisdnNorm,
		Channel:               channelID,
	}

	return client.CheckOrderStatus(ctx, req)
}
