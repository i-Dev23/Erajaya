package testmocks

import (
	"context"
	"pps-services-tokopedia/internal/domain"
)

type MockPreorderRepository struct {
	PreorderFunc             func(ctx context.Context, req *domain.PreorderRequestDomain) (*domain.PreorderResponseDomain, error)
	UpdatePreorderStatusFunc func(ctx context.Context, msgid string, status string, message string) (*domain.UpdatePreorderStatusResponseDomain, error)
}

func (m *MockPreorderRepository) Preorder(ctx context.Context, req *domain.PreorderRequestDomain) (*domain.PreorderResponseDomain, error) {
	if m.PreorderFunc != nil {
		return m.PreorderFunc(ctx, req)
	}
	return &domain.PreorderResponseDomain{}, nil
}

func (m *MockPreorderRepository) UpdatePreorderStatus(ctx context.Context, msgid string, status string, message string) (*domain.UpdatePreorderStatusResponseDomain, error) {
	if m.UpdatePreorderStatusFunc != nil {
		return m.UpdatePreorderStatusFunc(ctx, msgid, status, message)
	}
	return &domain.UpdatePreorderStatusResponseDomain{}, nil
}
