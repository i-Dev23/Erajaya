package gamesync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pps-services-gateway-unipin/internal/domain/contract/repository"
	"pps-services-gateway-unipin/pkg/unipin"
)

// --- Mock Logger ---

type mockLogger struct {
	infos  []string
	warns  []string
	errors []string
}

func (m *mockLogger) Info(msg string, args ...any)  { m.infos = append(m.infos, fmt.Sprintf(msg, args...)) }
func (m *mockLogger) Warn(msg string, args ...any)  { m.warns = append(m.warns, fmt.Sprintf(msg, args...)) }
func (m *mockLogger) Error(msg string, args ...any) { m.errors = append(m.errors, fmt.Sprintf(msg, args...)) }

// --- Mock Repository ---

type mockGameListRepo struct {
	calls   []*repository.GameListRow
	errCode string
	errMsg  string
	err     error
}

func (m *mockGameListRepo) UpsertGameList(ctx context.Context, row *repository.GameListRow) (string, string, error) {
	m.calls = append(m.calls, row)
	if m.err != nil {
		return "", "", m.err
	}
	return m.errCode, m.errMsg, nil
}

// --- Helper: create mock Unipin server ---

func newMockUnipinServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/in-game-topup/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": 1,
			"reason": "success",
			"game_list": []map[string]any{
				{
					"game_code":     "MLBBD_ID",
					"game_name":     "Mobile Legends Diamonds",
					"game_category": "MLBB_ID",
					"game_status":   "active",
				},
				{
					"game_code":     "FFD_ID",
					"game_name":     "Free Fire",
					"game_category": "FFD_ID",
					"game_status":   "active",
				},
			},
		})
	})

	mux.HandleFunc("/in-game-topup/detail", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)

		denominations := []map[string]any{
			{"id": 324, "name": "154 Diamonds + 16 Bonus", "currency": "IDR", "amount": "58696.00"},
			{"id": 28, "name": "167 Diamonds + 18 Bonus", "currency": "IDR", "amount": "55000.00"},
		}
		if body["game_code"] == "FFD_ID" {
			denominations = []map[string]any{
				{"id": 501, "name": "100 Diamonds", "currency": "IDR", "amount": "15000.00"},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": 1,
			"reason": "success",
			"game": map[string]string{
				"name":     body["game_code"],
				"code":     body["game_code"],
				"category": "mobile_game",
			},
			"denominations": denominations,
			"fields": []map[string]string{
				{"name": "userid", "type": "string"},
				{"name": "zoneid", "type": "number"},
			},
		})
	})

	return httptest.NewServer(mux)
}

func newTestClient(t *testing.T, serverURL string) *unipin.Client {
	t.Helper()
	slogLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client, err := unipin.NewClient(serverURL, "test-partner", "test-secret", 10*time.Second, slogLogger)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client
}

// --- Tests ---

