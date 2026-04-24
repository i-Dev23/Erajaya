package smb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

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

	start := time.Now()
	resp, err := c.client.Do(req)
	durationMs := int(time.Since(start).Milliseconds())
	if err != nil {
		c.logAPICall(ctx, "/api/v1/pln-prepaid/inquiry", http.MethodPost, clientNumber, url, bodyBytes, nil, 0, durationMs, "", "", err.Error(), "NETWORK")
		return nil, nil, fmt.Errorf("execute inquiry request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read inquiry response: %w", err)
	}

	var inquiryResp InquiryResponse
	if err := json.Unmarshal(rawBody, &inquiryResp); err != nil {
		c.logAPICall(ctx, "/api/v1/pln-prepaid/inquiry", http.MethodPost, clientNumber, url, bodyBytes, rawBody, resp.StatusCode, durationMs, "", "", err.Error(), "PARSE")
		return nil, rawBody, fmt.Errorf("unmarshal inquiry response: %w", err)
	}

	c.logAPICall(ctx, "/api/v1/pln-prepaid/inquiry", http.MethodPost, clientNumber, url, bodyBytes, rawBody, resp.StatusCode, durationMs, inquiryResp.ResponseCode, inquiryResp.Message, "", "")

	c.logger.Info("SMB PLN Token inquiry completed",
		"client_number", clientNumber,
		"response_code", inquiryResp.ResponseCode,
		"message", inquiryResp.Message)

	return &inquiryResp, rawBody, nil
}

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

	start := time.Now()
	resp, err := c.client.Do(req)
	durationMs := int(time.Since(start).Milliseconds())
	if err != nil {
		c.logAPICall(ctx, "/api/v1/pln-prepaid/payment", http.MethodPost, clientNumber, url, bodyBytes, nil, 0, durationMs, "", "", err.Error(), "NETWORK")
		return nil, nil, fmt.Errorf("execute payment request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read payment response: %w", err)
	}

	var paymentResp PaymentResponse
	if err := json.Unmarshal(rawBody, &paymentResp); err != nil {
		c.logAPICall(ctx, "/api/v1/pln-prepaid/payment", http.MethodPost, clientNumber, url, bodyBytes, rawBody, resp.StatusCode, durationMs, "", "", err.Error(), "PARSE")
		return nil, rawBody, fmt.Errorf("unmarshal payment response: %w", err)
	}

	c.logAPICall(ctx, "/api/v1/pln-prepaid/payment", http.MethodPost, clientNumber, url, bodyBytes, rawBody, resp.StatusCode, durationMs, paymentResp.ResponseCode, paymentResp.Message, "", "")

	c.logger.Info("SMB PLN Token payment completed",
		"client_number", clientNumber,
		"ref_id", refID,
		"response_code", paymentResp.ResponseCode,
		"message", paymentResp.Message)

	return &paymentResp, rawBody, nil
}

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

	start := time.Now()
	resp, err := c.client.Do(req)
	durationMs := int(time.Since(start).Milliseconds())
	if err != nil {
		c.logAPICall(ctx, "/api/v1/pln-prepaid/advice", http.MethodPost, clientNumber, url, bodyBytes, nil, 0, durationMs, "", "", err.Error(), "NETWORK")
		return nil, nil, fmt.Errorf("execute advice request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read advice response: %w", err)
	}

	var adviceResp AdviceResponse
	if err := json.Unmarshal(rawBody, &adviceResp); err != nil {
		c.logAPICall(ctx, "/api/v1/pln-prepaid/advice", http.MethodPost, clientNumber, url, bodyBytes, rawBody, resp.StatusCode, durationMs, "", "", err.Error(), "PARSE")
		return nil, rawBody, fmt.Errorf("unmarshal advice response: %w", err)
	}

	c.logAPICall(ctx, "/api/v1/pln-prepaid/advice", http.MethodPost, clientNumber, url, bodyBytes, rawBody, resp.StatusCode, durationMs, adviceResp.ResponseCode, adviceResp.Message, "", "")

	c.logger.Info("SMB PLN Token advice completed",
		"client_number", clientNumber,
		"ref_id", refID,
		"response_code", adviceResp.ResponseCode,
		"message", adviceResp.Message)

	return &adviceResp, rawBody, nil
}

// logAPICall is a helper that logs API calls if an APILogger is configured.
func (c *Client) logAPICall(ctx context.Context, endpoint, method, clientNumber, requestURL string, requestBody, responseBody []byte, responseStatusCode, durationMs int, statusCode, statusDesc, errorMessage, errorType string) {
	if c.apiLogger == nil {
		return
	}
	c.apiLogger.Log(ctx, APICallLog{
		Endpoint:           endpoint,
		Method:             method,
		ClientNumber:       clientNumber,
		RequestURL:         requestURL,
		RequestBody:        requestBody,
		ResponseStatusCode: responseStatusCode,
		ResponseBody:       responseBody,
		ResponseDurationMs: durationMs,
		StatusCode:         statusCode,
		StatusDesc:         statusDesc,
		ErrorMessage:       errorMessage,
		ErrorType:          errorType,
	})
}
