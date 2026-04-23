package handler

import (
	"context"
	contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
)

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

// mockTransactionLogger implements contractsvc.TransactionLogger for testing.
type mockTransactionLogger struct {
	// GetTransactionByOurTrxID
	getByOurTrxIDResult *contractsvc.TransactionRecord
	getByOurTrxIDErr    error
	getByOurTrxIDCalls  []string

	// InsertCallbackResponse
	insertCallbackErr   error
	insertCallbackCalls []contractsvc.ResponseRecord

	// InsertTransaction
	insertTxErr   error
	insertTxCalls []contractsvc.TransactionRecord

	// UpdateTransactionStatus
	updateStatusErr   error
	updateStatusCalls []struct {
		MsgID  string
		Status string
	}

	// InsertSyncResponse
	insertSyncErr   error
	insertSyncCalls []contractsvc.ResponseRecord

	// GetResponsesByMsgID
	getResponsesResult []contractsvc.ResponseRecord
	getResponsesErr    error
	getResponsesCalls  []string
}

func (m *mockTransactionLogger) GetTransactionByOurTrxID(_ context.Context, ourTrxID string) (*contractsvc.TransactionRecord, error) {
	m.getByOurTrxIDCalls = append(m.getByOurTrxIDCalls, ourTrxID)
	return m.getByOurTrxIDResult, m.getByOurTrxIDErr
}

func (m *mockTransactionLogger) InsertCallbackResponse(_ context.Context, rec contractsvc.ResponseRecord) error {
	m.insertCallbackCalls = append(m.insertCallbackCalls, rec)
	return m.insertCallbackErr
}

func (m *mockTransactionLogger) InsertTransaction(_ context.Context, rec contractsvc.TransactionRecord) error {
	m.insertTxCalls = append(m.insertTxCalls, rec)
	return m.insertTxErr
}

func (m *mockTransactionLogger) UpdateTransactionStatus(_ context.Context, msgID string, status string) error {
	m.updateStatusCalls = append(m.updateStatusCalls, struct {
		MsgID  string
		Status string
	}{msgID, status})
	return m.updateStatusErr
}

func (m *mockTransactionLogger) InsertSyncResponse(_ context.Context, rec contractsvc.ResponseRecord) error {
	m.insertSyncCalls = append(m.insertSyncCalls, rec)
	return m.insertSyncErr
}

func (m *mockTransactionLogger) GetResponsesByMsgID(_ context.Context, msgID string) ([]contractsvc.ResponseRecord, error) {
	m.getResponsesCalls = append(m.getResponsesCalls, msgID)
	return m.getResponsesResult, m.getResponsesErr
}

func (m *mockTransactionLogger) GetTransactionStatusByMsgID(_ context.Context, msgID string) (string, error) {
	return "", nil
}

func (m *mockTransactionLogger) RunMigration(_ context.Context) error {
	return nil
}

func (m *mockTransactionLogger) Close() {}

// mockMQPublisher implements contractsvc.MQPublisher for testing.
type mockMQPublisher struct {
	publishErr   error
	publishCalls []struct {
		MQTransactionURL string
		QueueName        string
		Body             []byte
	}
}

func (m *mockMQPublisher) Publish(_ context.Context, mqTransactionURL string, queueName string, body []byte) error {
	m.publishCalls = append(m.publishCalls, struct {
		MQTransactionURL string
		QueueName        string
		Body             []byte
	}{mqTransactionURL, queueName, body})
	return m.publishErr
}
