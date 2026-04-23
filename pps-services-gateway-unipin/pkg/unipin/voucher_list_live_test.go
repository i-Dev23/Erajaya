package unipin_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"pps-services-gateway-unipin/pkg/unipin"
)

// TestVoucherListLive hits the real Unipin API using credentials from .env file.
// Run with: go test -v -run TestVoucherListLive ./pkg/unipin/
func TestVoucherListLive(t *testing.T) {
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
	resp, err := client.VoucherList(ctx)
	if err != nil {
		t.Fatalf("VoucherList failed: %v", err)
	}

	t.Logf("Status: %d", resp.Status)
	t.Logf("Total vouchers: %d", len(resp.VoucherList))

	for i, v := range resp.VoucherList {
		t.Logf("[%d] %s (%s) - %s", i+1, v.VoucherName, v.VoucherCode, v.IconURL)
	}
}
