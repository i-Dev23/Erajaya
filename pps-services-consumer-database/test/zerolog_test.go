// Package test - File ini berisi test untuk konfigurasi logging.
// Menguji PII masking, context logger, dan Telegram notification.
package test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"pps-services-consumer-database/internal/pkg/logger"
)

// TestPIIMasking_PhoneNumber menguji masking nomor telepon Indonesia.
func TestPIIMasking_PhoneNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Phone with 08 prefix",
			input:    "Customer phone: 081234567890",
			expected: "Customer phone: ***PHONE***",
		},
		{
			name:     "Phone with +62 prefix",
			input:    "Contact: +6281234567890",
			expected: "Contact: ***PHONE***",
		},
		{
			name:     "Phone with 62 prefix",
			input:    "Number: 6281234567890",
			expected: "Number: ***PHONE***",
		},
		{
			name:     "No phone number",
			input:    "Hello world",
			expected: "Hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logger.MaskPII(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPIIMasking_Email menguji masking alamat email.
func TestPIIMasking_Email(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple email",
			input:    "Email: user@example.com",
			expected: "Email: ***EMAIL***",
		},
		{
			name:     "Email with dots",
			input:    "Contact: first.last@company.co.id",
			expected: "Contact: ***EMAIL***",
		},
		{
			name:     "No email",
			input:    "Just a message",
			expected: "Just a message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logger.MaskPII(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPIIMasking_NIK menguji masking NIK (16 digit).
func TestPIIMasking_NIK(t *testing.T) {
	result := logger.MaskPII("NIK: 3201234567890123")
	assert.Equal(t, "NIK: ***NIK***", result)
}

// TestPIIMasking_MultiplePII menguji masking multiple PII dalam satu string.
func TestPIIMasking_MultiplePII(t *testing.T) {
	input := "Customer 081234567890 email user@test.com"
	result := logger.MaskPII(input)
	assert.NotContains(t, result, "081234567890")
	assert.NotContains(t, result, "user@test.com")
}

// TestPIIMasking_NoPII menguji string tanpa PII tidak berubah.
func TestPIIMasking_NoPII(t *testing.T) {
	input := "Transaction TXN-001 processed successfully"
	result := logger.MaskPII(input)
	assert.Equal(t, input, result)
}

// TestContextLogger menguji bahwa ContextLogger menambahkan trace_id.
func TestContextLogger(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf).With().Logger()
	ctxLog := logger.ContextLogger(log, "trace-123")

	ctxLog.Info().Msg("test message")

	var output map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &output)
	assert.Nil(t, err)
	assert.Equal(t, "trace-123", output["trace_id"])
	assert.Equal(t, "test message", output["message"])
}

// TestContextLogger_EmptyTraceID menguji ContextLogger dengan trace_id kosong.
func TestContextLogger_EmptyTraceID(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf).With().Logger()
	ctxLog := logger.ContextLogger(log, "")

	ctxLog.Info().Msg("test message")

	var output map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &output)
	assert.Nil(t, err)
	assert.Equal(t, "", output["trace_id"])
}
