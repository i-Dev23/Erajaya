package smbclient

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
	"pps-services-gateway-smb/pkg/smb"
)

type noopLogger struct{}

func (l *noopLogger) Info(msg string, kv ...any)  {}
func (l *noopLogger) Warn(msg string, kv ...any)  {}
func (l *noopLogger) Error(msg string, kv ...any) {}

func slogSilent() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestAdapter_InquiryPLNToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(smb.InquiryResponse{
			ResponseCode: "00",
			Message:      "Success",
			Data: &smb.InquiryData{
				RefID:        "REF-A1",
				ClientNumber: "111222333",
				ClientName:   "SITI",
				TarifDaya:    "R1/1300VA",
				AdminFee:     2500,
				TotalAmount:  72500,
			},
		})
	}))
	defer server.Close()

	client := smb.NewClient(server.URL, "P1", "S1", 5*time.Second, slogSilent())
	adapter := NewAdapter(client, &noopLogger{})

	resp, err := adapter.InquiryPLNToken(context.Background(), contractsvc.PLNTokenInquiryRequest{
		ClientNumber: "111222333",
		ProductCode:  "PLN70",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ResponseCode != "00" {
		t.Errorf("expected 00, got %s", resp.ResponseCode)
	}
	if resp.ClientName != "SITI" {
		t.Errorf("expected SITI, got %s", resp.ClientName)
	}
	if resp.RefID != "REF-A1" {
		t.Errorf("expected REF-A1, got %s", resp.RefID)
	}
	if resp.TotalAmount != 72500 {
		t.Errorf("expected 72500, got %f", resp.TotalAmount)
	}
}

func TestAdapter_PaymentPLNToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(smb.PaymentResponse{
			ResponseCode: "00",
			Data: &smb.PaymentData{
				Token:        "TOKEN-XYZ",
				SerialNumber: "SN-999",
				TotalAmount:  72500,
				AdminFee:     2500,
			},
		})
	}))
	defer server.Close()

	client := smb.NewClient(server.URL, "P1", "S1", 5*time.Second, slogSilent())
	adapter := NewAdapter(client, &noopLogger{})

	resp, err := adapter.PaymentPLNToken(context.Background(), contractsvc.PLNTokenPaymentRequest{
		ClientNumber: "111222333",
		ProductCode:  "PLN70",
		RefID:        "REF-A1",
		TotalAmount:  72500,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Token != "TOKEN-XYZ" {
		t.Errorf("expected TOKEN-XYZ, got %s", resp.Token)
	}
	if resp.SerialNumber != "SN-999" {
		t.Errorf("expected SN-999, got %s", resp.SerialNumber)
	}
}

func TestAdapter_AdvicePLNToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(smb.AdviceResponse{
			ResponseCode: "00",
			Data: &smb.AdviceData{
				Token:        "ADVICE-TOKEN",
				SerialNumber: "SN-ADV",
				TotalAmount:  72500,
			},
		})
	}))
	defer server.Close()

	client := smb.NewClient(server.URL, "P1", "S1", 5*time.Second, slogSilent())
	adapter := NewAdapter(client, &noopLogger{})

	resp, err := adapter.AdvicePLNToken(context.Background(), contractsvc.PLNTokenAdviceRequest{
		ClientNumber: "111222333",
		RefID:        "REF-A1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Token != "ADVICE-TOKEN" {
		t.Errorf("expected ADVICE-TOKEN, got %s", resp.Token)
	}
}

func TestAdapter_InquiryPLNToken_ServerError(t *testing.T) {
	client := smb.NewClient("http://localhost:1", "P1", "S1", 1*time.Second, slogSilent())
	adapter := NewAdapter(client, &noopLogger{})

	_, err := adapter.InquiryPLNToken(context.Background(), contractsvc.PLNTokenInquiryRequest{
		ClientNumber: "111222333",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
