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
	"pps-services-tokopedia/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/mock"
)

// MockInquiryUsecase for benchmarking
type MockInquiryUsecase struct {
	mock.Mock
}

func (m *MockInquiryUsecase) Inquiry(ctx context.Context, req *domain.InquiryRequestDomain) (*domain.InquiryResponseDomain, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.InquiryResponseDomain), args.Error(1)
}

// BenchmarkInquiryHandler_Inquiry benchmarks the Inquiry endpoint
// Expected: 300 TPS (Transactions Per Second) in production
// This benchmark simulates high-throughput inquiry processing
func BenchmarkInquiryHandler_Inquiry(b *testing.B) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	mockUsecase := new(MockInquiryUsecase)
	mockLogger := new(MockBenchmarkLogger)

	handler := NewInquiryHandler(mockUsecase, mockLogger)

	// Mock response - typical PLN inquiry response
	mockResponse := &domain.InquiryResponseDomain{
		PartnerInquiryID: "TEST-INQ-001",
		ClientNumber:     "1234567890",
		ProductCode:      "PLNPRE",
		ResponseCode:     "00",
		Message:          "Success",
		Timestamp:        time.Now().Format("2006-01-02 15:04:05"),
		TotalAmount:      50000,
		BillCount:        1,
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

	mockUsecase.On("Inquiry", mock.Anything, mock.Anything).Return(mockResponse, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()

	// Prepare request payload
	reqDto := dto.InquiryRequestDto{
		RefID:        "TEST-REF-001",
		ClientNumber: "1234567890",
		Category:     "PLN",
		Rsid:         "TOKOPEDIA",
		ProductCode:  "PLNPRE",
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
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
		req := httptest.NewRequest("POST", "/api/v1/inquiry", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req, -1)
		if resp.StatusCode != 200 {
			b.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
	}
}

// BenchmarkInquiryHandler_Inquiry_Parallel benchmarks with concurrent requests
// This simulates the actual 300 TPS load with parallel goroutines
func BenchmarkInquiryHandler_Inquiry_Parallel(b *testing.B) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	mockUsecase := new(MockInquiryUsecase)
	mockLogger := new(MockBenchmarkLogger)

	handler := NewInquiryHandler(mockUsecase, mockLogger)

	mockResponse := &domain.InquiryResponseDomain{
		PartnerInquiryID: "TEST-INQ-001",
		ClientNumber:     "1234567890",
		ProductCode:      "PLNPRE",
		ResponseCode:     "00",
		Message:          "Success",
		Timestamp:        time.Now().Format("2006-01-02 15:04:05"),
		TotalAmount:      50000,
		BillCount:        1,
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

	mockUsecase.On("Inquiry", mock.Anything, mock.Anything).Return(mockResponse, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()

	reqDto := dto.InquiryRequestDto{
		RefID:        "TEST-REF-001",
		ClientNumber: "1234567890",
		Category:     "PLN",
		Rsid:         "TOKOPEDIA",
		ProductCode:  "PLNPRE",
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
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
			req := httptest.NewRequest("POST", "/api/v1/inquiry", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			resp, _ := app.Test(req, -1)
			if resp.StatusCode != 200 {
				b.Fatalf("Expected status 200, got %d", resp.StatusCode)
			}
		}
	})
}

// MockBenchmarkLogger is a minimal logger for benchmarking
type MockBenchmarkLogger struct {
	mock.Mock
}

func (m *MockBenchmarkLogger) Info(msg string, keysAndValues ...interface{}) {
	// No-op for performance
}

func (m *MockBenchmarkLogger) Debug(msg string, keysAndValues ...interface{}) {
	// No-op for performance
}

func (m *MockBenchmarkLogger) Error(msg string, keysAndValues ...interface{}) {
	// No-op for performance
}

func (m *MockBenchmarkLogger) Warn(msg string, keysAndValues ...interface{}) {
	// No-op for performance
}

func (m *MockBenchmarkLogger) Fatal(msg string, keysAndValues ...interface{}) {
	// No-op for performance
}

var _ service.Logger = (*MockBenchmarkLogger)(nil)
