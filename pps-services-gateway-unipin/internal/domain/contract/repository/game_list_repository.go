package repository

import "context"

// GameListRow represents a single row to upsert via the stored procedure.
type GameListRow struct {
	GameCode       string
	GameDesc       string
	GameCategory   string
	DenominationID int
	GameName       string
	Currency       string
	Amount         string
	FieldRequest   string
	Provider       string
}

// GameListRepository defines the interface for game list persistence.
type GameListRepository interface {
	UpsertGameList(ctx context.Context, row *GameListRow) (errCode string, errMsg string, err error)
}

// VoucherListRow represents a single row to upsert via the voucher stored procedure.
type VoucherListRow struct {
	VoucherCode             string
	VoucherDesc             string
	VoucherCategory         string
	VoucherDenominationCode string
	VoucherName             string
	Currency                string
	Amount                  string
	Provider                string
}

// VoucherListRepository defines the interface for voucher list persistence.
type VoucherListRepository interface {
	UpsertVoucherList(ctx context.Context, row *VoucherListRow) (errCode string, errMsg string, err error)
}
