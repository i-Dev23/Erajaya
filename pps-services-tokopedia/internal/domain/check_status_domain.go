package domain

import "context"

type CheckStatusRequestDomain struct {
	RefID     string // Unique reference identifier from Tokopedia (same as payment ref_id)
	Timestamp string // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	Category  string // Product category identifier provided by Tokopedia
}

type CheckStatusResponseDomain struct {
	RefID        string             // Unique reference identifier from Tokopedia (same as payment ref_id)
	PartnerRefID string             // Unique reference identifier from Partner
	ClientNumber string             // Client bill identifier / MSISDN number
	ProductCode  string             // Code of product given by Partner
	ResponseCode string             // Response code as specified (see 10. Response Codes)
	Message      string             // Additional informational or error message
	AdminFee     float64            // Partner admin fee that already included on total_amount
	TotalAmount  float64            // Total bill amount or product price
	Timestamp    string             // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	BillCount    int                // Number of bills
	BillDetails  []BillDetailDomain // Details of a given bill
}

type CheckStatusUsecase interface {
	CheckStatus(ctx context.Context, req *CheckStatusRequestDomain) (*CheckStatusResponseDomain, error)
}
