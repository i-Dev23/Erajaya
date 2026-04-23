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

const orderInquiryPath = "/in-game-topup/order/inquiry"

// OrderInquiry calls Unipin In-Game Topup Order Inquiry endpoint.
func (c *Client) OrderInquiry(ctx context.Context, referenceNo string) (*OrderInquiryResponse, error) {
	referenceNo = strings.TrimSpace(referenceNo)
	if referenceNo == "" {
		return nil, fmt.Errorf("reference_no is required")
	}

	timestamp := unixNowUTC()
	auth := generateAuth(c.partnerID, timestamp, orderInquiryPath, c.secretKey)
	url := c.baseURL + orderInquiryPath

	bodyBytes, err := json.Marshal(orderInquiryRequestBody{ReferenceNo: referenceNo})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	attempts := c.maxRetries + 1
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, c.timeout)
		resp, err := c.doOrderInquiry(callCtx, url, timestamp, auth, bodyBytes)
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
		c.logger.Warn("retrying order inquiry",
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

func (c *Client) doOrderInquiry(
	ctx context.Context,
	url, timestamp, auth string,
	bodyBytes []byte,
) (*OrderInquiryResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("create request: %w", err)}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("partnerid", c.partnerID)
	httpReq.Header.Set("timestamp", timestamp)
	httpReq.Header.Set("path", strings.TrimPrefix(orderInquiryPath, "/"))
	httpReq.Header.Set("auth", auth)

	c.logger.Info("unipin outgoing request",
		"endpoint", orderInquiryPath,
		"method", http.MethodPost,
		"url", url,
		"headers", sanitizeHeadersForLog(httpReq.Header),
		"body", string(bodyBytes),
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
		"endpoint", orderInquiryPath,
		"method", http.MethodPost,
		"status_code", httpResp.StatusCode,
		"body", truncateForLog(string(bodyResp), 4096),
	)

	c.logger.Info("unipin request completed",
		"endpoint", orderInquiryPath,
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

	var apiResp OrderInquiryResponse
	if err := json.Unmarshal(bodyResp, &apiResp); err != nil {
		return nil, &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("unmarshal response body: %w", err)}
	}

	if apiResp.Status != 1 {
		return &apiResp, &BusinessError{Status: apiResp.Status, Reason: ResolveReason(apiResp.Reason, apiResp.Error)}
	}

	return &apiResp, nil
}
