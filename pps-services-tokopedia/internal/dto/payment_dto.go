package dto

type PaymentRequestDto struct {
	RefID               string                   `json:"ref_id" validate:"required"`                                 // Unique reference identifier from Tokopedia
	PartnerInquiryID    string                   `json:"partner_inquiry_id" validate:"required"`                     // Unique inquiry ID from previous inquiry request
	ClientNumber        string                   `json:"client_number" validate:"required"`                          // Client bill identifier / MSISDN number
	Category            string                   `json:"category" validate:"required"`                               // Product category identifier provided by Tokopedia
	Rsid                string                   `json:"rsid" validate:"required"`                                   // Tokopedia channel identifier (default will be filled with TOKOPEDIA)
	ProductCode         string                   `json:"product_code" validate:"required"`                           // Code of product given by Partner
	TotalAmount         float64                  `json:"total_amount" validate:"required"`                           // Total bill amount or product price
	Timestamp           string                   `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"` // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	AdditionalParameter []AdditionalParameterDto `json:"additional_parameter,omitempty"`                             // List of additional parameters needed
}
type PaymentResponseDto struct {
	RefID        string          `json:"ref_id" validate:"required"`                                 // Unique reference identifier from Tokopedia
	PartnerRefID string          `json:"partner_ref_id" validate:"required"`                         // Unique reference identifier from Partner
	ClientNumber string          `json:"client_number" validate:"required"`                          // Client bill identifier / MSISDN number
	ProductCode  string          `json:"product_code" validate:"required"`                           // Code of product given by Partner
	ResponseCode string          `json:"response_code" validate:"required"`                          // Response code as specified (see Response Codes)
	Message      string          `json:"message" validate:"required"`                                // Additional informational or error message
	AdminFee     float64         `json:"admin_fee,omitempty"`                                        // Partner admin fee that already included on total_amount
	TotalAmount  float64         `json:"total_amount" validate:"required"`                           // Total bill amount or product price
	Timestamp    string          `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"` // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	BillDetails  []BillDetailDto `json:"bill_details,omitempty"`                                     // Details of a given bill
}
