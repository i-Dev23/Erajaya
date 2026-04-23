package telkomsel

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var txSeq uint64

// InitiateRegularRechargeOnConsume wraps Telkomsel initiate regular recharge call to be executed by a queue consumer.
//
// Params:
// - msisdn: target MSISDN (supports 0xxxxxxxxxxx / 62xxxxxxxxxxx / +62xxxxxxxxxxx)
// - mid: third party id (merchant id)
// - queueName: source queue name (for logging)
// - msgID: upstream message id (used to derive transaction sequence)
// - amount: recharge amount
// - stockType: stock type (e.g. FIXED, BULK, BULK_VALUE_VAS_AKUISISI, etc.)
func InitiateRegularRechargeOnConsume(
	ctx context.Context,
	msisdn string,
	mid string,
	queueName string,
	msgID string,
	amount int,
	stockType string,
) (*InitiateRegularRechargeResponse, error) {
	return InitiateRegularRechargeOnConsumeWithTransactionID(ctx, msisdn, mid, queueName, msgID, "", amount, stockType)
}

// InitiateRegularRechargeOnConsumeWithTransactionID is the same as InitiateRegularRechargeOnConsume,
// but allows the caller to explicitly specify the transaction_id used in the request.
//
// When transactionID is empty, it will be generated using the same logic as the legacy wrapper.
func InitiateRegularRechargeOnConsumeWithTransactionID(
	ctx context.Context,
	msisdn string,
	mid string,
	queueName string,
	msgID string,
	transactionID string,
	amount int,
	stockType string,
) (*InitiateRegularRechargeResponse, error) {
	baseURL := strings.TrimSpace(os.Getenv("BASE_URL"))
	channelID := strings.TrimSpace(os.Getenv("CHANNEL_ID"))
	secretKey := strings.TrimSpace(os.Getenv("SECRET_KEY"))
	apiKey := strings.TrimSpace(os.Getenv("API_KEY"))
	thirdPartyPassword := strings.TrimSpace(os.Getenv("THIRD_PARTY_PASSWORD"))
	thirdPartyID := strings.TrimSpace(os.Getenv("THIRD_PARTY_ID"))
	deliveryChannel := strings.TrimSpace(os.Getenv("DELIVERY_CHANNEL"))
	encryptionKeyB64 := strings.TrimSpace(os.Getenv("ENCRYPTION_KEY"))

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
	if thirdPartyID == "" {
		return nil, fmt.Errorf("THIRD_PARTY_ID env is required")
	}
	if encryptionKeyB64 == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY env is required")
	}
	if thirdPartyPassword == "" {
		return nil, fmt.Errorf("THIRD_PARTY_PASSWORD env is required")
	}
	if deliveryChannel == "" {
		return nil, fmt.Errorf("DELIVERY_CHANNEL env is required")
	}

	if strings.TrimSpace(mid) == "" {
		return nil, fmt.Errorf("mid is required")
	}

	orgCode, pin, err := organizationCodeAndPINFromMIDEnv(mid)
	if err != nil {
		return nil, err
	}

	if amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than zero")
	}

	stockType = strings.ToUpper(strings.TrimSpace(stockType))
	if stockType == "" {
		return nil, fmt.Errorf("stock_type is required")
	}

	timeout := 30 * time.Second
	if raw := strings.TrimSpace(os.Getenv("TIMEOUT")); raw != "" {
		if sec, err := strconv.Atoi(raw); err == nil && sec > 0 {
			timeout = time.Duration(sec) * time.Second
		}
	}

	msisdnNorm, err := normalizeMSISDN(msisdn)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		seq := deriveSequence(msgID)
		transactionID = buildTelkomselTransactionID(orgCode, now, seq)
	}

	element1, err := EncryptElement1(pin, encryptionKeyB64)
	if err != nil {
		return nil, fmt.Errorf("encrypt element1: %w", err)
	}

	logger := slog.Default()
	logger.Info("telkomsel recharge consume wrapper",
		"queue", queueName,
		"msisdn", msisdnNorm,
		"mid", mid,
		"third_party_id", thirdPartyID,
		"msgid", msgID,
		"amount", amount,
		"stock_type", stockType,
		"transaction_id", transactionID,
	)

	client, err := NewClient(baseURL, channelID, secretKey, apiKey, timeout, logger, WithAPILogger(apiLoggerInstance), WithLogContext(msisdnNorm, mid, queueName, msgID))
	if err != nil {
		return nil, err
	}

	req := InitiateRegularRechargeRequest{
		Transaction: InitiateRegularRechargeTransaction{
			TransactionID: transactionID,
			Channel:       channelID,
		},
		Service: InitiateRegularRechargeService{
			OrganizationCode: orgCode,
			ServiceID:        msisdnNorm,
		},
		Recharge: InitiateRegularRechargeRecharge{
			Amount:    amount,
			StockType: stockType,
			Element1:  element1,
		},
		MerchantProfile: InitiateRegularRechargeMerchantProfile{
			ThirdPartyID:       thirdPartyID,
			ThirdPartyPassword: thirdPartyPassword,
			DeliveryChannel:    deliveryChannel,
		},
	}

	return client.InitiateRegularRecharge(ctx, req)
}

