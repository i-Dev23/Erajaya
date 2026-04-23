package telkomsel

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	initiateRegularRechargePath = "/esb/v1/modern/recharge/dealer"
	browseOfferPath             = "/esb/v1/modern/offer/dealer"
	orderDealerPath             = "/esb/v1/modern/order/dealer"
	checkOrderStatusPath        = "/esb/v1/modern/dealer/order/status"
	defaultMaxRetries           = 2
	defaultRetryBackoff         = 300 * time.Millisecond
)

// Client is a reusable Telkomsel ESB Modern Channel API client.
type Client struct {
	baseURL    string
	channelID  string
	secretKey  string
	apiKey     string
	timeout    time.Duration
	httpClient *http.Client
	logger     *slog.Logger
	maxRetries int
	apiLogger  APILogger

	// Context fields for API logging (set via WithLogContext option).
	logMSISDN    string
	logMID       string
	logQueueName string
	logMsgID     string
}

// NewClient builds a new Telkomsel client.
func NewClient(baseURL, channelID, secretKey, apiKey string, timeout time.Duration, logger *slog.Logger, opts ...ClientOption) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	channelID = strings.TrimSpace(channelID)
	secretKey = strings.TrimSpace(secretKey)
	apiKey = strings.TrimSpace(apiKey)

	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}
	if channelID == "" {
		return nil, fmt.Errorf("channelID is required")
	}
	if secretKey == "" {
		return nil, fmt.Errorf("secretKey is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("apiKey is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("timeout must be greater than zero")
	}
	if logger == nil {
		logger = slog.Default()
	}

	c := &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		channelID: channelID,
		secretKey: secretKey,
		apiKey:    apiKey,
		timeout:   timeout,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger:     logger,
		maxRetries: defaultMaxRetries,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// ClientOption configures optional Client settings.
type ClientOption func(*Client)

// WithAPILogger sets the API logger for persisting request/response logs.
func WithAPILogger(l APILogger) ClientOption {
	return func(c *Client) {
		c.apiLogger = l
	}
}

// WithLogContext sets contextual fields (msisdn, mid, queueName, msgID) for API log entries.
func WithLogContext(msisdn, mid, queueName, msgID string) ClientOption {
	return func(c *Client) {
		c.logMSISDN = msisdn
		c.logMID = mid
		c.logQueueName = queueName
		c.logMsgID = msgID
	}
}

// InitiateRegularRecharge calls Telkomsel Initiate Regular Recharge endpoint.
func (c *Client) InitiateRegularRecharge(ctx context.Context, req InitiateRegularRechargeRequest) (*InitiateRegularRechargeResponse, error) {
	if err := validateInitiateRegularRechargeRequest(req); err != nil {
		return nil, err
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	timestamp := unixNowUTC()
	signature, err := generateSignature(timestamp)
	if err != nil {
		return nil, fmt.Errorf("generate signature: %w", err)
	}

	url := c.baseURL + initiateRegularRechargePath
	externalTransactionID := req.Transaction.TransactionID

	attempts := c.maxRetries + 1
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, c.timeout)
		resp, err := c.doInitiateRegularRecharge(callCtx, url, externalTransactionID, timestamp, signature, bodyBytes)
		cancel()

		if err == nil {
			return resp, nil
		}

		lastErr = err

		if !isRetryableError(err) || attempt == attempts {
			if resp != nil {
				return resp, err
			}
			return nil, err
		}

		backoff := time.Duration(attempt) * defaultRetryBackoff
		c.logger.Warn("retrying initiate regular recharge",
			"attempt", attempt,
			"max_attempts", attempts,
			"backoff", backoff.String(),
			"external_transaction_id", externalTransactionID,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, &TechnicalError{Cause: ctx.Err()}
		case <-time.After(backoff):
		}
	}

	return nil, lastErr
}

// OrderDealer calls Telkomsel Order Dealer endpoint.
func (c *Client) OrderDealer(ctx context.Context, req OrderDealerRequest) (*OrderDealerResponse, error) {
	if err := validateOrderDealerRequest(req); err != nil {
		return nil, err
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	timestamp := unixNowUTC()
	signature, err := generateSignature(timestamp)
	if err != nil {
		return nil, fmt.Errorf("generate signature: %w", err)
	}

	url := c.baseURL + orderDealerPath
	externalTransactionID := req.Transaction.TransactionID

	attempts := c.maxRetries + 1
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, c.timeout)
		resp, err := c.doOrderDealer(callCtx, url, externalTransactionID, timestamp, signature, bodyBytes)
		cancel()

		if err == nil {
			return resp, nil
		}

		lastErr = err

		if !isRetryableError(err) || attempt == attempts {
			if resp != nil {
				return resp, err
			}
			return nil, err
		}

		backoff := time.Duration(attempt) * defaultRetryBackoff
		c.logger.Warn("retrying order dealer",
			"attempt", attempt,
			"max_attempts", attempts,
			"backoff", backoff.String(),
			"external_transaction_id", externalTransactionID,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, &TechnicalError{Cause: ctx.Err()}
		case <-time.After(backoff):
		}
	}

	return nil, lastErr
}

