package telkomsel

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestConsumePulsa_Swpps_RequestAndResponseJSON(t *testing.T) {
	// Arrange: mock Telkomsel endpoint.
	var capturedReqBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != initiateRegularRechargePath {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"error":"read body: %v"}`, err)))
			return
		}
		capturedReqBody = b

		externalTxID := r.Header.Get("External-Transaction-Id")
		channelID := r.Header.Get("Channel-Id")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"transaction":{"transaction_id":"%s","channel":"%s","status_code":"00000","status_desc":"SUCCESS"},"serial_number":"SN123"}`,
			externalTxID,
			channelID,
		)))
	}))
	defer srv.Close()

	// Arrange: env config.
	env := map[string]string{
		"BASE_URL":             srv.URL,
		"CHANNEL_ID":           "tp",
		"SECRET_KEY":           "secret",
		"API_KEY":              "apikey",
		"THIRD_PARTY_ID":       "NGRS_PPS",
		"THIRD_PARTY_PASSWORD": "dummy-password",
		"DELIVERY_CHANNEL":     "6014",
		"ENCRYPTION_KEY":       generateTestEncryptionKeyB64(t),
		"TIMEOUT":              "3",
		// mid-based mapping: ${ORGANIZATION_CODE}_${PIN}
		"swpps": "705009_002314",
	}
	restore := setEnvForTest(t, env)
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Act.
	resp, err := InitiateRegularRechargeOnConsume(ctx, "081234567890", "swpps", "queue.telkomsel", "msg-1", 10000, StockTypeFixed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected response, got nil")
	}

	// Assert & print JSON for debugging.
	var reqObj any
	if err := json.Unmarshal(capturedReqBody, &reqObj); err != nil {
		t.Fatalf("captured request is not valid JSON: %v\nbody=%s", err, string(capturedReqBody))
	}

	prettyReq, _ := json.MarshalIndent(reqObj, "", "  ")
	prettyResp, _ := json.MarshalIndent(resp, "", "  ")

	t.Logf("Telkomsel request JSON (mid=swpps):\n%s", string(prettyReq))
	t.Logf("Telkomsel response JSON (mock):\n%s", string(prettyResp))
}

func setEnvForTest(t *testing.T, pairs map[string]string) func() {
	t.Helper()

	previous := make(map[string]*string, len(pairs))
	for k, v := range pairs {
		if old, ok := os.LookupEnv(k); ok {
			tmp := old
			previous[k] = &tmp
		} else {
			previous[k] = nil
		}

		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("setenv %s: %v", k, err)
		}
	}

	return func() {
		for k, old := range previous {
			if old == nil {
				_ = os.Unsetenv(k)
				continue
			}
			_ = os.Setenv(k, *old)
		}
	}
}

func generateTestEncryptionKeyB64(t *testing.T) string {
	t.Helper()

	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate random encryption key: %v", err)
	}

	return base64.StdEncoding.EncodeToString(key)
}

func TestInitiateRegularRechargeOnConsume_MissingEnvVars(t *testing.T) {
	requiredEnvVars := []struct {
		envVar  string
		errText string
	}{
		{"BASE_URL", "BASE_URL"},
		{"CHANNEL_ID", "CHANNEL_ID"},
		{"SECRET_KEY", "SECRET_KEY"},
		{"API_KEY", "API_KEY"},
		{"THIRD_PARTY_ID", "THIRD_PARTY_ID"},
		{"ENCRYPTION_KEY", "ENCRYPTION_KEY"},
		{"THIRD_PARTY_PASSWORD", "THIRD_PARTY_PASSWORD"},
		{"DELIVERY_CHANNEL", "DELIVERY_CHANNEL"},
	}

	for _, tc := range requiredEnvVars {
		t.Run("missing_"+tc.envVar, func(t *testing.T) {
			env := map[string]string{
				"BASE_URL":             "http://localhost",
				"CHANNEL_ID":           "tp",
				"SECRET_KEY":           "secret",
				"API_KEY":              "apikey",
				"THIRD_PARTY_ID":       "NGRS_PPS",
				"THIRD_PARTY_PASSWORD": "dummy-password",
				"DELIVERY_CHANNEL":     "6014",
				"ENCRYPTION_KEY":       generateTestEncryptionKeyB64(t),
				"TIMEOUT":              "3",
				"swpps":                "705009_002314",
			}
			for k, v := range env {
				if k == tc.envVar {
					t.Setenv(k, "")
					continue
				}
				t.Setenv(k, v)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := InitiateRegularRechargeOnConsume(ctx, "081234567890", "swpps", "q", "msg-1", 10000, StockTypeFixed)
			if err == nil {
				t.Fatalf("expected error for missing %s, got nil", tc.envVar)
			}
			if !strings.Contains(err.Error(), tc.errText) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.errText)
			}
		})
	}
}

