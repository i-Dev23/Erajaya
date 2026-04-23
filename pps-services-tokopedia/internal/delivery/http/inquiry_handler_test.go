package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestInquiryHandler_handleError(t *testing.T) {
	app := fiber.New()
	h := NewInquiryHandler(&mockInquiryUsecase{}, &mockLogger{})
	app.Get("/", func(c *fiber.Ctx) error {
		startTime := time.Now()
		err := h.handleError(c, startTime, "42", "error message", "PCODE", "CNUM", assert.AnError)
		assert.NoError(t, err)
		return nil
	})
	req := httptest.NewRequest("GET", "/", nil)
	_, err := app.Test(req)
	assert.NoError(t, err)
}

func Test_newInquiryResponseOptimized(t *testing.T) {
	h := NewInquiryHandler(&mockInquiryUsecase{}, &mockLogger{})
	r := h.convertDomainToDtoOptimized(&domain.InquiryResponseDomain{
		ProductCode:  "PCODE",
		ClientNumber: "CNUM",
		ResponseCode: "42",
		Message:      "msg",
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
	})
	assert.Equal(t, "PCODE", r.ProductCode)
	assert.Equal(t, "CNUM", r.ClientNumber)
	assert.NotEmpty(t, r.ResponseCode)
	assert.NotEmpty(t, r.Message)
	assert.NotEmpty(t, r.Timestamp)
}

func Test_convertDomainToDtoOptimized(t *testing.T) {
	h := NewInquiryHandler(&mockInquiryUsecase{}, &mockLogger{})
	domainResp := &domain.InquiryResponseDomain{
		PartnerInquiryID: "pid",
		ClientNumber:     "cnum",
		ProductCode:      "pcode",
		ResponseCode:     "00",
		Message:          "msg",
		Timestamp:        "2026-01-28 12:00:00",
		TotalAmount:      12345,
		IsOpenAmount:     true,
		AdminFee:         123,
		BillDetails: []domain.InquiryBillDetail{
			{Name: "A", Value: "1", IsPII: false, IsShow: true},
		},
	}
	dtoResp := h.convertDomainToDtoOptimized(domainResp)
	assert.Equal(t, "pid", dtoResp.PartnerInquiryID)
	assert.Equal(t, "cnum", dtoResp.ClientNumber)
	assert.Equal(t, "pcode", dtoResp.ProductCode)
	assert.Equal(t, "00", dtoResp.ResponseCode)
	assert.Equal(t, "msg", dtoResp.Message)
	assert.NotEmpty(t, dtoResp.Timestamp)
	assert.Equal(t, 12345.0, dtoResp.TotalAmount)
	assert.Equal(t, true, dtoResp.IsOpenAmount)
	assert.Equal(t, 123.0, dtoResp.AdminFee)
	assert.Len(t, dtoResp.BillDetails, 1)
}

func Test_newInquiryResponse(t *testing.T) {
	r := newInquiryResponse("42", "PCODE", "CNUM")
	assert.Equal(t, "PCODE", r.ProductCode)
	assert.Equal(t, "CNUM", r.ClientNumber)
}

func Test_convertDomainToDto(t *testing.T) {
	domainResp := &domain.InquiryResponseDomain{
		PartnerInquiryID: "pid",
		ClientNumber:     "cnum",
		ProductCode:      "pcode",
		ResponseCode:     "00",
		Message:          "msg",
		Timestamp:        "2026-01-28 12:00:00",
		TotalAmount:      12345,
		IsOpenAmount:     true,
		AdminFee:         123,
		BillDetails: []domain.InquiryBillDetail{
			{Name: "A", Value: "1", IsPII: false, IsShow: true},
		},
	}
	dtoResp := convertDomainToDto(domainResp)
	assert.Equal(t, "pid", dtoResp.PartnerInquiryID)
	assert.Equal(t, "cnum", dtoResp.ClientNumber)
	assert.Equal(t, "pcode", dtoResp.ProductCode)
	assert.Equal(t, "00", dtoResp.ResponseCode)
	assert.Equal(t, "msg", dtoResp.Message)
	assert.NotEmpty(t, dtoResp.Timestamp)
	assert.Equal(t, 12345.0, dtoResp.TotalAmount)
	assert.Equal(t, true, dtoResp.IsOpenAmount)
	assert.Equal(t, 123.0, dtoResp.AdminFee)
	assert.Len(t, dtoResp.BillDetails, 1)
}

// ...existing code...

// Ensure mockInquiryUsecase implements domain.InquiryUsecase
var _ domain.InquiryUsecase = (*mockInquiryUsecase)(nil)

type mockInquiryUsecase struct {
	InquiryFunc func(ctx context.Context, req *domain.InquiryRequestDomain) (*domain.InquiryResponseDomain, error)
}

// Implements domain.InquiryUsecase
func (m *mockInquiryUsecase) Inquiry(ctx context.Context, req *domain.InquiryRequestDomain) (*domain.InquiryResponseDomain, error) {
	if m.InquiryFunc != nil {
		return m.InquiryFunc(ctx, req)
	}
	return nil, nil
}

