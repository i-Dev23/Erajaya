package config

import (
	"strings"
	"testing"
	"time"
)

func setAllTelkomselEnvVars(t *testing.T) {
	t.Helper()
	t.Setenv("BASE_URL", "https://api.telkomsel.com")
	t.Setenv("CHANNEL_ID", "ch-001")
	t.Setenv("ORGANIZATION_CODE", "ORG001")
	t.Setenv("SECRET_KEY", "secret123")
	t.Setenv("API_KEY", "apikey123")
	t.Setenv("THIRD_PARTY_ID", "tp-001")
	t.Setenv("THIRD_PARTY_PASSWORD", "tppass")
	t.Setenv("ENCRYPTION_KEY", "enckey123")
	t.Setenv("DELIVERY_CHANNEL", "DC01")
	t.Setenv("TIMEOUT", "10")
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    *Config
		wantErr string
	}{
		{
			name: "all valid env vars returns correct Config",
			envVars: map[string]string{
				"RABBITMQ_URL":        "amqp://guest:guest@localhost:5672/",
				"QUEUE_NAME_PROVIDER": "telkomsel-queue",
				"CONSUMER_TAG":        "my-consumer",
				"POSTGRES_DSN":        "postgres://localhost/db",
			},
			want: &Config{
				RabbitMQURL: "amqp://guest:guest@localhost:5672/",
				QueueName:   "telkomsel-queue",
				ConsumerTag: "my-consumer",
				ReadTimeout: 30 * time.Second,
				PostgresDSN: "postgres://localhost/db",
			},
		},
		{
			name: "missing RABBITMQ_URL returns error",
			envVars: map[string]string{
				"QUEUE_NAME_PROVIDER": "telkomsel-queue",
			},
			wantErr: "RABBITMQ_URL is required",
		},
		{
			name: "empty QUEUE_NAME_PROVIDER uses QUEUE_NAME fallback",
			envVars: map[string]string{
				"RABBITMQ_URL":        "amqp://localhost",
				"QUEUE_NAME_PROVIDER": "",
				"QUEUE_NAME":          "fallback-queue",
				"CONSUMER_TAG":        "tag",
			},
			want: &Config{
				RabbitMQURL: "amqp://localhost",
				QueueName:   "fallback-queue",
				ConsumerTag: "tag",
				ReadTimeout: 30 * time.Second,
			},
		},
		{
			name: "empty CONSUMER_TAG defaults",
			envVars: map[string]string{
				"RABBITMQ_URL":        "amqp://localhost",
				"QUEUE_NAME_PROVIDER": "q",
				"CONSUMER_TAG":        "",
			},
			want: &Config{
				RabbitMQURL: "amqp://localhost",
				QueueName:   "q",
				ConsumerTag: "pps-services-gateway-telkomsel-consumer",
				ReadTimeout: 30 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			got, err := Load()

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.RabbitMQURL != tt.want.RabbitMQURL {
				t.Errorf("RabbitMQURL = %q, want %q", got.RabbitMQURL, tt.want.RabbitMQURL)
			}
			if got.QueueName != tt.want.QueueName {
				t.Errorf("QueueName = %q, want %q", got.QueueName, tt.want.QueueName)
			}
			if got.ConsumerTag != tt.want.ConsumerTag {
				t.Errorf("ConsumerTag = %q, want %q", got.ConsumerTag, tt.want.ConsumerTag)
			}
			if got.ReadTimeout != tt.want.ReadTimeout {
				t.Errorf("ReadTimeout = %v, want %v", got.ReadTimeout, tt.want.ReadTimeout)
			}
			if got.PostgresDSN != tt.want.PostgresDSN {
				t.Errorf("PostgresDSN = %q, want %q", got.PostgresDSN, tt.want.PostgresDSN)
			}
		})
	}
}

