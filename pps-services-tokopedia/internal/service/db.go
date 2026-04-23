package service

import (
	"context"
	"database/sql"
)

// Database interface defines database operations
type Database interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Close() error
	PingContext(ctx context.Context) error
}

// DatabaseService manages database connections
type DatabaseService struct {
	db Database
}

// NewDatabaseService creates a new database service
func NewDatabaseService(db Database) *DatabaseService {
	return &DatabaseService{db: db}
}

// GetDB returns the database instance
func (d *DatabaseService) GetDB() Database {
	return d.db
}
