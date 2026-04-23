package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultConsumerTag              = "pps-services-gateway-unipin-consumer"
	defaultReadTimeout              = 30 * time.Second
	defaultTimeoutSec               = 30
	defaultVoucherRequestTimeoutSec = 60
	defaultOrderRequestTimeoutSec   = 60
	defaultRetryMaxAttempts         = 5
	defaultRetryWaitSeconds         = 5
)

// Config stores RabbitMQ consumer configuration.
type Config struct {
	RabbitMQURL string
	QueueName   string
	ConsumerTag string
	ReadTimeout time.Duration
	HTTPPort    string

	// Retry config for downstream status inquiry when Unipin returns pending state.
	RetryMaxAttempts int
	RetryWait        time.Duration
}

// OracleConfig stores Oracle database configuration.
type OracleConfig struct {
	DSN                string
	ProviderName       string
	SyncCron           string
	MaxOpenConns       int
	MaxIdleConns       int
	ConnMaxLifetimeMin int
	ConnMaxIdleTimeMin int
}

// PostgresConfig stores Postgres database configuration.
type PostgresConfig struct {
	DSN string
}

// UnipinConfig stores Unipin API client configuration.
type UnipinConfig struct {
	BaseURL               string
	PartnerID             string
	SecretKey             string
	Timeout               time.Duration
	VoucherRequestTimeout time.Duration
	OrderRequestTimeout   time.Duration
}

// Load reads required and optional environment variables for consumer startup.
func Load() (*Config, error) {
	rabbitMQURL := strings.TrimSpace(os.Getenv("RABBITMQ_URL"))
	queueName := strings.TrimSpace(os.Getenv("QUEUE_NAME_PROVIDER"))
	if queueName == "" {
		queueName = strings.TrimSpace(os.Getenv("QUEUE_NAME"))
	}
	consumerTag := strings.TrimSpace(os.Getenv("CONSUMER_TAG"))

	if rabbitMQURL == "" {
		return nil, fmt.Errorf("RABBITMQ_URL is required")
	}
	if queueName == "" {
		return nil, fmt.Errorf("QUEUE_NAME_PROVIDER is required")
	}
	if consumerTag == "" {
		consumerTag = defaultConsumerTag
	}

	httpPort := strings.TrimSpace(os.Getenv("HTTP_PORT"))
	if httpPort == "" {
		httpPort = "8080"
	}

	retryMaxAttempts := parseIntEnv("RETRY_MAX_ATTEMPTS", defaultRetryMaxAttempts)
	retryWaitSeconds := parseIntEnv("RETRY_WAIT_SECONDS", defaultRetryWaitSeconds)

	return &Config{
		RabbitMQURL:      rabbitMQURL,
		QueueName:        queueName,
		ConsumerTag:      consumerTag,
		ReadTimeout:      defaultReadTimeout,
		HTTPPort:         httpPort,
		RetryMaxAttempts: retryMaxAttempts,
		RetryWait:        time.Duration(retryWaitSeconds) * time.Second,
	}, nil
}

