package telkomsel

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// validCheckOrderStatusEnv returns a map of all required env vars for CheckOrderStatusOnConsume,
// pointing BASE_URL at the given server URL.
func validCheckOrderStatusEnv(srvURL string) map[string]string {
	return map[string]string{
		"BASE_URL":   srvURL,
		"CHANNEL_ID": "tp",
		"SECRET_KEY": "secret",
		"API_KEY":    "apikey",
		"TIMEOUT":    "3",
		"swpps":      "705009_002314",
	}
}

// checkOrderStatusMockServer returns an httptest server that responds with a
// successful CheckOrderStatus JSON response for GET requests.
func checkOrderStatusMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.URL.Path, checkOrderStatusPath) {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		externalTxID := r.Header.Get("External-Transaction-Id")
		channelID := r.Header.Get("Channel-Id")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
			"transaction":{"transaction_id":%q,"channel":%q,"status_code":"00000","status_desc":"SUCCESS"},
			"transaction_status":{"original_transaction_id":"ORIG-TX-001","serial_number":"SN001","status":"SUCCESS"}
		}`, externalTxID, channelID)
	}))
}

func TestCheckOrderStatusOnConsume_Success(t *testing.T) {
	srv := checkOrderStatusMockServer(t)
	defer srv.Close()

	env := validCheckOrderStatusEnv(srv.URL)
	for k, v := range env {
		t.Setenv(k, v)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := CheckOrderStatusOnConsume(ctx, "081234567890", "swpps", "queue.status", "msg-1", "ORIG-TX-001", "SN001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Transaction.StatusCode != "00000" {
		t.Fatalf("expected status_code 00000, got %q", resp.Transaction.StatusCode)
	}
	if resp.TransactionStatus == nil {
		t.Fatal("expected transaction_status, got nil")
	}
	if resp.TransactionStatus.OriginalTransactionID != "ORIG-TX-001" {
		t.Fatalf("expected original_transaction_id ORIG-TX-001, got %q", resp.TransactionStatus.OriginalTransactionID)
	}
}

func TestCheckOrderStatusOnConsume_MissingEnvVars(t *testing.T) {
	requiredEnvVars := []struct {
		envVar  string
		errText string
	}{
		{"BASE_URL", "BASE_URL"},
		{"CHANNEL_ID", "CHANNEL_ID"},
		{"SECRET_KEY", "SECRET_KEY"},
		{"API_KEY", "API_KEY"},
	}

	for _, tc := range requiredEnvVars {
		t.Run("missing_"+tc.envVar, func(t *testing.T) {
			srv := checkOrderStatusMockServer(t)
			defer srv.Close()

			env := validCheckOrderStatusEnv(srv.URL)
			for k, v := range env {
				if k == tc.envVar {
					t.Setenv(k, "")
					continue
				}
				t.Setenv(k, v)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := CheckOrderStatusOnConsume(ctx, "081234567890", "swpps", "q", "msg-1", "ORIG-TX-001", "SN001")
			if err == nil {
				t.Fatalf("expected error for missing %s, got nil", tc.envVar)
			}
			if !strings.Contains(err.Error(), tc.errText) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.errText)
			}
		})
	}
}

func TestCheckOrderStatusOnConsume_EmptyOriginalTransactionID(t *testing.T) {
	srv := checkOrderStatusMockServer(t)
	defer srv.Close()

	env := validCheckOrderStatusEnv(srv.URL)
	for k, v := range env {
		t.Setenv(k, v)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := CheckOrderStatusOnConsume(ctx, "081234567890", "swpps", "q", "msg-1", "", "SN001")
	if err == nil {
		t.Fatal("expected error for empty original_transaction_id, got nil")
	}
	if !strings.Contains(err.Error(), "original_transaction_id") {
		t.Fatalf("error %q does not contain 'original_transaction_id'", err.Error())
	}
}

func TestCheckOrderStatusOnConsume_EmptyMID(t *testing.T) {
	srv := checkOrderStatusMockServer(t)
	defer srv.Close()

	env := validCheckOrderStatusEnv(srv.URL)
	for k, v := range env {
		t.Setenv(k, v)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := CheckOrderStatusOnConsume(ctx, "081234567890", "", "q", "msg-1", "ORIG-TX-001", "SN001")
	if err == nil {
		t.Fatal("expected error for empty mid, got nil")
	}
	if !strings.Contains(err.Error(), "mid is required") {
		t.Fatalf("error %q does not contain 'mid is required'", err.Error())
	}
}
