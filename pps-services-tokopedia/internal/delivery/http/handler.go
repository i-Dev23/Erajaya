package http

import (
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/middleware"
	"pps-services-tokopedia/internal/service"

	"github.com/gofiber/fiber/v2"
)

// timeFormat is the format of the timestamp
var (
	timeFormat = "2006-01-02 15:04:05"
)

// Handler manages all HTTP handlers
type Handler struct {
	logger                  service.Logger
	redisClient             service.RedisClient
	digitalSignatureService service.DigitalSignatureService
	cryptoService           service.CryptoService
	productRepo             domain.ProductRepository
	// Handler instances (injected via Wire)
	tokenHandler        *TokenHandler
	balanceHandler      *BalanceHandler
	healthCheckHandler  *HealthCheckHandler
	inquiryHandler      *InquiryHandler
	paymentHandler      *PaymentHandler
	checkStatusHandler  *CheckStatusHandler
	errorMappingHandler *ErrorMappingHandler
	cryptoHandler       *DecryptSignatureGenerateHandler
	// Middleware dependencies
	httpLoggingRepo domain.HTTPLoggingRepository
	tokenUsecase    domain.TokenUsecase
}

// NewHandler creates a new HTTP handler with all sub-handlers injected via Wire
func NewHandler(
	logger service.Logger,
	redisClient service.RedisClient,
	digitalSignatureService service.DigitalSignatureService,
	cryptoService service.CryptoService,
	productRepo domain.ProductRepository,
	tokenHandler *TokenHandler,
	balanceHandler *BalanceHandler,
	healthCheckHandler *HealthCheckHandler,
	inquiryHandler *InquiryHandler,
	paymentHandler *PaymentHandler,
	checkStatusHandler *CheckStatusHandler,
	errorMappingHandler *ErrorMappingHandler,
	cryptoHandler *DecryptSignatureGenerateHandler,
	httpLoggingRepo domain.HTTPLoggingRepository,
	tokenUsecase domain.TokenUsecase,
) *Handler {
	return &Handler{
		logger:                  logger,
		redisClient:             redisClient,
		digitalSignatureService: digitalSignatureService,
		cryptoService:           cryptoService,
		productRepo:             productRepo,
		tokenHandler:            tokenHandler,
		balanceHandler:          balanceHandler,
		healthCheckHandler:      healthCheckHandler,
		inquiryHandler:          inquiryHandler,
		paymentHandler:          paymentHandler,
		checkStatusHandler:      checkStatusHandler,
		errorMappingHandler:     errorMappingHandler,
		cryptoHandler:           cryptoHandler,
		httpLoggingRepo:         httpLoggingRepo,
		tokenUsecase:            tokenUsecase,
	}
}

// RegisterRoutes registers all routes with middleware
func (h *Handler) RegisterRoutes(app *fiber.App) {
	// Configure HTTP logging middleware
	httpLoggingConfig := middleware.DefaultHTTPLoggingConfig(h.logger, h.httpLoggingRepo, h.cryptoService)

	// Protected routes (with middleware)
	protected := app.Group("/auth",
		middleware.DatabaseErrorHandlingMiddleware(h.logger),
		middleware.IPWhitelistMiddleware(h.redisClient, h.productRepo, h.cryptoService, h.digitalSignatureService, h.logger),
		middleware.RateLimitMiddleware(h.redisClient, h.cryptoService, h.digitalSignatureService, h.logger),
		middleware.DecryptRequestMiddleware(h.cryptoService, h.digitalSignatureService, h.logger),
		middleware.HTTPLoggingMiddlewareWithConfig(httpLoggingConfig),
		middleware.EncryptResponseMiddleware(h.cryptoService, h.digitalSignatureService, h.logger),
	)
	h.tokenHandler.RegisterRoutes(protected)

	// Balance routes now registered under /api/v1 group (with bearer token and crypto middleware)
	// Remove separate /balance group, add to /api/v1 group below

	// API routes (with bearer token + crypto middleware)
	api := app.Group("/api/v1",
		middleware.DatabaseErrorHandlingMiddleware(h.logger),
		middleware.IPWhitelistMiddleware(h.redisClient, h.productRepo, h.cryptoService, h.digitalSignatureService, h.logger),
		middleware.RateLimitMiddleware(h.redisClient, h.cryptoService, h.digitalSignatureService, h.logger),
		middleware.CheckBearerTokenMiddleware(h.tokenUsecase, h.cryptoService, h.digitalSignatureService, h.logger),
		middleware.DecryptRequestMiddleware(h.cryptoService, h.digitalSignatureService, h.logger),
		middleware.HTTPLoggingMiddlewareWithConfig(httpLoggingConfig),
		middleware.EncryptResponseMiddleware(h.cryptoService, h.digitalSignatureService, h.logger),
		middleware.ResponseFormatterMiddleware(h.logger),
	)
	h.healthCheckHandler.RegisterRoutes(api)
	h.inquiryHandler.RegisterRoutes(api)
	h.paymentHandler.RegisterRoutes(api)
	h.checkStatusHandler.RegisterRoutes(api)
	h.balanceHandler.RegisterRoutes(api)

	// Test routes
	test := app.Group("/test")
	h.cryptoHandler.RegisterRoutes(test)

	// Internal routes (public, no middleware, no authentication)
	internal := app.Group("/internal")
	h.errorMappingHandler.RegisterRoutes(internal)
}