func organizationCodeAndPINFromMIDEnv(mid string) (orgCode string, pin string, err error) {
	key := strings.TrimSpace(mid)
	if key == "" {
		return "", "", fmt.Errorf("mid is required")
	}

	// Expected env format: ${ORGANIZATION_CODE}_${PIN}
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		// Be flexible about case; some environments are case-sensitive.
		raw = strings.TrimSpace(os.Getenv(strings.ToUpper(key)))
	}
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv(strings.ToLower(key)))
	}
	if raw == "" {
		return "", "", fmt.Errorf("env %q is required (format: ORGANIZATION_CODE_PIN)", key)
	}

	parts := strings.SplitN(raw, "_", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("env %q must be in format ORGANIZATION_CODE_PIN (got %q)", key, raw)
	}

	orgCode = strings.TrimSpace(parts[0])
	pin = strings.TrimSpace(parts[1])
	if orgCode == "" {
		return "", "", fmt.Errorf("organization code is required in env %q", key)
	}
	if pin == "" {
		return "", "", fmt.Errorf("pin is required in env %q", key)
	}

	return orgCode, pin, nil
}

func normalizeMSISDN(msisdn string) (string, error) {
	m := strings.TrimSpace(msisdn)
	m = strings.TrimPrefix(m, "+")
	if m == "" {
		return "", fmt.Errorf("msisdn is required")
	}
	if strings.HasPrefix(m, "0") {
		m = "62" + strings.TrimPrefix(m, "0")
	}
	if !strings.HasPrefix(m, "62") {
		return "", fmt.Errorf("msisdn must start with 62")
	}
	return m, nil
}

func deriveSequence(msgID string) int {
	raw := strings.TrimSpace(msgID)
	if raw == "" {
		return int(atomic.AddUint64(&txSeq, 1) % 10000)
	}

	if n, err := strconv.Atoi(raw); err == nil {
		if n < 0 {
			n = -n
		}
		return n % 10000
	}

	h := sha1.Sum([]byte(raw))
	n := int(binary.BigEndian.Uint16(h[:2]))
	return n % 10000
}

func buildTelkomselTransactionID(orgCode string, t time.Time, seq int) string {
	org := strings.TrimSpace(orgCode)
	if len(org) > 6 {
		org = org[:6]
	}
	if len(org) < 6 {
		org = org + strings.Repeat("0", 6-len(org))
	}

	ts := t.Format("060102150405") + fmt.Sprintf("%03d", t.Nanosecond()/1e6)
	if seq < 0 {
		seq = -seq
	}
	seq = seq % 10000

	return fmt.Sprintf("%s%s%04d", org, ts, seq)
}
