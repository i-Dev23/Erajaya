package smb

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

// ═══════════════════════════════════════════════════════════════
// InquiryPLNToken
// ═══════════════════════════════════════════════════════════════

func TestInquiryPLNToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/pln-prepaid/inquiry" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content-type: %s", r.Header.Get("Content-Type"))
		}

		// Verify request body
		var req InquiryRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.ClientNumber != "12345678901" {
			t.Errorf("unexpected client_number: %s", req.ClientNumber)
		}
		if req.PartnerID != "PARTNER1" {
			t.Errorf("unexpected partner_id: %s", req.PartnerID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(InquiryResponse{
			ResponseCode: "00",
			Message:      "Success",
			Data: &InquiryData{
				RefID:        "REF-001",
				ClientNumber: "12345678901",
				ClientName:   "BUDI SANTOSO",
				TarifDaya:    "R1/900VA",
				AdminFee:     2500,
				TotalAmount:  52500,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "PARTNER1", "SECRET1", 5*time.Second, newTestLogger())
	resp, rawBody, err := client.InquiryPLNToken(context.Background(), "12345678901", "PLN50")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ResponseCode != "00" {
		t.Errorf("expected 00, got %s", resp.ResponseCode)
	}
	if resp.Data == nil {
		t.Fatal("expected data, got nil")
	}
	if resp.Data.RefID != "REF-001" {
		t.Errorf("expected REF-001, got %s", resp.Data.RefID)
	}
	if resp.Data.ClientName != "BUDI SANTOSO" {
		t.Errorf("expected BUDI SANTOSO, got %s", resp.Data.ClientName)
	}
	if resp.Data.TotalAmount != 52500 {
		t.Errorf("expected 52500, got %f", resp.Data.TotalAmount)
	}
	if len(rawBody) == 0 {
		t.Error("expected raw body, got empty")
	}
}

func TestInquiryPLNToken_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(InquiryResponse{
			ResponseCode: "94",
			Message:      "Error Inquiry Data",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "P1", "S1", 5*time.Second, newTestLogger())
	resp, _, err := client.InquiryPLNToken(context.Background(), "99999999999", "PLN50")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ResponseCode != "94" {
		t.Errorf("expected 94, got %s", resp.ResponseCode)
	}
	if resp.Data != nil {
		t.Error("expected nil data on error")
	}
}

func TestInquiryPLNToken_ServerDown(t *testing.T) {
	client := NewClient("http://localhost:1", "P1", "S1", 1*time.Second, newTestLogger())
	_, _, err := client.InquiryPLNToken(context.Background(), "12345678901", "PLN50")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestInquiryPLNToken_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "P1", "S1", 5*time.Second, newTestLogger())
	_, rawBody, err := client.InquiryPLNToken(context.Background(), "12345678901", "PLN50")

	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if len(rawBody) == 0 {
		t.Error("expected raw body even on parse error")
	}
}

// ═══════════════════════════════════════════════════════════════
// PaymentPLNToken
// ═══════════════════════════════════════════════════════════════

func TestPaymentPLNToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/pln-prepaid/payment" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req PaymentRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.RefID != "REF-001" {
			t.Errorf("unexpected ref_id: %s", req.RefID)
		}
		if req.TotalAmount != 52500 {
			t.Errorf("unexpected total_amount: %f", req.TotalAmount)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PaymentResponse{
			ResponseCode: "00",
			Message:      "Success",
			Data: &PaymentData{
				RefID:        "REF-001",
				ClientNumber: "12345678901",
				ClientName:   "BUDI SANTOSO",
				Token:        "1234-5678-9012-3456-7890",
				SerialNumber: "SN123",
				TotalAmount:  52500,
				AdminFee:     2500,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "P1", "S1", 5*time.Second, newTestLogger())
	resp, _, err := client.PaymentPLNToken(context.Background(), "12345678901", "PLN50", "REF-001", 52500)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ResponseCode != "00" {
		t.Errorf("expected 00, got %s", resp.ResponseCode)
	}
	if resp.Data.Token != "1234-5678-9012-3456-7890" {
		t.Errorf("expected token, got %s", resp.Data.Token)
	}
}

func TestPaymentPLNToken_Pending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PaymentResponse{
			ResponseCode: "28",
			Message:      "Timeout atau Pending Transaksi",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "P1", "S1", 5*time.Second, newTestLogger())
	resp, _, err := client.PaymentPLNToken(context.Background(), "12345678901", "PLN50", "REF-002", 52500)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ResponseCode != "28" {
		t.Errorf("expected 28, got %s", resp.ResponseCode)
	}
}

func TestPaymentPLNToken_ServerDown(t *testing.T) {
	client := NewClient("http://localhost:1", "P1", "S1", 1*time.Second, newTestLogger())
	_, _, err := client.PaymentPLNToken(context.Background(), "12345678901", "PLN50", "REF-003", 52500)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ═══════════════════════════════════════════════════════════════
// AdvicePLNToken
// ═══════════════════════════════════════════════════════════════

func TestAdvicePLNToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/pln-prepaid/advice" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AdviceResponse{
			ResponseCode: "00",
			Message:      "Success",
			Data: &AdviceData{
				RefID:        "REF-001",
				ClientNumber: "12345678901",
				Token:        "ADVICE-TOKEN-123",
				SerialNumber: "SN456",
				TotalAmount:  52500,
				AdminFee:     2500,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "P1", "S1", 5*time.Second, newTestLogger())
	resp, _, err := client.AdvicePLNToken(context.Background(), "12345678901", "REF-001")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ResponseCode != "00" {
		t.Errorf("expected 00, got %s", resp.ResponseCode)
	}
	if resp.Data.Token != "ADVICE-TOKEN-123" {
		t.Errorf("expected ADVICE-TOKEN-123, got %s", resp.Data.Token)
	}
}

func TestAdvicePLNToken_StillPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AdviceResponse{
			ResponseCode: "68",
			Message:      "Timeout atau Pending Transaksi",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "P1", "S1", 5*time.Second, newTestLogger())
	resp, _, err := client.AdvicePLNToken(context.Background(), "12345678901", "REF-002")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ResponseCode != "68" {
		t.Errorf("expected 68, got %s", resp.ResponseCode)
	}
}

func TestAdvicePLNToken_ServerDown(t *testing.T) {
	client := NewClient("http://localhost:1", "P1", "S1", 1*time.Second, newTestLogger())
	_, _, err := client.AdvicePLNToken(context.Background(), "12345678901", "REF-003")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ═══════════════════════════════════════════════════════════════
// Signature
// ═══════════════════════════════════════════════════════════════

func TestGenerateSignature(t *testing.T) {
	client := NewClient("http://localhost", "PARTNER1", "SECRET1", 5*time.Second, newTestLogger())
	sig1 := client.generateSignature("REF-001")
	sig2 := client.generateSignature("REF-001")
	sig3 := client.generateSignature("REF-002")

	if sig1 != sig2 {
		t.Error("same input should produce same signature")
	}
	if sig1 == sig3 {
		t.Error("different input should produce different signature")
	}
	if len(sig1) != 32 {
		t.Errorf("MD5 hex should be 32 chars, got %d", len(sig1))
	}
}
