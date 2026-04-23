package config

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"pps-services-publisher-database/internal/delivery/http"
	"pps-services-publisher-database/internal/delivery/http/route"
	"pps-services-publisher-database/internal/gateway/messaging"
	"pps-services-publisher-database/internal/repository"
	"pps-services-publisher-database/internal/usecase"
)

type BootstrapConfig struct {
	App      *fiber.App
	Log      zerolog.Logger
	Validate *validator.Validate
	Config   *viper.Viper
	RabbitMQ *RabbitMQConnection
}

// Bootstrap wires all dependencies and sets up routes.
func Bootstrap(config *BootstrapConfig) {
	transactionRepo := repository.NewTransactionRepository(
		config.Log,
	)

	publisher := messaging.NewPublisher(config.RabbitMQ, config.Log, config.Config)

	transactionUseCase := usecase.NewTransactionUseCase(
		transactionRepo,
		publisher,
		config.Validate,
		config.Log,
	)

	transactionController := http.NewTransactionController(transactionUseCase, config.Log)

	routeConfig := route.RouteConfig{
		App:                   config.App,
		TransactionController: transactionController,
		Config:                config.Config,
		Log:                   config.Log,
	}
	routeConfig.Setup()
}
