package unipin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"pps-services-gateway-unipin/internal/domain/contract/repository"
)

// LoggingTransport wraps http.RoundTripper to log requests/responses to Postgres.
type LoggingTransport struct {
	base    http.RoundTripper
	repo    repository.APILogRepository
	baseURL string
}

// NewLoggingTransport creates a LoggingTransport.
func NewLoggingTransport(base http.RoundTripper, repo repository.APILogRepository, baseURL string) *LoggingTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &LoggingTransport{base: base, repo: repo, baseURL: baseURL}
}

// RoundTrip executes the HTTP request and logs the result asynchronously.
func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Capture request body
	var reqBody string
	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err == nil {
			reqBody = string(bodyBytes)
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	}

	// Capture request headers as JSON
	headersMap := sanitizeHeadersForLog(req.Header)
	headersJSON, _ := json.Marshal(headersMap)

	endpoint := strings.TrimPrefix(req.URL.Path, "")
	method := req.Method
	requestURL := req.URL.String()

	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	duration := time.Since(start)

	// Capture response
	var respCode int
	var respBody string
	var errMsg string

	if err != nil {
		errMsg = err.Error()
	}

	if resp != nil {
		respCode = resp.StatusCode
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr == nil {
			respBody = string(bodyBytes)
			resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	}

	// Async insert to Postgres
	entry := &repository.APILogEntry{
		Endpoint:       endpoint,
		Method:         method,
		RequestURL:     requestURL,
		RequestHeaders: string(headersJSON),
		RequestBody:    reqBody,
		ResponseCode:   respCode,
		ResponseBody:   respBody,
		DurationMs:     duration.Milliseconds(),
		ErrorMessage:   errMsg,
		CreatedAt:      time.Now(),
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = t.repo.Insert(ctx, entry)
	}()

	return resp, err
}
