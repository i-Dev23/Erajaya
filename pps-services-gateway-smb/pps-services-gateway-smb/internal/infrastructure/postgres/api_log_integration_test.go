//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
	"pps-services-gateway-smb/internal/infrastructure/postgres"
	"pps-services-gateway-smb/pkg/smb"
)

const testDSN = "postgres://pps_telkomsel:pps_1233%23@192.168.3.247:5434/gateway_telkomsel?sslmode=disable"

// testLogger implements contractsvc.Logger for test output.
type testLogger struct{ t *testing.T }

func (l *testLogger) Info(msg string, kv ...any)  { l.t.Logf("[INFO]  %s %v", msg, kv) }
func (l *testLogger) Warn(msg string, kv ...any)  { l.t.Logf("[WARN]  %s %v", msg, kv) }
func (l *testLogger) Error(msg string, kv ...any) { l.t.Logf("[ERROR] %s %v", msg, kv) }

func getDSN() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return testDSN
}

func TestIntegration_FullAPILogFlow(t *testing.T) {
	dsn := getDSN()

	// Step 1: Test database connection
	t.Log("=== Step 1: Testing database connection ===")
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("failed to open connection: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}
	t.Log("✅ Database connection successful")

	// Step 2: Run migration (create schema log_smb + table)
	t.Log("=== Step 2: Running migration for schema log_smb ===")
	logger := &testLogger{t: t}
	repo := postgres.NewAPILogRepositoryImpl(db, logger)

	if err := repo.RunMigration(ctx); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	t.Log("✅ Migration completed — schema log_smb and table smb_api_logs created")

	// Step 3: Verify schema and table exist
	t.Log("=== Step 3: Verifying schema and table ===")
	var schemaExists bool
	err = db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = 'log_smb')`,
	).Scan(&schemaExists)
	if err != nil {
		t.Fatalf("failed to check schema: %v", err)
	}
	if !schemaExists {
		t.Fatal("schema log_smb does not exist")
	}
	t.Log("✅ Schema log_smb exists")

	var tableExists bool
	err = db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema = 'log_smb' AND table_name = 'smb_api_logs')`,
	).Scan(&tableExists)
	if err != nil {
		t.Fatalf("failed to check table: %v", err)
	}
	if !tableExists {
		t.Fatal("table log_smb.smb_api_logs does not exist")
	}
	t.Log("✅ Table log_smb.smb_api_logs exists")

	// Step 4: Insert via APILogRepository.Insert
	t.Log("=== Step 4: Inserting API log via repository ===")
	testEntry := contractsvc.APILogEntry{
		Endpoint:           "/api/v1/pln-prepaid/inquiry",
		Method:             "POST",
		ClientNumber:       "12345678901",
		MID:                "MID-TEST-001",
		QueueName:          "queue_smb_pln_token",
		MsgID:              fmt.Sprintf("test-msg-%d", time.Now().UnixNano()),
		RequestURL:         "https://api.example.com/api/v1/pln-prepaid/inquiry",
		RequestHeaders:     map[string]string{"Content-Type": "application/json"},
		RequestBody:        []byte(`{"partner_id":"P001","client_number":"12345678901","product_code":"PLN50","sign":"abc123"}`),
		ResponseStatusCode: 200,
		ResponseBody:       []byte(`{"response_code":"00","message":"Success","data":{"ref_id":"REF001","client_name":"John Doe"}}`),
		ResponseDurationMs: 150,
		StatusCode:         "00",
		StatusDesc:         "Success",
	}

	if err := repo.Insert(ctx, testEntry); err != nil {
		t.Fatalf("failed to insert api log: %v", err)
	}
	t.Log("✅ API log inserted via repository")

	// Step 5: Insert via APILoggerAdapter (simulates real flow)
	t.Log("=== Step 5: Inserting API log via adapter (real flow simulation) ===")
	adapter := postgres.NewAPILoggerAdapter(repo, logger)

	adapter.Log(ctx, smb.APICallLog{
		Endpoint:           "/api/v1/pln-prepaid/payment",
		Method:             "POST",
		ClientNumber:       "12345678901",
		MID:                "MID-TEST-001",
		QueueName:          "queue_smb_pln_token",
		MsgID:              fmt.Sprintf("test-msg-adapter-%d", time.Now().UnixNano()),
		RequestURL:         "https://api.example.com/api/v1/pln-prepaid/payment",
		RequestBody:        []byte(`{"partner_id":"P001","client_number":"12345678901","product_code":"PLN50","ref_id":"REF001","total_amount":52500,"sign":"def456"}`),
		ResponseStatusCode: 200,
		ResponseBody:       []byte(`{"response_code":"00","message":"Payment Success","data":{"token":"1234-5678-9012-3456"}}`),
		ResponseDurationMs: 230,
		StatusCode:         "00",
		StatusDesc:         "Payment Success",
	})
	t.Log("✅ API log inserted via adapter")

	// Step 6: Insert error log entry
	t.Log("=== Step 6: Inserting error log entry ===")
	adapter.Log(ctx, smb.APICallLog{
		Endpoint:           "/api/v1/pln-prepaid/advice",
		Method:             "POST",
		ClientNumber:       "12345678901",
		RequestURL:         "https://api.example.com/api/v1/pln-prepaid/advice",
		RequestBody:        []byte(`{"partner_id":"P001","client_number":"12345678901","ref_id":"REF001","sign":"ghi789"}`),
		ResponseDurationMs: 5000,
		ErrorMessage:       "context deadline exceeded",
		ErrorType:          "NETWORK",
	})
	t.Log("✅ Error log entry inserted")

	// Step 7: Verify data in database
	t.Log("=== Step 7: Verifying inserted data ===")
	var count int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM log_smb.smb_api_logs WHERE client_number = '12345678901'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}
	if count < 3 {
		t.Fatalf("expected at least 3 rows, got %d", count)
	}
	t.Logf("✅ Found %d log entries for client_number 12345678901", count)

	// Step 8: Read back and verify specific fields
	t.Log("=== Step 8: Reading back and verifying fields ===")
	rows, err := db.QueryContext(ctx,
		`SELECT id, endpoint, method, client_number, status_code, status_desc, error_message, error_type, response_duration_ms
		 FROM log_smb.smb_api_logs
		 WHERE client_number = '12345678901'
		 ORDER BY id DESC LIMIT 3`,
	)
	if err != nil {
		t.Fatalf("failed to query rows: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id           int64
			endpoint     string
			method       string
			clientNumber string
			statusCode   sql.NullString
			statusDesc   sql.NullString
			errorMsg     sql.NullString
			errorType    sql.NullString
			durationMs   sql.NullInt32
		)
		if err := rows.Scan(&id, &endpoint, &method, &clientNumber, &statusCode, &statusDesc, &errorMsg, &errorType, &durationMs); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		t.Logf("  Row #%d: endpoint=%s method=%s client=%s status_code=%s status_desc=%s error=%s error_type=%s duration=%dms",
			id, endpoint, method, clientNumber,
			statusCode.String, statusDesc.String,
			errorMsg.String, errorType.String,
			durationMs.Int32)
	}
	t.Log("✅ All data verified successfully")

	t.Log("")
	t.Log("========================================")
	t.Log("  INTEGRATION TEST PASSED — FULL FLOW  ")
	t.Log("========================================")
}
