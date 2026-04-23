package service

import "context"

// ErrorMappingRepository mendefinisikan kontrak untuk akses data error mapping.
type ErrorMappingRepository interface {
	// GetResponseCode mengembalikan nilai RC PPS berdasarkan kombinasi httpStatusCode dan esbStatusCode.
	// Mengembalikan 9 jika mapping tidak ditemukan atau rc_pps NULL.
	GetResponseCode(ctx context.Context, httpStatusCode int, esbStatusCode string) (int, error)

	// InsertIfNotExists menyisipkan row baru ke error mapping jika kombinasi httpStatusCode + esbStatusCode belum ada.
	// rc_pps di-set NULL agar bisa di-review manual.
	InsertIfNotExists(ctx context.Context, httpStatusCode int, esbStatusCode string) error
}