// CheckOrderStatus calls Telkomsel Check Order Status endpoint.
func (c *Client) CheckOrderStatus(ctx context.Context, req CheckOrderStatusRequest) (*CheckOrderStatusResponse, error) {
	if err := validateCheckOrderStatusRequest(req); err != nil {
		return nil, err
	}

	timestamp := unixNowUTC()
	signature, err := generateSignature(timestamp)
	if err != nil {
		return nil, fmt.Errorf("generate signature: %w", err)
	}

	fullURL, err := buildCheckOrderStatusURL(c.baseURL+checkOrderStatusPath, req)
	if err != nil {
		return nil, err
	}

	externalTransactionID := strings.TrimSpace(req.TransactionID)

	attempts := c.maxRetries + 1
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, c.timeout)
		resp, err := c.doCheckOrderStatus(callCtx, fullURL, externalTransactionID, timestamp, signature)
		cancel()

		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !isRetryableError(err) || attempt == attempts {
			if resp != nil {
				return resp, err
			}
			return nil, err
		}

		backoff := time.Duration(attempt) * defaultRetryBackoff
		c.logger.Warn("retrying check order status",
			"attempt", attempt,
			"max_attempts", attempts,
			"backoff", backoff.String(),
			"external_transaction_id", externalTransactionID,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, &TechnicalError{Cause: ctx.Err()}
		case <-time.After(backoff):
		}
	}

	return nil, lastErr
}

// BrowseOffer calls Telkomsel Browse Offer endpoint.
func (c *Client) BrowseOffer(ctx context.Context, req BrowseOfferRequest) (*BrowseOfferResponse, error) {
	if err := validateBrowseOfferRequest(req); err != nil {
		return nil, err
	}

	timestamp := unixNowUTC()
	signature, err := generateSignature(timestamp)
	if err != nil {
		return nil, fmt.Errorf("generate signature: %w", err)
	}

	fullURL, err := buildBrowseOfferURL(c.baseURL+browseOfferPath, req)
	if err != nil {
		return nil, err
	}

	externalTransactionID := strings.TrimSpace(req.TransactionID)

	attempts := c.maxRetries + 1
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, c.timeout)
		resp, err := c.doBrowseOffer(callCtx, fullURL, externalTransactionID, timestamp, signature)
		cancel()

		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !isRetryableError(err) || attempt == attempts {
			if resp != nil {
				return resp, err
			}
			return nil, err
		}

		backoff := time.Duration(attempt) * defaultRetryBackoff
		c.logger.Warn("retrying browse offer",
			"attempt", attempt,
			"max_attempts", attempts,
			"backoff", backoff.String(),
			"external_transaction_id", externalTransactionID,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, &TechnicalError{Cause: ctx.Err()}
		case <-time.After(backoff):
		}
	}

	return nil, lastErr
}

