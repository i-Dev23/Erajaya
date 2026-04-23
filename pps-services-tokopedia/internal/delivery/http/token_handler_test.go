package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock TokenUsecase
type MockTokenUsecase struct {
	mock.Mock
}

func (m *MockTokenUsecase) GenerateAndStoreToken(ctx context.Context, request *domain.GeneratedTokenRequestDomain) (string, error) {
	args := m.Called(ctx, request)
	return args.String(0), args.Error(1)
}

func (m *MockTokenUsecase) ValidateToken(ctx context.Context, tokenValue string) error {
	args := m.Called(ctx, tokenValue)
	return args.Error(0)
}

// Mock Logger for TokenHandler
type MockTokenLogger struct {
	mock.Mock
}

func (m *MockTokenLogger) Debug(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockTokenLogger) Info(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockTokenLogger) Warn(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockTokenLogger) Error(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func TestTokenHandler_GetToken_Success(t *testing.T) {
	// Setup
	mockUsecase := new(MockTokenUsecase)
	mockLogger := new(MockTokenLogger)
	handler := NewTokenHandler(mockUsecase, mockLogger)

	// Mock expectations
	expectedToken := "sample_jwt_token_12345"
	mockUsecase.On("GenerateAndStoreToken", mock.Anything, mock.AnythingOfType("*domain.GeneratedTokenRequestDomain")).Return(expectedToken, nil)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()

	// Create test request
	reqBody := dto.TokenRequestDto{
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		Timestamp:    time.Now().Format(timeFormat),
	}
	reqBodyBytes, _ := json.Marshal(reqBody)

	// Setup Fiber app with middleware to set decryptedBody
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("decryptedBody", reqBodyBytes)
		return c.Next()
	})
	app.Post("/token", handler.GetToken)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/token", nil)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := app.Test(req)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response dto.TokenResponseDto
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "00", response.ResponseCode)
	assert.Equal(t, "Success", response.Message)
	assert.Equal(t, expectedToken, response.Token)
	assert.NotEmpty(t, response.Timestamp)

	// Verify mock expectations
	mockUsecase.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestTokenHandler_GetToken_InvalidJSON(t *testing.T) {
	// Setup
	mockUsecase := new(MockTokenUsecase)
	mockLogger := new(MockTokenLogger)
	handler := NewTokenHandler(mockUsecase, mockLogger)

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
	app.Post("/token", handler.GetToken)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/token", nil)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := app.Test(req)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response dto.TokenResponseDto
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "42", response.ResponseCode)
	assert.Equal(t, "Invalid parameter", response.Message)
	assert.Empty(t, response.Token)
	assert.NotEmpty(t, response.Timestamp)

	// Verify mock expectations
	mockUsecase.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestTokenHandler_GetToken_InvalidClientID(t *testing.T) {
	// Setup
	mockUsecase := new(MockTokenUsecase)
	mockLogger := new(MockTokenLogger)
	handler := NewTokenHandler(mockUsecase, mockLogger)

	// Mock expectations
	mockUsecase.On("GenerateAndStoreToken", mock.Anything, mock.AnythingOfType("*domain.GeneratedTokenRequestDomain")).Return("", utils.ErrInvalidClientID)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()

	// Create test request
	reqBody := dto.TokenRequestDto{
		ClientID:     "invalid_client_id",
		ClientSecret: "test_client_secret",
		Timestamp:    time.Now().Format(timeFormat),
	}
	reqBodyBytes, _ := json.Marshal(reqBody)

	// Setup Fiber app with middleware to set decryptedBody
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("decryptedBody", reqBodyBytes)
		return c.Next()
	})
	app.Post("/token", handler.GetToken)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/token", nil)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := app.Test(req)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response dto.TokenResponseDto
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "31", response.ResponseCode)
	assert.Equal(t, "Invalid credential", response.Message)
	assert.Empty(t, response.Token)
	assert.NotEmpty(t, response.Timestamp)

	// Verify mock expectations
	mockUsecase.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestTokenHandler_GetToken_InvalidClientSecret(t *testing.T) {
	// Setup
	mockUsecase := new(MockTokenUsecase)
	mockLogger := new(MockTokenLogger)
	handler := NewTokenHandler(mockUsecase, mockLogger)

	// Mock expectations
	mockUsecase.On("GenerateAndStoreToken", mock.Anything, mock.AnythingOfType("*domain.GeneratedTokenRequestDomain")).Return("", utils.ErrInvalidClientSecret)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()

	// Create test request
	reqBody := dto.TokenRequestDto{
		ClientID:     "test_client_id",
		ClientSecret: "invalid_client_secret",
		Timestamp:    time.Now().Format(timeFormat),
	}
	reqBodyBytes, _ := json.Marshal(reqBody)

	// Setup Fiber app with middleware to set decryptedBody
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("decryptedBody", reqBodyBytes)
		return c.Next()
	})
	app.Post("/token", handler.GetToken)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/token", nil)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := app.Test(req)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response dto.TokenResponseDto
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "31", response.ResponseCode)
	assert.Equal(t, "Invalid credential", response.Message)
	assert.Empty(t, response.Token)
	assert.NotEmpty(t, response.Timestamp)

	// Verify mock expectations
	mockUsecase.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestTokenHandler_GetToken_InvalidDigitalSignature(t *testing.T) {
	// Setup
	mockUsecase := new(MockTokenUsecase)
	mockLogger := new(MockTokenLogger)
	handler := NewTokenHandler(mockUsecase, mockLogger)

	// Mock expectations
	mockUsecase.On("GenerateAndStoreToken", mock.Anything, mock.AnythingOfType("*domain.GeneratedTokenRequestDomain")).Return("", utils.ErrInvalidDigitalSignature)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()

	// Create test request
	reqBody := dto.TokenRequestDto{
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		Timestamp:    time.Now().Format(timeFormat),
	}
	reqBodyBytes, _ := json.Marshal(reqBody)

	// Setup Fiber app with middleware to set decryptedBody
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("decryptedBody", reqBodyBytes)
		return c.Next()
	})
	app.Post("/token", handler.GetToken)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/token", nil)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := app.Test(req)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response dto.TokenResponseDto
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "32", response.ResponseCode)
	assert.Equal(t, "Invalid signature", response.Message)
	assert.Empty(t, response.Token)
	assert.NotEmpty(t, response.Timestamp)

	// Verify mock expectations
	mockUsecase.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestTokenHandler_GetToken_GenericError(t *testing.T) {
	// Setup
	mockUsecase := new(MockTokenUsecase)
	mockLogger := new(MockTokenLogger)
	handler := NewTokenHandler(mockUsecase, mockLogger)

	// Mock expectations
	mockUsecase.On("GenerateAndStoreToken", mock.Anything, mock.AnythingOfType("*domain.GeneratedTokenRequestDomain")).Return("", assert.AnError)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()

	// Create test request
	reqBody := dto.TokenRequestDto{
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		Timestamp:    time.Now().Format(timeFormat),
	}
	reqBodyBytes, _ := json.Marshal(reqBody)

	// Setup Fiber app with middleware to set decryptedBody
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("decryptedBody", reqBodyBytes)
		return c.Next()
	})
	app.Post("/token", handler.GetToken)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/token", nil)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := app.Test(req)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response dto.TokenResponseDto
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "62", response.ResponseCode)
	assert.Equal(t, "Server error", response.Message)
	assert.Empty(t, response.Token)
	assert.NotEmpty(t, response.Timestamp)

	// Verify mock expectations
	mockUsecase.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestTokenHandler_RegisterRoutes(t *testing.T) {
	// Setup
	mockUsecase := new(MockTokenUsecase)
	mockLogger := new(MockTokenLogger)
	handler := NewTokenHandler(mockUsecase, mockLogger)

	// Setup Fiber app
	app := fiber.New()
	handler.RegisterRoutes(app)

	// Verify route is registered
	routes := app.GetRoutes()
	found := false
	for _, route := range routes {
		if route.Path == "/token" && route.Method == "POST" {
			found = true
			break
		}
	}
	assert.True(t, found, "Token route should be registered")
}

