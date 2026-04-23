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

// MockCheckStatusUsecase for benchmarking
type MockCheckStatusUsecase struct {
	mock.Mock
}

func (m *MockCheckStatusUsecase) CheckStatus(ctx context.Context, req *domain.CheckStatusRequestDomain) (*domain.CheckStatusResponseDomain, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CheckStatusResponseDomain), args.Error(1)
}

// BenchmarkCheckStatusHandler_CheckStatus benchmarks the CheckStatus endpoint
// Expected: 250 TPS (Transactions Per Second) in production
// This benchmark simulates high-throughput status checking
func BenchmarkCheckStatusHandler_CheckStatus(b *testing.B) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	mockUsecase := new(MockCheckStatusUsecase)
	mockLogger := new(MockBenchmarkLogger)

	handler := NewCheckStatusHandler(mockUsecase, mockLogger)

	// Mock response - typical check status success response
	mockResponse := &domain.CheckStatusResponseDomain{
		RefID:        "TEST-CHK-001",
		PartnerRefID: "PPS-123456789",
		ClientNumber: "1234567890",
		ProductCode:  "PLNPRE",
		ResponseCode: "00",
		Message:      "Success",
		AdminFee:     2500,
		TotalAmount:  52500,
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		BillCount:    1,
		BillDetails: []domain.BillDetailDomain{
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

	mockUsecase.On("CheckStatus", mock.Anything, mock.Anything).Return(mockResponse, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()

	// Prepare request payload
	reqDto := dto.CheckStatusRequestDto{
		RefID:     "TEST-CHK-001",
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Category:  "PLN",
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
		req := httptest.NewRequest("POST", "/api/v1/check-status", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req, -1)
		if resp != nil && resp.StatusCode != 200 {
			b.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}
}

// BenchmarkCheckStatusHandler_CheckStatus_Parallel benchmarks with concurrent requests
// This simulates the actual 250 TPS load with parallel goroutines
func BenchmarkCheckStatusHandler_CheckStatus_Parallel(b *testing.B) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	mockUsecase := new(MockCheckStatusUsecase)
	mockLogger := new(MockBenchmarkLogger)

	handler := NewCheckStatusHandler(mockUsecase, mockLogger)

	mockResponse := &domain.CheckStatusResponseDomain{
		RefID:        "TEST-CHK-001",
		PartnerRefID: "PPS-123456789",
		ClientNumber: "1234567890",
		ProductCode:  "PLNPRE",
		ResponseCode: "00",
		Message:      "Success",
		AdminFee:     2500,
		TotalAmount:  52500,
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		BillCount:    1,
		BillDetails: []domain.BillDetailDomain{
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

	mockUsecase.On("CheckStatus", mock.Anything, mock.Anything).Return(mockResponse, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()

	reqDto := dto.CheckStatusRequestDto{
		RefID:     "TEST-CHK-001",
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Category:  "PLN",
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
			req := httptest.NewRequest("POST", "/api/v1/check-status", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			resp, _ := app.Test(req, -1)
			if resp != nil && resp.StatusCode != 200 {
				b.Fatalf("Expected status 200, got %d", resp.StatusCode)
			}
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
		}
	})
}