func TestSyncGameList_FullFlow(t *testing.T) {
	server := newMockUnipinServer(t)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "0", errMsg: ""}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)

	err := svc.SyncGameList(context.Background())
	if err != nil {
		t.Fatalf("SyncGameList failed: %v", err)
	}

	// 2 games: MLBB (2 denoms) + FF (1 denom) = 3 calls
	if len(repo.calls) != 3 {
		t.Fatalf("expected 3 upsert calls, got %d", len(repo.calls))
	}

	// Verify first call — MLBB, denomination 324
	c1 := repo.calls[0]
	assertEqual(t, "call1.GameCode", "MLBBD_ID", c1.GameCode)
	assertEqual(t, "call1.GameDesc", "Mobile Legends Diamonds", c1.GameDesc)
	assertEqual(t, "call1.GameCategory", "MLBB_ID", c1.GameCategory)
	assertEqualInt(t, "call1.DenominationID", 324, c1.DenominationID)
	assertEqual(t, "call1.GameName", "154 Diamonds + 16 Bonus", c1.GameName)
	assertEqual(t, "call1.Currency", "IDR", c1.Currency)
	assertEqual(t, "call1.Amount", "58696.00", c1.Amount)
	assertEqual(t, "call1.Provider", "unipin-game", c1.Provider)
	if c1.FieldRequest == "" || c1.FieldRequest == "null" {
		t.Error("call1.FieldRequest should not be empty")
	}

	// Verify second call — MLBB, denomination 28
	c2 := repo.calls[1]
	assertEqual(t, "call2.GameCode", "MLBBD_ID", c2.GameCode)
	assertEqualInt(t, "call2.DenominationID", 28, c2.DenominationID)
	assertEqual(t, "call2.Amount", "55000.00", c2.Amount)

	// Verify third call — FF, denomination 501
	c3 := repo.calls[2]
	assertEqual(t, "call3.GameCode", "FFD_ID", c3.GameCode)
	assertEqualInt(t, "call3.DenominationID", 501, c3.DenominationID)
	assertEqual(t, "call3.Amount", "15000.00", c3.Amount)

	t.Logf("✅ All %d upsert calls verified", len(repo.calls))
	for i, c := range repo.calls {
		t.Logf("  Call %d: GameCode=%s DenomID=%d GameName=%s Amount=%s Provider=%s",
			i+1, c.GameCode, c.DenominationID, c.GameName, c.Amount, c.Provider)
	}
}

func TestSyncGameList_SPError_ContinuesProcessing(t *testing.T) {
	server := newMockUnipinServer(t)
	defer server.Close()

	// SP returns error code
	repo := &mockGameListRepo{errCode: "99", errMsg: "duplicate entry"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)

	err := svc.SyncGameList(context.Background())
	if err != nil {
		t.Fatalf("expected no error (SP errors are non-fatal), got: %v", err)
	}

	// All 3 calls should still be made even though SP returns errors
	if len(repo.calls) != 3 {
		t.Fatalf("expected 3 upsert calls, got %d", len(repo.calls))
	}

	// Should have warnings logged
	if len(logger.warns) == 0 {
		t.Error("expected SP error warnings to be logged")
	}

	t.Log("✅ SP errors handled gracefully, all games processed")
}

func TestSyncGameList_DBError_ContinuesProcessing(t *testing.T) {
	server := newMockUnipinServer(t)
	defer server.Close()

	// DB returns hard error
	repo := &mockGameListRepo{err: fmt.Errorf("connection refused")}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)

	err := svc.SyncGameList(context.Background())
	if err != nil {
		t.Fatalf("expected no error (DB errors are non-fatal per game), got: %v", err)
	}

	if len(logger.errors) == 0 {
		t.Error("expected DB errors to be logged")
	}

	t.Log("✅ DB errors handled gracefully, sync completed")
}

func TestSyncGameList_ContextCancelled(t *testing.T) {
	server := newMockUnipinServer(t)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := svc.SyncGameList(ctx)
	if err == nil {
		t.Fatal("expected context cancelled error")
	}

	t.Logf("✅ Context cancellation handled: %v", err)
}

func TestSyncGameList_FieldRequestJSON(t *testing.T) {
	server := newMockUnipinServer(t)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)

	err := svc.SyncGameList(context.Background())
	if err != nil {
		t.Fatalf("SyncGameList failed: %v", err)
	}

	// Verify FieldRequest is valid JSON containing field definitions
	c1 := repo.calls[0]
	var fields []map[string]string
	if err := json.Unmarshal([]byte(c1.FieldRequest), &fields); err != nil {
		t.Fatalf("FieldRequest is not valid JSON: %v\nGot: %s", err, c1.FieldRequest)
	}

	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}

	if fields[0]["name"] != "userid" || fields[1]["name"] != "zoneid" {
		t.Errorf("unexpected fields: %v", fields)
	}

	t.Logf("✅ FieldRequest JSON valid: %s", c1.FieldRequest)
}

// --- Helpers ---

func assertEqual(t *testing.T, name, expected, actual string) {
	t.Helper()
	if expected != actual {
		t.Errorf("%s: expected %q, got %q", name, expected, actual)
	}
}

