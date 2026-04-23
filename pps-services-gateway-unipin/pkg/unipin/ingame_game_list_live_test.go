package unipin_test

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"pps-services-gateway-unipin/pkg/unipin"
)

// TestGameListLive hits the real Unipin API using credentials from .env file.
// Run with: go test -v -run TestGameListLive ./pkg/unipin/
func TestGameListLive(t *testing.T) {
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
	resp, err := client.GameList(ctx)
	if err != nil {
		t.Fatalf("GameList failed: %v", err)
	}

	t.Logf("Status: %d", resp.Status)
	t.Logf("Reason: %s", resp.Reason)
	t.Logf("Total games: %d", len(resp.GameList))

	for i, g := range resp.GameList {
		t.Logf("[%d] %s (%s) - %s - %s", i+1, g.GameName, g.GameCode, g.ProductName, g.GameStatus)
	}
}

// loadEnvFile loads .env from project root into os environment.
func loadEnvFile(t *testing.T) {
	t.Helper()

	if os.Getenv("RUN_LIVE_TESTS") != "1" {
		t.Skip("set RUN_LIVE_TESTS=1 to run live tests")
	}

	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	envPath := filepath.Join(projectRoot, ".env")

	f, err := os.Open(envPath)
	if err != nil {
		t.Logf("no .env file found at %s, using existing env vars", envPath)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		os.Setenv(key, val)
	}
}
