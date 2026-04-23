package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

// mockErrorMappingRepository is a mock for testing
type mockErrorMappingRepository struct {
	GetMappingFunc func(ctx context.Context, systemType string, errorMessage string) (*domain.ErrorMessageMapping, error)
}

func (m *mockErrorMappingRepository) GetMapping(ctx context.Context, systemType string, errorMessage string) (*domain.ErrorMessageMapping, error) {
	if m.GetMappingFunc != nil {
		return m.GetMappingFunc(ctx, systemType, errorMessage)
	}
	return &domain.ErrorMessageMapping{ResponseCode: "63", Description: "Biller maintenance", Found: true}, nil
}

func TestErrorMappingHandler_MapErrorMessage(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*mockErrorMappingRepository)
		expectedStatus int
		validateResp   func(*testing.T, string)
	}{
		{
			name:        "success - found in DB (ultima)",
			requestBody: `{"error_message":"Biller sedang maintenance","system_type":"ultima"}`,
			mockSetup: func(m *mockErrorMappingRepository) {
				m.GetMappingFunc = func(ctx context.Context, systemType string, errorMessage string) (*domain.ErrorMessageMapping, error) {
					assert.Equal(t, "ultima", systemType)
					assert.Equal(t, "Biller sedang maintenance", errorMessage)
					return &domain.ErrorMessageMapping{
						ResponseCode: "63",
						Description:  "Biller maintenance",
						Found:        true,
					}, nil
				}
			},
			expectedStatus: 200,
			validateResp: func(t *testing.T, body string) {
				var resp dto.ErrorMappingResponseDto
				err := json.Unmarshal([]byte(body), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "63", resp.ResponseCode)
				assert.Equal(t, "Biller maintenance", resp.Message)
			},
		},
		{
			name:        "success - found in DB (oracle)",
			requestBody: `{"error_message":"ORA-12345: database error","system_type":"oracle"}`,
			mockSetup: func(m *mockErrorMappingRepository) {
				m.GetMappingFunc = func(ctx context.Context, systemType string, errorMessage string) (*domain.ErrorMessageMapping, error) {
					assert.Equal(t, "oracle", systemType)
					return &domain.ErrorMessageMapping{
						ResponseCode: "62",
						Description:  "Server error",
						Found:        true,
					}, nil
				}
			},
			expectedStatus: 200,
			validateResp: func(t *testing.T, body string) {
				var resp dto.ErrorMappingResponseDto
				err := json.Unmarshal([]byte(body), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "62", resp.ResponseCode)
				assert.Equal(t, "Server error", resp.Message)
			},
		},
		{
			name:        "not found in DB - returns 99",
			requestBody: `{"error_message":"unknown error","system_type":"ultima"}`,
			mockSetup: func(m *mockErrorMappingRepository) {
				m.GetMappingFunc = func(ctx context.Context, systemType string, errorMessage string) (*domain.ErrorMessageMapping, error) {
					return &domain.ErrorMessageMapping{Found: false}, nil
				}
			},
			expectedStatus: 200,
			validateResp: func(t *testing.T, body string) {
				var resp dto.ErrorMappingResponseDto
				err := json.Unmarshal([]byte(body), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "99", resp.ResponseCode)
				assert.Equal(t, "Other error", resp.Message)
			},
		},
		{
			name:        "DB error - returns 99",
			requestBody: `{"error_message":"test error","system_type":"ultima"}`,
			mockSetup: func(m *mockErrorMappingRepository) {
				m.GetMappingFunc = func(ctx context.Context, systemType string, errorMessage string) (*domain.ErrorMessageMapping, error) {
					return nil, errors.New("database connection failed")
				}
			},
			expectedStatus: 200,
			validateResp: func(t *testing.T, body string) {
				var resp dto.ErrorMappingResponseDto
				err := json.Unmarshal([]byte(body), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "99", resp.ResponseCode)
				assert.Equal(t, "Other error", resp.Message)
			},
		},
		{
			name:           "invalid request body",
			requestBody:    `{invalid json}`,
			mockSetup:      nil,
			expectedStatus: 400,
			validateResp: func(t *testing.T, body string) {
				assert.Contains(t, body, "Invalid request body")
			},
		},
		{
			name:           "missing error_message",
			requestBody:    `{"system_type":"ultima"}`,
			mockSetup:      nil,
			expectedStatus: 400,
			validateResp: func(t *testing.T, body string) {
				assert.Contains(t, body, "error_message and system_type are required")
			},
		},
		{
			name:           "missing system_type",
			requestBody:    `{"error_message":"test"}`,
			mockSetup:      nil,
			expectedStatus: 400,
			validateResp: func(t *testing.T, body string) {
				assert.Contains(t, body, "error_message and system_type are required")
			},
		},
		{
			name:           "invalid system_type",
			requestBody:    `{"error_message":"test","system_type":"invalid"}`,
			mockSetup:      nil,
			expectedStatus: 400,
			validateResp: func(t *testing.T, body string) {
				assert.Contains(t, body, "system_type must be 'ultima' or 'oracle'")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockLogger := &mockLogger{}
			repo := &mockErrorMappingRepository{}

			if tt.mockSetup != nil {
				tt.mockSetup(repo)
			}

			handler := NewErrorMappingHandler(mockLogger, repo)
			handler.RegisterRoutes(app)

			req := httptest.NewRequest("POST", "/error-mapping", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			body := make([]byte, 1024)
			n, _ := resp.Body.Read(body)
			tt.validateResp(t, string(body[:n]))
		})
	}
}

func TestErrorMappingHandler_RegisterRoutes(t *testing.T) {
	app := fiber.New()
	mockLogger := &mockLogger{}
	mockRepo := &mockErrorMappingRepository{}

	handler := NewErrorMappingHandler(mockLogger, mockRepo)
	handler.RegisterRoutes(app)

	// Test that route is registered
	req := httptest.NewRequest("POST", "/error-mapping", strings.NewReader(`{"error_message":"test","system_type":"ultima"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.NotEqual(t, 404, resp.StatusCode, "Route should be registered")
}
