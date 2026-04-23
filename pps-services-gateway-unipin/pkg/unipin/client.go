package unipin

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultMaxRetries   = 2
	defaultRetryBackoff = 300 * time.Millisecond
)

// Client is a reusable Unipin API client.
type Client struct {
	baseURL               string
	partnerID             string
	secretKey             string
	timeout               time.Duration
	voucherRequestTimeout time.Duration
	createOrderTimeout    time.Duration
	httpClient            *http.Client
	logger                *slog.Logger
	maxRetries            int
}

// NewClient builds a new Unipin client.
func NewClient(baseURL, partnerID, secretKey string, timeout time.Duration, logger *slog.Logger) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	partnerID = strings.TrimSpace(partnerID)
	secretKey = strings.TrimSpace(secretKey)

	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}
	if partnerID == "" {
		return nil, fmt.Errorf("partnerID is required")
	}
	if secretKey == "" {
		return nil, fmt.Errorf("secretKey is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("timeout must be greater than zero")
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		partnerID: partnerID,
		secretKey: secretKey,
		timeout:   timeout,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger:     logger,
		maxRetries: defaultMaxRetries,
	}, nil
}

// SetVoucherRequestTimeout sets a dedicated timeout for VoucherRequest calls.
// If not set (or <=0), VoucherRequest uses the client's default timeout.
func (c *Client) SetVoucherRequestTimeout(timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	c.voucherRequestTimeout = timeout
	c.recalculateHTTPClientTimeout()
}

// SetCreateOrderTimeout sets a dedicated timeout for CreateOrder calls.
// If not set (or <=0), CreateOrder uses the client's default timeout.
func (c *Client) SetCreateOrderTimeout(timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	c.createOrderTimeout = timeout
	c.recalculateHTTPClientTimeout()
}

func (c *Client) recalculateHTTPClientTimeout() {
	// http.Client.Timeout applies globally; keep it at least as large as the largest per-call timeout.
	if c.httpClient == nil {
		return
	}

	maxTimeout := c.timeout
	if c.voucherRequestTimeout > maxTimeout {
		maxTimeout = c.voucherRequestTimeout
	}
	if c.createOrderTimeout > maxTimeout {
		maxTimeout = c.createOrderTimeout
	}
	if c.httpClient.Timeout < maxTimeout {
		c.httpClient.Timeout = maxTimeout
	}
}

func (c *Client) voucherRequestTimeoutOrDefault() time.Duration {
	if c.voucherRequestTimeout > 0 {
		return c.voucherRequestTimeout
	}
	return c.timeout
}

func (c *Client) createOrderTimeoutOrDefault() time.Duration {
	if c.createOrderTimeout > 0 {
		return c.createOrderTimeout
	}
	return c.timeout
}

// SetTransport replaces the HTTP client transport (e.g. for logging).
func (c *Client) SetTransport(rt http.RoundTripper) {
	c.httpClient.Transport = rt
}

// generateAuth creates HMAC-SHA256 signature for Unipin API authentication.
// Formula: HMAC-SHA256(partnerID + timestamp + path, secretKey)
func generateAuth(partnerID, timestamp, path, secretKey string) string {
	p := strings.TrimPrefix(path, "/")
	message := partnerID + timestamp + p
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
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
		case "auth", "authorization":
			out[k] = maskForLog(joined)
		default:
			out[k] = joined
		}
	}

	return out
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
