package http

import (
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/usecase"

	"github.com/gofiber/fiber/v2"
)

// HealthCheckV2Handler handles v2 health check requests
type HealthCheckV2Handler struct {
	healthCheckV2Usecase usecase.HealthCheckV2Usecase
	logger               service.Logger
}

// NewHealthCheckV2Handler creates a new instance of HealthCheckV2Handler
func NewHealthCheckV2Handler(healthCheckV2Usecase usecase.HealthCheckV2Usecase, logger service.Logger) *HealthCheckV2Handler {
	return &HealthCheckV2Handler{
		healthCheckV2Usecase: healthCheckV2Usecase,
		logger:               logger,
	}
}

// RegisterRoutes registers the health check v2 routes
func (h *HealthCheckV2Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/health", h.CheckHealth)
}

// CheckHealth handles the health check v2 request
func (h *HealthCheckV2Handler) CheckHealth(c *fiber.Ctx) error {
	// Check health of all services
	response := h.healthCheckV2Usecase.CheckHealth(c.Context())

	// Log the health check result
	h.logger.Info("Health check v2 completed",
		"response_code", response.ResponseCode,
		"message", response.Message,
		"timestamp", response.Timestamp)

	// Return the response
	return c.JSON(response)
}
