package testmocks

import (
	"context"
	"pps-services-tokopedia/internal/domain"
)

type MockInquiryRepository struct {
	SaveFunc                      func(ctx context.Context, req *domain.InquiryRequestDomain) error
	GetByIDFunc                   func(ctx context.Context, id int64) (*domain.InquiryRequestDomain, error)
	CheckRefIDExistsFunc          func(ctx context.Context, refID string) (bool, error)
	GetBillDetailsByInquiryIDFunc func(ctx context.Context, ppsInquiryID string) ([]domain.InquiryBillDetail, error)
	GetInquiryByPartnerInquiryIDFunc func(ctx context.Context, partnerInquiryID string) (*domain.InquiryData, error)
	InsertBillDetailFunc          func(ctx context.Context, req *domain.BillDetailInsertRequest) (*domain.BillDetailInsertResponse, error)
	InsertInquiryRequestFunc      func(ctx context.Context, req *domain.InquiryRequestInsertRequest) (*domain.InquiryRequestInsertResponse, error)
	InsertInquiryResponseFunc     func(ctx context.Context, req *domain.InquiryResponseInsertRequest) (*domain.InquiryResponseInsertResponse, error)
	ValidateInquiryIdFunc         func(ctx context.Context, inquiryRequestId string, productCode string, clientNumber string) error
}

func (m *MockInquiryRepository) Save(ctx context.Context, req *domain.InquiryRequestDomain) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, req)
	}
	return nil
}
func (m *MockInquiryRepository) GetByID(ctx context.Context, id int64) (*domain.InquiryRequestDomain, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}
func (m *MockInquiryRepository) CheckRefIDExists(ctx context.Context, refID string) (bool, error) {
	if m.CheckRefIDExistsFunc != nil {
		return m.CheckRefIDExistsFunc(ctx, refID)
	}
	return false, nil
}
func (m *MockInquiryRepository) GetBillDetailsByInquiryID(ctx context.Context, ppsInquiryID string) ([]domain.InquiryBillDetail, error) {
	if m.GetBillDetailsByInquiryIDFunc != nil {
		return m.GetBillDetailsByInquiryIDFunc(ctx, ppsInquiryID)
	}
	return nil, nil
}
func (m *MockInquiryRepository) GetInquiryByPartnerInquiryID(ctx context.Context, partnerInquiryID string) (*domain.InquiryData, error) {
	if m.GetInquiryByPartnerInquiryIDFunc != nil {
		return m.GetInquiryByPartnerInquiryIDFunc(ctx, partnerInquiryID)
	}
	return nil, nil
}
func (m *MockInquiryRepository) InsertBillDetail(ctx context.Context, req *domain.BillDetailInsertRequest) (*domain.BillDetailInsertResponse, error) {
	if m.InsertBillDetailFunc != nil {
		return m.InsertBillDetailFunc(ctx, req)
	}
	return nil, nil
}
func (m *MockInquiryRepository) InsertInquiryRequest(ctx context.Context, req *domain.InquiryRequestInsertRequest) (*domain.InquiryRequestInsertResponse, error) {
	if m.InsertInquiryRequestFunc != nil {
		return m.InsertInquiryRequestFunc(ctx, req)
	}
	return nil, nil
}
func (m *MockInquiryRepository) InsertInquiryResponse(ctx context.Context, req *domain.InquiryResponseInsertRequest) (*domain.InquiryResponseInsertResponse, error) {
	if m.InsertInquiryResponseFunc != nil {
		return m.InsertInquiryResponseFunc(ctx, req)
	}
	return nil, nil
}
func (m *MockInquiryRepository) ValidateInquiryId(ctx context.Context, inquiryRequestId string, productCode string, clientNumber string) error {
	if m.ValidateInquiryIdFunc != nil {
		return m.ValidateInquiryIdFunc(ctx, inquiryRequestId, productCode, clientNumber)
	}
	return nil
}

// Add methods as needed for tests
// Example:
// func (m *MockInquiryRepository) SomeMethod(...) {...}
