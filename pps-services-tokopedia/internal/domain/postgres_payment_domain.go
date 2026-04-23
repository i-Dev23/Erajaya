package domain

import (
	"context"
)

// PaymentBillDetailInsertRequest represents the input parameters for payment_bill_detail_oninsert procedure
type PaymentBillDetailInsertRequest struct {
	PartnerRefID string `json:"partner_ref_id"`
	Name         string `json:"name"`
	Value        string `json:"value"`
	IsPII        bool   `json:"is_pii"`
	IsShow       bool   `json:"is_show"`
}

// PaymentBillDetailInsertResponse represents the output parameters for payment_bill_detail_oninsert procedure
type PaymentBillDetailInsertResponse struct {
	PaymentBillDetailID int64  `json:"payment_bill_detail_id"`
	Error               int    `json:"error"`
	Message             string `json:"message"`
}

// PaymentRequestInsertRequest represents the input parameters for payment_request_oninsert procedure
type PaymentRequestInsertRequest struct {
	RefID            string  `json:"ref_id"`
	PartnerInquiryID string  `json:"partner_inquiry_id"`
	ClientNumber     string  `json:"client_number"`
	Category         string  `json:"category"`
	Rsid             string  `json:"rsid"`
	ProductCode      string  `json:"product_code"`
	TotalAmount      float64 `json:"total_amount"`
	Timestamp        string  `json:"timestamp"`
}

// PaymentRequestInsertResponse represents the output parameters for payment_request_oninsert procedure
type PaymentRequestInsertResponse struct {
	PaymentRequestID int64  `json:"payment_request_id"`
	Error            int    `json:"error"`
	Message          string `json:"message"`
}

// PaymentResponseInsertRequest represents the input parameters for payment_response_oninsert procedure
type PaymentResponseInsertRequest struct {
	PaymentRequestID int64   `json:"payment_request_id"`
	PartnerRefID     string  `json:"partner_ref_id"`
	ClientNumber     string  `json:"client_number"`
	ProductCode      string  `json:"product_code"`
	ResponseCode     string  `json:"response_code"`
	Message          string  `json:"message"`
	AdminFee         float64 `json:"admin_fee"`
	TotalAmount      float64 `json:"total_amount"`
	Timestamp        string  `json:"timestamp"`
	BillCount        int     `json:"bill_count"`
}

// PaymentResponseInsertResponse represents the output parameters for payment_response_oninsert procedure
type PaymentResponseInsertResponse struct {
	PaymentResponseID int64  `json:"payment_response_id"`
	Error             int    `json:"error"`
	Message           string `json:"message"`
}

// PaymentStatusResult represents payment status information for check status endpoint
type PaymentStatusResult struct {
	RefID        string
	PartnerRefID string
	ClientNumber string
	ProductCode  string
	ResponseCode string
	Message      string
	AdminFee     float64
	TotalAmount  float64
	Timestamp    string
	BillCount    int
	BillDetails  []BillDetailDomain
}

// PostgresPaymentRepository defines the interface for PostgreSQL payment operations
type PostgresPaymentRepository interface {
	InsertPaymentBillDetail(ctx context.Context, req *PaymentBillDetailInsertRequest) (*PaymentBillDetailInsertResponse, error)
	InsertPaymentRequest(ctx context.Context, req *PaymentRequestInsertRequest) (*PaymentRequestInsertResponse, error)
	InsertPaymentResponse(ctx context.Context, req *PaymentResponseInsertRequest) (*PaymentResponseInsertResponse, error)
	CheckRefIDExists(ctx context.Context, refID string) (bool, error)
	CheckPartnerInquiryIDExists(ctx context.Context, partnerInquiryID string) (bool, error)
	ValidatePartnerInquiryID(ctx context.Context, partnerInquiryID string) error
	GetPaymentStatusByRefID(ctx context.Context, refID string) (*PaymentStatusResult, error)
}
