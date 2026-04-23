package unipin_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"pps-services-gateway-unipin/pkg/unipin"
)

// TestVoucherRequestLive hits the real Unipin API using credentials from .env file.
// Run with: go test -v -run TestVoucherRequestLive ./pkg/unipin/
func TestVoucherRequestLive(t *testing.T) {
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
	denomCode := os.Getenv("TEST_DENOMINATION_CODE")
	if denomCode == "" {
		t.Skip("TEST_DENOMINATION_CODE env var required for VoucherRequest live test")
	}
	referenceNo := os.Getenv("TEST_REFERENCE_NO")
	if referenceNo == "" {
		referenceNo = "ref-" + time.Now().UTC().Format("20060102150405")
	}

	resp, err := client.VoucherRequest(ctx, unipin.VoucherRequestReq{
		DenominationCode: denomCode,
		Quantity:         1,
		ReferenceNo:      referenceNo,
	})
	if err != nil {
		t.Fatalf("VoucherRequest failed: %v", err)
	}

	t.Logf("Status: %d", resp.Status)
	t.Logf("Message: %s", resp.Message)
	t.Logf("Order: %s", resp.Order)
	t.Logf("Reference No: %s", resp.ReferenceNo)
	t.Logf("Total Amount: %d %s", resp.TotalAmount, resp.Currency)
	t.Logf("Balance: %d", resp.Balance)
	t.Logf("Items: %d", len(resp.Items))
	for i, item := range resp.Items {
		t.Logf("  [%d] Serial1=%s Serial2=%s", i+1, item.Serial1, item.Serial2)
	}
}
