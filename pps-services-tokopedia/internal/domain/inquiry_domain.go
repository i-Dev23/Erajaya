package domain

import "context"

type InquiryRequestDomain struct {
	RefID               string                // Unique reference identifier from Tokopedia
	ClientNumber        string                // Client bill identifier / MSISDN number. Mandatory for postpaid product, not mandatory for prepaid product
	Category            string                // Product category identifier provided by Tokopedia
	Rsid                string                // Tokopedia channel identifier (default will be filled with TOKOPEDIA)
	ProductCode         string                // Code of product given by Partner
	Timestamp           string                // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	AdditionalParameter []AdditionalParameter // List of additional parameters needed
}

type InquiryResponseDomain struct {
	PartnerInquiryID   string                    // Unique inquiry ID from partner
	ClientNumber       string                    // Client bill identifier / MSISDN
	ProductCode        string                    // Code of product given by Partner
	ResponseCode       string                    // Response code as specified (see Response Codes)
	Message            string                    // Additional informational or error message
	DueDate            string                    // Due date for given bill in Indonesia date, format: YYYY-MM-DD
	BillGenerationDate string                    // Bill generation date for given bill in Indonesia date, format: YYYY-MM-DD
	IsOpenAmount       bool                      // Defines if the bill can be paid with open amount method
	AdminFee           float64                   // Partner admin fee that already included on total_amount
	TotalAmount        float64                   // Total bill amount or product price including partner admin_fee
	Timestamp          string                    // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	BillCount          int                       // Number of bills
	BillDetails        []InquiryBillDetail       // Details of a given bill
	AdditionalDetails  []InquiryAdditionalDetail // Additional details of a given bill
}

type InquiryAdditionalDetail struct {
	Label   string              // Additional label detail
	Details []InquiryBillDetail // Additional detail values
}

type InquiryBillDetail struct {
	Name   string // Bill detail name
	Value  string // Bill detail value
	IsPII  bool   // Defines if given detail contains PII value
	IsShow bool   // Defines if given detail needs to be shown to the user
}

type AdditionalParameter struct {
	Name  string // Additional parameter name
	Value string // Additional parameter value
}

// InquiryUsecase defines the interface for inquiry business logic
// * pointer because we don't want to pass by value, we want to pass by reference
type InquiryUsecase interface {
	Inquiry(ctx context.Context, req *InquiryRequestDomain) (*InquiryResponseDomain, error)
}