// LoadUnipin reads required environment variables for Unipin API client.
func LoadUnipin() (*UnipinConfig, error) {
	baseURL := strings.TrimSpace(os.Getenv("BASE_URL"))
	partnerID := strings.TrimSpace(os.Getenv("PARTNER_ID"))
	secretKey := strings.TrimSpace(os.Getenv("SECRET_KEY"))
	timeoutRaw := strings.TrimSpace(os.Getenv("TIMEOUT"))
	voucherRequestTimeoutRaw := strings.TrimSpace(os.Getenv("VOUCHER_REQUEST_TIMEOUT"))
	orderRequestTimeoutRaw := strings.TrimSpace(os.Getenv("ORDER_REQUEST_TIMEOUT"))

	if baseURL == "" {
		return nil, fmt.Errorf("BASE_URL is required")
	}
	if partnerID == "" {
		return nil, fmt.Errorf("PARTNER_ID is required")
	}
	if secretKey == "" {
		return nil, fmt.Errorf("SECRET_KEY is required")
	}

	timeoutSec := defaultTimeoutSec
	if timeoutRaw != "" {
		parsed, err := strconv.Atoi(timeoutRaw)
		if err != nil {
			return nil, fmt.Errorf("TIMEOUT must be integer seconds: %w", err)
		}
		if parsed <= 0 {
			return nil, fmt.Errorf("TIMEOUT must be greater than zero")
		}
		timeoutSec = parsed
	}

	voucherReqTimeoutSec := defaultVoucherRequestTimeoutSec
	if voucherRequestTimeoutRaw != "" {
		parsed, err := strconv.Atoi(voucherRequestTimeoutRaw)
		if err != nil {
			return nil, fmt.Errorf("VOUCHER_REQUEST_TIMEOUT must be integer seconds: %w", err)
		}
		if parsed <= 0 {
			return nil, fmt.Errorf("VOUCHER_REQUEST_TIMEOUT must be greater than zero")
		}
		voucherReqTimeoutSec = parsed
	}

	orderReqTimeoutSec := defaultOrderRequestTimeoutSec
	if orderRequestTimeoutRaw != "" {
		parsed, err := strconv.Atoi(orderRequestTimeoutRaw)
		if err != nil {
			return nil, fmt.Errorf("ORDER_REQUEST_TIMEOUT must be integer seconds: %w", err)
		}
		if parsed <= 0 {
			return nil, fmt.Errorf("ORDER_REQUEST_TIMEOUT must be greater than zero")
		}
		orderReqTimeoutSec = parsed
	}

	return &UnipinConfig{
		BaseURL:               baseURL,
		PartnerID:             partnerID,
		SecretKey:             secretKey,
		Timeout:               time.Duration(timeoutSec) * time.Second,
		VoucherRequestTimeout: time.Duration(voucherReqTimeoutSec) * time.Second,
		OrderRequestTimeout:   time.Duration(orderReqTimeoutSec) * time.Second,
	}, nil
}

// LoadOracle reads required environment variables for Oracle database.
func LoadOracle() (*OracleConfig, error) {
	dsn := strings.TrimSpace(os.Getenv("ORACLE_DSN"))
	providerName := strings.TrimSpace(os.Getenv("PROVIDER_NAME"))
	syncCron := strings.TrimSpace(os.Getenv("SYNC_CRON"))

	if dsn == "" {
		return nil, fmt.Errorf("ORACLE_DSN is required")
	}
	if providerName == "" {
		providerName = "UNIPIN"
	}
	if syncCron == "" {
		syncCron = "0 1 * * *"
	}

	maxOpen := parseIntEnv("ORACLE_MAX_OPEN_CONNS", 10)
	maxIdle := parseIntEnv("ORACLE_MAX_IDLE_CONNS", 5)
	connLifetime := parseIntEnv("ORACLE_CONN_MAX_LIFETIME_MIN", 30)
	connIdleTime := parseIntEnv("ORACLE_CONN_MAX_IDLE_TIME_MIN", 10)

	return &OracleConfig{
		DSN:                dsn,
		ProviderName:       providerName,
		SyncCron:           syncCron,
		MaxOpenConns:       maxOpen,
		MaxIdleConns:       maxIdle,
		ConnMaxLifetimeMin: connLifetime,
		ConnMaxIdleTimeMin: connIdleTime,
	}, nil
}

func parseIntEnv(key string, defaultVal int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(raw)
	if err != nil || val <= 0 {
		return defaultVal
	}
	return val
}

// LoadPostgres reads required environment variables for Postgres database.
func LoadPostgres() (*PostgresConfig, error) {
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_DSN"))
	if dsn == "" {
		return nil, fmt.Errorf("POSTGRES_DSN is required")
	}
	return &PostgresConfig{DSN: dsn}, nil
}
