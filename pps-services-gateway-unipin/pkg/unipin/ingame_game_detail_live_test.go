package unipin_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"pps-services-gateway-unipin/pkg/unipin"
)

// TestGameDetailLive hits the real Unipin API using credentials from .env file.
// Run with: go test -v -run TestGameDetailLive ./pkg/unipin/
func TestGameDetailLive(t *testing.T) {
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
	resp, err := client.GameDetail(ctx, "MLBBD_ID")
	if err != nil {
		t.Fatalf("GameDetail failed: %v", err)
	}

	t.Logf("Status: %d", resp.Status)
	t.Logf("Reason: %s", resp.Reason)
	t.Logf("Game: %s (%s) - Category: %s", resp.Game.Name, resp.Game.Code, resp.Game.Category)
	t.Logf("Help Image: %s", resp.HelpImageURL)
	t.Logf("Total denominations: %d", len(resp.Denominations))

	for i, d := range resp.Denominations {
		t.Logf("  [%d] ID=%d Name=%s Currency=%s Amount=%s", i+1, d.ID, d.Name, d.Currency, d.Amount)
	}

	t.Logf("Required fields: %d", len(resp.Fields))
	for _, f := range resp.Fields {
		t.Logf("  - %s (%s)", f.Name, f.Type)
	}
}
