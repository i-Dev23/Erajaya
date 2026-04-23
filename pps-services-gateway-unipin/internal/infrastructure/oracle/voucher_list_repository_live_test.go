package oracle_test

import (
	"context"
	"os"
	"testing"

	"pps-services-gateway-unipin/internal/domain/contract/repository"
	"pps-services-gateway-unipin/internal/infrastructure/oracle"
)

// TestUpsertVoucherListLive calls the real Oracle stored procedure.
// Run with: go test -v -run TestUpsertVoucherListLive ./internal/infrastructure/oracle/
func TestUpsertVoucherListLive(t *testing.T) {
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

	repo := oracle.NewVoucherListRepository(db)

	row := &repository.VoucherListRow{
		VoucherCode:             "POD_GSID",
		VoucherDesc:             "Garena ID",
		VoucherCategory:         "",
		VoucherDenominationCode: "POD_GSID001",
		VoucherName:             "Garena 33 Shells",
		Currency:                "IDR",
		Amount:                  "10000.00",
		Provider:                "unipin-voucher",
	}

	ctx := context.Background()
	errCode, errMsg, err := repo.UpsertVoucherList(ctx, row)
	if err != nil {
		t.Fatalf("UpsertVoucherList failed: %v", err)
	}

	t.Logf("ErrCode: %s", errCode)
	t.Logf("ErrMsg: %s", errMsg)
}
