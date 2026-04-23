package telkomsel

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// StockTypeFixed indicates FIXED stock type.
	StockTypeFixed = "FIXED"
	// StockTypeBulk indicates BULK stock type.
	StockTypeBulk = "BULK"
)

// InitiateRegularRechargeRequest is the request body for Initiate Regular Recharge API.
//
// Spec (required by this project):
// {"transaction": {"transaction_id": "...", "channel": "..."}, "service": {...}, "recharge": {...}, "merchant_profile": {...}}
type InitiateRegularRechargeRequest struct {
	Transaction     InitiateRegularRechargeTransaction     `json:"transaction"`
	Service         InitiateRegularRechargeService         `json:"service"`
	Recharge        InitiateRegularRechargeRecharge        `json:"recharge"`
	MerchantProfile InitiateRegularRechargeMerchantProfile `json:"merchant_profile"`
}

// InitiateRegularRechargeTransaction represents transaction block in the request.
type InitiateRegularRechargeTransaction struct {
	TransactionID string `json:"transaction_id"`
	Channel       string `json:"channel"`
}

// InitiateRegularRechargeService represents service block in the request.
type InitiateRegularRechargeService struct {
	OrganizationCode string `json:"organization_code"`
	ServiceID        string `json:"service_id"`
}

// InitiateRegularRechargeRecharge represents recharge details.
type InitiateRegularRechargeRecharge struct {
	Amount    int    `json:"amount"`
	StockType string `json:"stock_type"`
	Element1  string `json:"element1"`
}

// InitiateRegularRechargeMerchantProfile represents merchant profile details.
type InitiateRegularRechargeMerchantProfile struct {
	MerchantSignature  *string `json:"merchant_signature,omitempty"`
	ThirdPartyID       string  `json:"third_party_id"`
	ThirdPartyPassword string  `json:"third_party_password"`
	FundSource         *string `json:"fund_source,omitempty"`
	Address            *string `json:"address,omitempty"`
	Postcode           *string `json:"postcode,omitempty"`
	District           *string `json:"district,omitempty"`
	StoreID            *string `json:"store_id,omitempty"`
	City               *string `json:"city,omitempty"`
	Coordinate         *string `json:"coordinate,omitempty"`
	DeliveryChannel    string  `json:"delivery_channel"`
	TransmissionDate   *string `json:"transmission_date,omitempty"`
	CATI               *string `json:"cati,omitempty"`
	CAI                *string `json:"cai,omitempty"`
	CAN                *string `json:"can,omitempty"`
	Field1             *string `json:"field1,omitempty"`
	Field2             *string `json:"field2,omitempty"`
	Field3             *string `json:"field3,omitempty"`
	Field4             *string `json:"field4,omitempty"`
	Field5             *string `json:"field5,omitempty"`
}

// InitiateRegularRechargeResponse is the response body for Initiate Regular Recharge API.
type InitiateRegularRechargeResponse struct {
	Transaction    InitiateRegularRechargeResponseTransaction `json:"transaction"`
	SerialNumber   string                                     `json:"serial_number"`
	HTTPStatusCode int                                        `json:"-"` // HTTP status code from upstream response (not serialized)
}

// InitiateRegularRechargeResponseTransaction represents transaction block in the response.
type InitiateRegularRechargeResponseTransaction struct {
	TransactionID string `json:"transaction_id"`
	Channel       string `json:"channel"`
	StatusCode    string `json:"status_code"`
	StatusDesc    string `json:"status_desc"`
}

// BrowseOfferRequest represents query parameters for Browse Offer API.
//
// Example:
// /esb/v1/modern/offer/dealer?transaction_id=...&channel=...&organization_code=...&service_id=...&product_id=...&version=v2
type BrowseOfferRequest struct {
	TransactionID    string
	Channel          string
	OrganizationCode string
	ServiceID        string
	ProductID        string
	Version          string
}

// OrderDealerRequest is the request body for Order Dealer API.
type OrderDealerRequest struct {
	Transaction     OrderDealerTransaction     `json:"transaction"`
	Service         OrderDealerService         `json:"service"`
	Order           OrderDealerOrder           `json:"order"`
	MerchantProfile OrderDealerMerchantProfile `json:"merchant_profile"`
}