func (c *Client) doBrowseOffer(
	ctx context.Context,
	fullURL, externalTransactionID, timestamp, signature string,
) (*BrowseOfferResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("create request: %w", err)}
	}

	httpReq.Header.Set("api-key", c.apiKey)
	httpReq.Header.Set("Channel-Id", c.channelID)
	httpReq.Header.Set("Timestamp", timestamp)
	httpReq.Header.Set("External-Transaction-Id", externalTransactionID)
	httpReq.Header.Set("x-signature", signature)

	c.logger.Info("telkomsel outgoing request",
		"endpoint", browseOfferPath,
		"method", http.MethodGet,
		"url", fullURL,
		"headers", sanitizeHeadersForLog(httpReq.Header),
		"external_transaction_id", externalTransactionID,
	)

	start := time.Now()
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("execute request: %w", err)}
	}
	defer func() {
		_ = httpResp.Body.Close()
	}()

	duration := time.Since(start)
	bodyResp, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("read response body: %w", err)}
	}

	c.logger.Info("telkomsel incoming response",
		"endpoint", browseOfferPath,
		"method", http.MethodGet,
		"status_code", httpResp.StatusCode,
		"body", sanitizeJSONForLog(bodyResp),
		"external_transaction_id", externalTransactionID,
	)

	c.logger.Info("telkomsel request completed",
		"endpoint", browseOfferPath,
		"method", http.MethodGet,
		"status_code", httpResp.StatusCode,
		"duration_ms", duration.Milliseconds(),
		"external_transaction_id", externalTransactionID,
	)

	if supportID, ok := detectRequestRejected(bodyResp); ok {
		resultErr := &RejectedError{StatusCode: httpResp.StatusCode, SupportID: supportID}
		c.logAPICall(ctx, buildLogEntry(browseOfferPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	if httpResp.StatusCode >= http.StatusInternalServerError {
		resultErr := &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("server error: %s", strings.TrimSpace(string(bodyResp)))}
		c.logAPICall(ctx, buildLogEntry(browseOfferPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	// Telkomsel may return business failure as a 4xx with a JSON body containing transaction.status_code/status_desc.
	if httpResp.StatusCode >= http.StatusBadRequest {
		var apiResp BrowseOfferResponse
		if err := json.Unmarshal(bodyResp, &apiResp); err == nil {
			code := strings.TrimSpace(apiResp.Transaction.StatusCode)
			desc := strings.TrimSpace(apiResp.Transaction.StatusDesc)
			txID := strings.TrimSpace(apiResp.Transaction.TransactionID)
			if code != "" && code != "00000" && code != "RV-0000" {
				resultErr := &BusinessError{Code: code, Message: desc, TransactionID: txID}
				entry := buildLogEntry(browseOfferPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr)
				entry.StatusCode = code
				entry.StatusDesc = desc
				c.logAPICall(ctx, entry)
				return &apiResp, resultErr
			}
		}

		resultErr := &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("http error: %s", strings.TrimSpace(string(bodyResp)))}
		c.logAPICall(ctx, buildLogEntry(browseOfferPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	var apiResp BrowseOfferResponse
	if err := json.Unmarshal(bodyResp, &apiResp); err != nil {
		resultErr := &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("unmarshal response body: %w", err)}
		c.logAPICall(ctx, buildLogEntry(browseOfferPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	statusCode := strings.TrimSpace(apiResp.Transaction.StatusCode)
	if statusCode != "00000" && statusCode != "RV-0000" {
		resultErr := &BusinessError{
			Code:          statusCode,
			Message:       strings.TrimSpace(apiResp.Transaction.StatusDesc),
			TransactionID: strings.TrimSpace(apiResp.Transaction.TransactionID),
		}
		entry := buildLogEntry(browseOfferPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr)
		entry.StatusCode = statusCode
		entry.StatusDesc = strings.TrimSpace(apiResp.Transaction.StatusDesc)
		c.logAPICall(ctx, entry)
		return &apiResp, resultErr
	}

	entry := buildLogEntry(browseOfferPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), nil)
	entry.StatusCode = statusCode
	entry.StatusDesc = strings.TrimSpace(apiResp.Transaction.StatusDesc)
	c.logAPICall(ctx, entry)

	return &apiResp, nil
}

func (c *Client) doOrderDealer(
	ctx context.Context,
	url, externalTransactionID, timestamp, signature string,
	bodyBytes []byte,
) (*OrderDealerResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("create request: %w", err)}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", c.apiKey)
	httpReq.Header.Set("Channel-Id", c.channelID)
	httpReq.Header.Set("Timestamp", timestamp)
	httpReq.Header.Set("External-Transaction-Id", externalTransactionID)
	httpReq.Header.Set("x-signature", signature)

	c.logger.Info("telkomsel outgoing request",
		"endpoint", orderDealerPath,
		"method", http.MethodPost,
		"url", url,
		"headers", sanitizeHeadersForLog(httpReq.Header),
		"body", sanitizeJSONForLog(bodyBytes),
		"external_transaction_id", externalTransactionID,
	)

	start := time.Now()
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("execute request: %w", err)}
	}
	defer func() {
		_ = httpResp.Body.Close()
	}()

	duration := time.Since(start)
	bodyResp, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("read response body: %w", err)}
	}

	c.logger.Info("telkomsel incoming response",
		"endpoint", orderDealerPath,
		"method", http.MethodPost,
		"status_code", httpResp.StatusCode,
		"body", sanitizeJSONForLog(bodyResp),
		"external_transaction_id", externalTransactionID,
	)

	c.logger.Info("telkomsel request completed",
		"endpoint", orderDealerPath,
		"method", http.MethodPost,
		"status_code", httpResp.StatusCode,
		"duration_ms", duration.Milliseconds(),
		"external_transaction_id", externalTransactionID,
	)

	if supportID, ok := detectRequestRejected(bodyResp); ok {
		resultErr := &RejectedError{StatusCode: httpResp.StatusCode, SupportID: supportID}
		c.logAPICall(ctx, buildLogEntry(orderDealerPath, http.MethodPost, url, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), bodyBytes, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	if httpResp.StatusCode >= http.StatusInternalServerError {
		resultErr := &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("server error: %s", strings.TrimSpace(string(bodyResp)))}
		c.logAPICall(ctx, buildLogEntry(orderDealerPath, http.MethodPost, url, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), bodyBytes, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	if httpResp.StatusCode >= http.StatusBadRequest {
		resultErr := &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("http error: %s", strings.TrimSpace(string(bodyResp)))}
		c.logAPICall(ctx, buildLogEntry(orderDealerPath, http.MethodPost, url, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), bodyBytes, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	var apiResp OrderDealerResponse
	if err = json.Unmarshal(bodyResp, &apiResp); err != nil {
		resultErr := &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("unmarshal response body: %w", err)}
		c.logAPICall(ctx, buildLogEntry(orderDealerPath, http.MethodPost, url, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), bodyBytes, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	apiResp.HTTPStatusCode = httpResp.StatusCode

	if strings.TrimSpace(apiResp.Transaction.StatusCode) != "00000" {
		resultErr := &BusinessError{
			Code:          strings.TrimSpace(apiResp.Transaction.StatusCode),
			Message:       strings.TrimSpace(apiResp.Transaction.StatusDesc),
			TransactionID: strings.TrimSpace(apiResp.Transaction.TransactionID),
		}
		entry := buildLogEntry(orderDealerPath, http.MethodPost, url, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), bodyBytes, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr)
		entry.StatusCode = strings.TrimSpace(apiResp.Transaction.StatusCode)
		entry.StatusDesc = strings.TrimSpace(apiResp.Transaction.StatusDesc)
		c.logAPICall(ctx, entry)
		return &apiResp, resultErr
	}

	entry := buildLogEntry(orderDealerPath, http.MethodPost, url, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), bodyBytes, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), nil)
	entry.StatusCode = strings.TrimSpace(apiResp.Transaction.StatusCode)
	entry.StatusDesc = strings.TrimSpace(apiResp.Transaction.StatusDesc)
	c.logAPICall(ctx, entry)

	return &apiResp, nil
}

func (c *Client) doCheckOrderStatus(
	ctx context.Context,
	fullURL, externalTransactionID, timestamp, signature string,
) (*CheckOrderStatusResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("create request: %w", err)}
	}

	httpReq.Header.Set("api-key", c.apiKey)
	httpReq.Header.Set("Channel-Id", c.channelID)
	httpReq.Header.Set("Timestamp", timestamp)
	httpReq.Header.Set("External-Transaction-Id", externalTransactionID)
	httpReq.Header.Set("x-signature", signature)

	c.logger.Info("telkomsel outgoing request",
		"endpoint", checkOrderStatusPath,
		"method", http.MethodGet,
		"url", fullURL,
		"headers", sanitizeHeadersForLog(httpReq.Header),
		"external_transaction_id", externalTransactionID,
	)

	start := time.Now()
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("execute request: %w", err)}
	}
	defer func() {
		_ = httpResp.Body.Close()
	}()

	duration := time.Since(start)
	bodyResp, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("read response body: %w", err)}
	}

	c.logger.Info("telkomsel incoming response",
		"endpoint", checkOrderStatusPath,
		"method", http.MethodGet,
		"status_code", httpResp.StatusCode,
		"body", sanitizeJSONForLog(bodyResp),
		"external_transaction_id", externalTransactionID,
	)

	c.logger.Info("telkomsel request completed",
		"endpoint", checkOrderStatusPath,
		"method", http.MethodGet,
		"status_code", httpResp.StatusCode,
		"duration_ms", duration.Milliseconds(),
		"external_transaction_id", externalTransactionID,
	)

	if supportID, ok := detectRequestRejected(bodyResp); ok {
		resultErr := &RejectedError{StatusCode: httpResp.StatusCode, SupportID: supportID}
		c.logAPICall(ctx, buildLogEntry(checkOrderStatusPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	if httpResp.StatusCode >= http.StatusInternalServerError {
		resultErr := &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("server error: %s", strings.TrimSpace(string(bodyResp)))}
		c.logAPICall(ctx, buildLogEntry(checkOrderStatusPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	if httpResp.StatusCode >= http.StatusBadRequest {
		var apiResp CheckOrderStatusResponse
		if err := json.Unmarshal(bodyResp, &apiResp); err == nil {
			apiResp.HTTPStatusCode = httpResp.StatusCode
			code := strings.TrimSpace(apiResp.Transaction.StatusCode)
			desc := strings.TrimSpace(apiResp.Transaction.StatusDesc)
			txID := strings.TrimSpace(apiResp.Transaction.TransactionID)
			if code != "" && code != "00000" {
				resultErr := &BusinessError{Code: code, Message: desc, TransactionID: txID}
				entry := buildLogEntry(checkOrderStatusPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr)
				entry.StatusCode = code
				entry.StatusDesc = desc
				c.logAPICall(ctx, entry)
				return &apiResp, resultErr
			}
		}

		resultErr := &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("http error: %s", strings.TrimSpace(string(bodyResp)))}
		c.logAPICall(ctx, buildLogEntry(checkOrderStatusPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	var apiResp CheckOrderStatusResponse
	if err := json.Unmarshal(bodyResp, &apiResp); err != nil {
		resultErr := &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("unmarshal response body: %w", err)}
		c.logAPICall(ctx, buildLogEntry(checkOrderStatusPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	apiResp.HTTPStatusCode = httpResp.StatusCode

	if strings.TrimSpace(apiResp.Transaction.StatusCode) != "00000" {
		resultErr := &BusinessError{
			Code:          strings.TrimSpace(apiResp.Transaction.StatusCode),
			Message:       strings.TrimSpace(apiResp.Transaction.StatusDesc),
			TransactionID: strings.TrimSpace(apiResp.Transaction.TransactionID),
		}
		entry := buildLogEntry(checkOrderStatusPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr)
		entry.StatusCode = strings.TrimSpace(apiResp.Transaction.StatusCode)
		entry.StatusDesc = strings.TrimSpace(apiResp.Transaction.StatusDesc)
		c.logAPICall(ctx, entry)
		return &apiResp, resultErr
	}

	entry := buildLogEntry(checkOrderStatusPath, http.MethodGet, fullURL, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), nil, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), nil)
	entry.StatusCode = strings.TrimSpace(apiResp.Transaction.StatusCode)
	entry.StatusDesc = strings.TrimSpace(apiResp.Transaction.StatusDesc)
	c.logAPICall(ctx, entry)

	return &apiResp, nil
}

func (c *Client) doInitiateRegularRecharge(
	ctx context.Context,
	url, externalTransactionID, timestamp, signature string,
	bodyBytes []byte,
) (*InitiateRegularRechargeResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("create request: %w", err)}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", c.apiKey)
	httpReq.Header.Set("Channel-Id", c.channelID)
	httpReq.Header.Set("Timestamp", timestamp)
	httpReq.Header.Set("External-Transaction-Id", externalTransactionID)
	httpReq.Header.Set("x-signature", signature)

	c.logger.Info("telkomsel outgoing request",
		"endpoint", initiateRegularRechargePath,
		"method", http.MethodPost,
		"url", url,
		"headers", sanitizeHeadersForLog(httpReq.Header),
		"body", sanitizeJSONForLog(bodyBytes),
		"external_transaction_id", externalTransactionID,
	)

	start := time.Now()
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("execute request: %w", err)}
	}
	defer func() {
		_ = httpResp.Body.Close()
	}()

	duration := time.Since(start)
	bodyResp, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("read response body: %w", err)}
	}

	c.logger.Info("telkomsel incoming response",
		"endpoint", initiateRegularRechargePath,
		"method", http.MethodPost,
		"status_code", httpResp.StatusCode,
		"body", sanitizeJSONForLog(bodyResp),
		"external_transaction_id", externalTransactionID,
	)

	c.logger.Info("telkomsel request completed",
		"endpoint", initiateRegularRechargePath,
		"method", http.MethodPost,
		"status_code", httpResp.StatusCode,
		"duration_ms", duration.Milliseconds(),
		"external_transaction_id", externalTransactionID,
	)

	if supportID, ok := detectRequestRejected(bodyResp); ok {
		resultErr := &RejectedError{StatusCode: httpResp.StatusCode, SupportID: supportID}
		c.logAPICall(ctx, buildLogEntry(initiateRegularRechargePath, http.MethodPost, url, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), bodyBytes, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	if httpResp.StatusCode >= http.StatusInternalServerError {
		resultErr := &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("server error: %s", strings.TrimSpace(string(bodyResp)))}
		c.logAPICall(ctx, buildLogEntry(initiateRegularRechargePath, http.MethodPost, url, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), bodyBytes, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	if httpResp.StatusCode >= http.StatusBadRequest {
		resultErr := &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("http error: %s", strings.TrimSpace(string(bodyResp)))}
		// Parse response body so caller can access status_code/status_desc even on HTTP 4xx.
		var apiResp InitiateRegularRechargeResponse
		if jsonErr := json.Unmarshal(bodyResp, &apiResp); jsonErr == nil {
			apiResp.HTTPStatusCode = httpResp.StatusCode
			entry := buildLogEntry(initiateRegularRechargePath, http.MethodPost, url, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), bodyBytes, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr)
			entry.StatusCode = strings.TrimSpace(apiResp.Transaction.StatusCode)
			entry.StatusDesc = strings.TrimSpace(apiResp.Transaction.StatusDesc)
			c.logAPICall(ctx, entry)
			return &apiResp, resultErr
		}
		c.logAPICall(ctx, buildLogEntry(initiateRegularRechargePath, http.MethodPost, url, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), bodyBytes, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	var apiResp InitiateRegularRechargeResponse
	if err = json.Unmarshal(bodyResp, &apiResp); err != nil {
		resultErr := &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("unmarshal response body: %w", err)}
		c.logAPICall(ctx, buildLogEntry(initiateRegularRechargePath, http.MethodPost, url, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), bodyBytes, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr))
		return nil, resultErr
	}

	apiResp.HTTPStatusCode = httpResp.StatusCode

	if strings.TrimSpace(apiResp.Transaction.StatusCode) != "00000" {
		resultErr := &BusinessError{
			Code:          strings.TrimSpace(apiResp.Transaction.StatusCode),
			Message:       strings.TrimSpace(apiResp.Transaction.StatusDesc),
			TransactionID: strings.TrimSpace(apiResp.Transaction.TransactionID),
		}
		entry := buildLogEntry(initiateRegularRechargePath, http.MethodPost, url, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), bodyBytes, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), resultErr)
		entry.StatusCode = strings.TrimSpace(apiResp.Transaction.StatusCode)
		entry.StatusDesc = strings.TrimSpace(apiResp.Transaction.StatusDesc)
		c.logAPICall(ctx, entry)
		return &apiResp, resultErr
	}

	entry := buildLogEntry(initiateRegularRechargePath, http.MethodPost, url, externalTransactionID, sanitizeHeadersForLog(httpReq.Header), bodyBytes, bodyResp, httpResp.StatusCode, int(duration.Milliseconds()), nil)
	entry.StatusCode = strings.TrimSpace(apiResp.Transaction.StatusCode)
	entry.StatusDesc = strings.TrimSpace(apiResp.Transaction.StatusDesc)
	c.logAPICall(ctx, entry)

	return &apiResp, nil
}

