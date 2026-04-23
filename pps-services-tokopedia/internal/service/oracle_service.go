package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	_ "github.com/sijms/go-ora/v2"
)

// OracleService defines the interface for Oracle DB operations.
type OracleService interface {
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
	Ping(ctx context.Context) error
	Close() error
}

// oracleService implements OracleService with connection pooling using go-ora driver.
type oracleService struct {
	db   *sql.DB
	once sync.Once
}

// NewOracleService creates a new OracleService with connection pooling using go-ora driver.
// It reads configuration from environment variables:
//
//	ORACLE_DSN, ORACLE_MAX_OPEN_CONNS, ORACLE_MAX_IDLE_CONNS, ORACLE_CONN_MAX_LIFETIME (seconds)
//	Example DSN: "oracle://user:pass@host:port/sid"
func NewOracleService() (OracleService, error) {
	dsn := os.Getenv("ORACLE_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("ORACLE_DSN environment variable is required")
	}

	maxOpenConns := 10
	if val := os.Getenv("ORACLE_MAX_OPEN_CONNS"); val != "" {
		fmt.Sscanf(val, "%d", &maxOpenConns)
	}

	maxIdleConns := 5
	if val := os.Getenv("ORACLE_MAX_IDLE_CONNS"); val != "" {
		fmt.Sscanf(val, "%d", &maxIdleConns)
	}

	connMaxLifetime := 30 * time.Minute
	if val := os.Getenv("ORACLE_CONN_MAX_LIFETIME"); val != "" {
		var secs int
		if _, err := fmt.Sscanf(val, "%d", &secs); err == nil {
			connMaxLifetime = time.Duration(secs) * time.Second
		}
	}

	// Use go-ora driver name: "oracle"
	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open oracle connection: %w", err)
	}

	// Set connection pool parameters
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)

	// Test connection, but don't fail in development
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		// In development, return a mock Oracle service
		return &mockOracleService{}, nil
	}

	return &oracleService{db: db}, nil
}

func (o *oracleService) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return o.db.QueryContext(ctx, query, args...)
}

func (o *oracleService) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return o.db.ExecContext(ctx, query, args...)
}

func (o *oracleService) Ping(ctx context.Context) error {
	return o.db.PingContext(ctx)
}

func (o *oracleService) Close() error {
	var err error
	o.once.Do(func() {
		err = o.db.Close()
	})
	return err
}

// mockOracleService is a mock implementation for development when Oracle is not available
type mockOracleService struct{}

func (m *mockOracleService) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, fmt.Errorf("Oracle not available - using mock")
}

func (m *mockOracleService) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return nil, fmt.Errorf("Oracle not available - using mock")
}

func (m *mockOracleService) Ping(ctx context.Context) error {
	return fmt.Errorf("Oracle not available - using mock")
}

func (m *mockOracleService) Close() error {
	return nil
}