func TestNewTokenHandler(t *testing.T) {
	// Setup
	mockUsecase := new(MockTokenUsecase)
	mockLogger := new(MockTokenLogger)

	// Execute
	handler := NewTokenHandler(mockUsecase, mockLogger)

	// Assertions
	assert.NotNil(t, handler)
	assert.Equal(t, mockUsecase, handler.tokenUsecase)
	assert.Equal(t, mockLogger, handler.logger)
}

func TestNewTokenResponse(t *testing.T) {
	// Test data
	code := "00"
	message := "Success"
	token := "sample_token"

	// Execute
	response := newTokenResponse(code, message, token)

	// Assertions
	assert.Equal(t, code, response.ResponseCode)
	assert.Equal(t, message, response.Message)
	assert.Equal(t, token, response.Token)
	assert.NotEmpty(t, response.Timestamp)
}

func TestMapErrorToResponse(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedCode   string
		expectedStatus int
	}{
		{
			name:           "Invalid Client ID",
			err:            utils.ErrInvalidClientID,
			expectedCode:   "31",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid Client Secret",
			err:            utils.ErrInvalidClientSecret,
			expectedCode:   "31",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid Digital Signature",
			err:            utils.ErrInvalidDigitalSignature,
			expectedCode:   "32",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Generic Error",
			err:            assert.AnError,
			expectedCode:   "62",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, response := mapErrorToResponse(tt.err)

			assert.Equal(t, tt.expectedStatus, status)
			assert.Equal(t, tt.expectedCode, response.ResponseCode)
			assert.NotEmpty(t, response.Message)
			assert.NotEmpty(t, response.Timestamp)
			assert.Empty(t, response.Token)
		})
	}
}
