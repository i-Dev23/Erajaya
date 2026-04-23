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

type CheckStatusHandler struct {
	checkStatusUsecase domain.CheckStatusUsecase
	logger             service.Logger
}

func NewCheckStatusHandler(checkStatusUsecase domain.CheckStatusUsecase, logger service.Logger) *CheckStatusHandler {
	return &CheckStatusHandler{
		checkStatusUsecase: checkStatusUsecase,
		logger:             logger,
	}
}

func (h *CheckStatusHandler) RegisterRoutes(router fiber.Router) {
	router.Post("/check-status", h.CheckStatus)
}

func (h *CheckStatusHandler) CheckStatus(c *fiber.Ctx) error {
	startTime := time.Now()

	ctx := c.UserContext()
	if ctx == nil {
		ctx = context.Background()
	}
	decryptedBody := c.Locals("decryptedBody").([]byte)
	h.logger.Info("Check Status API Request Started",
		"endpoint", "/check-status",
		"method", "POST",
		"timestamp", startTime.Format(timeFormat),
		"request_body", string(decryptedBody))

	//parse request body
	var reqDto dto.CheckStatusRequestDto
	if err := json.Unmarshal(decryptedBody, &reqDto); err != nil {
		duration := time.Since(startTime)
		h.logger.Error("failed to parse request body", "error", err, "rawBody", string(decryptedBody), "duration_ms", duration.Milliseconds())
		rc, _ := utils.GetResponseCode("42")
		response := newCheckStatusResponse(rc.Code, rc.Message, reqDto.RefID, "", "")
		h.logger.Info("Check Status API Response",
			"endpoint", "/check-status",
			"status_code", http.StatusOK,
			"duration_ms", duration.Milliseconds(),
			"response_body", response)
		return c.Status(http.StatusOK).JSON(response)
	}

	//call check status usecase
	h.logger.Info("Check Status request received", "RefID", reqDto.RefID, "Category", reqDto.Category)
	resp, err := h.checkStatusUsecase.CheckStatus(ctx, &domain.CheckStatusRequestDomain{
		RefID:     reqDto.RefID,
		Timestamp: reqDto.Timestamp,
		Category:  reqDto.Category,
	})
	if err != nil {
		duration := time.Since(startTime)
		h.logger.Error("Failed to check status", "error", err, "duration_ms", duration.Milliseconds())
		rc, _ := utils.GetResponseCode("62")
		response := newCheckStatusResponse(rc.Code, rc.Message, reqDto.RefID, "", "")
		h.logger.Info("Check Status API Response",
			"endpoint", "/check-status",
			"status_code", http.StatusOK,
			"duration_ms", duration.Milliseconds(),
			"response_body", response)
		return c.Status(http.StatusOK).JSON(response)
	}
	h.logger.Info("Check Status response", "response", resp)

	duration := time.Since(startTime)
	response := convertCheckStatusDomainToDto(resp)
	h.logger.Info("Check Status API Response",
		"endpoint", "/check-status",
		"status_code", http.StatusOK,
		"duration_ms", duration.Milliseconds(),
		"response_body", response)
	return c.Status(http.StatusOK).JSON(response)
}

func newCheckStatusResponse(code, message, refID, clientNumber, productCode string) dto.CheckStatusResponseDto {
	rid := refID

	cn := clientNumber

	pc := productCode

	return dto.CheckStatusResponseDto{
		RefID:        rid,
		ClientNumber: cn,
		ProductCode:  pc,
		ResponseCode: code,
		Message:      message,
		Timestamp:    time.Now().Format(timeFormat),
	}
}

func convertCheckStatusDomainToDto(domain *domain.CheckStatusResponseDomain) dto.CheckStatusResponseDto {
	billDetails := make([]dto.BillDetailDto, len(domain.BillDetails))
	for i, detail := range domain.BillDetails {
		billDetails[i] = dto.BillDetailDto{
			Name:   detail.Name,
			Value:  detail.Value,
			IsPII:  &detail.IsPII,
			IsShow: &detail.IsShow,
		}
	}
	return dto.CheckStatusResponseDto{
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