func validateInitiateRegularRechargeRequest(req InitiateRegularRechargeRequest) error {
	transactionID := strings.TrimSpace(req.Transaction.TransactionID)
	if transactionID == "" {
		return fmt.Errorf("transaction.transaction_id is required")
	}
	if len(transactionID) > 25 {
		return fmt.Errorf("transaction.transaction_id must be at most 25 characters")
	}

	channel := strings.TrimSpace(req.Transaction.Channel)
	if channel == "" {
		return fmt.Errorf("transaction.channel is required")
	}

	organizationCode := strings.TrimSpace(req.Service.OrganizationCode)
	if organizationCode == "" {
		return fmt.Errorf("service.organization_code is required")
	}

	serviceID := strings.TrimSpace(req.Service.ServiceID)
	if serviceID == "" {
		return fmt.Errorf("service.service_id is required")
	}
	if !strings.HasPrefix(serviceID, "62") {
		return fmt.Errorf("service.service_id must start with 62")
	}

	amount := req.Recharge.Amount
	if amount <= 0 {
		return fmt.Errorf("recharge.amount must be greater than zero")
	}

	stockType := strings.TrimSpace(req.Recharge.StockType)
	if stockType == "" {
		return fmt.Errorf("recharge.stock_type is required")
	}

	element1 := strings.TrimSpace(req.Recharge.Element1)
	if element1 == "" {
		return fmt.Errorf("recharge.element1 is required")
	}

	thirdPartyID := strings.TrimSpace(req.MerchantProfile.ThirdPartyID)
	if thirdPartyID == "" {
		return fmt.Errorf("merchant_profile.third_party_id is required")
	}

	thirdPartyPassword := strings.TrimSpace(req.MerchantProfile.ThirdPartyPassword)
	if thirdPartyPassword == "" {
		return fmt.Errorf("merchant_profile.third_party_password is required")
	}

	deliveryChannel := strings.TrimSpace(req.MerchantProfile.DeliveryChannel)
	if deliveryChannel == "" {
		return fmt.Errorf("merchant_profile.delivery_channel is required")
	}

	return nil
}

