package http

import (
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

// MockTokenUsecaseBenchmark for benchmarking
type MockTokenUsecaseBenchmark struct {
	mock.Mock
}

func (m *MockTokenUsecaseBenchmark) GenerateAndStoreToken(ctx context.Context, req *domain.GeneratedTokenRequestDomain) (string, error) {
	args := m.Called(ctx, req)
	return args.String(0), args.Error(1)
}

func (m *MockTokenUsecaseBenchmark) RevokeToken(ctx context.Context, tokenID string) error {
	args := m.Called(ctx, tokenID)
	return args.Error(0)
}

func (m *MockTokenUsecaseBenchmark) ValidateToken(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

// BenchmarkTokenHandler_GetToken benchmarks the GetToken endpoint
// Expected: 2 hits per hour in production (very low traffic, ~0.0006 TPS)
// This benchmark ensures the token endpoint can handle occasional authentication requests
func BenchmarkTokenHandler_GetToken(b *testing.B) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	mockUsecase := new(MockTokenUsecaseBenchmark)
	mockLogger := new(MockBenchmarkLogger)

	// Mock response
	mockToken := "test-access-token-1234567890"

	mockUsecase.On("GenerateAndStoreToken", mock.Anything, mock.Anything).Return(mockToken, nil)

	// Prepare request payload
	reqDto := dto.TokenRequestDto{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
	}

	bodyBytes, _ := json.Marshal(reqDto)

	// Middleware to simulate decrypted body (register before routes)
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("decryptedBody", bodyBytes)
		return c.Next()
	})

	handler := NewTokenHandler(mockUsecase, mockLogger)
	handler.RegisterRoutes(app.Group("/auth"))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/auth/token", nil)
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req, -1)
		if resp.StatusCode != 200 {
			b.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// BenchmarkTokenHandler_GetToken_Parallel benchmarks with concurrent requests
// Even though production traffic is low, this tests the service under concurrent load
func BenchmarkTokenHandler_GetToken_Parallel(b *testing.B) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	mockUsecase := new(MockTokenUsecaseBenchmark)
	mockLogger := new(MockBenchmarkLogger)

	mockToken := "test-access-token-1234567890"

	mockUsecase.On("GenerateAndStoreToken", mock.Anything, mock.Anything).Return(mockToken, nil)

	reqDto := dto.TokenRequestDto{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
	}

	bodyBytes, _ := json.Marshal(reqDto)

	// Middleware to simulate decrypted body (register before routes)
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("decryptedBody", bodyBytes)
		return c.Next()
	})

	handler := NewTokenHandler(mockUsecase, mockLogger)
	handler.RegisterRoutes(app.Group("/auth"))

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/auth/token", nil)
			req.Header.Set("Content-Type", "application/json")

			resp, _ := app.Test(req, -1)
			if resp.StatusCode != 200 {
				b.Fatalf("Expected status 200, got %d", resp.StatusCode)
			}
			resp.Body.Close()
		}
	})
}
