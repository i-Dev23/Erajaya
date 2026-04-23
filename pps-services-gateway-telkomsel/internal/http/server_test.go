package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
	"pps-services-gateway-telkomsel/internal/handler"
)

// mockLogger implements contractsvc.Logger for testing.
type mockLogger struct {
	infoCalls  [][]any
	warnCalls  [][]any
	errorCalls [][]any
}

func (m *mockLogger) Info(msg string, args ...any) {
	m.infoCalls = append(m.infoCalls, append([]any{msg}, args...))
}

func (m *mockLogger) Warn(msg string, args ...any) {
	m.warnCalls = append(m.warnCalls, append([]any{msg}, args...))
}

func (m *mockLogger) Error(msg string, args ...any) {
	m.errorCalls = append(m.errorCalls, append([]any{msg}, args...))
}

// mockTransactionLogger implements contractsvc.TransactionLogger for testing.
// Used to verify the callback handler is invoked when routing through the server.
type mockTransactionLogger struct {
	getByOurTrxIDCalls []string
}

func (m *mockTransactionLogger) GetTransactionByOurTrxID(_ context.Context, ourTrxID string) (*contractsvc.TransactionRecord, error) {
	m.getByOurTrxIDCalls = append(m.getByOurTrxIDCalls, ourTrxID)
	return &contractsvc.TransactionRecord{
		MsgID:    ourTrxID,
		OurTrxID: ourTrxID,
	}, nil
}

func (m *mockTransactionLogger) InsertCallbackResponse(_ context.Context, _ contractsvc.ResponseRecord) error {
	return nil
}

func (m *mockTransactionLogger) InsertTransaction(_ context.Context, _ contractsvc.TransactionRecord) error {
	return nil
}

func (m *mockTransactionLogger) UpdateTransactionStatus(_ context.Context, _ string, _ string) error {
	return nil
}

func (m *mockTransactionLogger) InsertSyncResponse(_ context.Context, _ contractsvc.ResponseRecord) error {
	return nil
}

func (m *mockTransactionLogger) GetResponsesByMsgID(_ context.Context, _ string) ([]contractsvc.ResponseRecord, error) {
	return nil, nil
}

func (m *mockTransactionLogger) GetTransactionStatusByMsgID(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockTransactionLogger) RunMigration(_ context.Context) error {
	return nil
}

func (m *mockTransactionLogger) Close() {}

func TestHealthEndpoint_Returns200(t *testing.T) {
	logger := &mockLogger{}
	cbHandler := handler.NewCallbackHandler(logger, nil, nil, "test-queue")
	srv := NewServer(ServerConfig{Port: 0}, cbHandler, logger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestCallbackRoute_RoutesToHandler(t *testing.T) {
	logger := &mockLogger{}
	mtl := &mockTransactionLogger{}
	cbHandler := handler.NewCallbackHandler(logger, mtl, nil, "test-queue")
	srv := NewServer(ServerConfig{Port: 0}, cbHandler, logger)

	query := "transaction_id=TRX999&organization_code=ORG123&service_id=6200000000001&status=SUCCESS&message=OK"
	req := httptest.NewRequest(http.MethodGet, "/callback/ext?"+query, nil)
	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d (body: %s)", resp.StatusCode, string(body))
	}

	// Verify the callback handler was actually invoked by checking the
	// transaction logger mock received the lookup call.
	if len(mtl.getByOurTrxIDCalls) != 1 {
		t.Fatalf("expected 1 GetTransactionByOurTrxID call, got %d", len(mtl.getByOurTrxIDCalls))
	}
	if mtl.getByOurTrxIDCalls[0] != "TRX999" {
		t.Errorf("expected lookup with TRX999, got %q", mtl.getByOurTrxIDCalls[0])
	}

	// Verify response body matches expected callback response.
	resp2, _ := srv.app.Test(httptest.NewRequest(http.MethodGet, "/callback/ext?"+query, nil), -1)
	defer resp2.Body.Close()
	body, _ := io.ReadAll(resp2.Body)
	var cr struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &cr); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if cr.Status != "OK" {
		t.Errorf("expected status OK, got %q", cr.Status)
	}
	if !strings.Contains(cr.Message, "Callback received") {
		t.Errorf("expected message containing 'Callback received', got %q", cr.Message)
	}
}
