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

const voucherRequestPath = "/voucher/request"

// VoucherRequest calls Unipin Voucher Request endpoint.
func (c *Client) VoucherRequest(ctx context.Context, req VoucherRequestReq) (*VoucherRequestResponse, error) {
	if strings.TrimSpace(req.DenominationCode) == "" {
		return nil, fmt.Errorf("denomination_code is required")
	}
	if req.Quantity <= 0 {
		return nil, fmt.Errorf("quantity must be greater than zero")
	}
	if strings.TrimSpace(req.ReferenceNo) == "" {
		return nil, fmt.Errorf("reference_no is required")
	}

	signature := generateVoucherRequestSignature(c.partnerID, strings.TrimSpace(req.ReferenceNo), req.DenominationCode, c.secretKey)
	url := c.baseURL + voucherRequestPath

	bodyBytes, err := json.Marshal(voucherRequestBody{
		PartnerGUID:      c.partnerID,
		DenominationCode: req.DenominationCode,
		Quantity:         req.Quantity,
		ReferenceNo:      req.ReferenceNo,
		Signature:        signature,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	attempts := c.maxRetries + 1
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, c.voucherRequestTimeoutOrDefault())
		resp, err := c.doVoucherRequest(callCtx, url, bodyBytes)
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
		c.logger.Warn("retrying voucher request",
			"attempt", attempt,
			"max_attempts", attempts,
			"backoff", backoff.String(),
			"denomination_code", req.DenominationCode,
			"reference_no", req.ReferenceNo,
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

func (c *Client) doVoucherRequest(
	ctx context.Context,
	url string,
	bodyBytes []byte,
) (*VoucherRequestResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("create request: %w", err)}
	}

	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.Info("unipin outgoing request",
		"endpoint", voucherRequestPath,
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
		"endpoint", voucherRequestPath,
		"method", http.MethodPost,
		"status_code", httpResp.StatusCode,
		"body", truncateForLog(string(bodyResp), 4096),
	)

	c.logger.Info("unipin request completed",
		"endpoint", voucherRequestPath,
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

	var apiResp VoucherRequestResponse
	if err := json.Unmarshal(bodyResp, &apiResp); err != nil {
		return nil, &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("unmarshal response body: %w", err)}
	}

	if apiResp.Status != 1 {
		return &apiResp, &BusinessError{Status: apiResp.Status, Reason: ResolveReason(apiResp.Reason, apiResp.Error)}
	}

	return &apiResp, nil
}

// generateVoucherRequestSignature creates SHA-256 hash for Voucher Request API.
// Formula: SHA256(partner_guid + reference_no + denomination_code + secret)
func generateVoucherRequestSignature(partnerGUID, referenceNo, denominationCode, secret string) string {
	message := partnerGUID + referenceNo + denominationCode + secret
	sum := sha256.Sum256([]byte(message))
	return hex.EncodeToString(sum[:])
}
