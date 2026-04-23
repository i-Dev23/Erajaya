package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"pps-services-publisher-database/internal/config"
)

func main() {
	viperConfig := config.NewViper()
	log := config.NewLogger(viperConfig)
	validate := config.NewValidator(viperConfig)
	app := config.NewFiber(viperConfig)
	rabbitMQ := config.NewRabbitMQ(viperConfig, log)

	config.Bootstrap(&config.BootstrapConfig{
		App:      app,
		Log:      log,
		Validate: validate,
		Config:   viperConfig,
		RabbitMQ: rabbitMQ,
	})

	webPort := viperConfig.GetInt("web.port")
	log.Info().Msgf("Starting HTTP server on port %d", webPort)

	go func() {
		if err := app.Listen(fmt.Sprintf(":%d", webPort)); err != nil {
			log.Fatal().Msgf("Failed to start HTTP server: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Info().Msgf("Received signal: %v. Starting graceful shutdown...", sig)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			log.Error().Msgf("Error shutting down HTTP server: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		rabbitMQ.Close()
	}()

	wg.Wait()
	log.Info().Msg("Graceful shutdown completed")
	os.Exit(0)
}
