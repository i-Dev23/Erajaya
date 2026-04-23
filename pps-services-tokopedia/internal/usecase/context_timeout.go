package usecase

import (
	"context"
	"pps-services-tokopedia/internal/utils"
	"time"
)

// withUsecaseTimeout applies an optional timeout from env USECASE_TIMEOUT_SECONDS.
// If not set or <= 0, it returns the original context and a no-op cancel.
func withUsecaseTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	seconds := utils.GetEnvAsInt("USECASE_TIMEOUT_SECONDS", 0)
	if seconds <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
}
