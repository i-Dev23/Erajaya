package unipin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const voucherStockPath = "/voucher/get_stock_count"

// VoucherStock calls Unipin Voucher Get Stock Count endpoint.
func (c *Client) VoucherStock(ctx context.Context) (VoucherStockResponse, error) {
	logID := time.Now().UTC().Unix()
	signature := generateVoucherSignature(c.partnerID, logID, c.secretKey)
	url := c.baseURL + voucherStockPath

	bodyBytes, err := json.Marshal(voucherListRequestBody{
		PartnerGUID: c.partnerID,
		LogID:       logID,
		Signature:   signature,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	attempts := c.maxRetries + 1
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, c.timeout)
		resp, err := c.doVoucherStock(callCtx, url, bodyBytes)
		cancel()

		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !isRetryableError(err) || attempt == attempts {
			return nil, err
		}

		backoff := time.Duration(attempt) * defaultRetryBackoff
		c.logger.Warn("retrying voucher stock",
			"attempt", attempt,
			"max_attempts", attempts,
			"backoff", backoff.String(),
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

func (c *Client) doVoucherStock(
	ctx context.Context,
	url string,
	bodyBytes []byte,
) (VoucherStockResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("create request: %w", err)}
	}

	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.Info("unipin outgoing request",
		"endpoint", voucherStockPath,
		"method", http.MethodPost,
		"url", url,
		"body", sanitizeVoucherBodyForLog(bodyBytes),
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

	c.logger.Info("unipin incoming response",
		"endpoint", voucherStockPath,
		"method", http.MethodPost,
		"status_code", httpResp.StatusCode,
		"body", truncateForLog(string(bodyResp), 4096),
	)

	c.logger.Info("unipin request completed",
		"endpoint", voucherStockPath,
		"method", http.MethodPost,
		"status_code", httpResp.StatusCode,
		"duration_ms", duration.Milliseconds(),
	)

	if httpResp.StatusCode >= http.StatusInternalServerError {
		return nil, &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("server error: %s", strings.TrimSpace(string(bodyResp)))}
	}

	if httpResp.StatusCode >= http.StatusBadRequest {
		return nil, &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("http error: %s", strings.TrimSpace(string(bodyResp)))}
	}

	var apiResp VoucherStockResponse
	if err := json.Unmarshal(bodyResp, &apiResp); err != nil {
		return nil, &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("unmarshal response body: %w", err)}
	}

	return apiResp, nil
}
