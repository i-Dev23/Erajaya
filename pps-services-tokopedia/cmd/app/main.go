package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize application using Wire DI
	appContainer, err := InitializeApp()
	if err != nil {
		log.Fatalf("failed to initialize application: %v", err)
	}

	// Setup cleanup for resources
	defer func() {
		// Stop scheduler
		if appContainer.SchedulerService != nil {
			shutdownCtx := appContainer.SchedulerService.Stop()
			<-shutdownCtx.Done()
		}

		// Close RabbitMQ service
		if appContainer.RabbitMQService != nil {
			if err := appContainer.RabbitMQService.Close(); err != nil {
				appContainer.Logger.Error("Failed to close RabbitMQ service", "error", err)
			} else {
				appContainer.Logger.Info("RabbitMQ service closed successfully")
			}
		}

		// Close Oracle service
		if appContainer.OracleService != nil {
			appContainer.OracleService.Close()
		}

		// Close Postgres service
		if appContainer.PostgresService != nil {
			appContainer.PostgresService.Close()
		}

		// Close Redis client
		if closer, ok := appContainer.RedisClient.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()

	// Handle graceful shutdown
	handleSigterm(ctx, cancel, appContainer.Logger)

	// Setup scheduled jobs
	setupScheduledJobs(appContainer)

	// Start the scheduler
	appContainer.SchedulerService.Start()
	appContainer.Logger.Info("Scheduler service started successfully")

	// Start callback consumer (RabbitMQ)
	if strings.ToUpper(strings.TrimSpace(os.Getenv("CONSUMER_CALLBACK"))) == "Y" {
		appContainer.Logger.Info("Callback consumer enabled", "env", "CONSUMER_CALLBACK")
		go func() {
			if err := appContainer.CallbackUsecase.ListenCallbackQueue(ctx); err != nil {
				appContainer.Logger.Error("Callback consumer stopped", "error", err)
			}
		}()
	} else {
		appContainer.Logger.Info("Callback consumer disabled by env", "env", "CONSUMER_CALLBACK")
	}

	// REMARKED: Cache product prices on startup
	// go func() {
	//     // Get username from environment variable, default to "ALFA-DEV"
	//     username := os.Getenv("TP_CLIENT_ID")
	//     if username == "" {
	//         username = "ALFA-DEV"
	//     }
	//
	//     appContainer.Logger.Info("Starting to cache product prices on startup", "username", username)
	//     // Get product prices and cache them
	//     prices, err := appContainer.ProductRepo.GetPriceByUser(ctx, username, os.Getenv("PROVIDER_CODE_GET_PRICE"))
	//     if err != nil {
	//         appContainer.Logger.Error("Failed to get product prices on startup", "error", err, "username", username)
	//     } else {
	//         appContainer.Logger.Info("Successfully retrieved product prices on startup", "count", len(*prices), "username", username)
	//         //save product prices to cache redis asynchronously
	//         for _, price := range *prices {
	//             // Save asynchronously with timeout to prevent blocking startup
	//             go func(p interface{}, priceVal interface{}) {
	//                 ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//                 defer cancel()
	//                 appContainer.RedisClient.Set(ctx, fmt.Sprintf("%v", p), fmt.Sprintf("%v", priceVal), 24*time.Hour)
	//             }(price.KodeVoucher, price.Price)
	//
	//             appContainer.Logger.Info("Queued product price for cache redis", "kodeVoucher", price.KodeVoucher, "price", price.Price)
	//         }
	//     }
	// }()

	// Cache products with status on startup
	go func() {
		// Get username from environment variable, default to "ALFA-DEV"
		username := os.Getenv("TP_CLIENT_ID")
		if username == "" {
			username = "ALFA-DEV"
		}

		appContainer.Logger.Info("Starting to cache products with status on startup", "username", username)
		// Get products with status and cache them
		products, err := appContainer.ProductRepo.GetProductByUser(ctx, username)
		if err != nil {
			appContainer.Logger.Error("Failed to get products with status on startup", "error", err, "username", username)
		} else {
			appContainer.Logger.Info("Successfully retrieved products with status on startup", "count", len(*products), "username", username)
			// Save products with status to cache Redis asynchronously
			for _, product := range *products {
				if product.KodeVoucher != "" {
					// Save asynchronously with timeout to prevent blocking startup
					go func(p domain.ProductPriceResponseDomain) {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						cacheKey := fmt.Sprintf("%s%s", utils.RedisKeyProductWithStatusPrefix, p.KodeVoucher)
						productJSON := fmt.Sprintf(`{"kodevoucher":"%s","price":%f,"status":"%s"}`,
							p.KodeVoucher, p.Price, p.Status)
						appContainer.RedisClient.Set(ctx, cacheKey, productJSON, 24*time.Hour)
					}(product)

					appContainer.Logger.Info("Queued product with status for cache redis",
						"kodeVoucher", product.KodeVoucher,
						"price", product.Price,
						"status", product.Status)
				}
			}
		}
	}()

	// Cache whitelisted IP on startup
	go func() {
		// Get username from environment variable, default to "ALFA-DEV"
		username := os.Getenv("TP_CLIENT_ID")
		if username == "" {
			username = "ALFA-DEV"
		}

		appContainer.Logger.Info("Starting to get whitelisted IP on startup", "username", username)
		// Get whitelisted IP from Oracle
		ipResponse, err := appContainer.ProductRepo.GetIpByUser(ctx, username)
		if err != nil {
			appContainer.Logger.Error("Failed to get whitelisted IP on startup", "error", err, "username", username)
		} else {
			// Only save to Redis if outerrcode == "0" (success)
			if ipResponse.OuterRCode == "0" && ipResponse.OutIp != "" {
				err = appContainer.RedisClient.Set(ctx, utils.RedisKeyWhitelistedIP, ipResponse.OutIp, 0).Err()
				if err != nil {
					appContainer.Logger.Error("Failed to save whitelisted IP to Redis", "error", err, "ip", ipResponse.OutIp)
				} else {
					appContainer.Logger.Info("Successfully saved whitelisted IP to Redis", "ip", ipResponse.OutIp, "key", utils.RedisKeyWhitelistedIP, "ttl", "no limit")
				}
			} else {
				appContainer.Logger.Warn("Skipped saving whitelisted IP - outerrcode is not 0 or IP is empty",
					"outerrcode", ipResponse.OuterRCode,
					"outerrmsg", ipResponse.OuterRMsg,
					"outIp", ipResponse.OutIp,
					"username", username)
			}
		}
	}()

	// Cache cut-off configuration on startup
	go func() {
		appContainer.Logger.Info("Starting to get cut-off configuration on startup")
		// Get cut-off data from Oracle
		cutOffResponse, err := appContainer.ProductRepo.GetCutOff(ctx)
		if err != nil {
			appContainer.Logger.Error("Failed to get cut-off configuration on startup", "error", err)
		} else {
			// Only save to Redis if outerrcode == "0" (success)
			if cutOffResponse.OutErrCode == "0" {
				// Save all cut-off fields to Redis with the specified key mappings
				mappings := map[string]string{
					utils.RedisKeyCutOffTimeStartTokopedia:      cutOffResponse.CutOffTimeStartTokopedia,
					utils.RedisKeyCutOffDurationSecondTokopedia: cutOffResponse.CutOffDurationTokopedia,
					utils.RedisKeyCutOffMessageTokopedia:        cutOffResponse.CutOffMessageTokopedia,
					utils.RedisKeyCutOffTimeStart:               cutOffResponse.CutOffTimeStart,
					utils.RedisKeyCutOffDurationSecond:          cutOffResponse.CutOffDuration,
					utils.RedisKeyCutOffMessage:                 cutOffResponse.CutOffMessage,
				}

				for key, value := range mappings {
					if value != "" {
						err = appContainer.RedisClient.Set(ctx, key, value, 0).Err()
						if err != nil {
							appContainer.Logger.Error("Failed to save cut-off data to Redis", "error", err, "key", key, "value", value)
						} else {
							appContainer.Logger.Info("Successfully saved cut-off data to Redis", "key", key, "value", value, "ttl", "no limit")
						}
					}
				}
			} else {
				appContainer.Logger.Warn("Skipped saving cut-off configuration - outerrcode is not 0",
					"outerrcode", cutOffResponse.OutErrCode,
					"outerrmsg", cutOffResponse.OutErrMsg)
			}
		}
	}()

	// Public liveness endpoint for container health checks (no auth/middleware)
	appContainer.App.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("OK")
	})

	// Register routes
	appContainer.Handler.RegisterRoutes(appContainer.App)

	// Start server
	go func() {
		port := os.Getenv("APP_PORT")
		if port == "" {
			port = "3001"
		}
		if err := appContainer.App.Listen(":" + port); err != nil {
			appContainer.Logger.Error("Fiber server error", "error", err)
			cancel()
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	appContainer.Logger.Info("Shutting down Fiber server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := appContainer.App.ShutdownWithContext(shutdownCtx); err != nil {
		appContainer.Logger.Error("Error during Fiber shutdown", "error", err)
	}

	appContainer.Logger.Info("Graceful shutdown complete.")
}

func handleSigterm(ctx context.Context, cancel context.CancelFunc, logger service.Logger) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		defer signal.Stop(sigCh)
		select {
		case sig := <-sigCh:
			logger.Info("Received signal, shutting down...", "signal", sig)
			cancel()
		case <-ctx.Done():
		}
	}()
}

// setupScheduledJobs configures all scheduled jobs
func setupScheduledJobs(appContainer *AppContainer) {
	if appContainer.ScheduledJobsUsecase == nil {
		appContainer.Logger.Warn("Scheduled jobs usecase is nil, skipping job setup")
		return
	}
	appContainer.ScheduledJobsUsecase.SetupScheduledJobs()
}
