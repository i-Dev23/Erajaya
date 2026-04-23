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

const voucherInquiryPath = "/voucher/inquiry"

// VoucherInquiry calls Unipin Voucher Inquiry endpoint.
func (c *Client) VoucherInquiry(ctx context.Context, referenceNo string) (*VoucherInquiryResponse, error) {
	referenceNo = strings.TrimSpace(referenceNo)
	if referenceNo == "" {
		return nil, fmt.Errorf("reference_no is required")
	}

	signature := generateVoucherInquirySignature(c.partnerID, referenceNo, c.secretKey)
	url := c.baseURL + voucherInquiryPath

	bodyBytes, err := json.Marshal(voucherInquiryRequestBody{
		PartnerGUID: c.partnerID,
		ReferenceNo: referenceNo,
		Signature:   signature,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	attempts := c.maxRetries + 1
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, c.timeout)
		resp, err := c.doVoucherInquiry(callCtx, url, bodyBytes)
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
		c.logger.Warn("retrying voucher inquiry",
			"attempt", attempt,
			"max_attempts", attempts,
			"backoff", backoff.String(),
			"reference_no", referenceNo,
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

func (c *Client) doVoucherInquiry(
	ctx context.Context,
	url string,
	bodyBytes []byte,
) (*VoucherInquiryResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("create request: %w", err)}
	}

	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.Info("unipin outgoing request",
		"endpoint", voucherInquiryPath,
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
		"endpoint", voucherInquiryPath,
		"method", http.MethodPost,
		"status_code", httpResp.StatusCode,
		"body", truncateForLog(string(bodyResp), 4096),
	)

	c.logger.Info("unipin request completed",
		"endpoint", voucherInquiryPath,
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

	var apiResp VoucherInquiryResponse
	if err := json.Unmarshal(bodyResp, &apiResp); err != nil {
		return nil, &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("unmarshal response body: %w", err)}
	}

	if apiResp.Status != 1 {
		return &apiResp, &BusinessError{Status: apiResp.Status, Reason: ResolveReason(apiResp.Reason, apiResp.Error)}
	}

	return &apiResp, nil
}

// generateVoucherInquirySignature creates SHA-256 hash for Voucher Inquiry API.
// Formula: SHA256(partner_guid + reference_no + secret)
func generateVoucherInquirySignature(partnerGUID, referenceNo, secret string) string {
	message := partnerGUID + referenceNo + secret
	sum := sha256.Sum256([]byte(message))
	return hex.EncodeToString(sum[:])
}
