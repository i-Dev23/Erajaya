package http

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"pps-services-tokopedia/internal/dto"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

// mockHealthCheckV2Usecase is a mock implementation for testing
type mockHealthCheckV2Usecase struct {
	CheckHealthFunc func(ctx context.Context) dto.HealthCheckV2ResponseDto
}

func (m *mockHealthCheckV2Usecase) CheckHealth(ctx context.Context) dto.HealthCheckV2ResponseDto {
	if m.CheckHealthFunc != nil {
		return m.CheckHealthFunc(ctx)
	}
	return dto.HealthCheckV2ResponseDto{
		ResponseCode: "00",
		Message:      "Success",
		Timestamp:    "2026-01-26 10:00:00",
	}
}

func TestHealthCheckV2Handler_CheckHealth(t *testing.T) {
	tests := []struct {
		name         string
		mockSetup    func(*mockHealthCheckV2Usecase)
		validateResp func(*testing.T, string)
	}{
		{
			name: "success - all services healthy",
			mockSetup: func(m *mockHealthCheckV2Usecase) {
				m.CheckHealthFunc = func(ctx context.Context) dto.HealthCheckV2ResponseDto {
					return dto.HealthCheckV2ResponseDto{
						ResponseCode: "00",
						Message:      "Success",
						Timestamp:    "2026-01-26 10:00:00",
					}
				}
			},
			validateResp: func(t *testing.T, body string) {
				var resp dto.HealthCheckV2ResponseDto
				err := json.Unmarshal([]byte(body), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "00", resp.ResponseCode)
				assert.Equal(t, "Success", resp.Message)
				assert.NotEmpty(t, resp.Timestamp)
			},
		},
		{
			name: "partial failure - some services down",
			mockSetup: func(m *mockHealthCheckV2Usecase) {
				m.CheckHealthFunc = func(ctx context.Context) dto.HealthCheckV2ResponseDto {
					return dto.HealthCheckV2ResponseDto{
						ResponseCode: "62",
						Message:      "Server error",
						Timestamp:    "2026-01-26 10:00:00",
					}
				}
			},
			validateResp: func(t *testing.T, body string) {
				var resp dto.HealthCheckV2ResponseDto
				err := json.Unmarshal([]byte(body), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "62", resp.ResponseCode)
				assert.Equal(t, "Server error", resp.Message)
			},
		},
		{
			name: "maintenance mode",
			mockSetup: func(m *mockHealthCheckV2Usecase) {
				m.CheckHealthFunc = func(ctx context.Context) dto.HealthCheckV2ResponseDto {
					return dto.HealthCheckV2ResponseDto{
						ResponseCode: "61",
						Message:      "Server maintenance",
						Timestamp:    "2026-01-26 10:00:00",
					}
				}
			},
			validateResp: func(t *testing.T, body string) {
				var resp dto.HealthCheckV2ResponseDto
				err := json.Unmarshal([]byte(body), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "61", resp.ResponseCode)
				assert.Equal(t, "Server maintenance", resp.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockLogger := &mockLogger{}
			mockUsecase := &mockHealthCheckV2Usecase{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockUsecase)
			}

			handler := NewHealthCheckV2Handler(mockUsecase, mockLogger)
			handler.RegisterRoutes(app)

			req := httptest.NewRequest("POST", "/health", nil)
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)

			body := make([]byte, 1024)
			n, _ := resp.Body.Read(body)
			tt.validateResp(t, string(body[:n]))
		})
	}
}

func TestHealthCheckV2Handler_RegisterRoutes(t *testing.T) {
	app := fiber.New()
	mockLogger := &mockLogger{}
	mockUsecase := &mockHealthCheckV2Usecase{}

	handler := NewHealthCheckV2Handler(mockUsecase, mockLogger)
	handler.RegisterRoutes(app)

	// Test that route is registered
	req := httptest.NewRequest("POST", "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.NotEqual(t, 404, resp.StatusCode, "Route should be registered")
	assert.Equal(t, 200, resp.StatusCode)
}

func TestNewHealthCheckV2Handler(t *testing.T) {
	mockLogger := &mockLogger{}
	mockUsecase := &mockHealthCheckV2Usecase{}

	handler := NewHealthCheckV2Handler(mockUsecase, mockLogger)

	assert.NotNil(t, handler)
	assert.Equal(t, mockUsecase, handler.healthCheckV2Usecase)
	assert.Equal(t, mockLogger, handler.logger)
}
