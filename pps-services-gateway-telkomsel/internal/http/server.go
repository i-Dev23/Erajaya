package http

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"pps-services-gateway-telkomsel/internal/handler"
	contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
)

// ServerConfig berisi konfigurasi HTTP server.
type ServerConfig struct {
	Port int
}

// Server mengelola gofiber HTTP server.
type Server struct {
	app    *fiber.App
	config ServerConfig
	logger contractsvc.Logger
}

// NewServer membuat instance Server baru dengan routing yang sudah dikonfigurasi.
func NewServer(cfg ServerConfig, callbackHandler *handler.CallbackHandler, logger contractsvc.Logger) *Server {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(recover.New())

	app.Get("/callback/ext", callbackHandler.Handle)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	return &Server{app: app, config: cfg, logger: logger}
}

// Listen memulai HTTP server dan menunggu context di-cancel untuk graceful shutdown.
func (s *Server) Listen(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		addr := fmt.Sprintf(":%d", s.config.Port)
		s.logger.Info("http server starting", "address", addr)
		errCh <- s.app.Listen(addr)
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("http server shutting down")
		return s.app.Shutdown()
	case err := <-errCh:
		return fmt.Errorf("http server error: %w", err)
	}
}
