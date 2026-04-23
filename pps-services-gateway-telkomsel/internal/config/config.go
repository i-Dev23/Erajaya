package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultConsumerTag      = "pps-services-gateway-telkomsel-consumer"
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

// CallbackServerConfig stores HTTP callback server configuration.
type CallbackServerConfig struct {
	Port int
}

// LoadCallbackServer reads callback server configuration from environment variables.
func LoadCallbackServer() (*CallbackServerConfig, error) {
	portStr := strings.TrimSpace(os.Getenv("CALLBACK_PORT"))
	if portStr == "" {
		return &CallbackServerConfig{Port: 8080}, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("CALLBACK_PORT must be a valid number: %w", err)
	}
	return &CallbackServerConfig{Port: port}, nil
}

// TelkomselConfig stores Telkomsel ESB REST client configuration.
type TelkomselConfig struct {
	BaseURL            string
	ChannelID          string
	OrganizationCode   string
	SecretKey          string
	APIKey             string
	ThirdPartyID       string
	ThirdPartyPassword string
	EncryptionKey      string
	DeliveryChannel    string
	Timeout            time.Duration
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

	return &Config{
		RabbitMQURL: rabbitMQURL,
		QueueName:   queueName,
		ConsumerTag: consumerTag,
		ReadTimeout: defaultReadTimeout,
		PostgresDSN: strings.TrimSpace(os.Getenv("POSTGRES_DSN")),
	}, nil
}

// LoadTelkomsel reads required environment variables for Telkomsel REST client.
func LoadTelkomsel() (*TelkomselConfig, error) {
	baseURL := strings.TrimSpace(os.Getenv("BASE_URL"))
	channelID := strings.TrimSpace(os.Getenv("CHANNEL_ID"))
	organizationCode := strings.TrimSpace(os.Getenv("ORGANIZATION_CODE"))
	secretKey := strings.TrimSpace(os.Getenv("SECRET_KEY"))
	apiKey := strings.TrimSpace(os.Getenv("API_KEY"))
	thirdPartyID := strings.TrimSpace(os.Getenv("THIRD_PARTY_ID"))
	thirdPartyPassword := strings.TrimSpace(os.Getenv("THIRD_PARTY_PASSWORD"))
	encryptionKey := strings.TrimSpace(os.Getenv("ENCRYPTION_KEY"))
	deliveryChannel := strings.TrimSpace(os.Getenv("DELIVERY_CHANNEL"))
	timeoutRaw := strings.TrimSpace(os.Getenv("TIMEOUT"))

	if baseURL == "" {
		return nil, fmt.Errorf("BASE_URL is required")
	}
	if channelID == "" {
		return nil, fmt.Errorf("CHANNEL_ID is required")
	}
	if organizationCode == "" {
		return nil, fmt.Errorf("ORGANIZATION_CODE is required")
	}
	if secretKey == "" {
		return nil, fmt.Errorf("SECRET_KEY is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API_KEY is required")
	}
	if thirdPartyID == "" {
		return nil, fmt.Errorf("THIRD_PARTY_ID is required")
	}
	if thirdPartyPassword == "" {
		return nil, fmt.Errorf("THIRD_PARTY_PASSWORD is required")
	}
	if encryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required")
	}
	if deliveryChannel == "" {
		return nil, fmt.Errorf("DELIVERY_CHANNEL is required")
	}

	timeoutSec := defaultTimeoutSec
	if timeoutRaw != "" {
		parsedTimeout, err := strconv.Atoi(timeoutRaw)
		if err != nil {
			return nil, fmt.Errorf("TIMEOUT must be integer seconds: %w", err)
		}
		if parsedTimeout <= 0 {
			return nil, fmt.Errorf("TIMEOUT must be greater than zero")
		}
		timeoutSec = parsedTimeout
	}

	return &TelkomselConfig{
		BaseURL:            baseURL,
		ChannelID:          channelID,
		OrganizationCode:   organizationCode,
		SecretKey:          secretKey,
		APIKey:             apiKey,
		ThirdPartyID:       thirdPartyID,
		ThirdPartyPassword: thirdPartyPassword,
		EncryptionKey:      encryptionKey,
		DeliveryChannel:    deliveryChannel,
		Timeout:            time.Duration(timeoutSec) * time.Second,
	}, nil
}

// RetryConfig menyimpan konfigurasi retry check status.
type RetryConfig struct {
	MaxAttempts  int
	WaitDuration time.Duration
}

// LoadRetryConfig membaca RETRY_MAX_ATTEMPTS dan RETRY_WAIT_SECONDS dari environment.
func LoadRetryConfig() (*RetryConfig, error) {
	maxAttemptsRaw := strings.TrimSpace(os.Getenv("RETRY_MAX_ATTEMPTS"))
	waitSecondsRaw := strings.TrimSpace(os.Getenv("RETRY_WAIT_SECONDS"))

	maxAttempts := defaultRetryMaxAttempts
	if maxAttemptsRaw != "" {
		parsed, err := strconv.Atoi(maxAttemptsRaw)
		if err != nil {
			return nil, fmt.Errorf("RETRY_MAX_ATTEMPTS must be integer: %w", err)
		}
		if parsed <= 0 {
			return nil, fmt.Errorf("RETRY_MAX_ATTEMPTS must be greater than zero")
		}
		maxAttempts = parsed
	}

	waitSeconds := defaultRetryWaitSeconds
	if waitSecondsRaw != "" {
		parsed, err := strconv.Atoi(waitSecondsRaw)
		if err != nil {
			return nil, fmt.Errorf("RETRY_WAIT_SECONDS must be integer: %w", err)
		}
		if parsed <= 0 {
			return nil, fmt.Errorf("RETRY_WAIT_SECONDS must be greater than zero")
		}
		waitSeconds = parsed
	}

	return &RetryConfig{
		MaxAttempts:  maxAttempts,
		WaitDuration: time.Duration(waitSeconds) * time.Second,
	}, nil
}