func assertEqualInt(t *testing.T, name string, expected, actual int) {
	t.Helper()
	if expected != actual {
		t.Errorf("%s: expected %d, got %d", name, expected, actual)
	}
}

// --- Mock Voucher Repository ---

type mockVoucherListRepo struct {
	calls   []*repository.VoucherListRow
	errCode string
	errMsg  string
	err     error
}

func (m *mockVoucherListRepo) UpsertVoucherList(_ context.Context, row *repository.VoucherListRow) (string, string, error) {
	m.calls = append(m.calls, row)
	if m.err != nil {
		return "", "", m.err
	}
	return m.errCode, m.errMsg, nil
}

// --- Helper: mock server with voucher endpoints ---

func newMockUnipinServerWithVouchers(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// Game endpoints (reuse from existing)
	mux.HandleFunc("/in-game-topup/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": 1, "reason": "success",
			"game_list": []map[string]any{
				{"game_code": "MLBBD_ID", "game_name": "Mobile Legends Diamonds", "game_category": "MLBB_ID", "game_status": "active"},
				{"game_code": "FFD_ID", "game_name": "Free Fire", "game_category": "FFD_ID", "game_status": "active"},
			},
		})
	})

	mux.HandleFunc("/in-game-topup/detail", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		denoms := []map[string]any{
			{"id": 324, "name": "154 Diamonds", "currency": "IDR", "amount": "58696.00"},
		}
		if body["game_code"] == "FFD_ID" {
			denoms = []map[string]any{
				{"id": 501, "name": "100 Diamonds", "currency": "IDR", "amount": "15000.00"},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": 1, "reason": "success",
			"game":          map[string]string{"name": body["game_code"], "code": body["game_code"], "category": "mobile_game"},
			"denominations": denoms,
			"fields":        []map[string]string{{"name": "userid", "type": "string"}},
		})
	})

	// Voucher endpoints
	mux.HandleFunc("/voucher/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": 1, "reason": "success",
			"voucher_list": []map[string]any{
				{"voucher_code": "STEAM", "voucher_name": "Steam Wallet"},
				{"voucher_code": "GARENA", "voucher_name": "Garena Shells"},
			},
		})
	})

	mux.HandleFunc("/voucher/details", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		vc, _ := body["voucher_code"].(string)
		denoms := []map[string]string{
			{"denomination_code": "STEAM-50K", "denomination_name": "Steam 50K", "denomination_currency": "IDR", "denomination_amount": "50000"},
		}
		if vc == "GARENA" {
			denoms = []map[string]string{
				{"denomination_code": "GARENA-25K", "denomination_name": "Garena 25K", "denomination_currency": "IDR", "denomination_amount": "25000"},
				{"denomination_code": "GARENA-50K", "denomination_name": "Garena 50K", "denomination_currency": "IDR", "denomination_amount": "50000"},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": 1, "reason": "success",
			"voucher_name": vc, "voucher_code": vc,
			"denominations": denoms,
		})
	})

	return httptest.NewServer(mux)
}

// ─── SyncVoucherList Tests ───

func TestSyncVoucherList_FullFlow(t *testing.T) {
	server := newMockUnipinServerWithVouchers(t)
	defer server.Close()

	voucherRepo := &mockVoucherListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, nil, voucherRepo, logger)
	err := svc.SyncVoucherList(context.Background())
	if err != nil {
		t.Fatalf("SyncVoucherList failed: %v", err)
	}

	// STEAM (1 denom) + GARENA (2 denoms) = 3 calls
	if len(voucherRepo.calls) != 3 {
		t.Fatalf("expected 3 upsert calls, got %d", len(voucherRepo.calls))
	}

	c1 := voucherRepo.calls[0]
	if c1.VoucherCode != "STEAM" {
		t.Errorf("call1.VoucherCode: got %q", c1.VoucherCode)
	}
	if c1.VoucherDenominationCode != "STEAM-50K" {
		t.Errorf("call1.DenominationCode: got %q", c1.VoucherDenominationCode)
	}
	if c1.Provider != "unipin-voucher" {
		t.Errorf("call1.Provider: got %q", c1.Provider)
	}
}

