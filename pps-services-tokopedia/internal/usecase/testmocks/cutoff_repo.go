package testmocks

import (
	"context"
	"pps-services-tokopedia/internal/domain"
)

type MockCutOffRepository struct {
	CutOffFunc func(ctx context.Context, flag string) (*domain.CutOffResponseDomain, error)
}

func (m *MockCutOffRepository) CutOff(ctx context.Context, flag string) (*domain.CutOffResponseDomain, error) {
	if m.CutOffFunc != nil {
		return m.CutOffFunc(ctx, flag)
	}
	return &domain.CutOffResponseDomain{OutErrCode: "0", OutMsgErr: "OK"}, nil
}

// Add methods as needed for tests
// Example:
// func (m *MockCutOffRepository) SomeMethod(...) {...}
