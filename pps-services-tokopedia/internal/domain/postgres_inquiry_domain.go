package domain

import (
	"context"
)

// ValidationError represents a validation error
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError creates a new validation error
func NewValidationError(message string) error {
	return &ValidationError{Message: message}
}

// BillDetailInsertRequest represents the input parameters for bill_detail_oninsert procedure
type BillDetailInsertRequest struct {
	PpsInquiryID string `json:"pps_inquiry_id"`
	Name         string `json:"name"`
	Value        string `json:"value"`
	IsPII        bool   `json:"is_pii"`
	IsShow       bool   `json:"is_show"`
}

// BillDetailInsertResponse represents the output parameters for bill_detail_oninsert procedure
type BillDetailInsertResponse struct {
	BillDetailID int64  `json:"bill_detail_id"`
	Error        int    `json:"error"`
	Message      string `json:"message"`
}

// InquiryRequestInsertRequest represents the input parameters for inquiry_request_oninsert procedure
type InquiryRequestInsertRequest struct {
	RefID        string `json:"ref_id"`
	ClientNumber string `json:"client_number"`
	Category     string `json:"category"`
	Rsid         string `json:"rsid"`
	ProductCode  string `json:"product_code"`
	Timestamp    string `json:"timestamp"`
}

// InquiryRequestInsertResponse represents the output parameters for inquiry_request_oninsert procedure
type InquiryRequestInsertResponse struct {
	InquiryRequestID int64  `json:"inquiry_request_id"`
	Error            int    `json:"error"`
	Message          string `json:"message"`
}

// InquiryResponseInsertRequest represents the input parameters for inquiry_response_oninsert procedure
type InquiryResponseInsertRequest struct {
	InquiryRequestID int64   `json:"inquiry_request_id"`
	PpsInquiryID     string  `json:"pps_inquiry_id"`
	ClientNumber     string  `json:"client_number"`
	ProductCode      string  `json:"product_code"`
	ResponseCode     string  `json:"response_code"`
	Message          string  `json:"message"`
	TotalAmount      float64 `json:"total_amount"`
	Timestamp        string  `json:"timestamp"`
	BillCount        float64 `json:"bill_count"`
}

// InquiryResponseInsertResponse represents the output parameters for inquiry_response_oninsert procedure
type InquiryResponseInsertResponse struct {
	InquiryResponseID int64  `json:"inquiry_response_id"`
	Error             int    `json:"error"`
	Message           string `json:"message"`
}

// InquiryData represents inquiry data for validation
type InquiryData struct {
	PartnerInquiryID string  `json:"partner_inquiry_id"`
	ClientNumber     string  `json:"client_number"`
	ProductCode      string  `json:"product_code"`
	TotalAmount      float64 `json:"total_amount"`
	ResponseCode     string  `json:"response_code"`
}

// PostgresInquiryRepository defines the interface for PostgreSQL inquiry operations
type PostgresInquiryRepository interface {
	InsertBillDetail(ctx context.Context, req *BillDetailInsertRequest) (*BillDetailInsertResponse, error)
	InsertInquiryRequest(ctx context.Context, req *InquiryRequestInsertRequest) (*InquiryRequestInsertResponse, error)
	InsertInquiryResponse(ctx context.Context, req *InquiryResponseInsertRequest) (*InquiryResponseInsertResponse, error)
	ValidateInquiryId(ctx context.Context, inquiryRequestId string, productCode string, clientNumber string) error
	CheckRefIDExists(ctx context.Context, refID string) (bool, error)
	GetBillDetailsByInquiryID(ctx context.Context, ppsInquiryID string) ([]InquiryBillDetail, error)
	GetInquiryByPartnerInquiryID(ctx context.Context, partnerInquiryID string) (*InquiryData, error)
}
