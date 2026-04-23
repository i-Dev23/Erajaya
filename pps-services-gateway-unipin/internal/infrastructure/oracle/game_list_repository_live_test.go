package oracle_test

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"pps-services-gateway-unipin/internal/domain/contract/repository"
	"pps-services-gateway-unipin/internal/infrastructure/oracle"
)

// TestUpsertGameListLive calls the real Oracle stored procedure.
// Run with: go test -v -run TestUpsertGameListLive ./internal/infrastructure/oracle/
func TestUpsertGameListLive(t *testing.T) {
	loadEnvFile(t)

	dsn := os.Getenv("ORACLE_DSN")
	if dsn == "" {
		t.Skip("ORACLE_DSN env var required for live test")
	}

	db, err := oracle.NewDB(dsn, oracle.DefaultPoolConfig())
	if err != nil {
		t.Fatalf("failed to connect oracle: %v", err)
	}
	defer db.Close()

	repo := oracle.NewGameListRepository(db)

	row := &repository.GameListRow{
		GameCode:       "MLBBD_ID",
		GameDesc:       "Mobile Legends Diamonds",
		GameCategory:   "MLBB_ID",
		DenominationID: 28,
		GameName:       "185 Diamonds",
		Currency:       "IDR",
		Amount:         "50000.00",
		FieldRequest:   `[{"name":"userid","type":"string"},{"name":"zoneid","type":"number"}]`,
		Provider:       "UNIPIN",
	}

	ctx := context.Background()
	errCode, errMsg, err := repo.UpsertGameList(ctx, row)
	if err != nil {
		t.Fatalf("UpsertGameList failed: %v", err)
	}

	t.Logf("ErrCode: %s", errCode)
	t.Logf("ErrMsg: %s", errMsg)
}

// loadEnvFile loads .env from project root into os environment.
func loadEnvFile(t *testing.T) {
	t.Helper()

	if os.Getenv("RUN_LIVE_TESTS") != "1" {
		t.Skip("set RUN_LIVE_TESTS=1 to run live tests")
	}

	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..")
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
