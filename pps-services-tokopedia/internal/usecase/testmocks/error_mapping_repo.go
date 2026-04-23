package testmocks

import (
	"context"
	"pps-services-tokopedia/internal/domain"
)

type MockErrorMappingRepository struct {
	GetMappingFunc func(ctx context.Context, systemType string, errorMessage string) (*domain.ErrorMessageMapping, error)
}

func (m *MockErrorMappingRepository) GetMapping(ctx context.Context, systemType string, errorMessage string) (*domain.ErrorMessageMapping, error) {
	if m.GetMappingFunc != nil {
		return m.GetMappingFunc(ctx, systemType, errorMessage)
	}
	return &domain.ErrorMessageMapping{Found: false}, nil
}
