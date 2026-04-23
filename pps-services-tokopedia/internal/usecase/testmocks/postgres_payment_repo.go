package testmocks

import (
	"context"
	"pps-services-tokopedia/internal/domain"
)

type MockPostgresPaymentRepository struct {
	InsertPaymentBillDetailFunc     func(ctx context.Context, req *domain.PaymentBillDetailInsertRequest) (*domain.PaymentBillDetailInsertResponse, error)
	InsertPaymentRequestFunc        func(ctx context.Context, req *domain.PaymentRequestInsertRequest) (*domain.PaymentRequestInsertResponse, error)
	InsertPaymentResponseFunc       func(ctx context.Context, req *domain.PaymentResponseInsertRequest) (*domain.PaymentResponseInsertResponse, error)
	CheckRefIDExistsFunc            func(ctx context.Context, refID string) (bool, error)
	CheckPartnerInquiryIDExistsFunc func(ctx context.Context, partnerInquiryID string) (bool, error)
	ValidatePartnerInquiryIDFunc    func(ctx context.Context, partnerInquiryID string) error
	GetPaymentStatusByRefIDFunc     func(ctx context.Context, refID string) (*domain.PaymentStatusResult, error)
}

func (m *MockPostgresPaymentRepository) InsertPaymentBillDetail(ctx context.Context, req *domain.PaymentBillDetailInsertRequest) (*domain.PaymentBillDetailInsertResponse, error) {
	if m.InsertPaymentBillDetailFunc != nil {
		return m.InsertPaymentBillDetailFunc(ctx, req)
	}
	return &domain.PaymentBillDetailInsertResponse{}, nil
}

func (m *MockPostgresPaymentRepository) InsertPaymentRequest(ctx context.Context, req *domain.PaymentRequestInsertRequest) (*domain.PaymentRequestInsertResponse, error) {
	if m.InsertPaymentRequestFunc != nil {
		return m.InsertPaymentRequestFunc(ctx, req)
	}
	return &domain.PaymentRequestInsertResponse{}, nil
}

func (m *MockPostgresPaymentRepository) InsertPaymentResponse(ctx context.Context, req *domain.PaymentResponseInsertRequest) (*domain.PaymentResponseInsertResponse, error) {
	if m.InsertPaymentResponseFunc != nil {
		return m.InsertPaymentResponseFunc(ctx, req)
	}
	return &domain.PaymentResponseInsertResponse{}, nil
}

func (m *MockPostgresPaymentRepository) CheckRefIDExists(ctx context.Context, refID string) (bool, error) {
	if m.CheckRefIDExistsFunc != nil {
		return m.CheckRefIDExistsFunc(ctx, refID)
	}
	return false, nil
}

func (m *MockPostgresPaymentRepository) CheckPartnerInquiryIDExists(ctx context.Context, partnerInquiryID string) (bool, error) {
	if m.CheckPartnerInquiryIDExistsFunc != nil {
		return m.CheckPartnerInquiryIDExistsFunc(ctx, partnerInquiryID)
	}
	return false, nil
}

func (m *MockPostgresPaymentRepository) ValidatePartnerInquiryID(ctx context.Context, partnerInquiryID string) error {
	if m.ValidatePartnerInquiryIDFunc != nil {
		return m.ValidatePartnerInquiryIDFunc(ctx, partnerInquiryID)
	}
	return nil
}

func (m *MockPostgresPaymentRepository) GetPaymentStatusByRefID(ctx context.Context, refID string) (*domain.PaymentStatusResult, error) {
	if m.GetPaymentStatusByRefIDFunc != nil {
		return m.GetPaymentStatusByRefIDFunc(ctx, refID)
	}
	return &domain.PaymentStatusResult{}, nil
}
