package http

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/utils"

	"github.com/stretchr/testify/mock"
)

// --- Mock Token Usecase for Benchmark ---
type BenchmarkMockTokenUsecase struct {
	mock.Mock
}

func (m *BenchmarkMockTokenUsecase) GenerateAndStoreToken(ctx context.Context, request *domain.GeneratedTokenRequestDomain) (string, error) {
	args := m.Called(ctx, request)
	return args.String(0), args.Error(1)
}

func (m *BenchmarkMockTokenUsecase) ValidateToken(ctx context.Context, tokenValue string) error {
	args := m.Called(ctx, tokenValue)
	return args.Error(0)
}

func (m *BenchmarkMockTokenUsecase) RevokeToken(ctx context.Context, tokenID string) error {
	args := m.Called(ctx, tokenID)
	return args.Error(0)
}

// --- Mock Logger for Benchmark ---
type BenchmarkMockLogger struct {
	mock.Mock
}

func (m *BenchmarkMockLogger) Debug(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *BenchmarkMockLogger) Info(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *BenchmarkMockLogger) Warn(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *BenchmarkMockLogger) Error(msg string, args ...interface{}) {
	m.Called(msg, args)
}

// setupBenchmarkEnvironment sets up the environment for benchmark tests
func setupTokenHandlerBenchmarkEnvironment() {
	os.Setenv("TP_CLIENT_ID", "benchmark_client_id")
	os.Setenv("TP_CLIENT_SECRET", "benchmark_client_secret")
	os.Setenv("TOKEN_EXPIRATION_SECONDS", "3600")
}

// BenchmarkTokenHandlerGetToken benchmarks the GetToken handler method
func BenchmarkTokenHandlerGetToken(b *testing.B) {
	setupTokenHandlerBenchmarkEnvironment()

	mockTokenUsecase := new(BenchmarkMockTokenUsecase)
	mockLogger := new(BenchmarkMockLogger)

	// Setup mock expectations
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockTokenUsecase.On("GenerateAndStoreToken", mock.Anything, mock.AnythingOfType("*domain.GeneratedTokenRequestDomain")).Return("benchmark_token_123456789012345678901234567890123456789012345678901234567890", nil)

	handler := &TokenHandler{
		tokenUsecase: mockTokenUsecase,
		logger:       mockLogger,
	}

	// Create test request body
	reqDto := dto.TokenRequestDto{
		ClientID:     "benchmark_client_id",
		ClientSecret: "benchmark_client_secret",
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
	}
	reqBody, _ := json.Marshal(reqDto)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate the core logic without Fiber context complexity
		ctx := context.Background()

		// Parse request body (simulating what the handler does)
		var parsedReqDto dto.TokenRequestDto
		_ = json.Unmarshal(reqBody, &parsedReqDto)

		// Call usecase directly (simulating what the handler does)
		_, _ = handler.tokenUsecase.GenerateAndStoreToken(ctx, &domain.GeneratedTokenRequestDomain{
			ClientID:     parsedReqDto.ClientID,
			ClientSecret: parsedReqDto.ClientSecret,
		})
	}
}

// BenchmarkTokenHandlerGetTokenWithError benchmarks the GetToken handler with error
func BenchmarkTokenHandlerGetTokenWithError(b *testing.B) {
	setupTokenHandlerBenchmarkEnvironment()

	mockTokenUsecase := new(BenchmarkMockTokenUsecase)
	mockLogger := new(BenchmarkMockLogger)

	// Setup mock expectations for error case
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockTokenUsecase.On("GenerateAndStoreToken", mock.Anything, mock.AnythingOfType("*domain.GeneratedTokenRequestDomain")).Return("", utils.ErrInvalidClientID)

	handler := &TokenHandler{
		tokenUsecase: mockTokenUsecase,
		logger:       mockLogger,
	}

	// Create test request body with invalid credentials
	reqDto := dto.TokenRequestDto{
		ClientID:     "invalid_client_id",
		ClientSecret: "invalid_client_secret",
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
	}
	reqBody, _ := json.Marshal(reqDto)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate the core logic without Fiber context complexity
		ctx := context.Background()

		// Parse request body (simulating what the handler does)
		var parsedReqDto dto.TokenRequestDto
		_ = json.Unmarshal(reqBody, &parsedReqDto)

		// Call usecase directly (simulating what the handler does)
		_, _ = handler.tokenUsecase.GenerateAndStoreToken(ctx, &domain.GeneratedTokenRequestDomain{
			ClientID:     parsedReqDto.ClientID,
			ClientSecret: parsedReqDto.ClientSecret,
		})
	}
}

