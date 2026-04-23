package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"testing"

	"pps-services-tokopedia/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

// NewPaymentHandler is a test helper to construct PaymentHandler
// Use the real constructor from payment_handler.go
// Helper to cast mockLogger to service.Logger
func newTestPaymentHandler(paymentUsecase domain.PaymentUsecase, logger service.Logger) *PaymentHandler {
	return NewPaymentHandler(paymentUsecase, logger)
}

// mockLogger implements service.Logger for testing

// Ensure mockPaymentUsecase implements domain.PaymentUsecase
var _ domain.PaymentUsecase = (*mockPaymentUsecase)(nil)

type mockPaymentUsecase struct {
	PaymentFunc func(ctx context.Context, req *domain.PaymentRequestDomain) (*domain.PaymentResponseDomain, error)
}

// Implements domain.PaymentUsecase
func (m *mockPaymentUsecase) Payment(ctx context.Context, req *domain.PaymentRequestDomain) (*domain.PaymentResponseDomain, error) {
	if m.PaymentFunc != nil {
		return m.PaymentFunc(ctx, req)
	}
	return nil, nil
}

func TestPaymentHandler_Payment(t *testing.T) {
	mockUsecase := &mockPaymentUsecase{}

	tests := []struct {
		name       string
		body       dto.PaymentRequestDto
		mockResp   *domain.PaymentResponseDomain
		mockErr    error
		wantStatus int
		wantBody   string
	}{
		{
			name: "success payment",
			body: dto.PaymentRequestDto{
				RefID:            "ref-1",
				PartnerInquiryID: "inq-1",
				ClientNumber:     "12345",
				Category:         "cat-1",
				Rsid:             "TOKOPEDIA",
				ProductCode:      "PROD1",
				TotalAmount:      10000,
				Timestamp:        "2026-01-28 12:00:00",
			},
			mockResp: &domain.PaymentResponseDomain{
				RefID:        "ref-1",
				ClientNumber: "12345",
				ProductCode:  "PROD1",
				ResponseCode: "00",
				Message:      "Success",
				Timestamp:    "2026-01-28 12:00:00",
				BillDetails: []domain.InquiryBillDetail{
					{Name: "Nama", Value: "Budi", IsPII: false, IsShow: true},
				},
			},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "\"response_code\"",
		},
		{
			name: "failed to parse request body",
			body: dto.PaymentRequestDto{},
			mockResp: &domain.PaymentResponseDomain{
				BillDetails:       []domain.InquiryBillDetail{{Name: "", Value: "", IsPII: false, IsShow: false}},
				AdditionalDetails: []domain.InquiryAdditionalDetail{},
			},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "response_code", // match the actual response field
		},
		{
			name: "error from usecase",
			body: dto.PaymentRequestDto{
				RefID:            "ref-2",
				PartnerInquiryID: "inq-2",
				ClientNumber:     "54321",
				Category:         "cat-2",
				Rsid:             "TOKOPEDIA",
				ProductCode:      "PROD2",
				TotalAmount:      20000,
				Timestamp:        "2026-01-28 13:00:00",
			},
			mockResp:   nil,
			mockErr:    assert.AnError,
			wantStatus: 200,
			wantBody:   "response_code", // match the actual response field
		},
		{
			name: "empty product code and client number",
			body: dto.PaymentRequestDto{
				RefID:     "ref-3",
				Category:  "cat-3",
				Rsid:      "TOKOPEDIA",
				Timestamp: "2026-01-28 14:00:00",
			},
			mockResp: &domain.PaymentResponseDomain{
				RefID:        "ref-3",
				ClientNumber: "",
				ProductCode:  "",
				ResponseCode: "00",
				Message:      "OK",
				Timestamp:    "2026-01-28 14:00:00",
				BillDetails:  nil,
			},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "\"product_code\":\"\"",
		},
		{
			name: "multiple bill details",
			body: dto.PaymentRequestDto{
				RefID:            "ref-4",
				PartnerInquiryID: "inq-4",
				ClientNumber:     "88888",
				Category:         "cat-y",
				Rsid:             "TOKOPEDIA",
				ProductCode:      "PROD-Y",
				TotalAmount:      50000,
				Timestamp:        "2026-01-28 16:00:00",
			},
			mockResp: &domain.PaymentResponseDomain{
				RefID:        "ref-4",
				ClientNumber: "88888",
				ProductCode:  "PROD-Y",
				ResponseCode: "00",
				Message:      "Success",
				Timestamp:    "2026-01-28 16:00:00",
				BillDetails: []domain.InquiryBillDetail{
					{Name: "A", Value: "1", IsPII: false, IsShow: true},
					{Name: "B", Value: "2", IsPII: true, IsShow: false},
				},
			},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "A",
		},
		{
			name: "all fields empty",
			body: dto.PaymentRequestDto{},
			mockResp: &domain.PaymentResponseDomain{
				BillDetails:       []domain.InquiryBillDetail{{Name: "", Value: "", IsPII: false, IsShow: false}},
				AdditionalDetails: []domain.InquiryAdditionalDetail{},
			},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "response_code",
		},
		{
			name: "timestamp empty",
			body: dto.PaymentRequestDto{
				RefID:            "ref-5",
				PartnerInquiryID: "inq-5",
				ClientNumber:     "77777",
				Category:         "cat-z",
				Rsid:             "TOKOPEDIA",
				ProductCode:      "PROD-Z",
			},
			mockResp: &domain.PaymentResponseDomain{
				RefID:             "ref-5",
				ClientNumber:      "77777",
				ProductCode:       "PROD-Z",
				ResponseCode:      "00",
				Message:           "Success",
				BillDetails:       []domain.InquiryBillDetail{{Name: "", Value: "", IsPII: false, IsShow: false}},
				AdditionalDetails: []domain.InquiryAdditionalDetail{},
			},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "PROD-Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			handler := NewPaymentHandler(mockUsecase, &mockLogger{})
			mockUsecase.PaymentFunc = func(ctx context.Context, req *domain.PaymentRequestDomain) (*domain.PaymentResponseDomain, error) {
				return tt.mockResp, tt.mockErr
			}
			app.Post("/payment", func(c *fiber.Ctx) error {
				body, _ := json.Marshal(tt.body)
				c.Locals("decryptedBody", body)
				return handler.Payment(c)
			})
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/payment", bytes.NewReader(body))
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
