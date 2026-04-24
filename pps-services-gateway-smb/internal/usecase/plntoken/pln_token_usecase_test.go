package plntoken

import (
	"context"
	"fmt"
	"testing"
	"time"

	"pps-services-gateway-smb/internal/config"
	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
)

// ═══════════════════════════════════════════════════════════════
// Mock SMB Client
// ═══════════════════════════════════════════════════════════════

type mockSMBClient struct {
	inquiryFunc func(ctx context.Context, req contractsvc.PLNTokenInquiryRequest) (*contractsvc.PLNTokenInquiryResponse, error)
	paymentFunc func(ctx context.Context, req contractsvc.PLNTokenPaymentRequest) (*contractsvc.PLNTokenPaymentResponse, error)
	adviceFunc  func(ctx context.Context, req contractsvc.PLNTokenAdviceRequest) (*contractsvc.PLNTokenAdviceResponse, error)
}

func (m *mockSMBClient) InquiryPLNToken(ctx context.Context, req contractsvc.PLNTokenInquiryRequest) (*contractsvc.PLNTokenInquiryResponse, error) {
	return m.inquiryFunc(ctx, req)
}
func (m *mockSMBClient) PaymentPLNToken(ctx context.Context, req contractsvc.PLNTokenPaymentRequest) (*contractsvc.PLNTokenPaymentResponse, error) {
	return m.paymentFunc(ctx, req)
}
func (m *mockSMBClient) AdvicePLNToken(ctx context.Context, req contractsvc.PLNTokenAdviceRequest) (*contractsvc.PLNTokenAdviceResponse, error) {
	return m.adviceFunc(ctx, req)
}

// ═══════════════════════════════════════════════════════════════
// Mock Logger (no-op)
// ═══════════════════════════════════════════════════════════════

type mockLogger struct{}

func (m *mockLogger) Info(msg string, kv ...any)  {}
func (m *mockLogger) Warn(msg string, kv ...any)  {}
func (m *mockLogger) Error(msg string, kv ...any) {}

// ═══════════════════════════════════════════════════════════════
// Tests: ProcessTransaction
// ═══════════════════════════════════════════════════════════════

func TestProcessTransaction_InquirySuccess_PaymentSuccess(t *testing.T) {
	client := &mockSMBClient{
		inquiryFunc: func(ctx context.Context, req contractsvc.PLNTokenInquiryRequest) (*contractsvc.PLNTokenInquiryResponse, error) {
			return &contractsvc.PLNTokenInquiryResponse{
				ResponseCode: "00",
				RefID:        "REF-001",
				TotalAmount:  52500,
				ClientName:   "BUDI",
			}, nil
		},
		paymentFunc: func(ctx context.Context, req contractsvc.PLNTokenPaymentRequest) (*contractsvc.PLNTokenPaymentResponse, error) {
			return &contractsvc.PLNTokenPaymentResponse{
				ResponseCode: "00",
				Token:        "1234-5678-9012",
				TotalAmount:  52500,
			}, nil
		},
	}

	uc := NewUsecase(client, nil, &mockLogger{})
	result, err := uc.ProcessTransaction(context.Background(), "12345678901", "PLN50", "MSG001", 50000)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
	if result.StatusToBe != "F" {
		t.Errorf("expected F, got %s", result.StatusToBe)
	}
	if result.Token != "1234-5678-9012" {
		t.Errorf("expected token 1234-5678-9012, got %s", result.Token)
	}
}

func TestProcessTransaction_InquiryError(t *testing.T) {
	client := &mockSMBClient{
		inquiryFunc: func(ctx context.Context, req contractsvc.PLNTokenInquiryRequest) (*contractsvc.PLNTokenInquiryResponse, error) {
			return nil, fmt.Errorf("connection timeout")
		},
	}

	uc := NewUsecase(client, nil, &mockLogger{})
	result, _ := uc.ProcessTransaction(context.Background(), "12345678901", "PLN50", "MSG002", 50000)

	if result.Status != "FAILED" {
		t.Errorf("expected FAILED, got %s", result.Status)
	}
	if result.StatusToBe != "C" {
		t.Errorf("expected C, got %s", result.StatusToBe)
	}
}

func TestProcessTransaction_InquiryNonSuccess(t *testing.T) {
	client := &mockSMBClient{
		inquiryFunc: func(ctx context.Context, req contractsvc.PLNTokenInquiryRequest) (*contractsvc.PLNTokenInquiryResponse, error) {
			return &contractsvc.PLNTokenInquiryResponse{
				ResponseCode: "94",
				Message:      "Error Inquiry Data",
			}, nil
		},
	}

	uc := NewUsecase(client, nil, &mockLogger{})
	result, _ := uc.ProcessTransaction(context.Background(), "12345678901", "PLN50", "MSG003", 50000)

	if result.Status != "FAILED" {
		t.Errorf("expected FAILED, got %s", result.Status)
	}
	if result.StatusToBe != "C" {
		t.Errorf("expected C, got %s", result.StatusToBe)
	}
}