func TestSyncVoucherList_SPError_ContinuesProcessing(t *testing.T) {
	server := newMockUnipinServerWithVouchers(t)
	defer server.Close()

	voucherRepo := &mockVoucherListRepo{errCode: "99", errMsg: "duplicate"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, nil, voucherRepo, logger)
	err := svc.SyncVoucherList(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(voucherRepo.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(voucherRepo.calls))
	}
	if len(logger.warns) == 0 {
		t.Error("expected SP error warnings")
	}
}

func TestSyncVoucherList_DBError_ContinuesProcessing(t *testing.T) {
	server := newMockUnipinServerWithVouchers(t)
	defer server.Close()

	voucherRepo := &mockVoucherListRepo{err: fmt.Errorf("db error")}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, nil, voucherRepo, logger)
	err := svc.SyncVoucherList(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(logger.errors) == 0 {
		t.Error("expected DB error logs")
	}
}

func TestSyncVoucherList_ContextCancelled(t *testing.T) {
	server := newMockUnipinServerWithVouchers(t)
	defer server.Close()

	voucherRepo := &mockVoucherListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, nil, voucherRepo, logger)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.SyncVoucherList(ctx)
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
}

// ─── SyncSingleGame Tests ───

func TestSyncSingleGame_Success(t *testing.T) {
	server := newMockUnipinServerWithVouchers(t)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)
	err := svc.SyncSingleGame(context.Background(), "MLBBD_ID")
	if err != nil {
		t.Fatalf("SyncSingleGame failed: %v", err)
	}
	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 upsert call, got %d", len(repo.calls))
	}
	if repo.calls[0].GameCode != "MLBBD_ID" {
		t.Errorf("GameCode: got %q", repo.calls[0].GameCode)
	}
}

func TestSyncSingleGame_NotFound(t *testing.T) {
	server := newMockUnipinServerWithVouchers(t)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)
	err := svc.SyncSingleGame(context.Background(), "NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for non-existent game")
	}
}

func TestSyncSingleGame_SPError_ContinuesProcessing(t *testing.T) {
	server := newMockUnipinServerWithVouchers(t)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "99", errMsg: "dup"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)
	err := svc.SyncSingleGame(context.Background(), "MLBBD_ID")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(logger.warns) == 0 {
		t.Error("expected SP error warnings")
	}
}

func TestSyncSingleGame_DBError(t *testing.T) {
	server := newMockUnipinServerWithVouchers(t)
	defer server.Close()

	repo := &mockGameListRepo{err: fmt.Errorf("db error")}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)
	err := svc.SyncSingleGame(context.Background(), "MLBBD_ID")
	if err != nil {
		t.Fatalf("expected no error (non-fatal), got: %v", err)
	}
	if len(logger.errors) == 0 {
		t.Error("expected DB error logs")
	}
}

// ─── SyncSingleDenomination Tests ───

func TestSyncSingleDenomination_Success(t *testing.T) {
	server := newMockUnipinServerWithVouchers(t)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)
	err := svc.SyncSingleDenomination(context.Background(), "MLBBD_ID", 324)
	if err != nil {
		t.Fatalf("SyncSingleDenomination failed: %v", err)
	}
	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(repo.calls))
	}
	if repo.calls[0].DenominationID != 324 {
		t.Errorf("DenominationID: got %d", repo.calls[0].DenominationID)
	}
}

func TestSyncSingleDenomination_GameNotFound(t *testing.T) {
	server := newMockUnipinServerWithVouchers(t)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)
	err := svc.SyncSingleDenomination(context.Background(), "NONEXISTENT", 1)
	if err == nil {
		t.Fatal("expected error for non-existent game")
	}
}

func TestSyncSingleDenomination_DenomNotFound(t *testing.T) {
	server := newMockUnipinServerWithVouchers(t)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)
	err := svc.SyncSingleDenomination(context.Background(), "MLBBD_ID", 99999)
	if err == nil {
		t.Fatal("expected error for non-existent denomination")
	}
}

