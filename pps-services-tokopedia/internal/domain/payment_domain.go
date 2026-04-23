package domain

import "context"

const FLAG_SUCCESS_PUBLISH string = "10"
const FLAG_FAILED_PUBLISH string = "-1"
const PRE_ORDER_TYPE string = "PRE_ORDER"

type PaymentRequestDomain struct {
	RefID               string                // Unique reference identifier from Tokopedia
	PartnerInquiryID    string                // Unique inquiry ID from previous inquiry request
	ClientNumber        string                // Client bill identifier / MSISDN number
	Category            string                // Product category identifier provided by Tokopedia
	Rsid                string                // Tokopedia channel identifier (default will be filled with TOKOPEDIA)
	ProductCode         string                // Code of product given by Partner
	TotalAmount         float64               // Total bill amount or product price
	Timestamp           string                // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	ClientIP            string                // Client IP address from X-Real-IP header
	AdditionalParameter []AdditionalParameter // List of additional parameters needed
}
type PaymentResponseDomain struct {
	RefID             string                    // Unique reference identifier from Tokopedia
	PartnerRefID      string                    // Unique reference identifier from Partner
	ClientNumber      string                    // Client bill identifier / MSISDN number
	ProductCode       string                    // Code of product given by Partner
	ResponseCode      string                    // Response code as specified (see Response Codes)
	Message           string                    // Additional informational or error message
	AdminFee          float64                   // Partner admin fee that already included on total_amount
	TotalAmount       float64                   // Total bill amount or product price
	Timestamp         string                    // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	BillCount         int                       // Number of bills
	BillDetails       []InquiryBillDetail       // Details of a given bill
	AdditionalDetails []InquiryAdditionalDetail // Additional details of a given bill
}

// PaymentUsecase defines the interface for payment business logic
// * pointer because we don't want to pass by value, we want to pass by reference
type PaymentUsecase interface {
	Payment(ctx context.Context, req *PaymentRequestDomain) (*PaymentResponseDomain, error)
}
