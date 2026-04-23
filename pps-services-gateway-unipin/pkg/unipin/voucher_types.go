package unipin

// voucherListRequestBody is the internal request body for Voucher List API.
type voucherListRequestBody struct {
	PartnerGUID string `json:"partner_guid"`
	LogID       int64  `json:"logid"`
	Signature   string `json:"signature"`
}

// VoucherListResponse is the response body for Voucher List API.
type VoucherListResponse struct {
	VoucherList []Voucher `json:"voucher_list"`
	Status      int       `json:"status"`
	Reason      string    `json:"reason"`
	Error       *APIError `json:"error,omitempty"`
}

// Voucher represents a single voucher entry in the voucher list response.
type Voucher struct {
	VoucherCode string `json:"voucher_code"`
	VoucherName string `json:"voucher_name"`
	IconURL     string `json:"icon_url"`
}

// VoucherStockResponse is the response body for Voucher Get Stock Count API.
// Keys are voucher SKU codes, values are stock counts (int) or "API" (string).
type VoucherStockResponse map[string]any

// voucherDetailRequestBody is the internal request body for Voucher Detail API.
type voucherDetailRequestBody struct {
	VoucherCode string `json:"voucher_code"`
	PartnerGUID string `json:"partner_guid"`
	LogID       int64  `json:"logid"`
	Signature   string `json:"signature"`
}

// VoucherDetailResponse is the response body for Voucher Detail API.
type VoucherDetailResponse struct {
	VoucherName   string                `json:"voucher_name"`
	VoucherCode   string                `json:"voucher_code"`
	IconURL       string                `json:"icon_url"`
	Denominations []VoucherDenomination `json:"denominations"`
	Status        int                   `json:"status"`
	Reason        string                `json:"reason"`
	Error         *APIError             `json:"error,omitempty"`
}

// VoucherDenomination represents a denomination entry in the voucher detail response.
type VoucherDenomination struct {
	DenominationCode     string `json:"denomination_code"`
	DenominationName     string `json:"denomination_name"`
	DenominationCurrency string `json:"denomination_currency"`
	DenominationAmount   string `json:"denomination_amount"`
}

// VoucherRequestReq is the request for Voucher Request API.
type VoucherRequestReq struct {
	DenominationCode string
	Quantity         int
	ReferenceNo      string
}

// voucherRequestBody is the internal request body for Voucher Request API.
type voucherRequestBody struct {
	PartnerGUID      string `json:"partner_guid"`
	DenominationCode string `json:"denomination_code"`
	Quantity         int    `json:"quantity"`
	ReferenceNo      string `json:"reference_no"`
	Signature        string `json:"signature"`
}

// VoucherRequestResponse is the response body for Voucher Request API.
type VoucherRequestResponse struct {
	Message     string               `json:"message"`
	ReferenceNo string               `json:"reference_no"`
	Order       string               `json:"order"`
	TotalAmount int                  `json:"total_amount"`
	Currency    string               `json:"currency"`
	Items       []VoucherRequestItem `json:"items"`
	Signature   string               `json:"signature"`
	Balance     int                  `json:"balance"`
	Status      int                  `json:"status"`
	Reason      string               `json:"reason"`
	Error       *APIError            `json:"error,omitempty"`
}

// VoucherRequestItem represents a voucher item (serial/pin) in the response.
type VoucherRequestItem struct {
	Serial1 string `json:"serial_1"`
	Serial2 string `json:"serial_2"`
}

// voucherInquiryRequestBody is the internal request body for Voucher Inquiry API.
type voucherInquiryRequestBody struct {
	PartnerGUID string `json:"partner_guid"`
	ReferenceNo string `json:"reference_no"`
	Signature   string `json:"signature"`
}

// VoucherInquiryResponse is the response body for Voucher Inquiry API.
type VoucherInquiryResponse struct {
	Message     string               `json:"message"`
	ReferenceNo string               `json:"reference_no"`
	Order       string               `json:"order"`
	TotalAmount int                  `json:"total_amount"`
	Currency    string               `json:"currency"`
	Items       []VoucherRequestItem `json:"items"`
	Signature   string               `json:"signature"`
	Balance     int                  `json:"balance"`
	Status      int                  `json:"status"`
	Reason      string               `json:"reason"`
	Error       *APIError            `json:"error,omitempty"`
}
