package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration
// Only add fields needed for dependency extraction, do not change business logic
// All logic and flow must remain production-tested

//nolint:structcheck,unused // Some fields may be unused in initial extraction

type Config struct {
	AppPort                           string
	TPClientID                        string
	TPClientSecret                    string
	ProviderCodeGetPrice              string
	CallbackURL                       string
	GUID                              string
	ConsumerCallback                  bool
	TelegramBotToken                  string
	TelegramChatID                    string
	PrivateKey                        string
	PublicKey                         string
	HTTPLogRetentionDays              int
	CallbackLogRetentionDays          int
	InquiryAndPaymentLogRetentionDays int
	FileLogRetentionDays              int
	FileLogRetentionCron              string
	RunJobsLogRetention               bool
	RateLimitToken                    int
	RateLimitHealthCheck              int
	RateLimitInquiry                  int
	RateLimitPayment                  int
	RateLimitCheckStatus              int
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		AppPort:                           getEnv("APP_PORT", "3001"),
		TPClientID:                        getEnv("TP_CLIENT_ID", ""),
		TPClientSecret:                    getEnv("TP_CLIENT_SECRET", ""),
		ProviderCodeGetPrice:              getEnv("PROVIDER_CODE_GET_PRICE", ""),
		CallbackURL:                       getEnv("CALLBACK_URL", ""),
		GUID:                              getEnv("GUID", ""),
		ConsumerCallback:                  getEnvBool("CONSUMER_CALLBACK", false),
		TelegramBotToken:                  getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:                    getEnv("TELEGRAM_CHAT_ID", ""),
		PrivateKey:                        getEnv("PRIVATE_KEY", ""),
		PublicKey:                         getEnv("PUBLIC_KEY", ""),
		HTTPLogRetentionDays:              getEnvInt("HTTP_LOG_RETENTION_DAYS", 30),
		CallbackLogRetentionDays:          getEnvInt("CALLBACK_LOG_RETENTION_DAYS", 30),
		InquiryAndPaymentLogRetentionDays: getEnvInt("INQUIRY_AND_PAYMENT_LOG_RETENTION_DAYS", 30),
		FileLogRetentionDays:              getEnvInt("FILE_LOG_RETENTION_DAYS", 50),
		FileLogRetentionCron:              getEnv("FILE_LOG_RETENTION_CRON", "0 40 1 * * *"),
		RunJobsLogRetention:               getEnvBool("RUN_JOBS_LOG_RETENTION", true),
		RateLimitToken:                    getEnvInt("RATE_LIMIT_TOKEN", 100),
		RateLimitHealthCheck:              getEnvInt("RATE_LIMIT_HEALTH_CHECK", 100),
		RateLimitInquiry:                  getEnvInt("RATE_LIMIT_INQUIRY", 100),
		RateLimitPayment:                  getEnvInt("RATE_LIMIT_PAYMENT", 100),
		RateLimitCheckStatus:              getEnvInt("RATE_LIMIT_CHECK_STATUS", 100),
	}

	// Validate required fields (do not change logic)
	if cfg.TPClientID == "" {
		return nil, fmt.Errorf("TP_CLIENT_ID is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
