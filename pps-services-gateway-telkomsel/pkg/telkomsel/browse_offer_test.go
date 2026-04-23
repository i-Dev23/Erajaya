package telkomsel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBrowseOffer_RequestQueryAndResponseJSON(t *testing.T) {
	// Arrange: mock Telkomsel endpoint.
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
		if q.Get("transaction_id") == "" || q.Get("channel") == "" || q.Get("organization_code") == "" || q.Get("service_id") == "" || q.Get("product_id") == "" || q.Get("version") == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"missing query params"}`))
			return
		}

		externalTxID := r.Header.Get("External-Transaction-Id")
		channelID := r.Header.Get("Channel-Id")
		if externalTxID != q.Get("transaction_id") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"error":"unexpected external tx id: %s"}`, externalTxID)))
			return
		}
		if channelID != q.Get("channel") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"error":"unexpected channel id: %s"}`, channelID)))
			return
		}

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
		}`, q.Get("transaction_id"), q.Get("channel"), q.Get("product_id"))))
	}))
	defer srv.Close()

	restore := setEnvForTest(t, map[string]string{
		"API_KEY":    "apikey",
		"SECRET_KEY": "secret",
	})
	defer restore()

	client, err := NewClient(srv.URL, "ta", "secret", "apikey", 2*time.Second, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	req := BrowseOfferRequest{
		TransactionID:    "ORG0011908101038001000001",
		Channel:          "ta",
		OrganizationCode: "ORG909",
		ServiceID:        "6281234567890",
		ProductID:        "00001234",
		Version:          "v2",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Act.
	resp, err := client.BrowseOffer(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected response, got nil")
	}
	if resp.Product == nil {
		t.Fatalf("expected product, got nil")
	}
	if resp.Transaction.StatusCode != "00000" {
		t.Fatalf("expected status_code 00000, got %q", resp.Transaction.StatusCode)
	}
	if resp.Product.ID != req.ProductID {
		t.Fatalf("expected product.id %q, got %q", req.ProductID, resp.Product.ID)
	}

	pretty, _ := json.MarshalIndent(resp, "", "  ")
	t.Logf("Browse Offer response (mock):\n%s", string(pretty))
}

func TestBrowseOffer_HTTP4xxWithJSON_ReturnsBusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != browseOfferPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "ORG0011908101038001000001",
				"channel": "i9",
				"status_code": "30RV-0001",
				"status_desc": "Could not get eligible product information"
			}
		}`))
	}))
	defer srv.Close()

	restore := setEnvForTest(t, map[string]string{
		"API_KEY":    "apikey",
		"SECRET_KEY": "secret",
	})
	defer restore()

	client, err := NewClient(srv.URL, "ta", "secret", "apikey", 2*time.Second, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	req := BrowseOfferRequest{
		TransactionID:    "ORG0011908101038001000001",
		Channel:          "ta",
		OrganizationCode: "ORG909",
		ServiceID:        "6281234567890",
		ProductID:        "00001234",
		Version:          "v2",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := client.BrowseOffer(ctx, req)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	var businessErr *BusinessError
	if !errors.As(err, &businessErr) {
		t.Fatalf("expected BusinessError, got %T: %v", err, err)
	}

	if businessErr.Code != "30RV-0001" {
		t.Fatalf("expected code 30RV-0001, got %q", businessErr.Code)
	}
	if resp == nil {
		t.Fatalf("expected response alongside error, got nil")
	}
	if resp.Transaction.StatusCode != "30RV-0001" {
		t.Fatalf("expected status_code 30RV-0001, got %q", resp.Transaction.StatusCode)
	}
}

func TestBrowseOffer_ResponseProductAsArray_IsAccepted(t *testing.T) {
	// Arrange: mock Telkomsel endpoint returning product as array.
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{
			"transaction": {
				"transaction_id": %q,
				"channel": %q,
				"status_code": "RV-0000",
				"status_desc": "Success"
			},
			"product": [{
				"id": %q,
				"commercial_name": "Combo 50 GB",
				"allowance_description": "Internet 4G 20 GB, Internet Malam 30 GB",
				"product_length": "30 Days",
				"price": "120000",
				"stock_type": "BULK;FIXED"
			}]
		}`, q.Get("transaction_id"), q.Get("channel"), q.Get("product_id"))))
	}))
	defer srv.Close()

	restore := setEnvForTest(t, map[string]string{
		"API_KEY":    "apikey",
		"SECRET_KEY": "secret",
	})
	defer restore()

	client, err := NewClient(srv.URL, "ta", "secret", "apikey", 2*time.Second, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	req := BrowseOfferRequest{
		TransactionID:    "ORG0011908101038001000001",
		Channel:          "ta",
		OrganizationCode: "ORG909",
		ServiceID:        "6281234567890",
		ProductID:        "00001234",
		Version:          "v2",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Act.
	resp, err := client.BrowseOffer(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Product == nil {
		t.Fatalf("expected product, got %+v", resp)
	}
	if resp.Product.ID != req.ProductID {
		t.Fatalf("expected product.id %q, got %q", req.ProductID, resp.Product.ID)
	}
}