func TestInquiryHandler_Inquiry(t *testing.T) {
	mockUsecase := &mockInquiryUsecase{}
	tests := []struct {
		name       string
		body       dto.InquiryRequestDto
		mockResp   *domain.InquiryResponseDomain
		mockErr    error
		wantStatus int
		wantBody   string
	}{
		{
			name: "success inquiry",
			body: dto.InquiryRequestDto{
				RefID:        "ref-1",
				ClientNumber: "12345",
				Category:     "cat-1",
				Rsid:         "TOKOPEDIA",
				ProductCode:  "PROD1",
				Timestamp:    "2026-01-28 12:00:00",
			},
			mockResp: &domain.InquiryResponseDomain{
				PartnerInquiryID: "inq-1",
				ClientNumber:     "12345",
				ProductCode:      "PROD1",
				ResponseCode:     "00",
				Message:          "Success",
				Timestamp:        "2026-01-28 12:00:00",
				TotalAmount:      10000,
				IsOpenAmount:     false,
				AdminFee:         1000,
				BillDetails: []domain.InquiryBillDetail{
					{Name: "Nama", Value: "Budi", IsPII: false, IsShow: true},
				},
			},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "\"response_code\"",
		},
		{
			name: "error from usecase",
			body: dto.InquiryRequestDto{
				RefID:        "ref-2",
				ClientNumber: "54321",
				Category:     "cat-2",
				Rsid:         "TOKOPEDIA",
				ProductCode:  "PROD2",
				Timestamp:    "2026-01-28 13:00:00",
			},
			mockResp:   nil,
			mockErr:    assert.AnError,
			wantStatus: 200,
			wantBody:   "Server error",
		},
		{
			name: "empty product code and client number",
			body: dto.InquiryRequestDto{
				RefID:     "ref-3",
				Category:  "cat-3",
				Rsid:      "TOKOPEDIA",
				Timestamp: "2026-01-28 14:00:00",
			},
			mockResp: &domain.InquiryResponseDomain{
				PartnerInquiryID: "inq-3",
				ClientNumber:     "",
				ProductCode:      "",
				ResponseCode:     "00",
				Message:          "OK",
				Timestamp:        "2026-01-28 14:00:00",
				TotalAmount:      0,
				IsOpenAmount:     false,
				AdminFee:         0,
				BillDetails:      []domain.InquiryBillDetail{},
			},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "\"product_code\":\"\"",
		},
		{
			name: "response code not found",
			body: dto.InquiryRequestDto{
				RefID:        "ref-4",
				ClientNumber: "99999",
				Category:     "cat-x",
				Rsid:         "TOKOPEDIA",
				ProductCode:  "PROD-X",
				Timestamp:    "2026-01-28 15:00:00",
			},
			mockResp: &domain.InquiryResponseDomain{
				PartnerInquiryID: "inq-4",
				ClientNumber:     "99999",
				ProductCode:      "PROD-X",
				ResponseCode:     "99",
				Message:          "Unknown",
				Timestamp:        "2026-01-28 15:00:00",
				TotalAmount:      0,
				IsOpenAmount:     false,
				AdminFee:         0,
				BillDetails:      []domain.InquiryBillDetail{},
			},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "PROD-X",
		},
		{
			name: "multiple bill details",
			body: dto.InquiryRequestDto{
				RefID:        "ref-5",
				ClientNumber: "88888",
				Category:     "cat-y",
				Rsid:         "TOKOPEDIA",
				ProductCode:  "PROD-Y",
				Timestamp:    "2026-01-28 16:00:00",
			},
			mockResp: &domain.InquiryResponseDomain{
				PartnerInquiryID: "inq-5",
				ClientNumber:     "88888",
				ProductCode:      "PROD-Y",
				ResponseCode:     "00",
				Message:          "Success",
				Timestamp:        "2026-01-28 16:00:00",
				TotalAmount:      50000,
				IsOpenAmount:     true,
				AdminFee:         500,
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
			name:       "all fields empty",
			body:       dto.InquiryRequestDto{},
			mockResp:   &domain.InquiryResponseDomain{},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "response_code",
		},
		{
			name: "timestamp empty",
			body: dto.InquiryRequestDto{
				RefID:        "ref-6",
				ClientNumber: "77777",
				Category:     "cat-z",
				Rsid:         "TOKOPEDIA",
				ProductCode:  "PROD-Z",
			},
			mockResp: &domain.InquiryResponseDomain{
				PartnerInquiryID: "inq-6",
				ClientNumber:     "77777",
				ProductCode:      "PROD-Z",
				ResponseCode:     "00",
				Message:          "Success",
				TotalAmount:      0,
				IsOpenAmount:     false,
				AdminFee:         0,
				BillDetails:      []domain.InquiryBillDetail{},
			},
			mockErr:    nil,
			wantStatus: 200,
			wantBody:   "PROD-Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			handler := NewInquiryHandler(mockUsecase, &mockLogger{})
			mockUsecase.InquiryFunc = func(ctx context.Context, req *domain.InquiryRequestDomain) (*domain.InquiryResponseDomain, error) {
				return tt.mockResp, tt.mockErr
			}
			app.Post("/inquiry", func(c *fiber.Ctx) error {
				// Simulasi error parsing: set Locals ke tipe yang salah (bukan []byte)
				if tt.name == "failed to parse request body" {
					c.Locals("decryptedBody", 12345) // tipe salah, akan error saat type assertion
				} else {
					body, _ := json.Marshal(tt.body)
					c.Locals("decryptedBody", body)
				}
				return handler.Inquiry(c)
			})
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/inquiry", bytes.NewReader(body))
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