// CheckOrderStatusRequest represents query parameters for Check Order Status API.
type CheckOrderStatusRequest struct {
	TransactionID         string
	OriginalTransactionID string
	SerialNumber          string
	ServiceID             string
	Channel               string
}

// CheckOrderStatusResponse is the response body for Check Order Status API.
type CheckOrderStatusResponse struct {
	Transaction       CheckOrderStatusResponseTransaction `json:"transaction"`
	TransactionStatus *CheckOrderStatusTransactionStatus  `json:"transaction_status,omitempty"`
	HTTPStatusCode    int                                 `json:"-"` // HTTP status code from upstream response (not serialized)
}

// CheckOrderStatusResponseTransaction represents transaction block in the check order status response.
type CheckOrderStatusResponseTransaction struct {
	TransactionID string `json:"transaction_id"`
	Channel       string `json:"channel"`
	StatusCode    string `json:"status_code"`
	StatusDesc    string `json:"status_desc"`
}

// CheckOrderStatusTransactionStatus represents transaction_status block in the check order status response.
type CheckOrderStatusTransactionStatus struct {
	OriginalTransactionID string `json:"original_transaction_id"`
	SerialNumber          string `json:"serial_number"`
	Status                string `json:"status"`
}

// OrderDealerTransaction represents transaction block in the order dealer request.
type OrderDealerTransaction struct {
	TransactionID string `json:"transaction_id"`
	Channel       string `json:"channel"`
}

// OrderDealerService represents service block in the order dealer request.
type OrderDealerService struct {
	OrganizationCode string `json:"organization_code"`
	ServiceID        string `json:"service_id"`
}

// OrderDealerOrder represents order details in the order dealer request.
type OrderDealerOrder struct {
	ChannelSLA       string `json:"channel_sla"`
	ChannelTimestamp string `json:"channel_timestamp"`
	ProductID        string `json:"product_id"`
	StockType        string `json:"stock_type"`
	Element1         string `json:"element1"`
	CallbackURL      string `json:"callback_url"`
}

// OrderDealerMerchantProfile represents merchant profile in the order dealer request.
type OrderDealerMerchantProfile struct {
	MerchantSignature  string `json:"merchant_signature"`
	ThirdPartyID       string `json:"third_party_id"`
	ThirdPartyPassword string `json:"third_party_password"`
	FundSource         string `json:"fund_source"`
	Address            string `json:"address"`
	Postcode           string `json:"postcode"`
	District           string `json:"district"`
	StoreID            string `json:"store_id"`
	City               string `json:"city"`
	Coordinate         string `json:"coordinate"`
	DeliveryChannel    string `json:"delivery_channel"`
	TransmissionDate   string `json:"transmission_date"`
}

// OrderDealerResponse is the response body for Order Dealer API.
type OrderDealerResponse struct {
	Transaction     OrderDealerResponseTransaction     `json:"transaction"`
	Service         OrderDealerResponseService         `json:"service"`
	Order           OrderDealerResponseOrder           `json:"order"`
	MerchantProfile OrderDealerResponseMerchantProfile `json:"merchant_profile"`
	HTTPStatusCode  int                                `json:"-"` // HTTP status code from upstream response (not serialized)
}

// OrderDealerResponseTransaction represents transaction block in the order dealer response.
type OrderDealerResponseTransaction struct {
	TransactionID string `json:"transaction_id"`
	Channel       string `json:"channel"`
	StatusCode    string `json:"status_code"`
	StatusDesc    string `json:"status_desc"`
}

// OrderDealerResponseService represents service block in the order dealer response.
type OrderDealerResponseService struct {
	OrganizationCode string `json:"organization_code"`
	ServiceID        string `json:"service_id"`
}

// OrderDealerResponseOrder represents order block in the order dealer response.
type OrderDealerResponseOrder struct {
	ProductID string `json:"product_id"`
	StockType string `json:"stock_type"`
	Element1  string `json:"element1"`
	APFlag    string `json:"ap_flag"`
}

