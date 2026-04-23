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

// HealthCheckHandler handles health check related HTTP requests.
type HealthCheckHandler struct {
	healthCheckUsecase domain.HealthCheckUsecase
	logger             service.Logger
}

// NewHealthCheckHandler creates a new HealthCheckHandler with the given usecase.
func NewHealthCheckHandler(healthCheckUsecase domain.HealthCheckUsecase, logger service.Logger) *HealthCheckHandler {
	return &HealthCheckHandler{
		healthCheckUsecase: healthCheckUsecase,
		logger:             logger,
	}
}

// RegisterRoutes registers the health check handler routes to the given Fiber app/group.
func (h *HealthCheckHandler) RegisterRoutes(router fiber.Router) {
	router.Post("/health", h.CheckHealth)
	router.Post("/health/deep", h.DeepHealth)
}

// CheckHealth handles the POST /health-check route.
func (h *HealthCheckHandler) CheckHealth(c *fiber.Ctx) error {
	startTime := time.Now()
	var req dto.HealthCheckRequestDto

	decryptedBody := c.Locals("decryptedBody").([]byte)
	h.logger.Info("Health Check API Request Started",
		"endpoint", "/health",
		"method", "POST",
		"timestamp", startTime.Format(timeFormat),
		"request_body", string(decryptedBody))

	if err := json.Unmarshal(decryptedBody, &req); err != nil {
		duration := time.Since(startTime)
		h.logger.Error("failed to parse request body", "error", err, "duration_ms", duration.Milliseconds())
		rc, _ := utils.GetResponseCode("42")
		response := newHealthCheckResponse(rc.Code, rc.Message)
		h.logger.Info("Health Check API Response",
			"endpoint", "/health",
			"status_code", http.StatusOK,
			"duration_ms", duration.Milliseconds(),
			"response_body", response)
		return c.Status(http.StatusOK).JSON(response)
	}

	ctx := c.UserContext()
	if ctx == nil {
		ctx = context.Background()
	}

	// Map DTO to domain entity
	domainReq := &domain.HealthCheckRequestDomain{
		Timestamp: req.Timestamp,
	}

	// Make sure the response returned is according to the dto.HealthCheckResponseDto structure
	resp, err := h.healthCheckUsecase.HealthCheck(ctx, domainReq)
	if err != nil {
		duration := time.Since(startTime)
		h.logger.Error("Health check failed", "error", err, "duration_ms", duration.Milliseconds())
		status, res := mapHealthCheckErrorToResponse(err)
		h.logger.Info("Health Check API Response",
			"endpoint", "/health",
			"status_code", status,
			"duration_ms", duration.Milliseconds(),
			"response_body", res)
		return c.Status(status).JSON(res)
	}

	duration := time.Since(startTime)
	response := newHealthCheckResponse(resp.ResponseCode, resp.Message)
	h.logger.Info("Health Check API Response",
		"endpoint", "/health",
		"status_code", http.StatusOK,
		"duration_ms", duration.Milliseconds(),
		"response_body", response)
	return c.Status(http.StatusOK).JSON(response)
}

// DeepHealth handles the POST /health/deep route.
func (h *HealthCheckHandler) DeepHealth(c *fiber.Ctx) error {
	startTime := time.Now()
	var req dto.HealthCheckRequestDto

	decryptedBody := c.Locals("decryptedBody").([]byte)
	h.logger.Info("Deep Health Check API Request Started",
		"endpoint", "/health/deep",
		"method", "POST",
		"timestamp", startTime.Format(timeFormat),
		"request_body", string(decryptedBody))

	if err := json.Unmarshal(decryptedBody, &req); err != nil {
		duration := time.Since(startTime)
		h.logger.Error("failed to parse request body", "error", err, "duration_ms", duration.Milliseconds())
		rc, _ := utils.GetResponseCode("42")
		response := newDeepHealthCheckResponse(rc.Code, rc.Message, nil)
		h.logger.Info("Deep Health Check API Response",
			"endpoint", "/health/deep",
			"status_code", http.StatusOK,
			"duration_ms", duration.Milliseconds(),
			"response_body", response)
		return c.Status(http.StatusOK).JSON(response)
	}

	ctx := c.UserContext()
	if ctx == nil {
		ctx = context.Background()
	}

	domainReq := &domain.HealthCheckRequestDomain{
		Timestamp: req.Timestamp,
	}

	resp, err := h.healthCheckUsecase.DeepHealthCheck(ctx, domainReq)
	if err != nil {
		duration := time.Since(startTime)
		h.logger.Error("Deep health check failed", "error", err, "duration_ms", duration.Milliseconds())
		status, res := mapDeepHealthCheckErrorToResponse(err)
		h.logger.Info("Deep Health Check API Response",
			"endpoint", "/health/deep",
			"status_code", status,
			"duration_ms", duration.Milliseconds(),
			"response_body", res)
		return c.Status(status).JSON(res)
	}

	duration := time.Since(startTime)
	response := newDeepHealthCheckResponse(resp.ResponseCode, resp.Message, resp.Services)
	h.logger.Info("Deep Health Check API Response",
		"endpoint", "/health/deep",
		"status_code", http.StatusOK,
		"duration_ms", duration.Milliseconds(),
		"response_body", response)
	return c.Status(http.StatusOK).JSON(response)
}

func newHealthCheckResponse(code, message string) dto.HealthCheckResponseDto {
	return dto.HealthCheckResponseDto{
		ResponseCode: code,
		Message:      message,
		Timestamp:    time.Now().Format(timeFormat),
	}
}

func newDeepHealthCheckResponse(code, message string, services map[string]string) dto.DeepHealthCheckResponseDto {
	return dto.DeepHealthCheckResponseDto{
		ResponseCode: code,
		Message:      message,
		Timestamp:    time.Now().Format(timeFormat),
		Services:     services,
	}
}

// mapHealthCheckErrorToResponse mapping error usecase → http status + response DTO
func mapHealthCheckErrorToResponse(err error) (int, dto.HealthCheckResponseDto) {
	switch err {
	case utils.ErrInvalidParameter:
		rc, _ := utils.GetResponseCode("42")
		return http.StatusOK, newHealthCheckResponse(rc.Code, rc.Message)
	default:
		rc, _ := utils.GetResponseCode("62")
		return http.StatusOK, newHealthCheckResponse(rc.Code, rc.Message)
	}
}

// mapDeepHealthCheckErrorToResponse mapping error usecase → http status + response DTO
func mapDeepHealthCheckErrorToResponse(err error) (int, dto.DeepHealthCheckResponseDto) {
	switch err {
	case utils.ErrInvalidParameter:
		rc, _ := utils.GetResponseCode("42")
		return http.StatusOK, newDeepHealthCheckResponse(rc.Code, rc.Message, nil)
	default:
		rc, _ := utils.GetResponseCode("62")
		return http.StatusOK, newDeepHealthCheckResponse(rc.Code, rc.Message, nil)
	}
}
