package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"pps-services-gateway-telkomsel/internal/config"
	contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
	"pps-services-gateway-telkomsel/internal/handler"
	apphttp "pps-services-gateway-telkomsel/internal/http"
	"pps-services-gateway-telkomsel/internal/infrastructure/mqpublisher"
	"pps-services-gateway-telkomsel/internal/infrastructure/postgres"
	"pps-services-gateway-telkomsel/internal/infrastructure/rabbitmq"
	"pps-services-gateway-telkomsel/pkg/telkomsel"
)

func main() {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}

	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.TimeValue(t.In(loc))
				}
			}
			return a
		},
	})

	slogLogger := slog.New(h)
	slog.SetDefault(slogLogger)
	logger := &slogLoggerAdapter{l: slogLogger}

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	retryConfig, err := config.LoadRetryConfig()
	if err != nil {
		logger.Error("failed to load retry config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	consumer := rabbitmq.NewConsumerServiceImpl(cfg, logger)

	// Initialize TransactionLogger if PostgresDSN is configured
	var txLogger contractsvc.TransactionLogger
	if cfg.PostgresDSN != "" {
		pgLogger, err := postgres.NewTransactionLogger(cfg.PostgresDSN, logger)
		if err != nil {
			logger.Error("failed to initialize transaction logger", "error", err)
			os.Exit(1)
		}
		defer pgLogger.Close()

		txLogger = pgLogger
		consumer.SetTransactionLogger(txLogger)

		// Run auto-migration for transaction tables
		migCtx, migCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer migCancel()
		if err := pgLogger.RunMigration(migCtx); err != nil {
			logger.Error("failed to run transaction migration", "error", err)
			os.Exit(1)
		}
		logger.Info("transaction migration completed")

		// Initialize error mapping repository using shared DB connection
		errorMappingRepo := postgres.NewErrorMappingRepositoryImpl(pgLogger.DB(), logger)
		telkomsel.SetErrorMappingResolver(errorMappingRepo)
		logger.Info("error mapping resolver initialized")

		// Initialize API logger for persisting request/response logs to telkomsel_api_logs
		apiLogRepo := postgres.NewAPILogRepositoryImpl(pgLogger.DB(), logger)
		apiLogAdapter := postgres.NewAPILoggerAdapter(apiLogRepo, logger)
		telkomsel.SetAPILogger(apiLogAdapter)
		logger.Info("api logger initialized")
	}

	// Inject retry config to consumer
	consumer.SetRetryConfig(retryConfig)

	// Initialize MQ Publisher for downstream RabbitMQ publishing
	mqPub := mqpublisher.NewAMQPPublisher(logger)
	consumer.SetMQPublisher(mqPub)
	logger.Info("mq publisher initialized")

	// Initialize callback server
	callbackCfg, err := config.LoadCallbackServer()
	if err != nil {
		logger.Error("failed to load callback server config", "error", err)
		os.Exit(1)
	}

	callbackHandler := handler.NewCallbackHandler(logger, txLogger, mqPub, cfg.QueueName)
	httpServer := apphttp.NewServer(apphttp.ServerConfig{Port: callbackCfg.Port}, callbackHandler, logger)

	logger.Info("service started", "queue", cfg.QueueName, "consumerTag", cfg.ConsumerTag)

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error { return consumer.Start(gCtx) })
	g.Go(func() error { return httpServer.Listen(gCtx) })

	if err := g.Wait(); err != nil {
		logger.Error("service stopped with error", "error", err)
		os.Exit(1)
	}
}

var _ contractsvc.Logger = (*slogLoggerAdapter)(nil)

type slogLoggerAdapter struct {
	l *slog.Logger
}

func (a *slogLoggerAdapter) Info(msg string, args ...any) {
	if a == nil || a.l == nil {
		return
	}
	a.l.Info(msg, args...)
}

func (a *slogLoggerAdapter) Warn(msg string, args ...any) {
	if a == nil || a.l == nil {
		return
	}
	a.l.Warn(msg, args...)
}

func (a *slogLoggerAdapter) Error(msg string, args ...any) {
	if a == nil || a.l == nil {
		return
	}
	a.l.Error(msg, args...)
}
