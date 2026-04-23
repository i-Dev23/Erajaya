package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	go_ora "github.com/sijms/go-ora/v2"

	"pps-services-gateway-unipin/internal/domain/contract/repository"
)

var _ repository.GameListRepository = (*GameListRepositoryImpl)(nil)

// GameListRepositoryImpl implements GameListRepository using Oracle stored procedure.
type GameListRepositoryImpl struct {
	db *sql.DB
}

// NewGameListRepository creates a new GameListRepositoryImpl.
func NewGameListRepository(db *sql.DB) *GameListRepositoryImpl {
	return &GameListRepositoryImpl{db: db}
}

// UpsertGameList calls MSG.PKG_UNIPIN.INSUPDGAMELIST stored procedure.
// SP params: INGAMECODE(VARCHAR2), INGAMEDESC(VARCHAR2), INGAMECATEGORY(VARCHAR2),
// INGAMEDENOMINATIONID(NUMBER), INGAMENAME(VARCHAR2), INGAMECURRENCY(VARCHAR2),
// INGAMEAMOUNT(NUMBER), INGAMEFIELDREQUEST(CLOB), INPROVIDER(VARCHAR2),
// OUTERRCODE(NUMBER OUT), OUTERRMSG(VARCHAR2 OUT)
func (r *GameListRepositoryImpl) UpsertGameList(ctx context.Context, row *repository.GameListRow) (string, string, error) {
	var errCode int
	var errMsgStr string
	amount, _ := strconv.ParseFloat(row.Amount, 64)

	_, err := r.db.ExecContext(ctx,
		"BEGIN MSG.PKG_UNIPIN.INSUPDGAMELIST(:1, :2, :3, :4, :5, :6, :7, :8, :9, :10, :11); END;",
		row.GameCode,
		row.GameDesc,
		row.GameCategory,
		row.DenominationID,
		row.GameName,
		row.Currency,
		amount,
		go_ora.Clob{String: row.FieldRequest, Valid: true},
		row.Provider,
		go_ora.Out{Dest: &errCode},
		go_ora.Out{Dest: &errMsgStr, Size: 4000},
	)
	if err != nil {
		return "", "", fmt.Errorf("exec INSUPDGAMELIST: %w", err)
	}

	return fmt.Sprintf("%d", errCode), errMsgStr, nil
}