func TestInitiateRegularRechargeOnConsume_EmptyMID(t *testing.T) {
	t.Setenv("BASE_URL", "http://localhost")
	t.Setenv("CHANNEL_ID", "tp")
	t.Setenv("SECRET_KEY", "secret")
	t.Setenv("API_KEY", "apikey")
	t.Setenv("THIRD_PARTY_ID", "NGRS_PPS")
	t.Setenv("THIRD_PARTY_PASSWORD", "dummy-password")
	t.Setenv("DELIVERY_CHANNEL", "6014")
	t.Setenv("ENCRYPTION_KEY", generateTestEncryptionKeyB64(t))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := InitiateRegularRechargeOnConsume(ctx, "081234567890", "", "q", "msg-1", 10000, StockTypeFixed)
	if err == nil {
		t.Fatal("expected error for empty mid, got nil")
	}
	if !strings.Contains(err.Error(), "mid is required") {
		t.Fatalf("error %q does not contain 'mid is required'", err.Error())
	}
}

func TestInitiateRegularRechargeOnConsume_ZeroAmount(t *testing.T) {
	t.Setenv("BASE_URL", "http://localhost")
	t.Setenv("CHANNEL_ID", "tp")
	t.Setenv("SECRET_KEY", "secret")
	t.Setenv("API_KEY", "apikey")
	t.Setenv("THIRD_PARTY_ID", "NGRS_PPS")
	t.Setenv("THIRD_PARTY_PASSWORD", "dummy-password")
	t.Setenv("DELIVERY_CHANNEL", "6014")
	t.Setenv("ENCRYPTION_KEY", generateTestEncryptionKeyB64(t))
	t.Setenv("swpps", "705009_002314")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := InitiateRegularRechargeOnConsume(ctx, "081234567890", "swpps", "q", "msg-1", 0, StockTypeFixed)
	if err == nil {
		t.Fatal("expected error for zero amount, got nil")
	}
	if !strings.Contains(err.Error(), "amount must be greater than zero") {
		t.Fatalf("error %q does not contain 'amount must be greater than zero'", err.Error())
	}
}

func TestInitiateRegularRechargeOnConsume_InvalidStockType(t *testing.T) {
	t.Setenv("BASE_URL", "http://localhost")
	t.Setenv("CHANNEL_ID", "tp")
	t.Setenv("SECRET_KEY", "secret")
	t.Setenv("API_KEY", "apikey")
	t.Setenv("THIRD_PARTY_ID", "NGRS_PPS")
	t.Setenv("THIRD_PARTY_PASSWORD", "dummy-password")
	t.Setenv("DELIVERY_CHANNEL", "6014")
	t.Setenv("ENCRYPTION_KEY", generateTestEncryptionKeyB64(t))
	t.Setenv("swpps", "705009_002314")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Empty stock_type should still fail
	_, err := InitiateRegularRechargeOnConsume(ctx, "081234567890", "swpps", "q", "msg-1", 10000, "")
	if err == nil {
		t.Fatal("expected error for empty stock_type, got nil")
	}
	if !strings.Contains(err.Error(), "stock_type") {
		t.Fatalf("error %q does not contain 'stock_type'", err.Error())
	}
}
