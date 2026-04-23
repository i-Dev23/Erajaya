package unipin_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"pps-services-gateway-unipin/pkg/unipin"
)

// TestOrderInquiryLive hits the real Unipin API using credentials from .env file.
// Run with: go test -v -run TestOrderInquiryLive ./pkg/unipin/
func TestOrderInquiryLive(t *testing.T) {
	loadEnvFile(t)

	baseURL := os.Getenv("BASE_URL")
	partnerID := os.Getenv("PARTNER_ID")
	secretKey := os.Getenv("SECRET_KEY")

	if baseURL == "" || partnerID == "" || secretKey == "" {
		t.Skip("BASE_URL, PARTNER_ID, SECRET_KEY env vars required for live test")
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	client, err := unipin.NewClient(baseURL, partnerID, secretKey, 30*time.Second, logger)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	resp, err := client.OrderInquiry(ctx, "BSD11102222f")
	if err != nil {
		t.Fatalf("OrderInquiry failed: %v", err)
	}

	t.Logf("Status: %d", resp.Status)
	t.Logf("Reason: %s", resp.Reason)
	t.Logf("Transaction Number: %s", resp.TransactionNumber)
	t.Logf("Reference No: %s", resp.ReferenceNo)
	t.Logf("Amount: %d %s", resp.Amount, resp.Currency)
	t.Logf("Item Name: %s", resp.ItemName)
}
