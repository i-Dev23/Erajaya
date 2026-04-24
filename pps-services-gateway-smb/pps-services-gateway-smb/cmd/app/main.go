package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"

	_ "github.com/jackc/pgx/v5/stdlib"

	"pps-services-gateway-smb/internal/config"
	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
	"pps-services-gateway-smb/internal/infrastructure/mqpublisher"
	"pps-services-gateway-smb/internal/infrastructure/postgres"
	"pps-services-gateway-smb/internal/infrastructure/rabbitmq"
	"pps-services-gateway-smb/internal/infrastructure/smbclient"
	"pps-services-gateway-smb/pkg/smb"
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

	smbCfg, err := config.LoadSMB()
	if err != nil {
		logger.Error("failed to load SMB configuration", "error", err)
		os.Exit(1)
	}

	retryConfig, err := config.LoadRetryConfig()
	if err != nil {
		logger.Error("failed to load retry config", "error", err)
		os.Exit(1)
	}

	httpCfg, err := config.LoadCallbackServer()
	if err != nil {
		logger.Error("failed to load HTTP config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	smbHTTPClient := smb.NewClient(smbCfg.BaseURL, smbCfg.PartnerID, smbCfg.SecretKey, smbCfg.Timeout, slogLogger)
	smbAdapter := smbclient.NewAdapter(smbHTTPClient, logger)

	consumer := rabbitmq.NewConsumerServiceImpl(cfg, logger)
	consumer.SetSMBClient(smbAdapter)
	consumer.SetRetryConfig(retryConfig)

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

		migCtx, migCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer migCancel()
		if err := pgLogger.RunMigration(migCtx); err != nil {
			logger.Error("failed to run transaction migration", "error", err)
			os.Exit(1)
		}
		logger.Info("transaction migration completed")

		// Initialize API Log Repository (schema: log_smb)
		apiLogRepo := postgres.NewAPILogRepositoryImpl(pgLogger.DB(), logger)
		apiLogMigCtx, apiLogMigCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer apiLogMigCancel()
		if err := apiLogRepo.RunMigration(apiLogMigCtx); err != nil {
			logger.Error("failed to run api log migration", "error", err)
			os.Exit(1)
		}
		logger.Info("api log migration completed (schema: log_smb)")

		// Wire API logger adapter to SMB HTTP client
		apiLoggerAdapter := postgres.NewAPILoggerAdapter(apiLogRepo, logger)
		smbHTTPClient.SetAPILogger(apiLoggerAdapter)
	}

	mqPub := mqpublisher.NewAMQPPublisher(logger)
	consumer.SetMQPublisher(mqPub)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ReadTimeout:           cfg.ReadTimeout,
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "pps-services-gateway-smb"})
	})

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		addr := fmt.Sprintf(":%d", httpCfg.Port)
		logger.Info("starting HTTP server", "addr", addr)
		return app.Listen(addr)
	})

	g.Go(func() error {
		<-gCtx.Done()
		logger.Info("shutting down HTTP server")
		return app.ShutdownWithTimeout(10 * time.Second)
	})

	g.Go(func() error {
		return consumer.Start(gCtx)
	})

	if err := g.Wait(); err != nil && err != context.Canceled {
		logger.Error("service exited with error", "error", err)
		os.Exit(1)
	}

	logger.Info("service stopped gracefully")
}

type slogLoggerAdapter struct {
	l *slog.Logger
}

func (a *slogLoggerAdapter) Info(msg string, keysAndValues ...any)  { a.l.Info(msg, keysAndValues...) }
func (a *slogLoggerAdapter) Warn(msg string, keysAndValues ...any)  { a.l.Warn(msg, keysAndValues...) }
func (a *slogLoggerAdapter) Error(msg string, keysAndValues ...any) { a.l.Error(msg, keysAndValues...) }
