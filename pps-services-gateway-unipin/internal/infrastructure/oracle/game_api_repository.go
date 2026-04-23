package oracle

import (
	"context"
	"database/sql"
	"fmt"

	go_ora "github.com/sijms/go-ora/v2"

	"pps-services-gateway-unipin/internal/domain/contract/repository"
)

var _ repository.GameAPIRepository = (*GameAPIRepositoryImpl)(nil)

// GameAPIRepositoryImpl implements GameAPIRepository using Oracle.
type GameAPIRepositoryImpl struct {
	db *sql.DB
}

// NewGameAPIRepository creates a new GameAPIRepositoryImpl.
func NewGameAPIRepository(db *sql.DB) *GameAPIRepositoryImpl {
	return &GameAPIRepositoryImpl{db: db}
}

// ValidateSignature calls MSG.PKG_UNIPIN.validateSignatureGameList.
func (r *GameAPIRepositoryImpl) ValidateSignature(ctx context.Context, user, signature string) (int, string, error) {
	var outError int
	var outMessage string

	_, err := r.db.ExecContext(ctx,
		"BEGIN MSG.PKG_UNIPIN.validateSignatureGameList(:1, :2, :3, :4); END;",
		user,
		signature,
		go_ora.Out{Dest: &outError},
		go_ora.Out{Dest: &outMessage, Size: 4000},
	)
	if err != nil {
		return 0, "", fmt.Errorf("exec validateSignatureGameList: %w", err)
	}

	return outError, outMessage, nil
}

// GetGameList queries Oracle for game product list.
// TODO: Replace placeholder SP/function name with actual name when available.
// Returns one row per field — caller must compose into grouped product array.
func (r *GameAPIRepositoryImpl) GetGameList(ctx context.Context) ([]repository.GameProductRow, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT product_code, product_desc, fields FROM TABLE(MSG.PKG_UNIPIN.getGameList())")
	if err != nil {
		return nil, fmt.Errorf("query getGameList: %w", err)
	}
	defer rows.Close()

	var results []repository.GameProductRow
	for rows.Next() {
		var row repository.GameProductRow
		if err := rows.Scan(&row.ProductCode, &row.ProductDesc, &row.Fields); err != nil {
			return nil, fmt.Errorf("scan game product row: %w", err)
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate game product rows: %w", err)
	}

	return results, nil
}
