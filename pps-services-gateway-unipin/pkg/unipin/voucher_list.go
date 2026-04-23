package unipin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const voucherListPath = "/voucher/list"

// VoucherList calls Unipin Voucher List endpoint.
func (c *Client) VoucherList(ctx context.Context) (*VoucherListResponse, error) {
	logID := time.Now().UTC().Unix()
	signature := generateVoucherSignature(c.partnerID, logID, c.secretKey)
	url := c.baseURL + voucherListPath

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
		resp, err := c.doVoucherList(callCtx, url, bodyBytes)
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
		c.logger.Warn("retrying voucher list",
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

func (c *Client) doVoucherList(
	ctx context.Context,
	url string,
	bodyBytes []byte,
) (*VoucherListResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("create request: %w", err)}
	}

	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.Info("unipin outgoing request",
		"endpoint", voucherListPath,
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
		"endpoint", voucherListPath,
		"method", http.MethodPost,
		"status_code", httpResp.StatusCode,
		"body", truncateForLog(string(bodyResp), 4096),
	)

	c.logger.Info("unipin request completed",
		"endpoint", voucherListPath,
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

	var apiResp VoucherListResponse
	if err := json.Unmarshal(bodyResp, &apiResp); err != nil {
		return nil, &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("unmarshal response body: %w", err)}
	}

	if apiResp.Status != 1 {
		return &apiResp, &BusinessError{Status: apiResp.Status, Reason: ResolveReason(apiResp.Reason, apiResp.Error)}
	}

	return &apiResp, nil
}

// generateVoucherSignature creates SHA-256 hash for Voucher API authentication.
// Formula: SHA256(partner_guid + logid + secret)
func generateVoucherSignature(partnerGUID string, logID int64, secret string) string {
	message := fmt.Sprintf("%s%d%s", partnerGUID, logID, secret)
	sum := sha256.Sum256([]byte(message))
	return hex.EncodeToString(sum[:])
}

// sanitizeVoucherBodyForLog redacts signature from voucher request body for logging.
func sanitizeVoucherBodyForLog(body []byte) string {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return truncateForLog(string(body), 4096)
	}
	if _, ok := m["signature"]; ok {
		m["signature"] = "***"
	}
	out, _ := json.Marshal(m)
	return string(out)
}
