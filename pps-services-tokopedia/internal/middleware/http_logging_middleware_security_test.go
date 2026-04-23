package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsEncryptedOrEncoded(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{
			name:     "Empty body",
			body:     "",
			expected: false,
		},
		{
			name:     "Plain text JSON",
			body:     `{"name":"John","age":30}`,
			expected: false,
		},
		{
			name:     "Base64 encoded",
			body:     "SGVsbG8gV29ybGQ=",
			expected: true,
		},
		{
			name:     "Hex encoded",
			body:     "48656c6c6f20576f726c64",
			expected: true,
		},
		{
			name:     "High entropy (encrypted-like)",
			body:     "aBcDeFgHiJkLmNoPqRsTuVwXyZ1234567890!@#$%^&*()",
			expected: true,
		},
		{
			name:     "Contains encryption indicator",
			body:     `{"data":"encrypted","value":"abc123"}`,
			expected: true,
		},
		{
			name:     "Normal text",
			body:     "Hello World! This is a normal text message.",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isEncryptedOrEncoded(tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsBase64Like(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Valid base64",
			input:    "SGVsbG8gV29ybGQ=",
			expected: true,
		},
		{
			name:     "Base64 without padding",
			input:    "SGVsbG8gV29ybGQ",
			expected: true,
		},
		{
			name:     "Not base64",
			input:    "Hello World!",
			expected: false,
		},
		{
			name:     "Short string",
			input:    "abc",
			expected: false,
		},
		{
			name:     "Mixed base64 and normal",
			input:    "SGVsbG8gV29ybGQ= Hello World!",
			expected: false,
		},
		{
			name:     "Too short for base64",
			input:    "SGVs",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBase64Like(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsHexEncoded(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Valid hex",
			input:    "48656c6c6f20576f726c64",
			expected: true,
		},
		{
			name:     "Hex with uppercase",
			input:    "48656C6C6F20576F726C64",
			expected: true,
		},
		{
			name:     "Not hex",
			input:    "Hello World!",
			expected: false,
		},
		{
			name:     "Odd length",
			input:    "48656c6c6f20576f726c6",
			expected: false,
		},
		{
			name:     "Short string",
			input:    "ab",
			expected: false,
		},
		{
			name:     "Too short for hex",
			input:    "abc",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHexEncoded(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasHighEntropy(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "High entropy (encrypted-like)",
			input:    "aBcDeFgHiJkLmNoPqRsTuVwXyZ1234567890!@#$%^&*()",
			expected: true,
		},
		{
			name:     "Low entropy (repetitive)",
			input:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			expected: false,
		},
		{
			name:     "Medium entropy",
			input:    "Hello World! This is a normal text.",
			expected: false,
		},
		{
			name:     "Short string",
			input:    "abc",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasHighEntropy(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsSensitiveData(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{
			name:     "Contains password",
			body:     `{"username":"john","password":"secret123"}`,
			expected: true,
		},
		{
			name:     "Contains token",
			body:     `{"access_token":"abc123xyz"}`,
			expected: true,
		},
		{
			name:     "Contains credit card",
			body:     `{"card_number":"1234-5678-9012-3456"}`,
			expected: true,
		},
		{
			name:     "Contains SSN",
			body:     `{"ssn":"123-45-6789"}`,
			expected: true,
		},
		{
			name:     "Normal data",
			body:     `{"name":"John","age":30,"city":"New York"}`,
			expected: false,
		},
		{
			name:     "Empty body",
			body:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsSensitiveData(tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLoggableRequestBody(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "Plain text JSON",
			body:     `{"name":"John","age":30}`,
			expected: `{"name":"John","age":30}`,
		},
		{
			name:     "Encrypted body",
			body:     "SGVsbG8gV29ybGQ=",
			expected: "[ENCRYPTED_REQUEST_BODY]",
		},
		{
			name:     "Sensitive data",
			body:     `{"password":"secret123"}`,
			expected: "[SENSITIVE_REQUEST_BODY]",
		},
		{
			name:     "Large body",
			body:     string(make([]byte, 1024*1024+1)), // > 1MB
			expected: "[LARGE_REQUEST_BODY]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getLoggableRequestBody(nil, tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}
