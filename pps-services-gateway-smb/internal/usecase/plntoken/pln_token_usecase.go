package plntoken

import (
	"context"
	"fmt"
	"time"

	"pps-services-gateway-smb/internal/config"
	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
	"pps-services-gateway-smb/internal/util"
)

type Result struct {
	Status         string
	StatusToBe     string
	Token          string
	SerialNumber   string
	Nominal        string
	Message        string
	ConversationID string
	NeedRetry      bool
	RefID          string
}

type Usecase struct {
	smbClient   contractsvc.SMBClient
	retryConfig *config.RetryConfig
	logger      contractsvc.Logger
}

func NewUsecase(smbClient contractsvc.SMBClient, retryConfig *config.RetryConfig, logger contractsvc.Logger) *Usecase {
	return &Usecase{
		smbClient:   smbClient,
		retryConfig: retryConfig,
		logger:      logger,
	}
}

func (u *Usecase) ProcessTransaction(ctx context.Context, clientNumber, productCode, msgID string, amount int) (*Result, error) {
	ourTrxID := util.GenerateTransactionID("", msgID, time.Now())

	u.logger.Info("step 1: inquiry PLN Token",
		"msg_id", msgID,
		"client_number", clientNumber)

	inquiryResp, err := u.smbClient.InquiryPLNToken(ctx, contractsvc.PLNTokenInquiryRequest{
		ClientNumber: clientNumber,
		ProductCode:  productCode,
		MsgID:        msgID,
	})
	if err != nil {
		u.logger.Error("inquiry PLN Token failed", "error", err, "msg_id", msgID)
		return &Result{
			Status:         "FAILED",
			StatusToBe:     "C",
			Nominal:        fmt.Sprintf("%d", amount),
			Message:        "Inquiry failed: " + err.Error(),
			ConversationID: ourTrxID,
		}, nil
	}

	rcInquiry := util.ResolveRCPPS(inquiryResp.ResponseCode)
	if rcInquiry != 0 {
		u.logger.Error("inquiry PLN Token returned non-success",
			"response_code", inquiryResp.ResponseCode,
			"msg_id", msgID)
		return &Result{
			Status:         "FAILED",
			StatusToBe:     util.StatusToBeFromRC(rcInquiry),
			Nominal:        fmt.Sprintf("%d", amount),
			Message:        inquiryResp.Message,
			ConversationID: ourTrxID,
		}, nil
	}

	u.logger.Info("step 2: payment PLN Token",
		"msg_id", msgID,
		"ref_id", inquiryResp.RefID,
		"total_amount", inquiryResp.TotalAmount)

	paymentResp, err := u.smbClient.PaymentPLNToken(ctx, contractsvc.PLNTokenPaymentRequest{
		ClientNumber: clientNumber,
		ProductCode:  productCode,
		RefID:        inquiryResp.RefID,
		TotalAmount:  inquiryResp.TotalAmount,
		MsgID:        msgID,
	})
	if err != nil {
		u.logger.Error("payment PLN Token failed (network/timeout)", "error", err, "msg_id", msgID)
		return &Result{
			Status:         "PENDING",
			StatusToBe:     "S",
			Nominal:        fmt.Sprintf("%.0f", inquiryResp.TotalAmount),
			Message:        "Payment error, retrying advice: " + err.Error(),
			ConversationID: ourTrxID,
			NeedRetry:      true,
			RefID:          inquiryResp.RefID,
		}, nil
	}

	rcPayment := util.ResolveRCPPS(paymentResp.ResponseCode)

	switch rcPayment {
	case 0:
		u.logger.Info("payment PLN Token success",
			"msg_id", msgID,
			"token", paymentResp.Token)
		return &Result{
			Status:         "SUCCESS",
			StatusToBe:     "F",
			Token:          paymentResp.Token,
			SerialNumber:   paymentResp.SerialNumber,
			Nominal:        fmt.Sprintf("%.0f", paymentResp.TotalAmount),
			Message:        fmt.Sprintf("Token PLN: %s", paymentResp.Token),
			ConversationID: ourTrxID,
		}, nil

	case 1:
		u.logger.Error("payment PLN Token failed",
			"response_code", paymentResp.ResponseCode,
			"msg_id", msgID)
		return &Result{
			Status:         "FAILED",
			StatusToBe:     "C",
			Nominal:        fmt.Sprintf("%d", amount),
			Message:        paymentResp.Message,
			ConversationID: ourTrxID,
		}, nil

	case 9:
		u.logger.Warn("payment PLN Token pending, need advice retry",
			"response_code", paymentResp.ResponseCode,
			"msg_id", msgID)
		return &Result{
			Status:         "PENDING",
			StatusToBe:     "S",
			Nominal:        fmt.Sprintf("%.0f", paymentResp.TotalAmount),
			Message:        paymentResp.Message,
			ConversationID: ourTrxID,
			NeedRetry:      true,
			RefID:          inquiryResp.RefID,
		}, nil
	}

	return &Result{
		Status:         "FAILED",
		StatusToBe:     "C",
		Nominal:        fmt.Sprintf("%d", amount),
		Message:        "Unknown payment result",
		ConversationID: ourTrxID,
	}, nil
}

