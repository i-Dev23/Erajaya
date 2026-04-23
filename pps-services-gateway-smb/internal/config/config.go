package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultConsumerTag      = "pps-services-gateway-smb-consumer"
	defaultReadTimeout      = 30 * time.Second
	defaultTimeoutSec       = 30
	defaultRetryMaxAttempts = 4
	defaultRetryWaitSeconds = 10
)

// Config stores application configuration loaded from environment variables.
type Config struct {
	RabbitMQURL string
	QueueName   string
	ConsumerTag string
	ReadTimeout time.Duration
	PostgresDSN string
}

// SMBConfig stores SMB/Loket Bayar API client configuration.
type SMBConfig struct {
	BaseURL   string
	PartnerID string
	SecretKey string
	Timeout   time.Duration
}

// RetryConfig stores retry configuration for check status operations.
type RetryConfig struct {
	MaxAttempts  int
	WaitDuration time.Duration
}

// CallbackServerConfig stores HTTP server configuration.
type CallbackServerConfig struct {
	Port int
}

// Load reads required environment variables for consumer startup.
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

	readTimeout := defaultReadTimeout
	if v := strings.TrimSpace(os.Getenv("READ_TIMEOUT_SEC")); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			readTimeout = time.Duration(sec) * time.Second
		}
	}

	postgresDSN := strings.TrimSpace(os.Getenv("POSTGRES_DSN"))

	return &Config{
		RabbitMQURL: rabbitMQURL,
		QueueName:   queueName,
		ConsumerTag: consumerTag,
		ReadTimeout: readTimeout,
		PostgresDSN: postgresDSN,
	}, nil
}

// LoadSMB reads SMB/Loket Bayar API configuration from environment variables.
func LoadSMB() (*SMBConfig, error) {
	baseURL := strings.TrimSpace(os.Getenv("SMB_BASE_URL"))
	partnerID := strings.TrimSpace(os.Getenv("SMB_PARTNER_ID"))
	secretKey := strings.TrimSpace(os.Getenv("SMB_SECRET_KEY"))

	if baseURL == "" {
		return nil, fmt.Errorf("SMB_BASE_URL is required")
	}
	if partnerID == "" {
		return nil, fmt.Errorf("SMB_PARTNER_ID is required")
	}
	if secretKey == "" {
		return nil, fmt.Errorf("SMB_SECRET_KEY is required")
	}

	timeout := time.Duration(defaultTimeoutSec) * time.Second
	if v := strings.TrimSpace(os.Getenv("SMB_TIMEOUT_SEC")); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			timeout = time.Duration(sec) * time.Second
		}
	}

	return &SMBConfig{
		BaseURL:   baseURL,
		PartnerID: partnerID,
		SecretKey: secretKey,
		Timeout:   timeout,
	}, nil
}

// LoadRetryConfig reads retry configuration from environment variables.
func LoadRetryConfig() (*RetryConfig, error) {
	maxAttempts := defaultRetryMaxAttempts
	if v := strings.TrimSpace(os.Getenv("RETRY_MAX_ATTEMPTS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxAttempts = n
		}
	}

	waitSeconds := defaultRetryWaitSeconds
	if v := strings.TrimSpace(os.Getenv("RETRY_WAIT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			waitSeconds = n
		}
	}

	return &RetryConfig{
		MaxAttempts:  maxAttempts,
		WaitDuration: time.Duration(waitSeconds) * time.Second,
	}, nil
}

// LoadCallbackServer reads HTTP server configuration from environment variables.
func LoadCallbackServer() (*CallbackServerConfig, error) {
	portStr := strings.TrimSpace(os.Getenv("HTTP_PORT"))
	if portStr == "" {
		return &CallbackServerConfig{Port: 8080}, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("HTTP_PORT must be a valid number: %w", err)
	}
	return &CallbackServerConfig{Port: port}, nil
}
