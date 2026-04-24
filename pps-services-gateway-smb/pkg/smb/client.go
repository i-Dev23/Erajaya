package smb

import (
	"crypto/md5"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type Client struct {
	baseURL   string
	partnerID string
	secretKey string
	timeout   time.Duration
	client    *http.Client
	logger    *slog.Logger
	apiLogger APILogger
}

// SetAPILogger sets the API logger for persisting API call logs.
func (c *Client) SetAPILogger(l APILogger) {
	c.apiLogger = l
}

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

func (c *Client) generateSignature(refID string) string {
	data := c.partnerID + c.secretKey + refID
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

type InquiryRequest struct {
	PartnerID    string `json:"partner_id"`
	ClientNumber string `json:"client_number"`
	ProductCode  string `json:"product_code"`
	Sign         string `json:"sign"`
}

type InquiryResponse struct {
	ResponseCode string       `json:"response_code"`
	Message      string       `json:"message"`
	Data         *InquiryData `json:"data,omitempty"`
}

type InquiryData struct {
	RefID        string  `json:"ref_id"`
	ClientNumber string  `json:"client_number"`
	ClientName   string  `json:"client_name"`
	TarifDaya    string  `json:"tarif_daya"`
	AdminFee     float64 `json:"admin_fee"`
	TotalAmount  float64 `json:"total_amount"`
}

type PaymentRequest struct {
	PartnerID    string  `json:"partner_id"`
	ClientNumber string  `json:"client_number"`
	ProductCode  string  `json:"product_code"`
	RefID        string  `json:"ref_id"`
	TotalAmount  float64 `json:"total_amount"`
	Sign         string  `json:"sign"`
}

type PaymentResponse struct {
	ResponseCode string       `json:"response_code"`
	Message      string       `json:"message"`
	Data         *PaymentData `json:"data,omitempty"`
}

type PaymentData struct {
	RefID        string  `json:"ref_id"`
	ClientNumber string  `json:"client_number"`
	ClientName   string  `json:"client_name"`
	Token        string  `json:"token"`
	SerialNumber string  `json:"serial_number"`
	TotalAmount  float64 `json:"total_amount"`
	AdminFee     float64 `json:"admin_fee"`
}

type AdviceRequest struct {
	PartnerID    string `json:"partner_id"`
	ClientNumber string `json:"client_number"`
	RefID        string `json:"ref_id"`
	Sign         string `json:"sign"`
}

type AdviceResponse struct {
	ResponseCode string      `json:"response_code"`
	Message      string      `json:"message"`
	Data         *AdviceData `json:"data,omitempty"`
}

type AdviceData struct {
	RefID        string  `json:"ref_id"`
	ClientNumber string  `json:"client_number"`
	ClientName   string  `json:"client_name"`
	Token        string  `json:"token"`
	SerialNumber string  `json:"serial_number"`
	TotalAmount  float64 `json:"total_amount"`
	AdminFee     float64 `json:"admin_fee"`
}