func TestLoadCallbackServer(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    *CallbackServerConfig
		wantErr string
	}{
		{
			name:    "empty CALLBACK_PORT defaults to 8080",
			envVars: map[string]string{},
			want:    &CallbackServerConfig{Port: 8080},
		},
		{
			name:    "valid CALLBACK_PORT",
			envVars: map[string]string{"CALLBACK_PORT": "9090"},
			want:    &CallbackServerConfig{Port: 9090},
		},
		{
			name:    "non-numeric CALLBACK_PORT returns error",
			envVars: map[string]string{"CALLBACK_PORT": "abc"},
			wantErr: "CALLBACK_PORT must be a valid number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			got, err := LoadCallbackServer()

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Port != tt.want.Port {
				t.Errorf("Port = %d, want %d", got.Port, tt.want.Port)
			}
		})
	}
}

func TestLoadTelkomsel(t *testing.T) {
	// Required env vars for a valid TelkomselConfig.
	requiredVars := []string{
		"BASE_URL", "CHANNEL_ID", "ORGANIZATION_CODE", "SECRET_KEY",
		"API_KEY", "THIRD_PARTY_ID", "THIRD_PARTY_PASSWORD",
		"ENCRYPTION_KEY", "DELIVERY_CHANNEL",
	}

	t.Run("all valid env vars returns correct TelkomselConfig", func(t *testing.T) {
		setAllTelkomselEnvVars(t)

		got, err := LoadTelkomsel()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.BaseURL != "https://api.telkomsel.com" {
			t.Errorf("BaseURL = %q, want %q", got.BaseURL, "https://api.telkomsel.com")
		}
		if got.ChannelID != "ch-001" {
			t.Errorf("ChannelID = %q, want %q", got.ChannelID, "ch-001")
		}
		if got.OrganizationCode != "ORG001" {
			t.Errorf("OrganizationCode = %q, want %q", got.OrganizationCode, "ORG001")
		}
		if got.SecretKey != "secret123" {
			t.Errorf("SecretKey = %q, want %q", got.SecretKey, "secret123")
		}
		if got.APIKey != "apikey123" {
			t.Errorf("APIKey = %q, want %q", got.APIKey, "apikey123")
		}
		if got.ThirdPartyID != "tp-001" {
			t.Errorf("ThirdPartyID = %q, want %q", got.ThirdPartyID, "tp-001")
		}
		if got.ThirdPartyPassword != "tppass" {
			t.Errorf("ThirdPartyPassword = %q, want %q", got.ThirdPartyPassword, "tppass")
		}
		if got.EncryptionKey != "enckey123" {
			t.Errorf("EncryptionKey = %q, want %q", got.EncryptionKey, "enckey123")
		}
		if got.DeliveryChannel != "DC01" {
			t.Errorf("DeliveryChannel = %q, want %q", got.DeliveryChannel, "DC01")
		}
		if got.Timeout != 10*time.Second {
			t.Errorf("Timeout = %v, want %v", got.Timeout, 10*time.Second)
		}
	})

	// Test each missing required env var returns error identifying the variable.
	for _, envVar := range requiredVars {
		t.Run("missing "+envVar+" returns error", func(t *testing.T) {
			setAllTelkomselEnvVars(t)
			t.Setenv(envVar, "")

			_, err := LoadTelkomsel()
			if err == nil {
				t.Fatalf("expected error for missing %s, got nil", envVar)
			}
			if !strings.Contains(err.Error(), envVar) {
				t.Fatalf("expected error to contain %q, got %q", envVar, err.Error())
			}
		})
	}

	t.Run("valid positive TIMEOUT parses correctly", func(t *testing.T) {
		setAllTelkomselEnvVars(t)
		t.Setenv("TIMEOUT", "45")

		got, err := LoadTelkomsel()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Timeout != 45*time.Second {
			t.Errorf("Timeout = %v, want %v", got.Timeout, 45*time.Second)
		}
	})

	t.Run("empty TIMEOUT uses default 30s", func(t *testing.T) {
		setAllTelkomselEnvVars(t)
		t.Setenv("TIMEOUT", "")

		got, err := LoadTelkomsel()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Timeout != 30*time.Second {
			t.Errorf("Timeout = %v, want %v", got.Timeout, 30*time.Second)
		}
	})

	t.Run("zero TIMEOUT returns error", func(t *testing.T) {
		setAllTelkomselEnvVars(t)
		t.Setenv("TIMEOUT", "0")

		_, err := LoadTelkomsel()
		if err == nil {
			t.Fatal("expected error for zero TIMEOUT, got nil")
		}
		if !strings.Contains(err.Error(), "TIMEOUT must be greater than zero") {
			t.Fatalf("expected error about TIMEOUT, got %q", err.Error())
		}
	})

	t.Run("negative TIMEOUT returns error", func(t *testing.T) {
		setAllTelkomselEnvVars(t)
		t.Setenv("TIMEOUT", "-5")

		_, err := LoadTelkomsel()
		if err == nil {
			t.Fatal("expected error for negative TIMEOUT, got nil")
		}
		if !strings.Contains(err.Error(), "TIMEOUT must be greater than zero") {
			t.Fatalf("expected error about TIMEOUT, got %q", err.Error())
		}
	})

	t.Run("non-integer TIMEOUT returns error", func(t *testing.T) {
		setAllTelkomselEnvVars(t)
		t.Setenv("TIMEOUT", "abc")

		_, err := LoadTelkomsel()
		if err == nil {
			t.Fatal("expected error for non-integer TIMEOUT, got nil")
		}
		if !strings.Contains(err.Error(), "TIMEOUT must be integer seconds") {
			t.Fatalf("expected error about TIMEOUT format, got %q", err.Error())
		}
	})
}

