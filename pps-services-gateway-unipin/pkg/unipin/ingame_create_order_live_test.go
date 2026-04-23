package unipin_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"pps-services-gateway-unipin/pkg/unipin"
)

// TestCreateOrderLive hits the real Unipin API using credentials from .env file.
// Run with: go test -v -run TestCreateOrderLive ./pkg/unipin/
func TestCreateOrderLive(t *testing.T) {
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

	validationToken := os.Getenv("TEST_VALIDATION_TOKEN")
	referenceNo := os.Getenv("TEST_REFERENCE_NO")
	denominationID := os.Getenv("TEST_DENOMINATION_ID")
	gameCode := os.Getenv("TEST_GAME_CODE")

	if validationToken == "" || referenceNo == "" || denominationID == "" || gameCode == "" {
		t.Skip("TEST_GAME_CODE, TEST_VALIDATION_TOKEN, TEST_REFERENCE_NO, TEST_DENOMINATION_ID env vars required for live test")
	}

	ctx := context.Background()
	resp, err := client.CreateOrder(ctx, unipin.CreateOrderRequest{
		GameCode:        gameCode,
		ValidationToken: validationToken,
		ReferenceNo:     referenceNo,
		DenominationID:  denominationID,
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	t.Logf("Status: %d", resp.Status)
	t.Logf("Reason: %s", resp.Reason)
	t.Logf("Transaction Number: %s", resp.TransactionNumber)
	t.Logf("Reference No: %s", resp.ReferenceNo)
	t.Logf("Amount: %d %s", resp.Amount, resp.Currency)
	t.Logf("Item Name: %s", resp.ItemName)
}
