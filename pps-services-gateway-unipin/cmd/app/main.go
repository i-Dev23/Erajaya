package main

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"pps-services-gateway-unipin/internal/config"
	"pps-services-gateway-unipin/internal/delivery/http/handler"
	contractsvc "pps-services-gateway-unipin/internal/domain/contract/service"
	"pps-services-gateway-unipin/internal/infrastructure/mqpublisher"
	"pps-services-gateway-unipin/internal/infrastructure/oracle"
	"pps-services-gateway-unipin/internal/infrastructure/postgres"
	"pps-services-gateway-unipin/internal/infrastructure/rabbitmq"
	"pps-services-gateway-unipin/internal/infrastructure/scheduler"
	"pps-services-gateway-unipin/internal/usecase/gamesync"
	"pps-services-gateway-unipin/pkg/unipin"
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
	loadDotEnv(slogLogger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	unipinCfg, err := config.LoadUnipin()
	if err != nil {
		logger.Error("failed to load unipin configuration", "error", err)
		os.Exit(1)
	}

	unipinClient, err := unipin.NewClient(unipinCfg.BaseURL, unipinCfg.PartnerID, unipinCfg.SecretKey, unipinCfg.Timeout, slogLogger)
	if err != nil {
		logger.Error("failed to create unipin client", "error", err)
		os.Exit(1)
	}
	unipinClient.SetVoucherRequestTimeout(unipinCfg.VoucherRequestTimeout)
	unipinClient.SetCreateOrderTimeout(unipinCfg.OrderRequestTimeout)

	// Load Postgres config and connect for API logging (optional)
	pgCfg, err := config.LoadPostgres()
	if err != nil {
		logger.Warn("postgres not configured, API logging disabled", "error", err)
	}

	if pgCfg != nil {
		pgDB, err := postgres.NewDB(pgCfg.DSN, postgres.DefaultPoolConfig())
		if err != nil {
			logger.Warn("failed to connect postgres, API logging disabled", "error", err)
		} else {
			defer pgDB.Close()
			apiLogRepo := postgres.NewAPILogRepository(pgDB)
			loggingTransport := unipin.NewLoggingTransport(nil, apiLogRepo, unipinCfg.BaseURL)
			unipinClient.SetTransport(loggingTransport)
			logger.Info("postgres connected, API logging enabled")
		}
	}

	// Load Oracle config and connect
	oracleCfg, err := config.LoadOracle()
	if err != nil {
		logger.Error("failed to load oracle configuration", "error", err)
		os.Exit(1)
	}

	db, err := oracle.NewDB(oracleCfg.DSN, oracle.PoolConfig{
		MaxOpenConns:    oracleCfg.MaxOpenConns,
		MaxIdleConns:    oracleCfg.MaxIdleConns,
		ConnMaxLifetime: time.Duration(oracleCfg.ConnMaxLifetimeMin) * time.Minute,
		ConnMaxIdleTime: time.Duration(oracleCfg.ConnMaxIdleTimeMin) * time.Minute,
	})
	if err != nil {
		logger.Error("failed to connect oracle", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Setup game sync service and cron
	gameListRepo := oracle.NewGameListRepository(db)
	voucherListRepo := oracle.NewVoucherListRepository(db)
	syncSvc := gamesync.NewSyncService(unipinClient, gameListRepo, voucherListRepo, logger)

	cronScheduler := scheduler.New(logger)
	if err := cronScheduler.AddGameSync(oracleCfg.SyncCron, syncSvc); err != nil {
		logger.Error("failed to register game sync cron", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)

	reportErr := func(err error) {
		if err == nil {
			return
		}
		errOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}

	// Start Fiber HTTP server
	app := fiber.New(fiber.Config{
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	})
	app.Use(recover.New())

	gameAPIRepo := oracle.NewGameAPIRepository(db)
	gameListHandler := handler.NewGameListAPIHandler(gameAPIRepo, logger)

	api := app.Group("/api/v1")
	api.Post("/game-list", gameListHandler.HandleGameList)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("http server started", "port", cfg.HTTPPort)
		err := app.Listen(":" + cfg.HTTPPort)
		if err != nil && ctx.Err() == nil {
			reportErr(err)
			return
		}
	}()

	// Start RabbitMQ consumer
	consumer := rabbitmq.NewConsumerServiceImpl(cfg, unipinClient, logger)

	// Initialize MQ Publisher for downstream RabbitMQ publishing
	mqPub := mqpublisher.NewAMQPPublisher(logger)
	consumer.SetMQPublisher(mqPub)
	logger.Info("mq publisher initialized")

	logger.Info("rabbitmq consumer started", "queue", cfg.QueueName, "consumerTag", cfg.ConsumerTag)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := consumer.Start(ctx); err != nil {
			reportErr(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		cronScheduler.Start()
		<-ctx.Done()
		cronScheduler.Stop()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		logger.Info("shutting down services")
		if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
			reportErr(err)
		}
	}()

	wg.Wait()
	if firstErr != nil {
		logger.Error("service stopped with error", "error", firstErr)
		os.Exit(1)
	}
}

// loadDotEnv loads key/value pairs from the project's .env file into the process
// environment (without overriding existing env vars). This is optional and is
// intended to make local development easier.
func loadDotEnv(logger *slog.Logger) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return
	}

	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	envPath := filepath.Join(projectRoot, ".env")

	f, err := os.Open(envPath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, val)
	}

	if err := scanner.Err(); err != nil {
		logger.Warn("failed to read .env", "path", envPath, "error", err)
		return
	}

	logger.Info("loaded .env", "path", envPath)
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
