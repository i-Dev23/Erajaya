//go:build wireinject
// +build wireinject

package main

import (
	"crypto/rsa"
	"fmt"
	"pps-services-tokopedia/internal/config"

	"pps-services-tokopedia/internal/delivery/http"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/repository"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/usecase"
	"pps-services-tokopedia/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/wire"
)

// ProviderSet groups providers together
var ProviderSet = wire.NewSet(
	// Config
	config.Load,
	// Services
	NewTelegramService,
	NewLoggerWithTelegram,
	service.NewRedisClient,
	service.NewOracleService,
	service.NewPostgresService,
	service.NewUltimaService,
	service.NewSchedulerService,
	service.NewRabbitMQService,

	// Crypto services
	NewCryptoServices,
	NewCryptoServiceProvider,
	NewDigitalSignatureServiceProvider,

	// Repositories
	repository.NewProductPriceOracleRepository,
	repository.NewBalanceOracleRepository,
	repository.NewCutOffOracleRepository,
	repository.NewPostgresInquiryRepository,
	repository.NewPostgresPaymentRepository,
	repository.NewErrorMappingPostgresRepository,
	repository.NewPreorderOracleRepository,
	repository.NewHTTPLoggingRepository,
	repository.NewCallbackLoggingRepository,
	repository.NewCleanupPostgresRepository,

	// Usecases
	usecase.NewTokenUsecase,
	usecase.NewHealthCheckUsecase,
	usecase.NewBalanceUsecase,
	usecase.NewInquiryUsecase,
	usecase.NewPaymentUsecase,
	usecase.NewCheckStatusUsecase,
	usecase.NewCleanupUsecase,
	usecase.NewScheduledJobsUsecase,
	usecase.NewDecryptSignatureGenerateUsecase,
	NewCallbackUsecaseWithDeps,

	// Handlers (all sub-handlers)
	http.NewTokenHandler,
	http.NewHealthCheckHandler,
	http.NewBalanceHandler,
	http.NewInquiryHandler,
	http.NewPaymentHandler,
	http.NewCheckStatusHandler,
	http.NewErrorMappingHandler,
	http.NewDecryptSignatureGenerateHandler,

	// Main handler (aggregates all sub-handlers)
	NewHandlerWithCrypto,

	// Fiber app
	NewFiberApp,
)

// CryptoServices holds the crypto-related services
type CryptoServices struct {
	DigitalSignatureService service.DigitalSignatureService
	CryptoService           service.CryptoService
}

// NewCryptoServices creates crypto services with loaded keys
func NewCryptoServices() (*CryptoServices, error) {
	privKey, pubKey, err := initKeys()
	if err != nil {
		return nil, err
	}

	// Type assertion to RSA keys
	rsaPrivKey, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA type")
	}
	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA type")
	}

	digitalSignatureService := service.NewDigitalSignatureService(rsaPrivKey, rsaPubKey)
	cryptoService := service.NewCryptoService(rsaPrivKey, rsaPubKey)

	return &CryptoServices{
		DigitalSignatureService: digitalSignatureService,
		CryptoService:           cryptoService,
	}, nil
}

// initKeys loads RSA keys from environment variables
func initKeys() (interface{}, interface{}, error) {
	privKey, err := utils.LoadPrivateKeyFromString(utils.GetEnv("PRIVATE_KEY", ""))
	if err != nil {
		return nil, nil, err
	}
	pubKey, err := utils.LoadPublicKeyFromString(utils.GetEnv("PUBLIC_KEY", ""))
	if err != nil {
		return nil, nil, err
	}
	return privKey, pubKey, nil
}

// AppContainer holds all application dependencies
type AppContainer struct {
	App                  *fiber.App
	Handler              *http.Handler
	Logger               service.Logger
	RedisClient          service.RedisClient
	OracleService        service.OracleService
	PostgresService      service.PostgresService
	UltimaService        domain.UltimaService
	ProductRepo          domain.ProductRepository
	CryptoServices       *CryptoServices
	SchedulerService     service.SchedulerService
	CleanupUsecase       usecase.CleanupUsecase
	ScheduledJobsUsecase usecase.ScheduledJobsUsecase
	CallbackUsecase      domain.CallbackUsecase
	RabbitMQService      service.RabbitMQService
}

// NewFiberApp creates a new Fiber app
func NewFiberApp() *fiber.App {
	return fiber.New()
}