func TestLoadRetryConfig(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    *RetryConfig
		wantErr string
	}{
		{
			name:    "defaults when env vars not set",
			envVars: map[string]string{},
			want:    &RetryConfig{MaxAttempts: 4, WaitDuration: 10 * time.Second},
		},
		{
			name:    "valid custom values",
			envVars: map[string]string{"RETRY_MAX_ATTEMPTS": "7", "RETRY_WAIT_SECONDS": "20"},
			want:    &RetryConfig{MaxAttempts: 7, WaitDuration: 20 * time.Second},
		},
		{
			name:    "only RETRY_MAX_ATTEMPTS set",
			envVars: map[string]string{"RETRY_MAX_ATTEMPTS": "5"},
			want:    &RetryConfig{MaxAttempts: 5, WaitDuration: 10 * time.Second},
		},
		{
			name:    "only RETRY_WAIT_SECONDS set",
			envVars: map[string]string{"RETRY_WAIT_SECONDS": "30"},
			want:    &RetryConfig{MaxAttempts: 4, WaitDuration: 30 * time.Second},
		},
		{
			name:    "zero RETRY_MAX_ATTEMPTS returns error",
			envVars: map[string]string{"RETRY_MAX_ATTEMPTS": "0"},
			wantErr: "RETRY_MAX_ATTEMPTS must be greater than zero",
		},
		{
			name:    "negative RETRY_MAX_ATTEMPTS returns error",
			envVars: map[string]string{"RETRY_MAX_ATTEMPTS": "-3"},
			wantErr: "RETRY_MAX_ATTEMPTS must be greater than zero",
		},
		{
			name:    "non-numeric RETRY_MAX_ATTEMPTS returns error",
			envVars: map[string]string{"RETRY_MAX_ATTEMPTS": "abc"},
			wantErr: "RETRY_MAX_ATTEMPTS must be integer",
		},
		{
			name:    "zero RETRY_WAIT_SECONDS returns error",
			envVars: map[string]string{"RETRY_WAIT_SECONDS": "0"},
			wantErr: "RETRY_WAIT_SECONDS must be greater than zero",
		},
		{
			name:    "negative RETRY_WAIT_SECONDS returns error",
			envVars: map[string]string{"RETRY_WAIT_SECONDS": "-5"},
			wantErr: "RETRY_WAIT_SECONDS must be greater than zero",
		},
		{
			name:    "non-numeric RETRY_WAIT_SECONDS returns error",
			envVars: map[string]string{"RETRY_WAIT_SECONDS": "xyz"},
			wantErr: "RETRY_WAIT_SECONDS must be integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			got, err := LoadRetryConfig()

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.MaxAttempts != tt.want.MaxAttempts {
				t.Errorf("MaxAttempts = %d, want %d", got.MaxAttempts, tt.want.MaxAttempts)
			}
			if got.WaitDuration != tt.want.WaitDuration {
				t.Errorf("WaitDuration = %v, want %v", got.WaitDuration, tt.want.WaitDuration)
			}
		})
	}
}
