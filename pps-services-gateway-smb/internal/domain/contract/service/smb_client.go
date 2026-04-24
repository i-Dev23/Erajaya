package service

import "context"

type PLNTokenInquiryRequest struct {
	PartnerID    string
	ClientNumber string
	ProductCode  string
	MsgID        string
}

type PLNTokenInquiryResponse struct {
	ResponseCode string
	Message      string
	ClientNumber string
	ClientName   string
	TarifDaya    string
	AdminFee     float64
	TotalAmount  float64
	RefID        string
	RawResponse  []byte
}

type PLNTokenPaymentRequest struct {
	PartnerID    string
	ClientNumber string
	ProductCode  string
	RefID        string
	TotalAmount  float64
	MsgID        string
}

type PLNTokenPaymentResponse struct {
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

type PLNTokenAdviceRequest struct {
	PartnerID    string
	ClientNumber string
	RefID        string
	MsgID        string
}

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

type SMBClient interface {
	InquiryPLNToken(ctx context.Context, req PLNTokenInquiryRequest) (*PLNTokenInquiryResponse, error)
	PaymentPLNToken(ctx context.Context, req PLNTokenPaymentRequest) (*PLNTokenPaymentResponse, error)
	AdvicePLNToken(ctx context.Context, req PLNTokenAdviceRequest) (*PLNTokenAdviceResponse, error)
}
