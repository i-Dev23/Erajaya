package telkomsel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// validOrderDealerEnv returns a map of all required env vars for OrderDealerOnConsume,
// pointing BASE_URL at the given server URL.
func validOrderDealerEnv(srvURL string) map[string]string {
	return map[string]string{
		"BASE_URL":             srvURL,
		"CHANNEL_ID":           "tp",
		"SECRET_KEY":           "secret",
		"API_KEY":              "apikey",
		"THIRD_PARTY_ID":       "NGRS_PPS",
		"THIRD_PARTY_PASSWORD": "dummy-password",
		"DELIVERY_CHANNEL":     "6014",
		"ENCRYPTION_KEY":       "", // filled per-test
		"TIMEOUT":              "3",
		"swpps":                "705009_002314",
	}
}

// orderDealerMockServer returns an httptest server that captures the request body
// and responds with a successful OrderDealer JSON response.
func orderDealerMockServer(t *testing.T, captured *[]byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != orderDealerPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if captured != nil {
			*captured = b
		}

		externalTxID := r.Header.Get("External-Transaction-Id")
		channelID := r.Header.Get("Channel-Id")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{
			"transaction":{"transaction_id":%q,"channel":%q,"status_code":"00000","status_desc":"SUCCESS"},
			"service":{"organization_code":"705009","service_id":"6281234567890"},
			"order":{"product_id":"PROD001","stock_type":"FIXED","element1":"enc","ap_flag":"Y"},
			"merchant_profile":{"third_party_id":"NGRS_PPS","delivery_channel":"6014"}
		}`, externalTxID, channelID)))
	}))
}

func TestOrderDealerOnConsume_Success(t *testing.T) {
	var capturedBody []byte
	srv := orderDealerMockServer(t, &capturedBody)
	defer srv.Close()

	env := validOrderDealerEnv(srv.URL)
	env["ENCRYPTION_KEY"] = generateTestEncryptionKeyB64(t)
	for k, v := range env {
		t.Setenv(k, v)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := OrderDealerOnConsume(ctx, "081234567890", "swpps", "queue.order", "msg-1", "PROD001", StockTypeFixed, "STORE01", "https://example.com/callback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Transaction.StatusCode != "00000" {
		t.Fatalf("expected status_code 00000, got %q", resp.Transaction.StatusCode)
	}

	// Verify request body contains expected fields.
	var reqBody map[string]json.RawMessage
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatalf("captured body is not valid JSON: %v", err)
	}

	var txn struct {
		TransactionID string `json:"transaction_id"`
		Channel       string `json:"channel"`
	}
	if err := json.Unmarshal(reqBody["transaction"], &txn); err != nil {
		t.Fatalf("unmarshal transaction: %v", err)
	}
	if txn.TransactionID == "" {
		t.Fatal("expected non-empty transaction_id in request body")
	}
	if len(txn.TransactionID) != 25 {
		t.Fatalf("expected 25-char transaction_id, got %d: %q", len(txn.TransactionID), txn.TransactionID)
	}
	if txn.Channel != "tp" {
		t.Fatalf("expected channel tp, got %q", txn.Channel)
	}

	var svc struct {
		OrganizationCode string `json:"organization_code"`
		ServiceID        string `json:"service_id"`
	}
	if err := json.Unmarshal(reqBody["service"], &svc); err != nil {
		t.Fatalf("unmarshal service: %v", err)
	}
	if svc.OrganizationCode != "705009" {
		t.Fatalf("expected organization_code 705009, got %q", svc.OrganizationCode)
	}
	if svc.ServiceID != "6281234567890" {
		t.Fatalf("expected service_id 6281234567890, got %q", svc.ServiceID)
	}

	var order struct {
		ProductID string `json:"product_id"`
		StockType string `json:"stock_type"`
		Element1  string `json:"element1"`
	}
	if err := json.Unmarshal(reqBody["order"], &order); err != nil {
		t.Fatalf("unmarshal order: %v", err)
	}
	if order.ProductID != "PROD001" {
		t.Fatalf("expected product_id PROD001, got %q", order.ProductID)
	}
	if order.StockType != "FIXED" {
		t.Fatalf("expected stock_type FIXED, got %q", order.StockType)
	}
	if order.Element1 == "" {
		t.Fatal("expected non-empty element1 in request body")
	}
}

func TestOrderDealerOnConsume_MissingEnvVars(t *testing.T) {
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
			srv := orderDealerMockServer(t, nil)
			defer srv.Close()

			env := validOrderDealerEnv(srv.URL)
			env["ENCRYPTION_KEY"] = generateTestEncryptionKeyB64(t)
			// Set all env vars except the one under test.
			for k, v := range env {
				if k == tc.envVar {
					t.Setenv(k, "")
					continue
				}
				t.Setenv(k, v)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := OrderDealerOnConsume(ctx, "081234567890", "swpps", "q", "msg-1", "PROD001", StockTypeFixed, "STORE01", "https://example.com/cb")
			if err == nil {
				t.Fatalf("expected error for missing %s, got nil", tc.envVar)
			}
			if !strings.Contains(err.Error(), tc.errText) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.errText)
			}
		})
	}
}

func TestOrderDealerOnConsume_EmptyMID(t *testing.T) {
	srv := orderDealerMockServer(t, nil)
	defer srv.Close()

	env := validOrderDealerEnv(srv.URL)
	env["ENCRYPTION_KEY"] = generateTestEncryptionKeyB64(t)
	for k, v := range env {
		t.Setenv(k, v)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := OrderDealerOnConsume(ctx, "081234567890", "", "q", "msg-1", "PROD001", StockTypeFixed, "STORE01", "")
	if err == nil {
		t.Fatal("expected error for empty mid, got nil")
	}
	if !strings.Contains(err.Error(), "mid is required") {
		t.Fatalf("error %q does not contain 'mid is required'", err.Error())
	}
}

func TestOrderDealerOnConsume_EmptyProductID(t *testing.T) {
	srv := orderDealerMockServer(t, nil)
	defer srv.Close()

	env := validOrderDealerEnv(srv.URL)
	env["ENCRYPTION_KEY"] = generateTestEncryptionKeyB64(t)
	for k, v := range env {
		t.Setenv(k, v)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := OrderDealerOnConsume(ctx, "081234567890", "swpps", "q", "msg-1", "", StockTypeFixed, "STORE01", "")
	if err == nil {
		t.Fatal("expected error for empty product_id, got nil")
	}
	if !strings.Contains(err.Error(), "product_id") {
		t.Fatalf("error %q does not contain 'product_id'", err.Error())
	}
}
