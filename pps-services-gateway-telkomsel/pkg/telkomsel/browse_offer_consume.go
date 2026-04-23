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

// BrowseOfferOnConsume wraps Telkomsel browse offer call to be executed by a queue consumer.
//
// Params:
// - msisdn: target MSISDN (supports 0xxxxxxxxxxx / 62xxxxxxxxxxx / +62xxxxxxxxxxx)
// - mid: used to derive organization code from env mapping
// - queueName: source queue name (for logging)
// - msgID: upstream message id (used to derive transaction sequence)
// - productID: Telkomsel product id (will be sent as product_id query param)
func BrowseOfferOnConsume(
	ctx context.Context,
	msisdn string,
	mid string,
	queueName string,
	msgID string,
	productID string,
) (*BrowseOfferResponse, error) {
	return BrowseOfferOnConsumeWithTransactionID(ctx, msisdn, mid, queueName, msgID, "", productID)
}

// BrowseOfferOnConsumeWithTransactionID is the same as BrowseOfferOnConsume, but allows
// the caller to explicitly specify the transaction_id used in the request.
//
// When transactionID is empty, it will be generated using the same logic as the legacy wrapper.
func BrowseOfferOnConsumeWithTransactionID(
	ctx context.Context,
	msisdn string,
	mid string,
	queueName string,
	msgID string,
	transactionID string,
	productID string,
) (*BrowseOfferResponse, error) {
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

	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("product_id is required")
	}

	const version = "v2"

	timeout := 30 * time.Second
	if raw := strings.TrimSpace(os.Getenv("TIMEOUT")); raw != "" {
		if sec, err := strconv.Atoi(raw); err == nil && sec > 0 {
			timeout = time.Duration(sec) * time.Second
		}
	}

	seq := deriveSequence(msgID)
	now := time.Now()
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		transactionID = buildTelkomselTransactionID(orgCode, now, seq)
	}

	logger := slog.Default()
	logger.Info("telkomsel browse offer consume wrapper",
		"queue", queueName,
		"msisdn", msisdnNorm,
		"mid", mid,
		"msgid", msgID,
		"product_id", productID,
		"version", version,
		"transaction_id", transactionID,
	)

	client, err := NewClient(baseURL, channelID, secretKey, apiKey, timeout, logger, WithAPILogger(apiLoggerInstance), WithLogContext(msisdnNorm, mid, queueName, msgID))
	if err != nil {
		return nil, err
	}

	req := BrowseOfferRequest{
		TransactionID:    transactionID,
		Channel:          channelID,
		OrganizationCode: orgCode,
		ServiceID:        msisdnNorm,
		ProductID:        productID,
		Version:          version,
	}

	return client.BrowseOffer(ctx, req)
}
