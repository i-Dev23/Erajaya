package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/service"

	"github.com/gofiber/fiber/v2"

	"github.com/stretchr/testify/assert"
)

// Ensure mockCheckStatusUsecase implements domain.CheckStatusUsecase
var _ domain.CheckStatusUsecase = (*mockCheckStatusUsecase)(nil)

type mockCheckStatusUsecase struct {
	CheckStatusFunc func(ctx context.Context, req *domain.CheckStatusRequestDomain) (*domain.CheckStatusResponseDomain, error)
}

// Implements domain.CheckStatusUsecase
func (m *mockCheckStatusUsecase) CheckStatus(ctx context.Context, req *domain.CheckStatusRequestDomain) (*domain.CheckStatusResponseDomain, error) {
	if m.CheckStatusFunc != nil {
		return m.CheckStatusFunc(ctx, req)
	}
	return nil, nil
}

// mockLogger implements service.Logger for testing
// Used for all handler tests

// Helper to construct handler for tests

func newTestCheckStatusHandler(usecase domain.CheckStatusUsecase, logger service.Logger) *CheckStatusHandler {
	return NewCheckStatusHandler(usecase, logger)
}

func TestCheckStatusHandler_CheckStatus(t *testing.T) {
	mockUsecase := &mockCheckStatusUsecase{}

	tests := []struct {
		name       string
		body       dto.CheckStatusRequestDto
		mockResp   *domain.CheckStatusResponseDomain
		mockErr    error
		wantStatus int
		wantBody   string
	}{
		{
			name: "success",
			body: dto.CheckStatusRequestDto{
				RefID:     "ref-1",
				Timestamp: "2026-01-28 12:00:00",
				Category:  "cat-1",
			},
			mockResp: &domain.CheckStatusResponseDomain{
				RefID:        "ref-1",
				PartnerRefID: "partner-1",
				ClientNumber: "12345",
				ProductCode:  "PROD1",
				ResponseCode: "00",
				Message:      "Success",
				Timestamp:    "2026-01-28 12:00:00",
				BillDetails:  []domain.BillDetailDomain{{Name: "Nama", Value: "Budi", IsPII: false, IsShow: true}},
			},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "response_code",
		},
		{
			name: "failed to parse request body",
			body: dto.CheckStatusRequestDto{},
			mockResp: &domain.CheckStatusResponseDomain{
				BillDetails: []domain.BillDetailDomain{{Name: "", Value: "", IsPII: false, IsShow: false}},
			},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "response_code",
		},
		{
			name: "error from usecase",
			body: dto.CheckStatusRequestDto{
				RefID:     "ref-2",
				Timestamp: "2026-01-28 13:00:00",
				Category:  "cat-2",
			},
			mockResp:   nil,
			mockErr:    assert.AnError,
			wantStatus: 200,
			wantBody:   "response_code",
		},
		{
			name: "all fields empty",
			body: dto.CheckStatusRequestDto{},
			mockResp: &domain.CheckStatusResponseDomain{
				BillDetails: []domain.BillDetailDomain{{Name: "", Value: "", IsPII: false, IsShow: false}},
			},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "response_code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			handler := newTestCheckStatusHandler(mockUsecase, &mockLogger{})
			mockUsecase.CheckStatusFunc = func(ctx context.Context, req *domain.CheckStatusRequestDomain) (*domain.CheckStatusResponseDomain, error) {
				return tt.mockResp, tt.mockErr
			}
			app.Post("/check-status", func(c *fiber.Ctx) error {
				body, _ := json.Marshal(tt.body)
				c.Locals("decryptedBody", body)
				return handler.CheckStatus(c)
			})
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/check-status", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			assert.Contains(t, buf.String(), tt.wantBody)
		})
	}
}
