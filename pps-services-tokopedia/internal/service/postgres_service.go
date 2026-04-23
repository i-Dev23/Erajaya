package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresService defines the interface for Postgres DB operations.
type PostgresService interface {
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	Ping(ctx context.Context) error
	Close()
	// IsAvailable returns true if a real Postgres connection is active (not mock).
	IsAvailable() bool
}

// postgresService implements PostgresService with connection pooling using pgx.
type postgresService struct {
	pool *pgxpool.Pool
	once sync.Once
}

// NewPostgresService creates a new PostgresService with connection pooling using pgx.
// It reads configuration from environment variables:
//   - POSTGRES_DSN
//   - POSTGRES_MAX_CONNS
//   - POSTGRES_MIN_CONNS
//   - POSTGRES_MAX_CONN_LIFETIME (seconds)
//   - POSTGRES_MOCK (Y to explicitly allow mock mode — default N)
//   - POSTGRES_RETRY_ATTEMPTS (number of connection attempts — default 5)
//   - POSTGRES_RETRY_DELAY (seconds between retries — default 3)
func NewPostgresService(logger Logger) (PostgresService, error) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("POSTGRES_DSN environment variable is required")
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres dsn: %w", err)
	}

	if val := os.Getenv("POSTGRES_MAX_CONNS"); val != "" {
		var maxConns int32
		fmt.Sscanf(val, "%d", &maxConns)
		if maxConns > 0 {
			config.MaxConns = maxConns
		}
	}

	if val := os.Getenv("POSTGRES_MIN_CONNS"); val != "" {
		var minConns int32
		fmt.Sscanf(val, "%d", &minConns)
		if minConns > 0 {
			config.MinConns = minConns
		}
	}

	if val := os.Getenv("POSTGRES_MAX_CONN_LIFETIME"); val != "" {
		var secs int
		if _, err := fmt.Sscanf(val, "%d", &secs); err == nil && secs > 0 {
			config.MaxConnLifetime = time.Duration(secs) * time.Second
		}
	}

	// Retry configuration
	retryAttempts := 5
	if val := os.Getenv("POSTGRES_RETRY_ATTEMPTS"); val != "" {
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil && n > 0 {
			retryAttempts = n
		}
	}
	retryDelay := 3 * time.Second
	if val := os.Getenv("POSTGRES_RETRY_DELAY"); val != "" {
		var secs int
		if _, err := fmt.Sscanf(val, "%d", &secs); err == nil && secs > 0 {
			retryDelay = time.Duration(secs) * time.Second
		}
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgx pool: %w", err)
	}

	// Retry ping with backoff to handle Postgres containers that are still starting
	var pingErr error
	for i := 0; i < retryAttempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pingErr = pool.Ping(ctx)
		cancel()
		if pingErr == nil {
			logger.Info("Postgres connection established", "attempt", i+1)
			return &postgresService{pool: pool}, nil
		}
		logger.Warn("Postgres ping failed, retrying...", "attempt", i+1, "maxAttempts", retryAttempts, "error", pingErr)
		if i < retryAttempts-1 {
			time.Sleep(retryDelay)
		}
	}

	// All retries exhausted — check if mock mode is explicitly allowed
	pool.Close()
	if strings.ToUpper(strings.TrimSpace(os.Getenv("POSTGRES_MOCK"))) == "Y" {
		logger.Warn("Postgres not reachable after retries — running in MOCK mode (POSTGRES_MOCK=Y)")
		return &mockPostgresService{}, nil
	}

	return nil, fmt.Errorf("failed to connect to Postgres after %d attempts: %w", retryAttempts, pingErr)
}

func (p *postgresService) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return p.pool.Query(ctx, query, args...)
}

func (p *postgresService) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return p.pool.Exec(ctx, query, args...)
}

func (p *postgresService) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *postgresService) IsAvailable() bool {
	return true
}

func (p *postgresService) Close() {
	p.once.Do(func() {
		p.pool.Close()
	})
}

// mockPostgresService is a mock implementation for development when Postgres is not available
type mockPostgresService struct{}

func (m *mockPostgresService) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return nil, errors.New("Postgres not available - using mock")
}

func (m *mockPostgresService) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), errors.New("Postgres not available - using mock")
}

func (m *mockPostgresService) Ping(ctx context.Context) error {
	return errors.New("Postgres not available - using mock")
}

func (m *mockPostgresService) IsAvailable() bool {
	return false
}

func (m *mockPostgresService) Close() {
	// No-op for mock
}