func TestSyncSingleDenomination_SPError(t *testing.T) {
	server := newMockUnipinServerWithVouchers(t)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "99", errMsg: "sp error"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)
	err := svc.SyncSingleDenomination(context.Background(), "MLBBD_ID", 324)
	if err == nil {
		t.Fatal("expected SP error")
	}
}

func TestSyncSingleDenomination_DBError(t *testing.T) {
	server := newMockUnipinServerWithVouchers(t)
	defer server.Close()

	repo := &mockGameListRepo{err: fmt.Errorf("connection refused")}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)
	err := svc.SyncSingleDenomination(context.Background(), "MLBBD_ID", 324)
	if err == nil {
		t.Fatal("expected DB error")
	}
}

// ─── NewSyncService Tests ───

func TestNewSyncService(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, &mockLogger{})
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestSyncVoucherList_DetailFetchError_ContinuesProcessing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/voucher/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": 1, "reason": "success",
			"voucher_list": []map[string]any{
				{"voucher_code": "STEAM", "voucher_name": "Steam"},
			},
		})
	})
	mux.HandleFunc("/voucher/details", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	voucherRepo := &mockVoucherListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, nil, voucherRepo, logger)
	err := svc.SyncVoucherList(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(voucherRepo.calls) != 0 {
		t.Errorf("expected 0 upsert calls, got %d", len(voucherRepo.calls))
	}
	if len(logger.errors) == 0 {
		t.Error("expected error log for detail fetch failure")
	}
}

func TestSyncGameList_DetailFetchError_ContinuesProcessing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/in-game-topup/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": 1, "reason": "success",
			"game_list": []map[string]any{
				{"game_code": "MLBB", "game_name": "MLBB", "game_category": "CAT"},
			},
		})
	})
	mux.HandleFunc("/in-game-topup/detail", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)
	err := svc.SyncGameList(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(repo.calls) != 0 {
		t.Errorf("expected 0 upsert calls, got %d", len(repo.calls))
	}
	if len(logger.errors) == 0 {
		t.Error("expected error log for detail fetch failure")
	}
}

func TestSyncSingleGame_DetailFetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/in-game-topup/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": 1, "reason": "success",
			"game_list": []map[string]any{
				{"game_code": "MLBB", "game_name": "MLBB", "game_category": "CAT"},
			},
		})
	})
	mux.HandleFunc("/in-game-topup/detail", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)
	err := svc.SyncSingleGame(context.Background(), "MLBB")
	if err == nil {
		t.Fatal("expected error for detail fetch failure")
	}
}

func TestSyncSingleGame_NoDenominations(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/in-game-topup/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": 1, "reason": "success",
			"game_list": []map[string]any{
				{"game_code": "MLBB", "game_name": "MLBB", "game_category": "CAT"},
			},
		})
	})
	mux.HandleFunc("/in-game-topup/detail", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": 1, "reason": "success",
			"game":          map[string]string{"name": "MLBB", "code": "MLBB", "category": "cat"},
			"denominations": []map[string]any{},
			"fields":        []map[string]string{},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)
	err := svc.SyncSingleGame(context.Background(), "MLBB")
	if err == nil {
		t.Fatal("expected error for no denominations")
	}
}

func TestSyncSingleDenomination_DetailFetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/in-game-topup/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": 1, "reason": "success",
			"game_list": []map[string]any{
				{"game_code": "MLBB", "game_name": "MLBB", "game_category": "CAT"},
			},
		})
	})
	mux.HandleFunc("/in-game-topup/detail", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &mockGameListRepo{errCode: "0"}
	logger := &mockLogger{}
	client := newTestClient(t, server.URL)

	svc := NewSyncService(client, repo, nil, logger)
	err := svc.SyncSingleDenomination(context.Background(), "MLBB", 1)
	if err == nil {
		t.Fatal("expected error for detail fetch failure")
	}
}
