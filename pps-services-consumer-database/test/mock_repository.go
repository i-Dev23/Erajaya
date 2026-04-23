package test

import (
	"context"

	"pps-services-consumer-database/internal/model"
)

// MockRepository implements usecase.Repository for testing.
type MockRepository struct {
	CallSetTransactionStatusFn     func(ctx context.Context, event *model.TransactionEvent) (*model.SPResult, error)
	CallSetTransactionStatus2Fn    func(ctx context.Context, event *model.TransactionEvent) (*model.SPResult, error)
	CallUpdPreOrderConsumeFn       func(ctx context.Context, event *model.TransactionEvent) (*model.PreOrderResult, *model.SPResult, error)
	CallRequest2JualRandomWithIDFn func(ctx context.Context, event *model.TransactionEvent) (*model.SPResult, error)
	PingOracleFn                   func(ctx context.Context) error
}

func (m *MockRepository) CallSetTransactionStatus(ctx context.Context, event *model.TransactionEvent) (*model.SPResult, error) {
	return m.CallSetTransactionStatusFn(ctx, event)
}

func (m *MockRepository) CallSetTransactionStatus2(ctx context.Context, event *model.TransactionEvent) (*model.SPResult, error) {
	return m.CallSetTransactionStatus2Fn(ctx, event)
}

func (m *MockRepository) CallUpdPreOrderConsume(ctx context.Context, event *model.TransactionEvent) (*model.PreOrderResult, *model.SPResult, error) {
	return m.CallUpdPreOrderConsumeFn(ctx, event)
}

func (m *MockRepository) CallRequest2JualRandomWithID(ctx context.Context, event *model.TransactionEvent) (*model.SPResult, error) {
	return m.CallRequest2JualRandomWithIDFn(ctx, event)
}

func (m *MockRepository) PingOracle(ctx context.Context) error {
	if m.PingOracleFn != nil {
		return m.PingOracleFn(ctx)
	}
	return nil
}

// MockDownstreamClient implements usecase.DownstreamClient for testing.
type MockDownstreamClient struct {
	SendOrderResultFn func(ctx context.Context, req *model.DownstreamRequest) error
}

func (m *MockDownstreamClient) SendOrderResult(ctx context.Context, req *model.DownstreamRequest) error {
	return m.SendOrderResultFn(ctx, req)
}
