package smb

import (
	"crypto/md5"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

// Client adalah HTTP client untuk berkomunikasi dengan SMB/Loket Bayar API.
type Client struct {
	baseURL   string
	partnerID string
	secretKey string
	timeout   time.Duration
	client    *http.Client
	logger    *slog.Logger
}

// NewClient membuat instance baru SMB Client.
func NewClient(baseURL, partnerID, secretKey string, timeout time.Duration, logger *slog.Logger) *Client {
	return &Client{
		baseURL:   baseURL,
		partnerID: partnerID,
		secretKey: secretKey,
		timeout:   timeout,
		client: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

// generateSignature menghasilkan MD5 signature: md5(partnerID + secretKey + refID).
func (c *Client) generateSignature(refID string) string {
	data := c.partnerID + c.secretKey + refID
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// --- Request/Response structs untuk PLN Token ---

// InquiryRequest adalah request body untuk inquiry PLN Token.
type InquiryRequest struct {
	PartnerID    string `json:"partner_id"`
	ClientNumber string `json:"client_number"`
	ProductCode  string `json:"product_code"`
	Sign         string `json:"sign"`
}

// InquiryResponse adalah response body dari inquiry PLN Token.
type InquiryResponse struct {
	ResponseCode string  `json:"response_code"`
	Message      string  `json:"message"`
	Data         *InquiryData `json:"data,omitempty"`
}

// InquiryData berisi detail data inquiry PLN Token.
type InquiryData struct {
	RefID        string  `json:"ref_id"`
	ClientNumber string  `json:"client_number"`
	ClientName   string  `json:"client_name"`
	TarifDaya    string  `json:"tarif_daya"`
	AdminFee     float64 `json:"admin_fee"`
	TotalAmount  float64 `json:"total_amount"`
}

// PaymentRequest adalah request body untuk payment PLN Token.
type PaymentRequest struct {
	PartnerID    string  `json:"partner_id"`
	ClientNumber string  `json:"client_number"`
	ProductCode  string  `json:"product_code"`
	RefID        string  `json:"ref_id"`
	TotalAmount  float64 `json:"total_amount"`
	Sign         string  `json:"sign"`
}

// PaymentResponse adalah response body dari payment PLN Token.
type PaymentResponse struct {
	ResponseCode string       `json:"response_code"`
	Message      string       `json:"message"`
	Data         *PaymentData `json:"data,omitempty"`
}

// PaymentData berisi detail data payment PLN Token.
type PaymentData struct {
	RefID        string  `json:"ref_id"`
	ClientNumber string  `json:"client_number"`
	ClientName   string  `json:"client_name"`
	Token        string  `json:"token"`
	SerialNumber string  `json:"serial_number"`
	TotalAmount  float64 `json:"total_amount"`
	AdminFee     float64 `json:"admin_fee"`
}

// AdviceRequest adalah request body untuk advice PLN Token.
type AdviceRequest struct {
	PartnerID    string `json:"partner_id"`
	ClientNumber string `json:"client_number"`
	RefID        string `json:"ref_id"`
	Sign         string `json:"sign"`
}

// AdviceResponse adalah response body dari advice PLN Token.
type AdviceResponse struct {
	ResponseCode string      `json:"response_code"`
	Message      string      `json:"message"`
	Data         *AdviceData `json:"data,omitempty"`
}

// AdviceData berisi detail data advice PLN Token.
type AdviceData struct {
	RefID        string  `json:"ref_id"`
	ClientNumber string  `json:"client_number"`
	ClientName   string  `json:"client_name"`
	Token        string  `json:"token"`
	SerialNumber string  `json:"serial_number"`
	TotalAmount  float64 `json:"total_amount"`
	AdminFee     float64 `json:"admin_fee"`
}
