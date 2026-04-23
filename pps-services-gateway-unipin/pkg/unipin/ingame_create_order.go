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

const createOrderPath = "/in-game-topup/order/create"

// CreateOrder calls Unipin In-Game Topup Create Order endpoint.
func (c *Client) CreateOrder(ctx context.Context, req CreateOrderRequest) (*CreateOrderResponse, error) {
	if strings.TrimSpace(req.GameCode) == "" {
		return nil, fmt.Errorf("game_code is required")
	}
	if strings.TrimSpace(req.ValidationToken) == "" {
		return nil, fmt.Errorf("validation_token is required")
	}
	if strings.TrimSpace(req.ReferenceNo) == "" {
		return nil, fmt.Errorf("reference_no is required")
	}
	if strings.TrimSpace(req.DenominationID) == "" {
		return nil, fmt.Errorf("denomination_id is required")
	}

	timestamp := unixNowUTC()
	auth := generateAuth(c.partnerID, timestamp, createOrderPath, c.secretKey)
	url := c.baseURL + createOrderPath

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	attempts := c.maxRetries + 1
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, c.createOrderTimeoutOrDefault())
		resp, err := c.doCreateOrder(callCtx, url, timestamp, auth, bodyBytes)
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
		c.logger.Warn("retrying create order",
			"attempt", attempt,
			"max_attempts", attempts,
			"backoff", backoff.String(),
			"game_code", req.GameCode,
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

func (c *Client) doCreateOrder(
	ctx context.Context,
	url, timestamp, auth string,
	bodyBytes []byte,
) (*CreateOrderResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &TechnicalError{Cause: fmt.Errorf("create request: %w", err)}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("partnerid", c.partnerID)
	httpReq.Header.Set("timestamp", timestamp)
	httpReq.Header.Set("path", strings.TrimPrefix(createOrderPath, "/"))
	httpReq.Header.Set("auth", auth)

	c.logger.Info("unipin outgoing request",
		"endpoint", createOrderPath,
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
		"endpoint", createOrderPath,
		"method", http.MethodPost,
		"status_code", httpResp.StatusCode,
		"body", truncateForLog(string(bodyResp), 4096),
	)

	c.logger.Info("unipin request completed",
		"endpoint", createOrderPath,
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

	var apiResp CreateOrderResponse
	if err := json.Unmarshal(bodyResp, &apiResp); err != nil {
		return nil, &TechnicalError{StatusCode: httpResp.StatusCode, Cause: fmt.Errorf("unmarshal response body: %w", err)}
	}

	if apiResp.Status != 1 {
		return &apiResp, &BusinessError{Status: apiResp.Status, Reason: ResolveReason(apiResp.Reason, apiResp.Error)}
	}

	return &apiResp, nil
}
