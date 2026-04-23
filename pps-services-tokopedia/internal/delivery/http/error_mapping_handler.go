package http

import (
	"context"
	"net/http"
	"time"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"

	"github.com/gofiber/fiber/v2"
)

// ErrorMappingHandler handles error message mapping requests
type ErrorMappingHandler struct {
	logger service.Logger
	repo   domain.ErrorMessageMappingRepository
}

// NewErrorMappingHandler creates a new ErrorMappingHandler
func NewErrorMappingHandler(logger service.Logger, repo domain.ErrorMessageMappingRepository) *ErrorMappingHandler {
	return &ErrorMappingHandler{
		logger: logger,
		repo:   repo,
	}
}

// RegisterRoutes registers the error mapping routes to the given Fiber app/group (NO middleware)
func (h *ErrorMappingHandler) RegisterRoutes(router fiber.Router) {
	router.Post("/error-mapping", h.MapErrorMessage)
}

// MapErrorMessage maps an error message to a response code and message
// POST /internal/error-mapping
// Request: { "error_message": "...", "system_type": "ultima|oracle" }
// Response: { "response_code": "63", "message": "Biller maintenance" }
func (h *ErrorMappingHandler) MapErrorMessage(c *fiber.Ctx) error {
	startTime := time.Now()

	var reqDto dto.ErrorMappingRequestDto

	// Parse request body
	if err := c.BodyParser(&reqDto); err != nil {
		h.logger.Warn("Failed to parse error mapping request",
			"error", err)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if reqDto.ErrorMessage == "" || reqDto.SystemType == "" {
		h.logger.Warn("Error mapping request missing required fields",
			"error_message", reqDto.ErrorMessage,
			"system_type", reqDto.SystemType)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "error_message and system_type are required",
		})
	}

	// Normalize system_type
	systemType := reqDto.SystemType
	if systemType != "ultima" && systemType != "oracle" {
		h.logger.Warn("Invalid system_type",
			"system_type", systemType)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "system_type must be 'ultima' or 'oracle'",
		})
	}

	h.logger.Info("Mapping error message",
		"system_type", systemType,
		"error_message", reqDto.ErrorMessage)

	// Try DB if repository is available
	var responseCode, message string
	if h.repo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		mapping, err := h.repo.GetMapping(ctx, systemType, reqDto.ErrorMessage)
		if err == nil && mapping != nil && mapping.Found {
			if rc, ok := utils.GetResponseCode(mapping.ResponseCode); ok {
				responseCode, message = rc.Code, rc.Message
			}
		}
	}

	// If DB did not provide a mapping, return code 99 (Other error)
	if responseCode == "" {
		if rc, ok := utils.GetResponseCode("99"); ok {
			responseCode, message = rc.Code, rc.Message
		} else {
			responseCode, message = "99", "Other error"
		}
	}

	// Build response
	respDto := dto.ErrorMappingResponseDto{
		ResponseCode: responseCode,
		Message:      message,
	}

	duration := time.Since(startTime)
	h.logger.Info("Error mapping completed",
		"system_type", systemType,
		"response_code", responseCode,
		"duration_ms", duration.Milliseconds())

	return c.Status(http.StatusOK).JSON(respDto)
}
