package postgres

import (
	"context"
	"errors"
	"testing"

	"pps-services-gateway-telkomsel/pkg/telkomsel"
)

func TestAPILoggerAdapter_Log_FieldMapping(t *testing.T) {
	repo := &mockAPILogRepository{}
	logger := &mockLogger{}
	adapter := NewAPILoggerAdapter(repo, logger)

	input := telkomsel.APICallLog{
		Endpoint:              "/api/v1/recharge",
		Method:                "POST",
		ExternalTransactionID: "ext-txn-123",
		RequestURL:            "https://example.com/api/v1/recharge",
		RequestHeaders:        map[string]string{"Content-Type": "application/json"},
		RequestBody:           []byte(`{"amount":10000}`),
		ResponseStatusCode:    200,
		ResponseBody:          []byte(`{"status":"ok"}`),
		ResponseDurationMs:    150,
		StatusCode:            "00000",
		StatusDesc:            "Success",
		ErrorMessage:          "",
		ErrorType:             "",
	}

	adapter.Log(context.Background(), input)

	if len(repo.insertCalls) != 1 {
		t.Fatalf("expected 1 Insert call, got %d", len(repo.insertCalls))
	}

	got := repo.insertCalls[0]

	if got.Endpoint != input.Endpoint {
		t.Errorf("Endpoint = %q, want %q", got.Endpoint, input.Endpoint)
	}
	if got.Method != input.Method {
		t.Errorf("Method = %q, want %q", got.Method, input.Method)
	}
	if got.ExternalTransactionID != input.ExternalTransactionID {
		t.Errorf("ExternalTransactionID = %q, want %q", got.ExternalTransactionID, input.ExternalTransactionID)
	}
	if got.RequestURL != input.RequestURL {
		t.Errorf("RequestURL = %q, want %q", got.RequestURL, input.RequestURL)
	}
	if len(got.RequestHeaders) != len(input.RequestHeaders) {
		t.Errorf("RequestHeaders length = %d, want %d", len(got.RequestHeaders), len(input.RequestHeaders))
	}
	for k, v := range input.RequestHeaders {
		if got.RequestHeaders[k] != v {
			t.Errorf("RequestHeaders[%q] = %q, want %q", k, got.RequestHeaders[k], v)
		}
	}
	if string(got.RequestBody) != string(input.RequestBody) {
		t.Errorf("RequestBody = %q, want %q", got.RequestBody, input.RequestBody)
	}
	if got.ResponseStatusCode != input.ResponseStatusCode {
		t.Errorf("ResponseStatusCode = %d, want %d", got.ResponseStatusCode, input.ResponseStatusCode)
	}
	if string(got.ResponseBody) != string(input.ResponseBody) {
		t.Errorf("ResponseBody = %q, want %q", got.ResponseBody, input.ResponseBody)
	}
	if got.ResponseDurationMs != input.ResponseDurationMs {
		t.Errorf("ResponseDurationMs = %d, want %d", got.ResponseDurationMs, input.ResponseDurationMs)
	}
	if got.StatusCode != input.StatusCode {
		t.Errorf("StatusCode = %q, want %q", got.StatusCode, input.StatusCode)
	}
	if got.StatusDesc != input.StatusDesc {
		t.Errorf("StatusDesc = %q, want %q", got.StatusDesc, input.StatusDesc)
	}
	if got.ErrorMessage != input.ErrorMessage {
		t.Errorf("ErrorMessage = %q, want %q", got.ErrorMessage, input.ErrorMessage)
	}
	if got.ErrorType != input.ErrorType {
		t.Errorf("ErrorType = %q, want %q", got.ErrorType, input.ErrorType)
	}

	if len(logger.errorCalls) != 0 {
		t.Errorf("expected no error logs, got %d", len(logger.errorCalls))
	}
}

func TestAPILoggerAdapter_Log_InsertError(t *testing.T) {
	insertErr := errors.New("db connection failed")
	repo := &mockAPILogRepository{insertErr: insertErr}
	logger := &mockLogger{}
	adapter := NewAPILoggerAdapter(repo, logger)

	input := telkomsel.APICallLog{
		Endpoint: "/api/v1/browse",
		Method:   "GET",
	}

	adapter.Log(context.Background(), input)

	if len(repo.insertCalls) != 1 {
		t.Fatalf("expected 1 Insert call, got %d", len(repo.insertCalls))
	}

	if len(logger.errorCalls) != 1 {
		t.Fatalf("expected 1 error log, got %d", len(logger.errorCalls))
	}

	logArgs := logger.errorCalls[0]
	msg, ok := logArgs[0].(string)
	if !ok || msg != "failed to insert api log" {
		t.Errorf("error log message = %q, want %q", msg, "failed to insert api log")
	}

	// Verify the error and endpoint are included in the log args
	foundError := false
	foundEndpoint := false
	for i := 1; i < len(logArgs)-1; i += 2 {
		key, _ := logArgs[i].(string)
		switch key {
		case "error":
			if logArgs[i+1] == insertErr {
				foundError = true
			}
		case "endpoint":
			if logArgs[i+1] == input.Endpoint {
				foundEndpoint = true
			}
		}
	}
	if !foundError {
		t.Error("expected error log to contain the insert error")
	}
	if !foundEndpoint {
		t.Error("expected error log to contain the endpoint")
	}
}
