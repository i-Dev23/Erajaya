package service

import "context"

// PLNTokenInquiryRequest adalah request untuk inquiry PLN Token ke SMB API.
type PLNTokenInquiryRequest struct {
	PartnerID    string
	ClientNumber string // Nomor meter PLN
	ProductCode  string
	MsgID        string
}

// PLNTokenInquiryResponse adalah response dari inquiry PLN Token SMB API.
type PLNTokenInquiryResponse struct {
	ResponseCode string
	Message      string
	ClientNumber string
	ClientName   string
	TarifDaya    string
	AdminFee     float64
	TotalAmount  float64
	RefID        string // Reference ID dari SMB
	RawResponse  []byte
}

// PLNTokenPaymentRequest adalah request untuk payment PLN Token ke SMB API.
type PLNTokenPaymentRequest struct {
	PartnerID    string
	ClientNumber string
	ProductCode  string
	RefID        string // Reference ID dari inquiry
	TotalAmount  float64
	MsgID        string
}

// PLNTokenPaymentResponse adalah response dari payment PLN Token SMB API.
type PLNTokenPaymentResponse struct {
	ResponseCode string
	Message      string
	ClientNumber string
	ClientName   string
	Token        string // Token PLN yang didapat
	SerialNumber string
	RefID        string
	TotalAmount  float64
	AdminFee     float64
	RawResponse  []byte
}

// PLNTokenAdviceRequest adalah request untuk advice PLN Token ke SMB API.
type PLNTokenAdviceRequest struct {
	PartnerID    string
	ClientNumber string
	RefID        string
	MsgID        string
}

// PLNTokenAdviceResponse adalah response dari advice PLN Token SMB API.
type PLNTokenAdviceResponse struct {
	ResponseCode string
	Message      string
	ClientNumber string
	ClientName   string
	Token        string
	SerialNumber string
	RefID        string
	TotalAmount  float64
	AdminFee     float64
	RawResponse  []byte
}

// SMBClient mendefinisikan kontrak untuk komunikasi dengan SMB/Loket Bayar API.
type SMBClient interface {
	// InquiryPLNToken melakukan inquiry PLN Token ke SMB API.
	InquiryPLNToken(ctx context.Context, req PLNTokenInquiryRequest) (*PLNTokenInquiryResponse, error)

	// PaymentPLNToken melakukan payment PLN Token ke SMB API.
	PaymentPLNToken(ctx context.Context, req PLNTokenPaymentRequest) (*PLNTokenPaymentResponse, error)

	// AdvicePLNToken melakukan advice/check status PLN Token ke SMB API.
	AdvicePLNToken(ctx context.Context, req PLNTokenAdviceRequest) (*PLNTokenAdviceResponse, error)
}
