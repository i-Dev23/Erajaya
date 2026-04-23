package telkomsel

import "context"

// ErrorMappingResolver adalah interface untuk lookup RC PPS.
type ErrorMappingResolver interface {
	GetResponseCode(ctx context.Context, httpStatusCode int, esbStatusCode string) (int, error)
	InsertIfNotExists(ctx context.Context, httpStatusCode int, esbStatusCode string) error
}

// errorMappingResolver adalah package-level variable yang di-set saat startup.
var errorMappingResolver ErrorMappingResolver

// SetErrorMappingResolver menyuntikkan resolver saat aplikasi startup.
func SetErrorMappingResolver(r ErrorMappingResolver) {
	errorMappingResolver = r
}

// ResolveRCPPS menentukan RC PPS dari HTTP Status Code dan ESB Status Code.
// Mengembalikan 9 jika resolver belum di-set, terjadi error, atau mapping tidak ditemukan.
// Jika mapping tidak ditemukan (rc=9), otomatis insert row baru dengan rc_pps NULL.
func ResolveRCPPS(ctx context.Context, httpStatusCode int, esbStatusCode string) int {
	if errorMappingResolver == nil {
		return 9
	}

	rc, err := errorMappingResolver.GetResponseCode(ctx, httpStatusCode, esbStatusCode)
	if err != nil {
		return 9
	}

	// rc 9 means not found or NULL — auto-insert for future review.
	if rc == 9 && esbStatusCode != "" {
		_ = errorMappingResolver.InsertIfNotExists(ctx, httpStatusCode, esbStatusCode)
	}

	return rc
}
