package service

// Logger defines the contract for structured logging operations.
type Logger interface {
	// Info logs an informational message with optional structured key-value pairs.
	Info(msg string, args ...any)
	// Warn logs a warning message with optional structured key-value pairs.
	Warn(msg string, args ...any)
	// Error logs an error message with optional structured key-value pairs.
	Error(msg string, args ...any)
}
