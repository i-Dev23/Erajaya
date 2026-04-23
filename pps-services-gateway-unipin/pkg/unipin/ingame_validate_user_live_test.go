package unipin_test

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"testing"
	"time"

	"pps-services-gateway-unipin/pkg/unipin"
)

// TestValidateUserLive hits the real Unipin API using credentials from .env file.
// Run with: go test -v -run TestValidateUserLive ./pkg/unipin/
func TestValidateUserLive(t *testing.T) {
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
	gameCode := os.Getenv("TEST_GAME_CODE")
	if gameCode == "" {
		gameCode = "MLBBD_ID"
	}
	userID := os.Getenv("TEST_USERID")
	zoneIDStr := os.Getenv("TEST_ZONEID")
	if userID == "" || zoneIDStr == "" {
		t.Skip("TEST_USERID and TEST_ZONEID env vars required for ValidateUser live test")
	}
	zoneID, err := strconv.Atoi(zoneIDStr)
	if err != nil {
		t.Fatalf("invalid TEST_ZONEID: %v", err)
	}

	resp, err := client.ValidateUser(ctx, unipin.ValidateUserRequest{
		GameCode: gameCode,
		Fields: map[string]any{
			"userid": userID,
			"zoneid": zoneID,
		},
	})
	if err != nil {
		t.Fatalf("ValidateUser failed: %v", err)
	}

	t.Logf("Status: %d", resp.Status)
	t.Logf("Reason: %s", resp.Reason)
	t.Logf("Username: %s", resp.Username)
	t.Logf("Validation Token: %s", resp.ValidationToken)
}