// OrderDealerResponseMerchantProfile represents merchant profile in the order dealer response.
type OrderDealerResponseMerchantProfile struct {
	MerchantSignature  string `json:"merchant_signature"`
	PartnerMID         string `json:"partner_mid"`
	ThirdPartyID       string `json:"third_party_id"`
	ThirdPartyPassword string `json:"third_party_password"`
	FundSource         string `json:"fund_source"`
	Address            string `json:"address"`
	Postcode           string `json:"postcode"`
	District           string `json:"district"`
	StoreID            string `json:"store_id"`
	City               string `json:"city"`
	Coordinate         string `json:"coordinate"`
	DeliveryChannel    string `json:"delivery_channel"`
	TransmissionDate   string `json:"transmission_date"`
}

// BrowseOfferResponse is the response body for Browse Offer API.
type BrowseOfferResponse struct {
	Transaction BrowseOfferResponseTransaction `json:"transaction"`
	Product     *BrowseOfferProduct            `json:"product,omitempty"`
}

func (r *BrowseOfferResponse) UnmarshalJSON(data []byte) error {
	var raw struct {
		Transaction BrowseOfferResponseTransaction `json:"transaction"`
		Product     json.RawMessage                `json:"product"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.Transaction = raw.Transaction
	if len(raw.Product) == 0 || string(raw.Product) == "null" {
		r.Product = nil
		return nil
	}

	switch raw.Product[0] {
	case '[':
		var products []BrowseOfferProduct
		if err := json.Unmarshal(raw.Product, &products); err != nil {
			return err
		}
		if len(products) == 0 {
			r.Product = nil
			return nil
		}
		r.Product = &products[0]
		return nil
	default:
		var product BrowseOfferProduct
		if err := json.Unmarshal(raw.Product, &product); err != nil {
			return err
		}
		r.Product = &product
		return nil
	}
}

// BrowseOfferResponseTransaction represents transaction block in the response.
type BrowseOfferResponseTransaction struct {
	TransactionID string `json:"transaction_id"`
	Channel       string `json:"channel"`
	StatusCode    string `json:"status_code"`
	StatusDesc    string `json:"status_desc"`
}

// BrowseOfferProduct represents product block in the response.
type BrowseOfferProduct struct {
	ID                   string `json:"id"`
	CommercialName       string `json:"commercial_name"`
	AllowanceDescription string `json:"allowance_description"`
	ProductLength        string `json:"product_length"`
	Price                string `json:"price"`
	StockType            string `json:"stock_type"`
}

// BusinessError represents a business-level rejection from ESB Modern Channel API.
type BusinessError struct {
	Code          string
	Message       string
	TransactionID string
}

// Error returns a readable business error message.
func (e *BusinessError) Error() string {
	return fmt.Sprintf("business error: code=%s message=%s transactionId=%s", e.Code, e.Message, e.TransactionID)
}

// TechnicalError represents technical failures such as timeout, network and 5xx response.
type TechnicalError struct {
	StatusCode int
	Cause      error
}

// RejectedError represents an upstream edge/WAF rejection that returns an HTML body
// such as "Request Rejected". This should be treated as a definitive failure and must not be retried.
type RejectedError struct {
	StatusCode int
	SupportID  string
}

func (e *RejectedError) Error() string {
	if strings.TrimSpace(e.SupportID) != "" {
		return fmt.Sprintf("request rejected (status=%d) support_id=%s", e.StatusCode, strings.TrimSpace(e.SupportID))
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("request rejected (status=%d)", e.StatusCode)
	}
	return "request rejected"
}

// Error returns a readable technical error message.
func (e *TechnicalError) Error() string {
	if e.StatusCode > 0 {
		if e.Cause != nil {
			return fmt.Sprintf("technical error: status=%d cause=%v", e.StatusCode, e.Cause)
		}
		return fmt.Sprintf("technical error: status=%d", e.StatusCode)
	}

	if e.Cause != nil {
		return fmt.Sprintf("technical error: %v", e.Cause)
	}

	return "technical error"
}

// Unwrap returns wrapped error cause for TechnicalError.
func (e *TechnicalError) Unwrap() error {
	return e.Cause
}
