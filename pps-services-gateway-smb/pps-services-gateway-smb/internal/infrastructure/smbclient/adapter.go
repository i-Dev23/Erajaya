package smbclient

import (
	"context"
	"encoding/json"
	"fmt"

	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
	"pps-services-gateway-smb/pkg/smb"
)

var _ contractsvc.SMBClient = (*Adapter)(nil)

type Adapter struct {
	client *smb.Client
	logger contractsvc.Logger
}

func NewAdapter(client *smb.Client, logger contractsvc.Logger) *Adapter {
	return &Adapter{client: client, logger: logger}
}

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

func toJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
