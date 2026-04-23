package usecase

import (
	"context"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
	"time"
)

// HealthCheckV2Usecase defines the interface for v2 health check usecase
type HealthCheckV2Usecase interface {
	CheckHealth(ctx context.Context) dto.HealthCheckV2ResponseDto
}

// healthCheckV2Usecase implements the HealthCheckV2Usecase interface
type healthCheckV2Usecase struct {
	redisClient     service.RedisClient
	postgresService service.PostgresService
	oracleService   service.OracleService
	rabbitMQService service.RabbitMQService
	ultimaService   domain.UltimaService
	logger          service.Logger
}

// NewHealthCheckV2Usecase creates a new instance of HealthCheckV2Usecase
func NewHealthCheckV2Usecase(
	redisClient service.RedisClient,
	postgresService service.PostgresService,
	oracleService service.OracleService,
	rabbitMQService service.RabbitMQService,
	ultimaService domain.UltimaService,
	logger service.Logger,
) HealthCheckV2Usecase {
	return &healthCheckV2Usecase{
		redisClient:     redisClient,
		postgresService: postgresService,
		oracleService:   oracleService,
		rabbitMQService: rabbitMQService,
		ultimaService:   ultimaService,
		logger:          logger,
	}
}

// CheckHealth checks the health of all services
func (h *healthCheckV2Usecase) CheckHealth(ctx context.Context) dto.HealthCheckV2ResponseDto {
	services := make(map[string]dto.ServiceHealth)

	// Check PostgreSQL
	postgresHealth := h.checkPostgreSQL(ctx)
	services["postgresql"] = postgresHealth

	// Check Oracle
	oracleHealth := h.checkOracle(ctx)
	services["oracle"] = oracleHealth

	// Check RabbitMQ
	rabbitMQHealth := h.checkRabbitMQ(ctx)
	services["rabbitmq"] = rabbitMQHealth

	// Check Ultima
	ultimaHealth := h.checkUltima(ctx)
	services["ultima"] = ultimaHealth

	// Determine overall health
	overall := h.determineOverallHealth(services)

	// Determine response code and message
	responseCode, message := h.getResponseCodeAndMessage(overall)

	return dto.HealthCheckV2ResponseDto{
		ResponseCode: responseCode,
		Message:      message,
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
	}
}

// checkRedis checks the health of Redis service
func (h *healthCheckV2Usecase) checkRedis(ctx context.Context) dto.ServiceHealth {
	start := time.Now()

	err := h.redisClient.Ping(ctx).Err()
	responseTime := time.Since(start).String()

	if err != nil {
		h.logger.Error("Redis health check failed", "error", err)
		return dto.ServiceHealth{
			Status:       "down",
			Message:      err.Error(),
			ResponseTime: responseTime,
		}
	}

	return dto.ServiceHealth{
		Status:       "up",
		Message:      "Redis is healthy",
		ResponseTime: responseTime,
	}
}

// checkPostgreSQL checks the health of PostgreSQL service
func (h *healthCheckV2Usecase) checkPostgreSQL(ctx context.Context) dto.ServiceHealth {
	start := time.Now()

	err := h.postgresService.Ping(ctx)
	responseTime := time.Since(start).String()

	if err != nil {
		h.logger.Error("PostgreSQL health check failed", "error", err)
		return dto.ServiceHealth{
			Status:       "down",
			Message:      err.Error(),
			ResponseTime: responseTime,
		}
	}

	return dto.ServiceHealth{
		Status:       "up",
		Message:      "PostgreSQL is healthy",
		ResponseTime: responseTime,
	}
}

// checkOracle checks the health of Oracle service
func (h *healthCheckV2Usecase) checkOracle(ctx context.Context) dto.ServiceHealth {
	start := time.Now()

	// Use query instead of ping for more reliable health check
	rows, err := h.oracleService.Query(ctx, "SELECT sysdate FROM dual")
	responseTime := time.Since(start).String()

	if err != nil {
		h.logger.Error("Oracle health check failed", "error", err)
		return dto.ServiceHealth{
			Status:       "down",
			Message:      err.Error(),
			ResponseTime: responseTime,
		}
	}
	rows.Close()

	return dto.ServiceHealth{
		Status:       "up",
		Message:      "Oracle is healthy",
		ResponseTime: responseTime,
	}
}

// checkRabbitMQ checks the health of RabbitMQ service
func (h *healthCheckV2Usecase) checkRabbitMQ(ctx context.Context) dto.ServiceHealth {
	start := time.Now()

	err := h.rabbitMQService.Ping(ctx)
	responseTime := time.Since(start).String()

	if err != nil {
		h.logger.Error("RabbitMQ health check failed", "error", err)
		return dto.ServiceHealth{
			Status:       "down",
			Message:      err.Error(),
			ResponseTime: responseTime,
		}
	}

	return dto.ServiceHealth{
		Status:       "up",
		Message:      "RabbitMQ is healthy",
		ResponseTime: responseTime,
	}
}

// checkUltima checks the health of Ultima service
func (h *healthCheckV2Usecase) checkUltima(ctx context.Context) dto.ServiceHealth {
	start := time.Now()

	err := h.ultimaService.Ping(ctx)
	responseTime := time.Since(start).String()

	if err != nil {
		h.logger.Error("Ultima health check failed", "error", err)
		return dto.ServiceHealth{
			Status:       "down",
			Message:      err.Error(),
			ResponseTime: responseTime,
		}
	}

	return dto.ServiceHealth{
		Status:       "up",
		Message:      "Ultima is healthy",
		ResponseTime: responseTime,
	}
}

// determineOverallHealth determines the overall health status
func (h *healthCheckV2Usecase) determineOverallHealth(services map[string]dto.ServiceHealth) string {
	for _, service := range services {
		if service.Status == "down" {
			return "down"
		}
	}
	return "up"
}

// getResponseCodeAndMessage returns the appropriate response code and message based on overall health
func (h *healthCheckV2Usecase) getResponseCodeAndMessage(overall string) (string, string) {
	if overall == "up" {
		responseCode, _ := utils.GetResponseCode("00")
		return responseCode.Code, responseCode.Message
	}
	responseCode, _ := utils.GetResponseCode("62")
	return responseCode.Code, responseCode.Message
}
