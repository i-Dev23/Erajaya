package route

import (
	"pps-services-publisher-database/internal/delivery/http"
	"pps-services-publisher-database/internal/delivery/http/middleware"

	_ "pps-services-publisher-database/docs"

	swagger "github.com/Flussen/swagger-fiber-v3"
	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type RouteConfig struct {
	App                   *fiber.App
	TransactionController *http.TransactionController
	Config                *viper.Viper
	Log                   zerolog.Logger
}

func (c *RouteConfig) Setup() {
	c.App.Use(middleware.NewRequestLogger(c.Log))

	c.App.Get("/health", c.TransactionController.HealthCheck)
	c.App.Get("/swagger/*", swagger.HandlerDefault)

	api := c.App.Group("/api")
	api.Use(middleware.NewAPIKeyAuth(c.Config, c.Log))
	api.Post("/callback/topup", c.TransactionController.SubmitTopupCallback)
	api.Post("/callback/data", c.TransactionController.SubmitDataCallback)
	// api.Post("/callback/pln-token", c.TransactionController.SubmitPlnTokenCallback)
	// api.Post("/callback/pln-postpaid", c.TransactionController.SubmitPlnPostpaidCallback)
	// api.Post("/callback/game", c.TransactionController.SubmitGameCallback)
}