func validateOrderDealerRequest(req OrderDealerRequest) error {
	transactionID := strings.TrimSpace(req.Transaction.TransactionID)
	if transactionID == "" {
		return fmt.Errorf("transaction.transaction_id is required")
	}
	if len(transactionID) > 25 {
		return fmt.Errorf("transaction.transaction_id must be at most 25 characters")
	}

	channel := strings.TrimSpace(req.Transaction.Channel)
	if channel == "" {
		return fmt.Errorf("transaction.channel is required")
	}

	organizationCode := strings.TrimSpace(req.Service.OrganizationCode)
	if organizationCode == "" {
		return fmt.Errorf("service.organization_code is required")
	}

	serviceID := strings.TrimSpace(req.Service.ServiceID)
	if serviceID == "" {
		return fmt.Errorf("service.service_id is required")
	}
	if !strings.HasPrefix(serviceID, "62") {
		return fmt.Errorf("service.service_id must start with 62")
	}

	productID := strings.TrimSpace(req.Order.ProductID)
	if productID == "" {
		return fmt.Errorf("order.product_id is required")
	}

	stockType := strings.TrimSpace(req.Order.StockType)
	if stockType == "" {
		return fmt.Errorf("order.stock_type is required")
	}

	element1 := strings.TrimSpace(req.Order.Element1)
	if element1 == "" {
		return fmt.Errorf("order.element1 is required")
	}

	thirdPartyID := strings.TrimSpace(req.MerchantProfile.ThirdPartyID)
	if thirdPartyID == "" {
		return fmt.Errorf("merchant_profile.third_party_id is required")
	}

	thirdPartyPassword := strings.TrimSpace(req.MerchantProfile.ThirdPartyPassword)
	if thirdPartyPassword == "" {
		return fmt.Errorf("merchant_profile.third_party_password is required")
	}

	deliveryChannel := strings.TrimSpace(req.MerchantProfile.DeliveryChannel)
	if deliveryChannel == "" {
		return fmt.Errorf("merchant_profile.delivery_channel is required")
	}

	return nil
}

