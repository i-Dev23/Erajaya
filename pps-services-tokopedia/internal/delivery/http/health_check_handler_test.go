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

// Mock HealthCheckUsecase
type MockHealthCheckUsecase struct {
	mock.Mock
}

func (m *MockHealthCheckUsecase) HealthCheck(ctx context.Context, req *domain.HealthCheckRequestDomain) (*domain.HealthCheckResponseDomain, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.HealthCheckResponseDomain), args.Error(1)
}

func (m *MockHealthCheckUsecase) DeepHealthCheck(ctx context.Context, req *domain.HealthCheckRequestDomain) (*domain.DeepHealthCheckResponseDomain, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DeepHealthCheckResponseDomain), args.Error(1)
}

// Mock Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Info(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Warn(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Error(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func TestHealthCheckHandler_CheckHealth_Success(t *testing.T) {
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
	mockUsecase.On("HealthCheck", mock.Anything, mock.Anything).Return(expectedResponse, nil)
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

	// Execute request
	resp, err := app.Test(req)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response dto.HealthCheckResponseDto
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "00", response.ResponseCode)
	assert.Equal(t, "Success", response.Message)
	assert.NotEmpty(t, response.Timestamp)

	// Verify mock expectations
	mockUsecase.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestHealthCheckHandler_CheckHealth_InvalidJSON(t *testing.T) {
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

	// Execute request
	resp, err := app.Test(req)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response dto.HealthCheckResponseDto
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "42", response.ResponseCode)
	assert.Equal(t, "Invalid parameter", response.Message)
	assert.NotEmpty(t, response.Timestamp)

	// Verify mock expectations
	mockUsecase.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestHealthCheckHandler_CheckHealth_UsecaseError(t *testing.T) {
	// Setup
	mockUsecase := new(MockHealthCheckUsecase)
	mockLogger := new(MockLogger)
	handler := NewHealthCheckHandler(mockUsecase, mockLogger)

	// Mock expectations
	mockUsecase.On("HealthCheck", mock.Anything, mock.Anything).Return(nil, assert.AnError)
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

	// Execute request
	resp, err := app.Test(req)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response dto.HealthCheckResponseDto
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "62", response.ResponseCode)
	assert.Equal(t, "Server error", response.Message)
	assert.NotEmpty(t, response.Timestamp)

	// Verify mock expectations
	mockUsecase.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestHealthCheckHandler_RegisterRoutes(t *testing.T) {
	// Setup
	mockUsecase := new(MockHealthCheckUsecase)
	mockLogger := new(MockLogger)
	handler := NewHealthCheckHandler(mockUsecase, mockLogger)

	// Setup Fiber app
	app := fiber.New()
	handler.RegisterRoutes(app)

	// Verify route is registered
	routes := app.GetRoutes()
	found := false
	for _, route := range routes {
		if route.Path == "/health" && route.Method == "POST" {
			found = true
			break
		}
	}
	assert.True(t, found, "Health check route should be registered")
}

func TestHealthCheckHandler_CheckHealth_MissingTimestamp(t *testing.T) {
	// Setup
	mockUsecase := new(MockHealthCheckUsecase)
	mockLogger := new(MockLogger)
	handler := NewHealthCheckHandler(mockUsecase, mockLogger)

	// Mock expectations - usecase should return ErrInvalidParameter
	mockUsecase.On("HealthCheck", mock.Anything, mock.Anything).Return(nil, utils.ErrInvalidParameter)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return().Maybe()

	// Create test request with empty timestamp
	reqBody := dto.HealthCheckRequestDto{
		Timestamp: "",
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

	// Execute request
	resp, err := app.Test(req)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response dto.HealthCheckResponseDto
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "42", response.ResponseCode)
	assert.Equal(t, "Invalid parameter", response.Message)
	assert.NotEmpty(t, response.Timestamp)

	// Verify mock expectations
	mockUsecase.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestNewHealthCheckHandler(t *testing.T) {
	// Setup
	mockUsecase := new(MockHealthCheckUsecase)
	mockLogger := new(MockLogger)

	// Execute
	handler := NewHealthCheckHandler(mockUsecase, mockLogger)

	// Assertions
	assert.NotNil(t, handler)
	assert.Equal(t, mockUsecase, handler.healthCheckUsecase)
	assert.Equal(t, mockLogger, handler.logger)
}
