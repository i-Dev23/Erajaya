package testmocks

import (
	"context"
	"pps-services-tokopedia/internal/domain"
)

type MockUltimaService struct {
	CheckIdPlnUltimaFunc func(ctx context.Context, req *domain.UltimaCheckIdPlnRequestDomain) (*domain.UltimaBaseResponseDomain, *domain.PLNTransactionInquiry, error)
	PingFunc             func(ctx context.Context) error
}

func (m *MockUltimaService) CheckIdPlnUltima(ctx context.Context, req *domain.UltimaCheckIdPlnRequestDomain) (*domain.UltimaBaseResponseDomain, *domain.PLNTransactionInquiry, error) {
	if m.CheckIdPlnUltimaFunc != nil {
		return m.CheckIdPlnUltimaFunc(ctx, req)
	}
	return nil, nil, nil
}
func (m *MockUltimaService) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil
}

// Add methods as needed for tests
// Example:
// func (m *MockUltimaService) SomeMethod(...) {...}
