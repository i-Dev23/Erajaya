package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/mock"
)

// MockPaymentUsecase for benchmarking
type MockPaymentUsecase struct {
	mock.Mock
}

func (m *MockPaymentUsecase) Payment(ctx context.Context, req *domain.PaymentRequestDomain) (*domain.PaymentResponseDomain, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PaymentResponseDomain), args.Error(1)
}

// BenchmarkPaymentHandler_Payment benchmarks the Payment endpoint
// Expected: 250 TPS (Transactions Per Second) in production
// This benchmark simulates high-throughput payment processing
func BenchmarkPaymentHandler_Payment(b *testing.B) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	mockUsecase := new(MockPaymentUsecase)
	mockLogger := new(MockBenchmarkLogger)

	handler := NewPaymentHandler(mockUsecase, mockLogger)

	// Mock response - typical payment success response
	mockResponse := &domain.PaymentResponseDomain{
		RefID:        "TEST-PAY-001",
		PartnerRefID: "PPS-123456789",
		ClientNumber: "1234567890",
		ProductCode:  "PLNPRE",
		ResponseCode: "00",
		Message:      "Success",
		AdminFee:     2500,
		TotalAmount:  52500,
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		BillCount:    1,
		BillDetails: []domain.InquiryBillDetail{
			{
				Name:   "Bill Number",
				Value:  "1",
				IsPII:  false,
				IsShow: true,
			},
			{
				Name:   "Bill Name",
				Value:  "PLN Prepaid",
				IsPII:  false,
				IsShow: true,
			},
		},
	}

	mockUsecase.On("Payment", mock.Anything, mock.Anything).Return(mockResponse, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()

	// Prepare request payload
	reqDto := dto.PaymentRequestDto{
		RefID:            "TEST-PAY-001",
		PartnerInquiryID: "INQ-123456789",
		ClientNumber:     "1234567890",
		Category:         "PLN",
		Rsid:             "TOKOPEDIA",
		ProductCode:      "PLNPRE",
		TotalAmount:      52500,
		Timestamp:        time.Now().Format("2006-01-02 15:04:05"),
	}

	bodyBytes, _ := json.Marshal(reqDto)

	// Middleware to simulate decrypted body
	apiGroup := app.Group("/api/v1")
	apiGroup.Use(func(c *fiber.Ctx) error {
		c.Locals("decryptedBody", bodyBytes)
		c.Locals("clientIP", "192.168.1.1")
		return c.Next()
	})
	handler.RegisterRoutes(apiGroup)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/v1/payment", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req, -1)
		if resp.StatusCode != 200 {
			b.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
	}
}

// BenchmarkPaymentHandler_Payment_Parallel benchmarks with concurrent requests
// This simulates the actual 250 TPS load with parallel goroutines
func BenchmarkPaymentHandler_Payment_Parallel(b *testing.B) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	mockUsecase := new(MockPaymentUsecase)
	mockLogger := new(MockBenchmarkLogger)

	handler := NewPaymentHandler(mockUsecase, mockLogger)

	mockResponse := &domain.PaymentResponseDomain{
		RefID:        "TEST-PAY-001",
		PartnerRefID: "PPS-123456789",
		ClientNumber: "1234567890",
		ProductCode:  "PLNPRE",
		ResponseCode: "00",
		Message:      "Success",
		AdminFee:     2500,
		TotalAmount:  52500,
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		BillCount:    1,
		BillDetails: []domain.InquiryBillDetail{
			{
				Name:   "Bill Number",
				Value:  "1",
				IsPII:  false,
				IsShow: true,
			},
			{
				Name:   "Bill Name",
				Value:  "PLN Prepaid",
				IsPII:  false,
				IsShow: true,
			},
		},
	}

	mockUsecase.On("Payment", mock.Anything, mock.Anything).Return(mockResponse, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()

	reqDto := dto.PaymentRequestDto{
		RefID:            "TEST-PAY-001",
		PartnerInquiryID: "INQ-123456789",
		ClientNumber:     "1234567890",
		Category:         "PLN",
		Rsid:             "TOKOPEDIA",
		ProductCode:      "PLNPRE",
		TotalAmount:      52500,
		Timestamp:        time.Now().Format("2006-01-02 15:04:05"),
	}

	bodyBytes, _ := json.Marshal(reqDto)

	// Middleware to simulate decrypted body
	apiGroup := app.Group("/api/v1")
	apiGroup.Use(func(c *fiber.Ctx) error {
		c.Locals("decryptedBody", bodyBytes)
		c.Locals("clientIP", "192.168.1.1")
		return c.Next()
	})
	handler.RegisterRoutes(apiGroup)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/api/v1/payment", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			resp, _ := app.Test(req, -1)
			if resp.StatusCode != 200 {
				b.Fatalf("Expected status 200, got %d", resp.StatusCode)
			}
		}
	})
}