func TestProcessTransaction_PaymentFailed(t *testing.T) {
	client := &mockSMBClient{
		inquiryFunc: func(ctx context.Context, req contractsvc.PLNTokenInquiryRequest) (*contractsvc.PLNTokenInquiryResponse, error) {
			return &contractsvc.PLNTokenInquiryResponse{
				ResponseCode: "00",
				RefID:        "REF-002",
				TotalAmount:  52500,
			}, nil
		},
		paymentFunc: func(ctx context.Context, req contractsvc.PLNTokenPaymentRequest) (*contractsvc.PLNTokenPaymentResponse, error) {
			return &contractsvc.PLNTokenPaymentResponse{
				ResponseCode: "93",
				Message:      "Error Payment",
			}, nil
		},
	}

	uc := NewUsecase(client, nil, &mockLogger{})
	result, _ := uc.ProcessTransaction(context.Background(), "12345678901", "PLN50", "MSG004", 50000)

	if result.Status != "FAILED" {
		t.Errorf("expected FAILED, got %s", result.Status)
	}
	if result.StatusToBe != "C" {
		t.Errorf("expected C, got %s", result.StatusToBe)
	}
}

func TestProcessTransaction_PaymentPending(t *testing.T) {
	client := &mockSMBClient{
		inquiryFunc: func(ctx context.Context, req contractsvc.PLNTokenInquiryRequest) (*contractsvc.PLNTokenInquiryResponse, error) {
			return &contractsvc.PLNTokenInquiryResponse{
				ResponseCode: "00",
				RefID:        "REF-003",
				TotalAmount:  52500,
			}, nil
		},
		paymentFunc: func(ctx context.Context, req contractsvc.PLNTokenPaymentRequest) (*contractsvc.PLNTokenPaymentResponse, error) {
			return &contractsvc.PLNTokenPaymentResponse{
				ResponseCode: "28",
				Message:      "Timeout atau Pending Transaksi",
			}, nil
		},
	}

	uc := NewUsecase(client, nil, &mockLogger{})
	result, _ := uc.ProcessTransaction(context.Background(), "12345678901", "PLN50", "MSG005", 50000)

	if result.Status != "PENDING" {
		t.Errorf("expected PENDING, got %s", result.Status)
	}
	if !result.NeedRetry {
		t.Error("expected NeedRetry=true")
	}
	if result.RefID != "REF-003" {
		t.Errorf("expected RefID REF-003, got %s", result.RefID)
	}
}

