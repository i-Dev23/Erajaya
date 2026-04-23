package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"

	"github.com/gofiber/fiber/v2"
)

type BalanceHandler struct {
	balanceUsecase domain.BalanceUsecase
	logger         service.Logger
}

func NewBalanceHandler(balanceUsecase domain.BalanceUsecase, logger service.Logger) *BalanceHandler {
	return &BalanceHandler{
		balanceUsecase: balanceUsecase,
		logger:         logger,
	}
}

func (h *BalanceHandler) RegisterRoutes(router fiber.Router) {
	router.Post("/get-balance", h.GetBalance)
}

func (h *BalanceHandler) GetBalance(c *fiber.Ctx) error {
	startTime := time.Now()
	var reqDto dto.BalanceRequestDTO

	ctx := c.UserContext()
	if ctx == nil {
		ctx = context.Background()
	}

	decryptedVal := c.Locals("decryptedBody")
	decryptedBody, ok := decryptedVal.([]byte)
	if !ok || decryptedBody == nil {
		duration := time.Since(startTime)
		h.logger.Error("decryptedBody missing or invalid", "duration_ms", duration.Milliseconds())
		rc, _ := utils.GetResponseCode("42")
		return c.Status(http.StatusOK).JSON(dto.BalanceResponseDTO{
			ResponseCode: rc.Code,
			Message:      rc.Message,
			Timestamp:    time.Now().Format(timeFormat),
		})
	}

	h.logger.Info("Balance API Request Started",
		"endpoint", "/get-balance",
		"method", "POST",
		"timestamp", startTime.Format(timeFormat))

	if err := json.Unmarshal(decryptedBody, &reqDto); err != nil {
		duration := time.Since(startTime)
		h.logger.Error("failed to parse request body", "error", err, "duration_ms", duration.Milliseconds())
		rc, _ := utils.GetResponseCode("42")
		return c.Status(http.StatusOK).JSON(dto.BalanceResponseDTO{
			ResponseCode: rc.Code,
			Message:      rc.Message,
			Timestamp:    time.Now().Format(timeFormat),
		})
	}

	domainReq := &domain.BalanceGetRequestDomain{Timestamp: reqDto.Timestamp}

	resp, err := h.balanceUsecase.GetBalance(ctx, domainReq)
	if err != nil {
		duration := time.Since(startTime)
		h.logger.Error("Failed to get balance", "error", err, "duration_ms", duration.Milliseconds())
		// Return response from usecase if available
		if resp != nil {
			return c.Status(http.StatusOK).JSON(h.convertDomainToDTO(resp))
		}
		rc, _ := utils.GetResponseCode("62")
		return c.Status(http.StatusOK).JSON(dto.BalanceResponseDTO{
			ResponseCode: rc.Code,
			Message:      rc.Message,
			Timestamp:    time.Now().Format(timeFormat),
		})
	}

	return c.Status(http.StatusOK).JSON(h.convertDomainToDTO(resp))
}

func (h *BalanceHandler) convertDomainToDTO(resp *domain.BalanceGetResponseDomain) dto.BalanceResponseDTO {
	details := make([]dto.BalanceDetailDTO, 0, len(resp.BalanceDetails))
	for _, d := range resp.BalanceDetails {
		details = append(details, dto.BalanceDetailDTO{Label: d.Label, Value: d.Value})
	}

	return dto.BalanceResponseDTO{
		ResponseCode:   resp.ResponseCode,
		Message:        resp.Message,
		BalanceDetails: details,
		Timestamp:      resp.Timestamp,
	}
}
