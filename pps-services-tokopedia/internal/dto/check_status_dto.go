package dto

type CheckStatusRequestDto struct {
	RefID     string `json:"ref_id" validate:"required"`                                 // Unique reference identifier from Tokopedia (same as payment ref_id)
	Timestamp string `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"` // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	Category  string `json:"category" validate:"required"`                               // Product category identifier provided by Tokopedia
}

type CheckStatusResponseDto struct {
	RefID        string          `json:"ref_id" validate:"required"`                                 // Unique reference identifier from Tokopedia (same as payment ref_id)
	PartnerRefID string          `json:"partner_ref_id" validate:"required"`                         // Unique reference identifier from Partner
	ClientNumber string          `json:"client_number" validate:"required"`                          // Client bill identifier / MSISDN number
	ProductCode  string          `json:"product_code" validate:"required"`                           // Code of product given by Partner
	ResponseCode string          `json:"response_code" validate:"required"`                          // Response code as specified (see 10. Response Codes)
	Message      string          `json:"message" validate:"required"`                                // Additional informational or error message
	AdminFee     float64         `json:"admin_fee,omitempty"`                                        // Partner admin fee that already included on total_amount
	TotalAmount  float64         `json:"total_amount" validate:"required"`                           // Total bill amount or product price
	Timestamp    string          `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"` // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	BillDetails  []BillDetailDto `json:"bill_details,omitempty"`                                     // Details of a given bill
}
