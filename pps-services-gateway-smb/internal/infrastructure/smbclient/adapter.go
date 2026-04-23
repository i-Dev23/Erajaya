package smbclient

import (
	"context"
	"encoding/json"
	"fmt"

	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
	"pps-services-gateway-smb/pkg/smb"
)

// Compile-time interface compliance check.
var _ contractsvc.SMBClient = (*Adapter)(nil)

// Adapter mengadaptasi smb.Client ke contractsvc.SMBClient.
type Adapter struct {
	client *smb.Client
	logger contractsvc.Logger
}

// NewAdapter membuat instance baru Adapter.
func NewAdapter(client *smb.Client, logger contractsvc.Logger) *Adapter {
	return &Adapter{client: client, logger: logger}
}

// InquiryPLNToken melakukan inquiry PLN Token ke SMB API.
func (a *Adapter) InquiryPLNToken(ctx context.Context, req contractsvc.PLNTokenInquiryRequest) (*contractsvc.PLNTokenInquiryResponse, error) {
	resp, rawBody, err := a.client.InquiryPLNToken(ctx, req.ClientNumber, req.ProductCode)
	if err != nil {
		return nil, fmt.Errorf("smb inquiry pln token: %w", err)
	}

	result := &contractsvc.PLNTokenInquiryResponse{
		ResponseCode: resp.ResponseCode,
		Message:      resp.Message,
		ClientNumber: req.ClientNumber,
		RawResponse:  rawBody,
	}

	if resp.Data != nil {
		result.ClientName = resp.Data.ClientName
		result.TarifDaya = resp.Data.TarifDaya
		result.AdminFee = resp.Data.AdminFee
		result.TotalAmount = resp.Data.TotalAmount
		result.RefID = resp.Data.RefID
		result.ClientNumber = resp.Data.ClientNumber
	}

	return result, nil
}

// PaymentPLNToken melakukan payment PLN Token ke SMB API.
func (a *Adapter) PaymentPLNToken(ctx context.Context, req contractsvc.PLNTokenPaymentRequest) (*contractsvc.PLNTokenPaymentResponse, error) {
	resp, rawBody, err := a.client.PaymentPLNToken(ctx, req.ClientNumber, req.ProductCode, req.RefID, req.TotalAmount)
	if err != nil {
		return nil, fmt.Errorf("smb payment pln token: %w", err)
	}

	result := &contractsvc.PLNTokenPaymentResponse{
		ResponseCode: resp.ResponseCode,
		Message:      resp.Message,
		ClientNumber: req.ClientNumber,
		RawResponse:  rawBody,
	}

	if resp.Data != nil {
		result.ClientName = resp.Data.ClientName
		result.Token = resp.Data.Token
		result.SerialNumber = resp.Data.SerialNumber
		result.RefID = resp.Data.RefID
		result.TotalAmount = resp.Data.TotalAmount
		result.AdminFee = resp.Data.AdminFee
	}

	return result, nil
}

// AdvicePLNToken melakukan advice/check status PLN Token ke SMB API.
func (a *Adapter) AdvicePLNToken(ctx context.Context, req contractsvc.PLNTokenAdviceRequest) (*contractsvc.PLNTokenAdviceResponse, error) {
	resp, rawBody, err := a.client.AdvicePLNToken(ctx, req.ClientNumber, req.RefID)
	if err != nil {
		return nil, fmt.Errorf("smb advice pln token: %w", err)
	}

	result := &contractsvc.PLNTokenAdviceResponse{
		ResponseCode: resp.ResponseCode,
		Message:      resp.Message,
		ClientNumber: req.ClientNumber,
		RawResponse:  rawBody,
	}

	if resp.Data != nil {
		result.ClientName = resp.Data.ClientName
		result.Token = resp.Data.Token
		result.SerialNumber = resp.Data.SerialNumber
		result.RefID = resp.Data.RefID
		result.TotalAmount = resp.Data.TotalAmount
		result.AdminFee = resp.Data.AdminFee
	}

	return result, nil
}

// toJSON is a helper to marshal struct to json.RawMessage.
func toJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
