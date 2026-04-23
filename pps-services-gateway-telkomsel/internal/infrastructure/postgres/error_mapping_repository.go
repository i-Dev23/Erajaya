package postgres

import (
	"context"
	"database/sql"
	"errors"

	contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
)

// Compile-time interface compliance check.
var _ contractsvc.ErrorMappingRepository = (*ErrorMappingRepositoryImpl)(nil)

// ErrorMappingRepositoryImpl mengimplementasikan ErrorMappingRepository menggunakan PostgreSQL.
type ErrorMappingRepositoryImpl struct {
	db     *sql.DB
	logger contractsvc.Logger
}

// NewErrorMappingRepositoryImpl membuat instance baru.
func NewErrorMappingRepositoryImpl(db *sql.DB, logger contractsvc.Logger) *ErrorMappingRepositoryImpl {
	return &ErrorMappingRepositoryImpl{db: db, logger: logger}
}

const getResponseCodeSQL = `
SELECT rc_pps FROM log.telkomsel_error_mapping
WHERE http_status_code = $1 AND esb_status_code = $2`

const insertIfNotExistsSQL = `
INSERT INTO log.telkomsel_error_mapping (http_status_code, esb_status_code, rc_pps, description)
SELECT $1::INTEGER, $2::VARCHAR, NULL, 'auto-inserted: unidentified error code'
WHERE NOT EXISTS (
	SELECT 1 FROM log.telkomsel_error_mapping WHERE http_status_code = $1::INTEGER AND esb_status_code = $2::VARCHAR
)`

// GetResponseCode melakukan SELECT ke telkomsel_error_mapping.
// Mengembalikan rc_pps jika ditemukan dan tidak NULL, atau 9 jika tidak ditemukan / NULL / error.
func (r *ErrorMappingRepositoryImpl) GetResponseCode(ctx context.Context, httpStatusCode int, esbStatusCode string) (int, error) {
	var rcPPS sql.NullInt64
	err := r.db.QueryRowContext(ctx, getResponseCodeSQL, httpStatusCode, esbStatusCode).Scan(&rcPPS)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 9, nil
		}
		r.logger.Error("failed to get response code from error mapping", "httpStatusCode", httpStatusCode, "esbStatusCode", esbStatusCode, "error", err)
		return 9, nil
	}
	if !rcPPS.Valid {
		// rc_pps is NULL — treat as unidentified, return 9.
		return 9, nil
	}
	return int(rcPPS.Int64), nil
}

// InsertIfNotExists menyisipkan row baru ke error mapping jika kombinasi httpStatusCode + esbStatusCode belum ada.
// rc_pps di-set NULL agar bisa di-review manual.
func (r *ErrorMappingRepositoryImpl) InsertIfNotExists(ctx context.Context, httpStatusCode int, esbStatusCode string) error {
	_, err := r.db.ExecContext(ctx, insertIfNotExistsSQL, httpStatusCode, esbStatusCode)
	if err != nil {
		r.logger.Error("failed to insert error mapping", "httpStatusCode", httpStatusCode, "esbStatusCode", esbStatusCode, "error", err)
		return err
	}
	return nil
}
