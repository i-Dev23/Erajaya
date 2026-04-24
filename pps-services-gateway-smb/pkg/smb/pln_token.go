package smb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// InquiryPLNToken melakukan inquiry PLN Token ke SMB API.
// Endpoint: POST {baseURL}/api/v1/pln-prepaid/inquiry
func (c *Client) InquiryPLNToken(ctx context.Context, clientNumber, productCode string) (*InquiryResponse, []byte, error) {
	sign := c.generateSignature(clientNumber)

	reqBody := InquiryRequest{
		PartnerID:    c.partnerID,
		ClientNumber: clientNumber,
		ProductCode:  productCode,
		Sign:         sign,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal inquiry request: %w", err)
	}

	url := c.baseURL + "/api/v1/pln-prepaid/inquiry"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("create inquiry request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("execute inquiry request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read inquiry response: %w", err)
	}

	var inquiryResp InquiryResponse
	if err := json.Unmarshal(rawBody, &inquiryResp); err != nil {
		return nil, rawBody, fmt.Errorf("unmarshal inquiry response: %w", err)
	}

	c.logger.Info("SMB PLN Token inquiry completed",
		"client_number", clientNumber,
		"response_code", inquiryResp.ResponseCode,
		"message", inquiryResp.Message)

	return &inquiryResp, rawBody, nil
}

// PaymentPLNToken melakukan payment PLN Token ke SMB API.
// Endpoint: POST {baseURL}/api/v1/pln-prepaid/payment
func (c *Client) PaymentPLNToken(ctx context.Context, clientNumber, productCode, refID string, totalAmount float64) (*PaymentResponse, []byte, error) {
	sign := c.generateSignature(refID)

	reqBody := PaymentRequest{
		PartnerID:    c.partnerID,
		ClientNumber: clientNumber,
		ProductCode:  productCode,
		RefID:        refID,
		TotalAmount:  totalAmount,
		Sign:         sign,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal payment request: %w", err)
	}

	url := c.baseURL + "/api/v1/pln-prepaid/payment"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("create payment request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("execute payment request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read payment response: %w", err)
	}

	var paymentResp PaymentResponse
	if err := json.Unmarshal(rawBody, &paymentResp); err != nil {
		return nil, rawBody, fmt.Errorf("unmarshal payment response: %w", err)
	}

	c.logger.Info("SMB PLN Token payment completed",
		"client_number", clientNumber,
		"ref_id", refID,
		"response_code", paymentResp.ResponseCode,
		"message", paymentResp.Message)

	return &paymentResp, rawBody, nil
}

// AdvicePLNToken melakukan advice/check status PLN Token ke SMB API.
// Endpoint: POST {baseURL}/api/v1/pln-prepaid/advice
func (c *Client) AdvicePLNToken(ctx context.Context, clientNumber, refID string) (*AdviceResponse, []byte, error) {
	sign := c.generateSignature(refID)

	reqBody := AdviceRequest{
		PartnerID:    c.partnerID,
		ClientNumber: clientNumber,
		RefID:        refID,
		Sign:         sign,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal advice request: %w", err)
	}

	url := c.baseURL + "/api/v1/pln-prepaid/advice"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("create advice request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("execute advice request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read advice response: %w", err)
	}

	var adviceResp AdviceResponse
	if err := json.Unmarshal(rawBody, &adviceResp); err != nil {
		return nil, rawBody, fmt.Errorf("unmarshal advice response: %w", err)
	}

	c.logger.Info("SMB PLN Token advice completed",
		"client_number", clientNumber,
		"ref_id", refID,
		"response_code", adviceResp.ResponseCode,
		"message", adviceResp.Message)

	return &adviceResp, rawBody, nil
}
