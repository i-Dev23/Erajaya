package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// BenchmarkHealthCheckHandler_CheckHealth benchmarks the health check endpoint
// Expected: Every 5 minutes in production (very low traffic, ~0.003 TPS)
// This benchmark ensures the health check can handle periodic monitoring
func BenchmarkHealthCheckHandler_CheckHealth(b *testing.B) {
	// Setup
	mockUsecase := new(MockHealthCheckUsecase)
	mockLogger := new(MockLogger)
	handler := NewHealthCheckHandler(mockUsecase, mockLogger)

	// Mock expectations
	expectedResponse := &domain.HealthCheckResponseDomain{
		ResponseCode: "00",
		Message:      "Success",
		Timestamp:    time.Now().Format(timeFormat),
	}
	mockUsecase.On("HealthCheck", mock.Anything).Return(expectedResponse, nil)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()

	// Create test request
	reqBody := dto.HealthCheckRequestDto{
		Timestamp: time.Now().Format(timeFormat),
	}
	reqBodyBytes, _ := json.Marshal(reqBody)

	// Setup Fiber app with middleware to set decryptedBody
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("decryptedBody", reqBodyBytes)
		return c.Next()
	})
	app.Post("/health", handler.CheckHealth)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	req.Header.Set("Content-Type", "application/json")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := app.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkHealthCheckHandler_CheckHealth_InvalidJSON benchmarks the invalid JSON scenario
func BenchmarkHealthCheckHandler_CheckHealth_InvalidJSON(b *testing.B) {
	// Setup
	mockUsecase := new(MockHealthCheckUsecase)
	mockLogger := new(MockLogger)
	handler := NewHealthCheckHandler(mockUsecase, mockLogger)

	// Mock expectations
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()

	// Create invalid JSON request
	invalidJSON := `{"invalid": json}`
	reqBodyBytes := []byte(invalidJSON)

	// Setup Fiber app with middleware to set decryptedBody
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("decryptedBody", reqBodyBytes)
		return c.Next()
	})
	app.Post("/health", handler.CheckHealth)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	req.Header.Set("Content-Type", "application/json")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := app.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkHealthCheckHandler_CheckHealth_UsecaseError benchmarks the usecase error scenario
func BenchmarkHealthCheckHandler_CheckHealth_UsecaseError(b *testing.B) {
	// Setup
	mockUsecase := new(MockHealthCheckUsecase)
	mockLogger := new(MockLogger)
	handler := NewHealthCheckHandler(mockUsecase, mockLogger)

	// Mock expectations
	mockUsecase.On("HealthCheck", mock.Anything).Return(nil, assert.AnError)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()

	// Create test request
	reqBody := dto.HealthCheckRequestDto{
		Timestamp: time.Now().Format(timeFormat),
	}
	reqBodyBytes, _ := json.Marshal(reqBody)

	// Setup Fiber app with middleware to set decryptedBody
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("decryptedBody", reqBodyBytes)
		return c.Next()
	})
	app.Post("/health", handler.CheckHealth)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	req.Header.Set("Content-Type", "application/json")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := app.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkNewHealthCheckResponse benchmarks the response creation function
func BenchmarkNewHealthCheckResponse(b *testing.B) {
	code := "00"
	message := "Success"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = newHealthCheckResponse(code, message)
	}
}

// BenchmarkHealthCheckHandler_Concurrent benchmarks concurrent health check requests
func BenchmarkHealthCheckHandler_Concurrent(b *testing.B) {
	// Setup
	mockUsecase := new(MockHealthCheckUsecase)
	mockLogger := new(MockLogger)
	handler := NewHealthCheckHandler(mockUsecase, mockLogger)

	// Mock expectations
	expectedResponse := &domain.HealthCheckResponseDomain{
		ResponseCode: "00",
		Message:      "Success",
		Timestamp:    time.Now().Format(timeFormat),
	}
	mockUsecase.On("HealthCheck", mock.Anything).Return(expectedResponse, nil)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()

	// Create test request
	reqBody := dto.HealthCheckRequestDto{
		Timestamp: time.Now().Format(timeFormat),
	}
	reqBodyBytes, _ := json.Marshal(reqBody)

	// Setup Fiber app with middleware to set decryptedBody
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("decryptedBody", reqBodyBytes)
		return c.Next()
	})
	app.Post("/health", handler.CheckHealth)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	req.Header.Set("Content-Type", "application/json")

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := app.Test(req)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}
