package testmocks

import (
	"context"
	"pps-services-tokopedia/internal/domain"
)

type MockProductRepository struct {
	GetPriceByUserAndProductCodeFunc   func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error)
	GetPriceByUserFunc                 func(ctx context.Context, username string, provider string) (*[]domain.ProductPriceResponseDomain, error)
	GetProductByUserFunc               func(ctx context.Context, username string) (*[]domain.ProductPriceResponseDomain, error)
	GetProductByUserAndProductCodeFunc func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error)
	GetIpByUserFunc                    func(ctx context.Context, username string) (*domain.WhitelistedIpResponseDomain, error)
	GetCutOffFunc                      func(ctx context.Context) (*domain.CutOffDataResponseDomain, error)
}

func (m *MockProductRepository) GetPriceByUser(ctx context.Context, username string, provider string) (*[]domain.ProductPriceResponseDomain, error) {
	if m.GetPriceByUserFunc != nil {
		return m.GetPriceByUserFunc(ctx, username, provider)
	}
	return nil, nil
}
func (m *MockProductRepository) GetPriceByUserAndProductCode(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
	if m.GetPriceByUserAndProductCodeFunc != nil {
		return m.GetPriceByUserAndProductCodeFunc(ctx, username, productCode)
	}
	return nil, nil
}
func (m *MockProductRepository) GetProductByUser(ctx context.Context, username string) (*[]domain.ProductPriceResponseDomain, error) {
	if m.GetProductByUserFunc != nil {
		return m.GetProductByUserFunc(ctx, username)
	}
	return nil, nil
}
func (m *MockProductRepository) GetProductByUserAndProductCode(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
	if m.GetProductByUserAndProductCodeFunc != nil {
		return m.GetProductByUserAndProductCodeFunc(ctx, username, productCode)
	}
	return nil, nil
}
func (m *MockProductRepository) GetIpByUser(ctx context.Context, username string) (*domain.WhitelistedIpResponseDomain, error) {
	if m.GetIpByUserFunc != nil {
		return m.GetIpByUserFunc(ctx, username)
	}
	return nil, nil
}
func (m *MockProductRepository) GetCutOff(ctx context.Context) (*domain.CutOffDataResponseDomain, error) {
	if m.GetCutOffFunc != nil {
		return m.GetCutOffFunc(ctx)
	}
	return nil, nil
}

// Add methods as needed for tests
// Example:
// func (m *MockProductRepository) SomeMethod(...) {...}
