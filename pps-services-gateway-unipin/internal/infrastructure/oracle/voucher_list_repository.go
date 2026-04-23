package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	go_ora "github.com/sijms/go-ora/v2"
	"pps-services-gateway-unipin/internal/domain/contract/repository"
)

var _ repository.VoucherListRepository = (*VoucherListRepositoryImpl)(nil)

// VoucherListRepositoryImpl implements VoucherListRepository using Oracle stored procedure.
type VoucherListRepositoryImpl struct {
	db *sql.DB
}

// NewVoucherListRepository creates a new VoucherListRepositoryImpl.
func NewVoucherListRepository(db *sql.DB) *VoucherListRepositoryImpl {
	return &VoucherListRepositoryImpl{db: db}
}

// UpsertVoucherList calls MSG.PKG_UNIPIN.INSUPDVOUCHERLIST stored procedure.
func (r *VoucherListRepositoryImpl) UpsertVoucherList(ctx context.Context, row *repository.VoucherListRow) (string, string, error) {
	var errCode int
	var errMsgStr string
	amount, _ := strconv.ParseFloat(row.Amount, 64)

	_, err := r.db.ExecContext(ctx,
		"BEGIN MSG.PKG_UNIPIN.INSUPDVOUCHERLIST(:1, :2, :3, :4, :5, :6, :7, :8, :9, :10); END;",
		row.VoucherCode,
		row.VoucherDesc,
		row.VoucherCategory,
		row.VoucherDenominationCode,
		row.VoucherName,
		row.Currency,
		amount,
		row.Provider,
		go_ora.Out{Dest: &errCode},
		go_ora.Out{Dest: &errMsgStr, Size: 4000},
	)
	if err != nil {
		return "", "", fmt.Errorf("exec INSUPDVOUCHERLIST: %w", err)
	}

	return fmt.Sprintf("%d", errCode), errMsgStr, nil
}
