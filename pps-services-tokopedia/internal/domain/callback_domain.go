package domain

import "context"

type CallbackRequestDomain struct {
	RefID        string             `json:"ref_id" validate:"required"`                                 // Unique reference identifier from Tokopedia (same as payment ref_id)
	PartnerRefID string             `json:"partner_ref_id" validate:"required"`                         // Unique reference identifier from Partner
	ClientNumber string             `json:"client_number" validate:"required"`                          // Client bill identifier / MSISDN number
	ProductCode  string             `json:"product_code" validate:"required"`                           // Code of product given by Partner
	ResponseCode string             `json:"response_code" validate:"required"`                          // Response code as specified (see 10. Response Codes)
	Message      string             `json:"message" validate:"required"`                                // Additional informational or error message
	AdminFee     float64            `json:"admin_fee,omitempty"`                                        // Partner admin fee that already included on total_amount
	TotalAmount  float64            `json:"total_amount" validate:"required"`                           // Total bill amount or product price
	Timestamp    string             `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"` // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	BillCount    int                `json:"bill_count" validate:"required"`                             // Number of bills
	BillDetails  []BillDetailDomain `json:"bill_details,omitempty"`
}

type CallbackUsecase interface {
	CallbackToTokopedia(ctx context.Context, req *CallbackRequestDomain) error
	ListenCallbackQueue(ctx context.Context) error
}