func TestProcessTransaction_PaymentNetworkError(t *testing.T) {
	client := &mockSMBClient{
		inquiryFunc: func(ctx context.Context, req contractsvc.PLNTokenInquiryRequest) (*contractsvc.PLNTokenInquiryResponse, error) {
			return &contractsvc.PLNTokenInquiryResponse{
				ResponseCode: "00",
				RefID:        "REF-004",
				TotalAmount:  52500,
			}, nil
		},
		paymentFunc: func(ctx context.Context, req contractsvc.PLNTokenPaymentRequest) (*contractsvc.PLNTokenPaymentResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	uc := NewUsecase(client, nil, &mockLogger{})
	result, _ := uc.ProcessTransaction(context.Background(), "12345678901", "PLN50", "MSG006", 50000)

	if result.Status != "PENDING" {
		t.Errorf("expected PENDING, got %s", result.Status)
	}
	if !result.NeedRetry {
		t.Error("expected NeedRetry=true")
	}
}

// ═══════════════════════════════════════════════════════════════
// Tests: RetryAdvice
// ═══════════════════════════════════════════════════════════════

func TestRetryAdvice_SuccessOnFirstAttempt(t *testing.T) {
	client := &mockSMBClient{
		adviceFunc: func(ctx context.Context, req contractsvc.PLNTokenAdviceRequest) (*contractsvc.PLNTokenAdviceResponse, error) {
			return &contractsvc.PLNTokenAdviceResponse{
				ResponseCode: "00",
				Token:        "9999-8888-7777",
				TotalAmount:  52500,
			}, nil
		},
	}

	retryCfg := &config.RetryConfig{MaxAttempts: 3, WaitDuration: 10 * time.Millisecond}
	uc := NewUsecase(client, retryCfg, &mockLogger{})
	result := uc.RetryAdvice(context.Background(), "12345678901", "REF-001", "MSG010", 50000)

	if result.Status != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
	if result.Token != "9999-8888-7777" {
		t.Errorf("expected token 9999-8888-7777, got %s", result.Token)
	}
}

func TestRetryAdvice_FailedOnFirstAttempt(t *testing.T) {
	client := &mockSMBClient{
		adviceFunc: func(ctx context.Context, req contractsvc.PLNTokenAdviceRequest) (*contractsvc.PLNTokenAdviceResponse, error) {
			return &contractsvc.PLNTokenAdviceResponse{
				ResponseCode: "93",
				Message:      "Error Payment",
			}, nil
		},
	}

	retryCfg := &config.RetryConfig{MaxAttempts: 3, WaitDuration: 10 * time.Millisecond}
	uc := NewUsecase(client, retryCfg, &mockLogger{})
	result := uc.RetryAdvice(context.Background(), "12345678901", "REF-002", "MSG011", 50000)

	if result.Status != "FAILED" {
		t.Errorf("expected FAILED, got %s", result.Status)
	}
	if result.StatusToBe != "C" {
		t.Errorf("expected C, got %s", result.StatusToBe)
	}
}

func TestRetryAdvice_SuccessOnThirdAttempt(t *testing.T) {
	attempt := 0
	client := &mockSMBClient{
		adviceFunc: func(ctx context.Context, req contractsvc.PLNTokenAdviceRequest) (*contractsvc.PLNTokenAdviceResponse, error) {
			attempt++
			if attempt < 3 {
				return &contractsvc.PLNTokenAdviceResponse{ResponseCode: "28", Message: "Pending"}, nil
			}
			return &contractsvc.PLNTokenAdviceResponse{
				ResponseCode: "00",
				Token:        "FINAL-TOKEN",
				TotalAmount:  52500,
			}, nil
		},
	}

	retryCfg := &config.RetryConfig{MaxAttempts: 4, WaitDuration: 10 * time.Millisecond}
	uc := NewUsecase(client, retryCfg, &mockLogger{})
	result := uc.RetryAdvice(context.Background(), "12345678901", "REF-003", "MSG012", 50000)

	if result.Status != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
	if attempt != 3 {
		t.Errorf("expected 3 attempts, got %d", attempt)
	}
}

func TestRetryAdvice_ExhaustedAllRetries(t *testing.T) {
	client := &mockSMBClient{
		adviceFunc: func(ctx context.Context, req contractsvc.PLNTokenAdviceRequest) (*contractsvc.PLNTokenAdviceResponse, error) {
			return &contractsvc.PLNTokenAdviceResponse{ResponseCode: "68", Message: "Pending"}, nil
		},
	}

	retryCfg := &config.RetryConfig{MaxAttempts: 2, WaitDuration: 10 * time.Millisecond}
	uc := NewUsecase(client, retryCfg, &mockLogger{})
	result := uc.RetryAdvice(context.Background(), "12345678901", "REF-004", "MSG013", 50000)

	if result.Status != "FAILED" {
		t.Errorf("expected FAILED, got %s", result.Status)
	}
	if result.StatusToBe != "C" {
		t.Errorf("expected C, got %s", result.StatusToBe)
	}
}

func TestRetryAdvice_NilRetryConfig(t *testing.T) {
	client := &mockSMBClient{}
	uc := NewUsecase(client, nil, &mockLogger{})
	result := uc.RetryAdvice(context.Background(), "12345678901", "REF-005", "MSG014", 50000)

	if result.Status != "FAILED" {
		t.Errorf("expected FAILED, got %s", result.Status)
	}
}

func TestRetryAdvice_NetworkErrorThenSuccess(t *testing.T) {
	attempt := 0
	client := &mockSMBClient{
		adviceFunc: func(ctx context.Context, req contractsvc.PLNTokenAdviceRequest) (*contractsvc.PLNTokenAdviceResponse, error) {
			attempt++
			if attempt == 1 {
				return nil, fmt.Errorf("network error")
			}
			return &contractsvc.PLNTokenAdviceResponse{
				ResponseCode: "00",
				Token:        "RECOVERED-TOKEN",
				TotalAmount:  52500,
			}, nil
		},
	}

	retryCfg := &config.RetryConfig{MaxAttempts: 3, WaitDuration: 10 * time.Millisecond}
	uc := NewUsecase(client, retryCfg, &mockLogger{})
	result := uc.RetryAdvice(context.Background(), "12345678901", "REF-006", "MSG015", 50000)

	if result.Status != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
	if result.Token != "RECOVERED-TOKEN" {
		t.Errorf("expected RECOVERED-TOKEN, got %s", result.Token)
	}
}

func TestRetryAdvice_ContextCancelled(t *testing.T) {
	client := &mockSMBClient{
		adviceFunc: func(ctx context.Context, req contractsvc.PLNTokenAdviceRequest) (*contractsvc.PLNTokenAdviceResponse, error) {
			return &contractsvc.PLNTokenAdviceResponse{ResponseCode: "28"}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	retryCfg := &config.RetryConfig{MaxAttempts: 3, WaitDuration: 10 * time.Millisecond}
	uc := NewUsecase(client, retryCfg, &mockLogger{})
	result := uc.RetryAdvice(ctx, "12345678901", "REF-007", "MSG016", 50000)

	if result.Status != "FAILED" {
		t.Errorf("expected FAILED, got %s", result.Status)
	}
}
