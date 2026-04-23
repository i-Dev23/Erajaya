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

// OrderDealerOnConsume wraps Telkomsel order dealer call to be executed by a queue consumer.
//
// Params:
// - msisdn: target MSISDN (supports 0xxxxxxxxxxx / 62xxxxxxxxxxx / +62xxxxxxxxxxx)
// - mid: third party id (merchant id)
// - queueName: source queue name (for logging)
// - msgID: upstream message id (used to derive transaction sequence)
// - productID: Telkomsel product id
// - stockType: stock type (e.g. FIXED, BULK, BULK_VALUE_VAS_AKUISISI, etc.)
// - storeID: merchant store id
// - callbackURL: callback URL for async notification
func OrderDealerOnConsume(
	ctx context.Context,
	msisdn string,
	mid string,
	queueName string,
	msgID string,
	productID string,
	stockType string,
	storeID string,
	callbackURL string,
) (*OrderDealerResponse, error) {
	return OrderDealerOnConsumeWithTransactionID(ctx, msisdn, mid, queueName, msgID, "", productID, stockType, storeID, callbackURL)
}

// OrderDealerOnConsumeWithTransactionID is the same as OrderDealerOnConsume, but allows
// the caller to explicitly specify the transaction_id used in the request.
//
// When transactionID is empty, it will be generated using the same logic as the legacy wrapper.
func OrderDealerOnConsumeWithTransactionID(
	ctx context.Context,
	msisdn string,
	mid string,
	queueName string,
	msgID string,
	transactionID string,
	productID string,
	stockType string,
	storeID string,
	callbackURL string,
) (*OrderDealerResponse, error) {
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

	stockType = strings.ToUpper(strings.TrimSpace(stockType))
	if stockType == "" {
		return nil, fmt.Errorf("stock_type is required")
	}

	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("product_id is required")
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

	channelTimestamp := fmt.Sprintf("%d", now.Unix())
	transmissionDate := now.Format("20060102150405")

	logger := slog.Default()
	logger.Info("telkomsel order dealer consume wrapper",
		"queue", queueName,
		"msisdn", msisdnNorm,
		"mid", mid,
		"third_party_id", thirdPartyID,
		"msgid", msgID,
		"product_id", productID,
		"stock_type", stockType,
		"store_id", storeID,
		"transaction_id", transactionID,
	)

	client, err := NewClient(baseURL, channelID, secretKey, apiKey, timeout, logger, WithAPILogger(apiLoggerInstance), WithLogContext(msisdnNorm, mid, queueName, msgID))
	if err != nil {
		return nil, err
	}

	req := OrderDealerRequest{
		Transaction: OrderDealerTransaction{
			TransactionID: transactionID,
			Channel:       channelID,
		},
		Service: OrderDealerService{
			OrganizationCode: orgCode,
			ServiceID:        msisdnNorm,
		},
		Order: OrderDealerOrder{
			ChannelSLA:       "60",
			ChannelTimestamp: channelTimestamp,
			ProductID:        productID,
			StockType:        stockType,
			Element1:         element1,
			CallbackURL:      strings.TrimSpace(callbackURL),
		},
		MerchantProfile: OrderDealerMerchantProfile{
			ThirdPartyID:       thirdPartyID,
			ThirdPartyPassword: thirdPartyPassword,
			DeliveryChannel:    deliveryChannel,
			StoreID:            storeID,
			TransmissionDate:   transmissionDate,
		},
	}

	return client.OrderDealer(ctx, req)
}