func validateCheckOrderStatusRequest(req CheckOrderStatusRequest) error {
	transactionID := strings.TrimSpace(req.TransactionID)
	if transactionID == "" {
		return fmt.Errorf("transaction_id is required")
	}
	if len(transactionID) > 25 {
		return fmt.Errorf("transaction_id must be at most 25 characters")
	}

	originalTransactionID := strings.TrimSpace(req.OriginalTransactionID)
	if originalTransactionID == "" {
		return fmt.Errorf("original_transaction_id is required")
	}

	serviceID := strings.TrimSpace(req.ServiceID)
	if serviceID == "" {
		return fmt.Errorf("service_id is required")
	}
	if !strings.HasPrefix(serviceID, "62") {
		return fmt.Errorf("service_id must start with 62")
	}

	return nil
}

func buildCheckOrderStatusURL(basePath string, req CheckOrderStatusRequest) (string, error) {
	u, err := url.Parse(basePath)
	if err != nil {
		return "", fmt.Errorf("parse check order status url: %w", err)
	}

	q := u.Query()
	q.Set("transaction_id", strings.TrimSpace(req.TransactionID))
	q.Set("original_transaction_id", strings.TrimSpace(req.OriginalTransactionID))
	if sn := strings.TrimSpace(req.SerialNumber); sn != "" {
		q.Set("serial_number", sn)
	}
	q.Set("service_id", strings.TrimSpace(req.ServiceID))
	q.Set("channel", strings.TrimSpace(req.Channel))
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func validateBrowseOfferRequest(req BrowseOfferRequest) error {
	transactionID := strings.TrimSpace(req.TransactionID)
	if transactionID == "" {
		return fmt.Errorf("transaction_id is required")
	}
	if len(transactionID) > 25 {
		return fmt.Errorf("transaction_id must be at most 25 characters")
	}

	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		return fmt.Errorf("channel is required")
	}

	organizationCode := strings.TrimSpace(req.OrganizationCode)
	if organizationCode == "" {
		return fmt.Errorf("organization_code is required")
	}

	serviceID := strings.TrimSpace(req.ServiceID)
	if serviceID == "" {
		return fmt.Errorf("service_id is required")
	}
	if !strings.HasPrefix(serviceID, "62") {
		return fmt.Errorf("service_id must start with 62")
	}

	productID := strings.TrimSpace(req.ProductID)
	if productID == "" {
		return fmt.Errorf("product_id is required")
	}

	version := strings.TrimSpace(req.Version)
	if version == "" {
		return fmt.Errorf("version is required")
	}

	return nil
}

