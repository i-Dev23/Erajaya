package postgres

import (
	"context"

	contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
)

// mockAPILogRepository implements contractsvc.APILogRepository for testing.
type mockAPILogRepository struct {
	insertErr   error
	insertCalls []contractsvc.APILogEntry
}

func (m *mockAPILogRepository) Insert(_ context.Context, entry contractsvc.APILogEntry) error {
	m.insertCalls = append(m.insertCalls, entry)
	return m.insertErr
}

// mockLogger implements contractsvc.Logger for testing.
type mockLogger struct {
	infoCalls  [][]any
	warnCalls  [][]any
	errorCalls [][]any
}

func (m *mockLogger) Info(msg string, args ...any) {
	m.infoCalls = append(m.infoCalls, append([]any{msg}, args...))
}

func (m *mockLogger) Warn(msg string, args ...any) {
	m.warnCalls = append(m.warnCalls, append([]any{msg}, args...))
}

func (m *mockLogger) Error(msg string, args ...any) {
	m.errorCalls = append(m.errorCalls, append([]any{msg}, args...))
}