// BenchmarkTokenHandlerGetTokenInvalidJSON benchmarks the GetToken handler with invalid JSON
func BenchmarkTokenHandlerGetTokenInvalidJSON(b *testing.B) {
	setupTokenHandlerBenchmarkEnvironment()

	mockTokenUsecase := new(BenchmarkMockTokenUsecase)
	mockLogger := new(BenchmarkMockLogger)

	// Setup mock expectations
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockTokenUsecase.On("GenerateAndStoreToken", mock.Anything, mock.AnythingOfType("*domain.GeneratedTokenRequestDomain")).Return("", nil).Maybe()

	handler := &TokenHandler{
		tokenUsecase: mockTokenUsecase,
		logger:       mockLogger,
	}

	// Create invalid JSON request body
	reqBody := []byte(`{"invalid": "json"`)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate the core logic without Fiber context complexity
		ctx := context.Background()

		// Parse request body (simulating what the handler does) - this will fail
		var parsedReqDto dto.TokenRequestDto
		_ = json.Unmarshal(reqBody, &parsedReqDto)

		// Call usecase directly (simulating what the handler does)
		_, _ = handler.tokenUsecase.GenerateAndStoreToken(ctx, &domain.GeneratedTokenRequestDomain{
			ClientID:     parsedReqDto.ClientID,
			ClientSecret: parsedReqDto.ClientSecret,
		})
	}
}

// BenchmarkNewTokenResponse benchmarks the newTokenResponse helper function
func BenchmarkNewTokenResponse(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = newTokenResponse("00", "Success", "benchmark_token_123456789012345678901234567890123456789012345678901234567890")
	}
}

// BenchmarkMapErrorToResponse benchmarks the mapErrorToResponse helper function
func BenchmarkMapErrorToResponse(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = mapErrorToResponse(utils.ErrInvalidClientID)
	}
}

// BenchmarkTokenHandlerConcurrent benchmarks concurrent token handler operations
func BenchmarkTokenHandlerConcurrent(b *testing.B) {
	setupTokenHandlerBenchmarkEnvironment()

	mockTokenUsecase := new(BenchmarkMockTokenUsecase)
	mockLogger := new(BenchmarkMockLogger)

	// Setup mock expectations
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockTokenUsecase.On("GenerateAndStoreToken", mock.Anything, mock.AnythingOfType("*domain.GeneratedTokenRequestDomain")).Return("benchmark_token_123456789012345678901234567890123456789012345678901234567890", nil)

	handler := &TokenHandler{
		tokenUsecase: mockTokenUsecase,
		logger:       mockLogger,
	}

	// Create test request body
	reqDto := dto.TokenRequestDto{
		ClientID:     "benchmark_client_id",
		ClientSecret: "benchmark_client_secret",
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
	}
	reqBody, _ := json.Marshal(reqDto)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate the core logic without Fiber context complexity
			ctx := context.Background()

			// Parse request body (simulating what the handler does)
			var parsedReqDto dto.TokenRequestDto
			_ = json.Unmarshal(reqBody, &parsedReqDto)

			// Call usecase directly (simulating what the handler does)
			_, _ = handler.tokenUsecase.GenerateAndStoreToken(ctx, &domain.GeneratedTokenRequestDomain{
				ClientID:     parsedReqDto.ClientID,
				ClientSecret: parsedReqDto.ClientSecret,
			})
		}
	})
}

// BenchmarkTokenHandlerFullFlow benchmarks the complete token handler flow
func BenchmarkTokenHandlerFullFlow(b *testing.B) {
	setupTokenHandlerBenchmarkEnvironment()

	mockTokenUsecase := new(BenchmarkMockTokenUsecase)
	mockLogger := new(BenchmarkMockLogger)

	// Setup mock expectations for full flow
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockTokenUsecase.On("GenerateAndStoreToken", mock.Anything, mock.AnythingOfType("*domain.GeneratedTokenRequestDomain")).Return("benchmark_token_123456789012345678901234567890123456789012345678901234567890", nil)
	mockTokenUsecase.On("ValidateToken", mock.Anything, mock.AnythingOfType("string")).Return(nil)
	mockTokenUsecase.On("RevokeToken", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	handler := &TokenHandler{
		tokenUsecase: mockTokenUsecase,
		logger:       mockLogger,
	}

	// Create test request body
	reqDto := dto.TokenRequestDto{
		ClientID:     "benchmark_client_id",
		ClientSecret: "benchmark_client_secret",
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
	}
	reqBody, _ := json.Marshal(reqDto)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate the core logic without Fiber context complexity
		ctx := context.Background()

		// Parse request body (simulating what the handler does)
		var parsedReqDto dto.TokenRequestDto
		_ = json.Unmarshal(reqBody, &parsedReqDto)

		// Call usecase directly (simulating what the handler does)
		token, _ := handler.tokenUsecase.GenerateAndStoreToken(ctx, &domain.GeneratedTokenRequestDomain{
			ClientID:     parsedReqDto.ClientID,
			ClientSecret: parsedReqDto.ClientSecret,
		})

		// Validate token
		_ = mockTokenUsecase.ValidateToken(ctx, token)

		// Revoke token
		_ = mockTokenUsecase.RevokeToken(ctx, token)
	}
}

// mockRequestBody is a helper struct for mocking request body
type mockRequestBody struct {
	data []byte
	pos  int
}

func (m *mockRequestBody) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.data) {
		return 0, nil
	}
	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *mockRequestBody) Close() error {
	return nil
}