func buildBrowseOfferURL(basePath string, req BrowseOfferRequest) (string, error) {
	u, err := url.Parse(basePath)
	if err != nil {
		return "", fmt.Errorf("parse browse offer url: %w", err)
	}

	q := u.Query()
	q.Set("transaction_id", strings.TrimSpace(req.TransactionID))
	q.Set("channel", strings.TrimSpace(req.Channel))
	q.Set("organization_code", strings.TrimSpace(req.OrganizationCode))
	q.Set("service_id", strings.TrimSpace(req.ServiceID))
	q.Set("product_id", strings.TrimSpace(req.ProductID))
	q.Set("version", strings.TrimSpace(req.Version))
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func generateSignature(timestamp string) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("API_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("SECRET_KEY"))

	if apiKey == "" {
		return "", fmt.Errorf("API_KEY env is required")
	}
	if secretKey == "" {
		return "", fmt.Errorf("SECRET_KEY env is required")
	}
	if strings.TrimSpace(timestamp) == "" {
		return "", fmt.Errorf("timestamp is required")
	}

	sum := md5.Sum([]byte(apiKey + secretKey + timestamp))
	return hex.EncodeToString(sum[:]), nil
}

func unixNowUTC() string {
	return fmt.Sprintf("%d", time.Now().UTC().Unix())
}

func sanitizeHeadersForLog(h http.Header) map[string]string {
	if h == nil {
		return nil
	}

	out := make(map[string]string, len(h))
	for k, v := range h {
		joined := strings.Join(v, ",")
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "api-key", "x-signature", "signature", "authorization":
			out[k] = maskForLog(joined)
		default:
			out[k] = joined
		}
	}

	return out
}

