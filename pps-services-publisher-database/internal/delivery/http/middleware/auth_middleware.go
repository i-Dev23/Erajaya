package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func NewAPIKeyAuth(config *viper.Viper, log zerolog.Logger) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		apiKey := ctx.Get("X-API-Key")
		validKey := config.GetString("security.api_key")

		if apiKey == "" || apiKey != validKey {
			log.Warn().Msgf("Unauthorized access attempt from IP: %s", ctx.IP())
			return fiber.NewError(fiber.StatusUnauthorized, "Invalid or missing API key")
		}

		return ctx.Next()
	}
}

func NewRequestLogger(log zerolog.Logger) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		err := ctx.Next()

		log.Info().
			Str("method", ctx.Method()).
			Str("path", ctx.Path()).
			Int("status", ctx.Response().StatusCode()).
			Str("ip", ctx.IP()).
			Msg("HTTP Request")

		return err
	}
}