func (u *Usecase) RetryAdvice(ctx context.Context, clientNumber, refID, msgID string, amount int) *Result {
	ourTrxID := util.GenerateTransactionID("", msgID, time.Now())

	if u.retryConfig == nil {
		u.logger.Warn("retry config is nil, marking as FAILED", "msg_id", msgID)
		return &Result{
			Status:         "FAILED",
			StatusToBe:     "C",
			Nominal:        fmt.Sprintf("%d", amount),
			Message:        "Payment pending, advice retry exhausted",
			ConversationID: ourTrxID,
		}
	}

	for attempt := 1; attempt <= u.retryConfig.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return &Result{
				Status:         "FAILED",
				StatusToBe:     "C",
				Nominal:        fmt.Sprintf("%d", amount),
				Message:        "Context cancelled during advice retry",
				ConversationID: ourTrxID,
			}
		case <-time.After(u.retryConfig.WaitDuration):
		}

		u.logger.Info("advice retry attempt",
			"attempt", attempt,
			"max", u.retryConfig.MaxAttempts,
			"msg_id", msgID,
			"ref_id", refID)

		adviceResp, err := u.smbClient.AdvicePLNToken(ctx, contractsvc.PLNTokenAdviceRequest{
			ClientNumber: clientNumber,
			RefID:        refID,
			MsgID:        msgID,
		})
		if err != nil {
			u.logger.Error("advice PLN Token failed", "error", err, "msg_id", msgID, "attempt", attempt)
			continue
		}

		rcAdvice := util.ResolveRCPPS(adviceResp.ResponseCode)

		switch rcAdvice {
		case 0:
			u.logger.Info("advice PLN Token success", "msg_id", msgID, "token", adviceResp.Token)
			return &Result{
				Status:         "SUCCESS",
				StatusToBe:     "F",
				Token:          adviceResp.Token,
				SerialNumber:   adviceResp.SerialNumber,
				Nominal:        fmt.Sprintf("%.0f", adviceResp.TotalAmount),
				Message:        fmt.Sprintf("Token PLN: %s", adviceResp.Token),
				ConversationID: ourTrxID,
			}

		case 1:
			u.logger.Error("advice PLN Token failed",
				"response_code", adviceResp.ResponseCode,
				"msg_id", msgID)
			return &Result{
				Status:         "FAILED",
				StatusToBe:     "C",
				Nominal:        fmt.Sprintf("%d", amount),
				Message:        adviceResp.Message,
				ConversationID: ourTrxID,
			}

		case 9:
			u.logger.Warn("advice still pending",
				"attempt", attempt,
				"msg_id", msgID)
			continue
		}
	}

	u.logger.Warn("advice retry exhausted, marking as FAILED", "msg_id", msgID)
	return &Result{
		Status:         "FAILED",
		StatusToBe:     "C",
		Nominal:        fmt.Sprintf("%d", amount),
		Message:        "Payment pending, advice retry exhausted",
		ConversationID: ourTrxID,
	}
}