// NewHandlerWithCrypto creates a new HTTP handler with all sub-handlers and crypto services
func NewHandlerWithCrypto(
	logger service.Logger,
	redisClient service.RedisClient,
	productRepo domain.ProductRepository,
	tokenHandler *http.TokenHandler,
	balanceHandler *http.BalanceHandler,
	healthCheckHandler *http.HealthCheckHandler,
	inquiryHandler *http.InquiryHandler,
	paymentHandler *http.PaymentHandler,
	checkStatusHandler *http.CheckStatusHandler,
	errorMappingHandler *http.ErrorMappingHandler,
	cryptoHandler *http.DecryptSignatureGenerateHandler,
	httpLoggingRepo domain.HTTPLoggingRepository,
	tokenUsecase domain.TokenUsecase,
	cryptoServices *CryptoServices,
) *http.Handler {
	return http.NewHandler(
		logger,
		redisClient,
		cryptoServices.DigitalSignatureService,
		cryptoServices.CryptoService,
		productRepo,
		tokenHandler,
		balanceHandler,
		healthCheckHandler,
		inquiryHandler,
		paymentHandler,
		checkStatusHandler,
		errorMappingHandler,
		cryptoHandler,
		httpLoggingRepo,
		tokenUsecase,
	)
}

// NewAppContainer creates a new AppContainer with all dependencies
func NewAppContainer(
	app *fiber.App,
	handler *http.Handler,
	logger service.Logger,
	redisClient service.RedisClient,
	oracleService service.OracleService,
	postgresService service.PostgresService,
	ultimaService domain.UltimaService,
	productRepo domain.ProductRepository,
	cryptoServices *CryptoServices,
	schedulerService service.SchedulerService,
	cleanupUsecase usecase.CleanupUsecase,
	scheduledJobsUsecase usecase.ScheduledJobsUsecase,
	callbackUsecase domain.CallbackUsecase,
	rabbitMQService service.RabbitMQService,
) *AppContainer {
	return &AppContainer{
		App:                  app,
		Handler:              handler,
		Logger:               logger,
		RedisClient:          redisClient,
		OracleService:        oracleService,
		PostgresService:      postgresService,
		UltimaService:        ultimaService,
		ProductRepo:          productRepo,
		CryptoServices:       cryptoServices,
		SchedulerService:     schedulerService,
		CleanupUsecase:       cleanupUsecase,
		ScheduledJobsUsecase: scheduledJobsUsecase,
		CallbackUsecase:      callbackUsecase,
		RabbitMQService:      rabbitMQService,
	}
}

// NewTelegramService creates a new Telegram service with credentials from environment
func NewTelegramService() service.TelegramService {
	// Bot token and chat ID for error alerts from environment variables
	botToken := utils.GetEnv("TELEGRAM_BOT_TOKEN", "")
	chatID := utils.GetEnv("TELEGRAM_CHAT_ID", "")

	return service.NewTelegramService(botToken, chatID)
}

// NewLoggerWithTelegram creates a logger with Telegram error alerting
func NewLoggerWithTelegram(telegramService service.TelegramService) service.Logger {
	return service.NewLoggerWithTelegram(telegramService)
}

// NewCryptoServiceProvider extracts CryptoService from CryptoServices
func NewCryptoServiceProvider(cryptoServices *CryptoServices) service.CryptoService {
	return cryptoServices.CryptoService
}

// NewDigitalSignatureServiceProvider extracts DigitalSignatureService from CryptoServices
func NewDigitalSignatureServiceProvider(cryptoServices *CryptoServices) service.DigitalSignatureService {
	return cryptoServices.DigitalSignatureService
}

// NewCallbackUsecaseWithDeps creates a callback usecase with all required dependencies
func NewCallbackUsecaseWithDeps(
	rabbitMQService service.RabbitMQService,
	logger service.Logger,
	cryptoService service.CryptoService,
	digitalSignatureService service.DigitalSignatureService,
	callbackLoggingRepository domain.CallbackLoggingRepository,
	postgresPaymentRepo domain.PostgresPaymentRepository,
) domain.CallbackUsecase {
	return usecase.NewCallbackUsecase(
		rabbitMQService,
		logger,
		cryptoService,
		digitalSignatureService,
		callbackLoggingRepository,
		postgresPaymentRepo,
	)
}

// InitializeApp creates and wires up the entire application
func InitializeApp() (*AppContainer, error) {
	wire.Build(ProviderSet, NewAppContainer)
	return nil, nil
}
