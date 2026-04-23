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

func TestBrowseOfferOnConsume_BuildsQueryAndHeaders(t *testing.T) {
	type captured struct {
		transactionID string
		channel       string
		orgCode       string
		serviceID     string
		productID     string
		version       string

		externalTxHeader string
		channelHeader    string
	}

	var got captured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != browseOfferPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		q := r.URL.Query()
		got.transactionID = q.Get("transaction_id")
		got.channel = q.Get("channel")
		got.orgCode = q.Get("organization_code")
		got.serviceID = q.Get("service_id")
		got.productID = q.Get("product_id")
		got.version = q.Get("version")
		got.externalTxHeader = r.Header.Get("External-Transaction-Id")
		got.channelHeader = r.Header.Get("Channel-Id")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{
			"transaction": {
				"transaction_id": %q,
				"channel": %q,
				"status_code": "00000",
				"status_desc": "Success"
			},
			"product": {
				"id": %q,
				"commercial_name": "Combo 50 GB",
				"allowance_description": "Internet 4G 20 GB, Internet Malam 30 GB",
				"product_length": "30 Days",
				"price": "120000",
				"stock_type": "BULK;FIXED"
			}
		}`,
			q.Get("transaction_id"),
			q.Get("channel"),
			q.Get("product_id"),
		)))
	}))
	defer srv.Close()

	restore := setEnvForTest(t, map[string]string{
		"BASE_URL":   srv.URL,
		"CHANNEL_ID": "ta",
		"SECRET_KEY": "secret",
		"API_KEY":    "apikey",
		"TIMEOUT":    "2",
		// mid-based mapping: ${ORGANIZATION_CODE}_${PIN}
		"swpps": "ORG909_002314",
	})
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := BrowseOfferOnConsume(ctx, "081234567890", "swpps", "trx_paket_data_queue", "20260305", "00001234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected response, got nil")
	}

	if got.orgCode != "ORG909" {
		t.Fatalf("expected organization_code ORG909, got %q", got.orgCode)
	}
	if got.serviceID != "6281234567890" {
		t.Fatalf("expected service_id 6281234567890, got %q", got.serviceID)
	}
	if got.productID != "00001234" {
		t.Fatalf("expected product_id 00001234, got %q", got.productID)
	}
	if got.version != "v2" {
		t.Fatalf("expected version v2, got %q", got.version)
	}
	if got.channel != "ta" {
		t.Fatalf("expected channel ta, got %q", got.channel)
	}
	if strings.TrimSpace(got.transactionID) == "" {
		t.Fatalf("expected transaction_id, got empty")
	}
	if got.externalTxHeader != got.transactionID {
		t.Fatalf("expected External-Transaction-Id %q, got %q", got.transactionID, got.externalTxHeader)
	}
	if got.channelHeader != got.channel {
		t.Fatalf("expected Channel-Id header %q, got %q", got.channel, got.channelHeader)
	}

	if resp.Transaction.StatusCode != "00000" {
		t.Fatalf("expected status_code 00000, got %q", resp.Transaction.StatusCode)
	}
	if resp.Product == nil || resp.Product.ID != "00001234" {
		t.Fatalf("expected product.id 00001234, got %+v", resp.Product)
	}
}

func TestBrowseOfferOnConsume_MissingEnvVars(t *testing.T) {
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
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"transaction":{"status_code":"00000"}}`))
			}))
			defer srv.Close()

			env := map[string]string{
				"BASE_URL":   srv.URL,
				"CHANNEL_ID": "ta",
				"SECRET_KEY": "secret",
				"API_KEY":    "apikey",
				"TIMEOUT":    "2",
				"swpps":      "ORG909_002314",
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

			_, err := BrowseOfferOnConsume(ctx, "081234567890", "swpps", "q", "msg-1", "PROD001")
			if err == nil {
				t.Fatalf("expected error for missing %s, got nil", tc.envVar)
			}
			if !strings.Contains(err.Error(), tc.errText) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.errText)
			}
		})
	}
}

func TestBrowseOfferOnConsume_EmptyMID(t *testing.T) {
	t.Setenv("BASE_URL", "http://localhost")
	t.Setenv("CHANNEL_ID", "ta")
	t.Setenv("SECRET_KEY", "secret")
	t.Setenv("API_KEY", "apikey")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := BrowseOfferOnConsume(ctx, "081234567890", "", "q", "msg-1", "PROD001")
	if err == nil {
		t.Fatal("expected error for empty mid, got nil")
	}
	if !strings.Contains(err.Error(), "mid is required") {
		t.Fatalf("error %q does not contain 'mid is required'", err.Error())
	}
}

func TestBrowseOfferOnConsume_EmptyProductID(t *testing.T) {
	t.Setenv("BASE_URL", "http://localhost")
	t.Setenv("CHANNEL_ID", "ta")
	t.Setenv("SECRET_KEY", "secret")
	t.Setenv("API_KEY", "apikey")
	t.Setenv("swpps", "ORG909_002314")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := BrowseOfferOnConsume(ctx, "081234567890", "swpps", "q", "msg-1", "")
	if err == nil {
		t.Fatal("expected error for empty product_id, got nil")
	}
	if !strings.Contains(err.Error(), "product_id") {
		t.Fatalf("error %q does not contain 'product_id'", err.Error())
	}
}
