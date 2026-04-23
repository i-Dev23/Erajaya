package dto

type InquiryRequestDto struct {
	RefID               string                   `json:"ref_id" validate:"required"`                                 // Unique reference identifier from Tokopedia
	ClientNumber        string                   `json:"client_number,omitempty"`                                    // Client bill identifier / MSISDN number. Mandatory for postpaid product, not mandatory for prepaid product
	Category            string                   `json:"category" validate:"required"`                               // Product category identifier provided by Tokopedia
	Rsid                string                   `json:"rsid" validate:"required"`                                   // Tokopedia channel identifier (default will be filled with TOKOPEDIA)
	ProductCode         string                   `json:"product_code" validate:"required"`                           // Code of product given by Partner
	Timestamp           string                   `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"` // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	AdditionalParameter []AdditionalParameterDto `json:"additional_parameter,omitempty"`                             // List of additional parameters needed
}

type InquiryResponseDto struct {
	PartnerInquiryID   string                `json:"partner_inquiry_id" validate:"required"`                     // Unique inquiry ID from partner
	ClientNumber       string                `json:"client_number" validate:"required"`                          // Client bill identifier / MSISDN
	ProductCode        string                `json:"product_code" validate:"required"`                           // Code of product given by Partner
	ResponseCode       string                `json:"response_code" validate:"required"`                          // Response code as specified (see Response Codes)
	Message            string                `json:"message" validate:"required"`                                // Additional informational or error message
	DueDate            string                `json:"due_date,omitempty"`                                         // Due date for given bill in Indonesia date, format: YYYY-MM-DD
	BillGenerationDate string                `json:"bill_generation_date,omitempty"`                             // Bill generation date for given bill in Indonesia date, format: YYYY-MM-DD
	IsOpenAmount       bool                  `json:"is_open_amount,omitempty"`                                   // Defines if the bill can be paid with open amount method
	AdminFee           float64               `json:"admin_fee,omitempty"`                                        // Partner admin fee that already included on total_amount
	TotalAmount        float64               `json:"total_amount" validate:"required"`                           // Total bill amount or product price including partner admin_fee
	Timestamp          string                `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"` // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	BillDetails        []BillDetailDto       `json:"bill_details,omitempty"`                                     // Details of a given bill
	AdditionalDetails  []AdditionalDetailDto `json:"additional_details,omitempty"`                               // Additional details of a given bill
}
