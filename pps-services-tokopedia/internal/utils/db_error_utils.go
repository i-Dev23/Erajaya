package utils

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// IsConnectionError checks if an error is related to database/service connection issues
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()

	// Check for PostgreSQL connection errors
	if errors.Is(err, pgx.ErrNoRows) == false && // pgx.ErrNoRows bukan connection error
		(strings.Contains(errMsg, "connection refused") ||
			strings.Contains(errMsg, "cannot connect") ||
			strings.Contains(errMsg, "connection reset") ||
			strings.Contains(errMsg, "connection timeout") ||
			strings.Contains(errMsg, "server closed the connection") ||
			strings.Contains(errMsg, "unexpected EOF") ||
			strings.Contains(errMsg, "dial") ||
			strings.Contains(errMsg, "certificate") ||
			strings.Contains(errMsg, "too many connections") ||
			strings.Contains(errMsg, "broken pipe")) {
		return true
	}

	// Check for Redis connection errors
	if strings.Contains(errMsg, "CLUSTERDOWN") ||
		strings.Contains(errMsg, "connection pool exhausted") ||
		strings.Contains(errMsg, "redis: client is closed") ||
		strings.Contains(errMsg, "i/o timeout") ||
		strings.Contains(errMsg, "network error") ||
		errors.Is(err, redis.Nil) == false && // redis.Nil bukan connection error
			(strings.Contains(errMsg, "redis") && strings.Contains(errMsg, "connect")) {
		return true
	}

	// Check for Oracle connection errors
	if strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "invalid username") ||
		strings.Contains(errMsg, "invalid password") ||
		strings.Contains(errMsg, "not available") {
		return true
	}

	// Check for RabbitMQ connection errors
	if strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "connection reset") ||
		strings.Contains(errMsg, "error waiting for frames") ||
		strings.Contains(errMsg, "unexpected connection closure") ||
		strings.Contains(errMsg, "closed channel") ||
		strings.Contains(errMsg, "failed to connect") {
		return true
	}

	return false
}

// GetDatabaseErrorResponseCode returns response code 62 (Server Error) for database/service connection issues
// along with an appropriate error message
func GetDatabaseErrorResponseCode() (ResponseCode, error) {
	responseCode, found := GetResponseCode("62")
	if !found {
		// Fallback to a generic server error response if code 62 is not found in mapping
		return ResponseCode{
			Code:        "62",
			Message:     "Server error",
			Description: "Internal error on Partner's server",
			Behavior:    "Failed and Retry",
			Expected:    "Pending",
		}, nil
	}
	return responseCode, nil
}

// WrapDatabaseError wraps a database error with context for proper error handling
// This function is used to propagate database connection errors up the stack
// so they can be caught by the DatabaseErrorHandlingMiddleware
type DatabaseError struct {
	Err           error
	ServiceName   string
	OperationType string
}

// Error implements the error interface for DatabaseError
func (de *DatabaseError) Error() string {
	return de.Err.Error()
}

// Unwrap returns the underlying error
func (de *DatabaseError) Unwrap() error {
	return de.Err
}

// NewDatabaseError creates a new DatabaseError with context
func NewDatabaseError(err error, serviceName, operationType string) error {
	if err == nil {
		return nil
	}

	if IsConnectionError(err) {
		return &DatabaseError{
			Err:           err,
			ServiceName:   serviceName,
			OperationType: operationType,
		}
	}

	return err
}
