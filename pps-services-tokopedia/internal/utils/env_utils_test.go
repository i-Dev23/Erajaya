package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		setValue   string
		defaultVal string
		want       string
	}{
		{
			name:       "env var exists",
			key:        "TEST_ENV_EXISTS",
			setValue:   "exists_value",
			defaultVal: "default_value",
			want:       "exists_value",
		},
		{
			name:       "env var not set, use default",
			key:        "TEST_ENV_NOT_SET",
			setValue:   "",
			defaultVal: "default_value",
			want:       "default_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setValue != "" {
				os.Setenv(tt.key, tt.setValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}
			got := GetEnv(tt.key, tt.defaultVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetEnvAsInt(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		setValue   string
		defaultVal int
		want       int
	}{
		{
			name:       "env var exists and valid int",
			key:        "TEST_ENV_INT_VALID",
			setValue:   "42",
			defaultVal: 10,
			want:       42,
		},
		{
			name:       "env var not set, use default",
			key:        "TEST_ENV_INT_NOT_SET",
			setValue:   "",
			defaultVal: 99,
			want:       99,
		},
		{
			name:       "env var invalid int, use default",
			key:        "TEST_ENV_INT_INVALID",
			setValue:   "not_an_int",
			defaultVal: 7,
			want:       7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setValue != "" {
				os.Setenv(tt.key, tt.setValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}
			got := GetEnvAsInt(tt.key, tt.defaultVal)
			assert.Equal(t, tt.want, got)
		})
	}
}
