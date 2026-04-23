package usecase

import (
	"context"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
	"time"
)

type healthCheckUsecaseImpl struct {
	logger          service.Logger
	redisClient     service.RedisClient
	postgresService service.PostgresService
	oracleService   service.OracleService
	rabbitMQService service.RabbitMQService
	ultimaService   domain.UltimaService
}

func NewHealthCheckUsecase(
	logger service.Logger,
	redisClient service.RedisClient,
	postgresService service.PostgresService,
	oracleService service.OracleService,
	rabbitMQService service.RabbitMQService,
	ultimaService domain.UltimaService,
) domain.HealthCheckUsecase {
	return &healthCheckUsecaseImpl{
		logger:          logger,
		redisClient:     redisClient,
		postgresService: postgresService,
		oracleService:   oracleService,
		rabbitMQService: rabbitMQService,
		ultimaService:   ultimaService,
	}
}

func (u *healthCheckUsecaseImpl) HealthCheck(ctx context.Context, req *domain.HealthCheckRequestDomain) (*domain.HealthCheckResponseDomain, error) {
	u.logger.Info("Health check executed")

	// Validate mandatory parameters
	if req.Timestamp == "" {
		u.logger.Warn("Missing timestamp in health check request")
		return nil, utils.ErrInvalidParameter
	}

	// Check health of all services
	servicesHealthy := true

	// Check PostgreSQL
	if err := u.postgresService.Ping(ctx); err != nil {
		u.logger.Error("PostgreSQL health check failed", "error", err)
		servicesHealthy = false
	}

	// Check Oracle with query instead of ping
	rows, err := u.oracleService.Query(ctx, "SELECT sysdate FROM dual")
	if err != nil {
		u.logger.Error("Oracle health check failed", "error", err)
		servicesHealthy = false
	} else {
		rows.Close()
	}

	// Check RabbitMQ
	if err := u.rabbitMQService.Ping(ctx); err != nil {
		u.logger.Error("RabbitMQ health check failed", "error", err)
		servicesHealthy = false
	}

	// Check Ultima
	if err := u.ultimaService.Ping(ctx); err != nil {
		u.logger.Error("Ultima health check failed", "error", err)
		servicesHealthy = false
	}

	// Determine response code and message based on service health
	var responseCode utils.ResponseCode
	if servicesHealthy {
		responseCode, _ = utils.GetResponseCode("00")
	} else {
		responseCode, _ = utils.GetResponseCode("62")
	}

	response := &domain.HealthCheckResponseDomain{
		ResponseCode: responseCode.Code,
		Message:      responseCode.Message,
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
	}
	return response, nil
}

// DeepHealthCheck checks dependencies and returns per-service status
func (u *healthCheckUsecaseImpl) DeepHealthCheck(ctx context.Context, req *domain.HealthCheckRequestDomain) (*domain.DeepHealthCheckResponseDomain, error) {
	u.logger.Info("Deep health check executed")

	if req.Timestamp == "" {
		u.logger.Warn("Missing timestamp in deep health check request")
		return nil, utils.ErrInvalidParameter
	}

	services := map[string]string{}
	servicesHealthy := true

	if err := u.postgresService.Ping(ctx); err != nil {
		u.logger.Error("PostgreSQL deep health check failed", "error", err)
		services["postgres"] = "unhealthy"
		servicesHealthy = false
	} else {
		services["postgres"] = "healthy"
	}

	rows, err := u.oracleService.Query(ctx, "SELECT sysdate FROM dual")
	if err != nil {
		u.logger.Error("Oracle deep health check failed", "error", err)
		services["oracle"] = "unhealthy"
		servicesHealthy = false
	} else {
		rows.Close()
		services["oracle"] = "healthy"
	}

	if err := u.rabbitMQService.Ping(ctx); err != nil {
		u.logger.Error("RabbitMQ deep health check failed", "error", err)
		services["rabbitmq"] = "unhealthy"
		servicesHealthy = false
	} else {
		services["rabbitmq"] = "healthy"
	}

	if err := u.ultimaService.Ping(ctx); err != nil {
		u.logger.Error("Ultima deep health check failed", "error", err)
		services["ultima"] = "unhealthy"
		servicesHealthy = false
	} else {
		services["ultima"] = "healthy"
	}

	if err := u.redisClient.Ping(ctx).Err(); err != nil {
		u.logger.Error("Redis deep health check failed", "error", err)
		services["redis"] = "unhealthy"
		servicesHealthy = false
	} else {
		services["redis"] = "healthy"
	}

	var responseCode utils.ResponseCode
	if servicesHealthy {
		responseCode, _ = utils.GetResponseCode("00")
	} else {
		responseCode, _ = utils.GetResponseCode("62")
	}

	response := &domain.DeepHealthCheckResponseDomain{
		ResponseCode: responseCode.Code,
		Message:      responseCode.Message,
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		Services:     services,
	}

	return response, nil
}