func sanitizeJSONForLog(body []byte) string {
	const maxLen = 4096
	if len(body) == 0 {
		return ""
	}

	var anyBody any
	if err := json.Unmarshal(body, &anyBody); err != nil {
		return truncateForLog(string(body), maxLen)
	}

	redacted := redactSensitive(anyBody)
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return truncateForLog(string(body), maxLen)
	}

	return truncateForLog(string(encoded), maxLen)
}

func redactSensitive(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			lowerKey := strings.ToLower(strings.TrimSpace(k))
			if lowerKey == "third_party_password" || lowerKey == "element1" || lowerKey == "secret" || strings.Contains(lowerKey, "password") {
				out[k] = "***"
				continue
			}
			out[k] = redactSensitive(val)
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, item := range t {
			out = append(out, redactSensitive(item))
		}
		return out
	default:
		return v
	}
}

func truncateForLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func maskForLog(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

func isRetryableError(err error) bool {
	var businessErr *BusinessError
	if errors.As(err, &businessErr) {
		return false
	}

	var technicalErr *TechnicalError
	if errors.As(err, &technicalErr) {
		if technicalErr.StatusCode >= http.StatusInternalServerError {
			return true
		}

		if technicalErr.Cause != nil {
			if errors.Is(technicalErr.Cause, context.DeadlineExceeded) || errors.Is(technicalErr.Cause, context.Canceled) {
				return true
			}

			var netErr net.Error
			if errors.As(technicalErr.Cause, &netErr) {
				return true
			}
		}
	}

	return false
}

func detectRequestRejected(body []byte) (supportID string, ok bool) {
	if len(body) == 0 {
		return "", false
	}

	lower := strings.ToLower(string(body))
	if !strings.Contains(lower, "request rejected") && !strings.Contains(lower, "the requested url was rejected") {
		return "", false
	}

	// Extract: "Your support ID is: 9849..." if present.
	marker := strings.ToLower("support id is")
	idx := strings.Index(lower, marker)
	if idx == -1 {
		return "", true
	}

	// From the original body to preserve digits.
	orig := string(body)
	start := idx + len(marker)
	if start > len(orig) {
		return "", true
	}
	rest := orig[start:]
	// Skip separators like ':' and spaces.
	rest = strings.TrimLeft(rest, ": \t\r\n")
	// Capture consecutive digits.
	end := 0
	for end < len(rest) {
		r := rest[end]
		if r < '0' || r > '9' {
			break
		}
		end++
	}
	sid := strings.TrimSpace(rest[:end])
	return sid, true
}
