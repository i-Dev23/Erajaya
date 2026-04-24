//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
	"pps-services-gateway-smb/internal/infrastructure/postgres"
)

func TestIntegration_FullTransactionFlow(t *testing.T) {
	dsn := getDSN()

	// Step 1: Connect
	t.Log("=== Step 1: Testing database connection ===")
	logger := &testLogger{t: t}
	txLogger, err := postgres.NewTransactionLogger(dsn, logger)
	if err != nil {
		t.Fatalf("failed to create transaction logger: %v", err)
	}
	defer txLogger.Close()
	t.Log("✅ Database connection successful")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 2: Run migration (schema transaction_smb + tables)
	t.Log("=== Step 2: Running migration for schema transaction_smb ===")
	if err := txLogger.RunMigration(ctx); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	t.Log("✅ Migration completed — schema transaction_smb, tables smb_transaction & smb_transaction_response")

	// Step 3: Verify schema and tables
	t.Log("=== Step 3: Verifying schema and tables ===")
	db := txLogger.DB()

	var schemaExists bool
	err = db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = 'transaction_smb')`,
	).Scan(&schemaExists)
	if err != nil || !schemaExists {
		t.Fatalf("schema transaction_smb does not exist")
	}
	t.Log("✅ Schema transaction_smb exists")

	for _, tbl := range []string{"smb_transaction", "smb_transaction_response"} {
		var exists bool
		err = db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema = 'transaction_smb' AND table_name = $1)`, tbl,
		).Scan(&exists)
		if err != nil || !exists {
			t.Fatalf("table transaction_smb.%s does not exist", tbl)
		}
		t.Logf("✅ Table transaction_smb.%s exists", tbl)
	}

	// Step 4: InsertTransaction (status = PROCESSING)
	t.Log("=== Step 4: InsertTransaction (PROCESSING) ===")
	msgID := fmt.Sprintf("test-trx-%d", time.Now().UnixNano())
	ourTrxID := fmt.Sprintf("TRX-SMB-%d", time.Now().UnixNano())

	rec := contractsvc.TransactionRecord{
		MsgID:         msgID,
		OurTrxID:      ourTrxID,
		ClientNumber:  "12345678901",
		MID:           "MID-SMB-001",
		ProductType:   "PLN_TOKEN",
		ProductCode:   "PLN50",
		Amount:        52500,
		QueueName:     "queue_smb_pln_token",
		MQTransaction: "amqp://localhost:5672",
	}

	if err := txLogger.InsertTransaction(ctx, rec); err != nil {
		t.Fatalf("InsertTransaction failed: %v", err)
	}
	t.Logf("✅ Transaction inserted: msg_id=%s, our_trx_id=%s", msgID, ourTrxID)

	// Step 5: GetTransactionStatusByMsgID (should be PROCESSING)
	t.Log("=== Step 5: GetTransactionStatusByMsgID ===")
	status, err := txLogger.GetTransactionStatusByMsgID(ctx, msgID)
	if err != nil {
		t.Fatalf("GetTransactionStatusByMsgID failed: %v", err)
	}
	if status != "PROCESSING" {
		t.Fatalf("expected PROCESSING, got %s", status)
	}
	t.Logf("✅ Status = %s (correct)", status)

	// Step 6: GetTransactionByOurTrxID
	t.Log("=== Step 6: GetTransactionByOurTrxID ===")
	gotRec, err := txLogger.GetTransactionByOurTrxID(ctx, ourTrxID)
	if err != nil {
		t.Fatalf("GetTransactionByOurTrxID failed: %v", err)
	}
	if gotRec.MsgID != msgID || gotRec.ClientNumber != "12345678901" || gotRec.Amount != 52500 {
		t.Fatalf("unexpected record: %+v", gotRec)
	}
	t.Logf("✅ Transaction retrieved: client=%s, amount=%d, product=%s", gotRec.ClientNumber, gotRec.Amount, gotRec.ProductCode)

	// Step 7: InsertSyncResponse (Inquiry SYNC)
	t.Log("=== Step 7: InsertSyncResponse (Inquiry) ===")
	syncResp := contractsvc.ResponseRecord{
		MsgID:             msgID,
		OurTrxID:          ourTrxID,
		SMBTrxID:          "SMB-REF-001",
		StatusCode:        "00",
		StatusDesc:        "Inquiry Success",
		RequestPayload:    json.RawMessage(`{"partner_id":"P001","client_number":"12345678901","product_code":"PLN50"}`),
		RawPayload:        json.RawMessage(`{"response_code":"00","message":"Success","data":{"ref_id":"SMB-REF-001","client_name":"John Doe"}}`),
		RequestedAt:       time.Now().Add(-200 * time.Millisecond),
		ResponseLatencyMs: 180,
	}
	if err := txLogger.InsertSyncResponse(ctx, syncResp); err != nil {
		t.Fatalf("InsertSyncResponse failed: %v", err)
	}
	t.Log("✅ Sync response (Inquiry) inserted")

	// Step 8: InsertSyncResponse (Payment SYNC)
	t.Log("=== Step 8: InsertSyncResponse (Payment) ===")
	paymentResp := contractsvc.ResponseRecord{
		MsgID:             msgID,
		OurTrxID:          ourTrxID,
		SMBTrxID:          "SMB-REF-001",
		StatusCode:        "00",
		StatusDesc:        "Payment Success",
		RequestPayload:    json.RawMessage(`{"partner_id":"P001","client_number":"12345678901","product_code":"PLN50","ref_id":"SMB-REF-001","total_amount":52500}`),
		RawPayload:        json.RawMessage(`{"response_code":"00","message":"Payment Success","data":{"token":"1234-5678-9012-3456"}}`),
		RequestedAt:       time.Now().Add(-100 * time.Millisecond),
		ResponseLatencyMs: 250,
	}
	if err := txLogger.InsertSyncResponse(ctx, paymentResp); err != nil {
		t.Fatalf("InsertSyncResponse (payment) failed: %v", err)
	}
	t.Log("✅ Sync response (Payment) inserted")

	// Step 9: UpdateTransactionStatus → SUCCESS
	t.Log("=== Step 9: UpdateTransactionStatus → SUCCESS ===")
	if err := txLogger.UpdateTransactionStatus(ctx, msgID, "SUCCESS"); err != nil {
		t.Fatalf("UpdateTransactionStatus failed: %v", err)
	}
	status, _ = txLogger.GetTransactionStatusByMsgID(ctx, msgID)
	if status != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %s", status)
	}
	t.Logf("✅ Status updated to %s", status)

	// Step 10: InsertCallbackResponse (simulates callback from SMB)
	t.Log("=== Step 10: InsertCallbackResponse ===")
	// First reset to PROCESSING for callback test
	txLogger.UpdateTransactionStatus(ctx, msgID, "PROCESSING")

	callbackResp := contractsvc.ResponseRecord{
		MsgID:             msgID,
		OurTrxID:          ourTrxID,
		SMBTrxID:          "SMB-REF-001",
		StatusCode:        "00",
		StatusDesc:        "Callback Success",
		RawPayload:        json.RawMessage(`{"response_code":"00","message":"Callback confirmed","data":{"token":"1234-5678-9012-3456"}}`),
		RequestedAt:       time.Now(),
		ResponseLatencyMs: 0,
	}
	if err := txLogger.InsertCallbackResponse(ctx, callbackResp); err != nil {
		t.Fatalf("InsertCallbackResponse failed: %v", err)
	}
	// Callback with status_code "00" should auto-update to SUCCESS
	status, _ = txLogger.GetTransactionStatusByMsgID(ctx, msgID)
	if status != "SUCCESS" {
		t.Fatalf("expected SUCCESS after callback, got %s", status)
	}
	t.Logf("✅ Callback response inserted, status auto-updated to %s", status)

	// Step 11: InsertCallbackResponse with FAILED status
	t.Log("=== Step 11: InsertCallbackResponse (FAILED) ===")
	msgID2 := fmt.Sprintf("test-trx-fail-%d", time.Now().UnixNano())
	ourTrxID2 := fmt.Sprintf("TRX-SMB-FAIL-%d", time.Now().UnixNano())
	rec2 := contractsvc.TransactionRecord{
		MsgID:        msgID2,
		OurTrxID:     ourTrxID2,
		ClientNumber: "99988877766",
		MID:          "MID-SMB-002",
		ProductType:  "PLN_TOKEN",
		ProductCode:  "PLN100",
		Amount:       105000,
		QueueName:    "queue_smb_pln_token",
	}
	txLogger.InsertTransaction(ctx, rec2)

	failedCallback := contractsvc.ResponseRecord{
		MsgID:             msgID2,
		OurTrxID:          ourTrxID2,
		SMBTrxID:          "SMB-REF-002",
		StatusCode:        "99",
		StatusDesc:        "Transaction Failed - Timeout",
		RawPayload:        json.RawMessage(`{"response_code":"99","message":"Timeout"}`),
		RequestedAt:       time.Now(),
		ResponseLatencyMs: 30000,
	}
	if err := txLogger.InsertCallbackResponse(ctx, failedCallback); err != nil {
		t.Fatalf("InsertCallbackResponse (failed) failed: %v", err)
	}
	status, _ = txLogger.GetTransactionStatusByMsgID(ctx, msgID2)
	if status != "FAILED" {
		t.Fatalf("expected FAILED after callback with status_code 99, got %s", status)
	}
	t.Logf("✅ Failed callback inserted, status auto-updated to %s", status)

	// Step 12: GetResponsesByMsgID
	t.Log("=== Step 12: GetResponsesByMsgID ===")
	responses, err := txLogger.GetResponsesByMsgID(ctx, msgID)
	if err != nil {
		t.Fatalf("GetResponsesByMsgID failed: %v", err)
	}
	if len(responses) != 3 { // 2 SYNC + 1 CALLBACK
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}
	for i, r := range responses {
		t.Logf("  Response #%d: type=%s status_code=%s desc=%s smb_trx=%s latency=%dms",
			i+1, r.ResponseType, r.StatusCode, r.StatusDesc, r.SMBTrxID, r.ResponseLatencyMs)
	}
	t.Logf("✅ Found %d responses for msg_id=%s", len(responses), msgID)

	// Step 13: Verify data directly in database
	t.Log("=== Step 13: Direct database verification ===")
	var totalTrx, totalResp int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM transaction_smb.smb_transaction WHERE msg_id IN ($1, $2)`, msgID, msgID2).Scan(&totalTrx)
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM transaction_smb.smb_transaction_response WHERE msg_id IN ($1, $2)`, msgID, msgID2).Scan(&totalResp)
	t.Logf("✅ Total transactions: %d, Total responses: %d", totalTrx, totalResp)

	// Verify timestamps
	var successAt, failedAt sql.NullTime
	db.QueryRowContext(ctx, `SELECT success_at FROM transaction_smb.smb_transaction WHERE msg_id = $1`, msgID).Scan(&successAt)
	db.QueryRowContext(ctx, `SELECT failed_at FROM transaction_smb.smb_transaction WHERE msg_id = $1`, msgID2).Scan(&failedAt)
	if !successAt.Valid {
		t.Fatal("success_at should be set for SUCCESS transaction")
	}
	if !failedAt.Valid {
		t.Fatal("failed_at should be set for FAILED transaction")
	}
	t.Logf("✅ Timestamps verified: success_at=%v, failed_at=%v", successAt.Time.Format(time.RFC3339), failedAt.Time.Format(time.RFC3339))

	// Step 14: Idempotence test
	t.Log("=== Step 14: Idempotence test (duplicate insert) ===")
	err = txLogger.InsertTransaction(ctx, rec)
	if err != nil {
		t.Fatalf("duplicate InsertTransaction should not error: %v", err)
	}
	var count int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM transaction_smb.smb_transaction WHERE msg_id = $1`, msgID).Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 row after duplicate insert, got %d", count)
	}
	t.Log("✅ Idempotence verified — duplicate insert ignored")

	t.Log("")
	t.Log("=============================================")
	t.Log("  TRANSACTION INTEGRATION TEST PASSED — ALL  ")
	t.Log("=============================================")
}
