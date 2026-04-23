package unipin_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"pps-services-gateway-unipin/pkg/unipin"
)

// TestVoucherDetailLive hits the real Unipin API using credentials from .env file.
// Run with: go test -v -run TestVoucherDetailLive ./pkg/unipin/
func TestVoucherDetailLive(t *testing.T) {
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
	voucherCode := os.Getenv("TEST_VOUCHER_CODE")
	if voucherCode == "" {
		t.Skip("TEST_VOUCHER_CODE env var required for VoucherDetail live test")
	}
	resp, err := client.VoucherDetail(ctx, voucherCode)
	if err != nil {
		t.Fatalf("VoucherDetail failed: %v", err)
	}

	t.Logf("Status: %d", resp.Status)
	t.Logf("Voucher: %s (%s)", resp.VoucherName, resp.VoucherCode)
	t.Logf("Icon: %s", resp.IconURL)
	t.Logf("Total denominations: %d", len(resp.Denominations))

	for i, d := range resp.Denominations {
		t.Logf("  [%d] %s - %s %s %s", i+1, d.DenominationCode, d.DenominationName, d.DenominationCurrency, d.DenominationAmount)
	}
}
