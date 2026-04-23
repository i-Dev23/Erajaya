package http

import (
	"context"
	"encoding/json"
	"net/http"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

type PaymentHandler struct {
	paymentUsecase domain.PaymentUsecase
	logger         service.Logger
}

func NewPaymentHandler(paymentUsecase domain.PaymentUsecase, logger service.Logger) *PaymentHandler {
	return &PaymentHandler{
		paymentUsecase: paymentUsecase,
		logger:         logger,
	}
}

func (h *PaymentHandler) RegisterRoutes(router fiber.Router) {
	router.Post("/payment", h.Payment)
}

func (h *PaymentHandler) Payment(c *fiber.Ctx) error {
	startTime := time.Now()

	ctx := c.UserContext()
	if ctx == nil {
		ctx = context.Background()
	}
	decryptedBody, ok := c.Locals("decryptedBody").([]byte)
	if !ok || decryptedBody == nil {
		rc, _ := utils.GetResponseCode("42")
		response := newPaymentResponse(rc.Code, rc.Message, "", "", "")
		return c.Status(http.StatusOK).JSON(response)
	}
	h.logger.Info("Payment API Request Started",
		"endpoint", "/payment",
		"method", "POST",
		"timestamp", startTime.Format(timeFormat))

	//parse request body
	var reqDto dto.PaymentRequestDto
	if err := json.Unmarshal(decryptedBody, &reqDto); err != nil {
		duration := time.Since(startTime)
		h.logger.Error("failed to parse request body", "error", err, "duration_ms", duration.Milliseconds())
		rc, _ := utils.GetResponseCode("42")
		response := newPaymentResponse(rc.Code, rc.Message, reqDto.RefID, reqDto.ClientNumber, reqDto.ProductCode)
		h.logger.Info("Payment API Response",
			"endpoint", "/payment",
			"status_code", http.StatusOK,
			"duration_ms", duration.Milliseconds(),
			"response_body", response)
		return c.Status(http.StatusOK).JSON(response)
	}

	// Get X-Real-IP header
	clientIP := c.Get("X-Real-IP")
	if clientIP == "" {
		clientIP = c.IP() // fallback to remote IP
	}

	//call payment usecase
	h.logger.Info("Payment request received", "RefID", reqDto.RefID, "PartnerInquiryID", reqDto.PartnerInquiryID, "ClientNumber", reqDto.ClientNumber, "ProductCode", reqDto.ProductCode, "TotalAmount", reqDto.TotalAmount, "ClientIP", clientIP)
	resp, err := h.paymentUsecase.Payment(ctx, &domain.PaymentRequestDomain{
		RefID:            reqDto.RefID,
		PartnerInquiryID: reqDto.PartnerInquiryID,
		Category:         reqDto.Category,
		Rsid:             reqDto.Rsid,
		ClientNumber:     reqDto.ClientNumber,
		ProductCode:      reqDto.ProductCode,
		TotalAmount:      reqDto.TotalAmount,
		Timestamp:        reqDto.Timestamp,
		ClientIP:         clientIP,
	})

	if err != nil {
		duration := time.Since(startTime)
		h.logger.Error("Failed to payment", "error", err, "duration_ms", duration.Milliseconds())
		rc, _ := utils.GetResponseCode("62")
		response := newPaymentResponse(rc.Code, rc.Message, reqDto.RefID, reqDto.ClientNumber, reqDto.ProductCode)
		h.logger.Info("Payment API Response",
			"endpoint", "/payment",
			"status_code", http.StatusOK,
			"duration_ms", duration.Milliseconds(),
			"response_body", response)
		return c.Status(http.StatusOK).JSON(response)
	}
	h.logger.Info("Payment response", "response", resp)

	duration := time.Since(startTime)
	response := convertPaymentDomainToDto(resp)
	// Ensure timestamp is set if empty
	if response.Timestamp == "" {
		response.Timestamp = time.Now().Format(timeFormat)
	}
	h.logger.Info("Payment API Response",
		"endpoint", "/payment",
		"status_code", http.StatusOK,
		"duration_ms", duration.Milliseconds(),
		"response_body", response)
	return c.Status(http.StatusOK).JSON(response)
}

func newPaymentResponse(code, message, refID, clientNumber, productCode string) dto.PaymentResponseDto {
	rid := refID
	if rid == "" {
		rid = "-"
	}
	cn := clientNumber
	if cn == "" {
		cn = "-"
	}
	pc := productCode
	if pc == "" {
		pc = "-"
	}
	return dto.PaymentResponseDto{
		RefID:        rid,
		ClientNumber: cn,
		ProductCode:  pc,
		ResponseCode: code,
		Message:      message,
		Timestamp:    time.Now().Format(timeFormat),
	}
}

func convertPaymentDomainToDto(domain *domain.PaymentResponseDomain) dto.PaymentResponseDto {
	billDetails := make([]dto.BillDetailDto, len(domain.BillDetails))
	for i, detail := range domain.BillDetails {
		billDetails[i] = dto.BillDetailDto{
			Name:   detail.Name,
			Value:  detail.Value,
			IsPII:  &detail.IsPII,
			IsShow: &detail.IsShow,
		}
	}
	return dto.PaymentResponseDto{
		RefID:        domain.RefID,
		PartnerRefID: domain.PartnerRefID,
		ClientNumber: domain.ClientNumber,
		ProductCode:  domain.ProductCode,
		ResponseCode: domain.ResponseCode,
		Message:      domain.Message,
		AdminFee:     domain.AdminFee,
		TotalAmount:  domain.TotalAmount,
		Timestamp:    domain.Timestamp,
		BillDetails:  billDetails,
	}
}
