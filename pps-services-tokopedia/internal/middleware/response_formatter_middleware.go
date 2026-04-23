package middleware

import (
	"encoding/json"
	"pps-services-tokopedia/internal/service"

	"github.com/gofiber/fiber/v2"
)

// ResponseFormatterConfig defines formatting rules for different endpoints
type ResponseFormatterConfig struct {
	Path      string
	Formatter ResponseFormatter
}

// ResponseFormatter adalah interface untuk format response yang berbeda per API
type ResponseFormatter interface {
	Format(responseBody []byte) (interface{}, error)
}

// DefaultResponseFormatter untuk API yang tidak memerlukan formatting khusus
type DefaultResponseFormatter struct{}

func (f *DefaultResponseFormatter) Format(responseBody []byte) (interface{}, error) {
	var response interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}
	return response, nil
}

// ResponseFormatterMiddleware memformat response body sesuai aturan per API
// sebelum dienkripsi dan dikirim ke client
func ResponseFormatterMiddleware(logger service.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Lanjutkan request ke handler
		if err := c.Next(); err != nil {
			return err
		}

		// Get path untuk menentukan formatter
		path := c.Path()

		// Get response body
		responseBody := c.Response().Body()
		if len(responseBody) == 0 {
			return nil
		}

		// Find dan apply formatter sesuai path
		formatter := findResponseFormatter(path)
		if formatter == nil {
			// Gunakan default formatter jika tidak ada custom formatter
			formatter = &DefaultResponseFormatter{}
		}

		// Validate JSON sebelum format
		if !json.Valid(responseBody) {
			logger.Debug("Response body is not valid JSON, skipping formatting",
				"path", path)
			return nil
		}

		// Format response body
		formattedResponse, err := formatter.Format(responseBody)
		if err != nil {
			logger.Error("Failed to format response",
				"error", err,
				"path", path)
			// Jika format error, return original response
			return nil
		}

		// Encode formatted response kembali ke JSON
		formattedBody, err := json.Marshal(formattedResponse)
		if err != nil {
			logger.Error("Failed to marshal formatted response",
				"error", err,
				"path", path)
			// Jika marshal error, return original response
			return nil
		}

		// Replace response body dengan formatted response
		c.Response().SetBody(formattedBody)

		logger.Debug("Response formatted successfully",
			"path", path,
			"original_size", len(responseBody),
			"formatted_size", len(formattedBody))

		return nil
	}
}

// findResponseFormatter mencari formatter yang sesuai untuk path
func findResponseFormatter(path string) ResponseFormatter {
	formatters := getResponseFormatters()

	for _, config := range formatters {
		if config.Path == path {
			return config.Formatter
		}
	}

	return nil
}

// getResponseFormatters mengembalikan list konfigurasi formatter
// untuk setiap API endpoint
func getResponseFormatters() []ResponseFormatterConfig {
	return []ResponseFormatterConfig{
		{
			Path:      "/api/v1/inquiry",
			Formatter: NewInquiryResponseFormatter(),
		},
		{
			Path:      "/api/v1/payment",
			Formatter: NewPaymentResponseFormatter(),
		},
		{
			Path:      "/api/v1/check-status",
			Formatter: NewCheckStatusResponseFormatter(),
		},
		{
			Path:      "/api/v1/token",
			Formatter: NewTokenResponseFormatter(),
		},
	}
}

// ============================================================================
// TEMPLATE FORMATTER UNTUK SETIAP API
// ============================================================================

// InquiryResponseFormatter - format response inquiry
// Logic:
// - Jika response_code != "00": filter hanya ke client_number, message, product_code, response_code, timestamp
// - Jika response_code == "00": return response apa adanya tanpa reformat
type InquiryResponseFormatter struct {
}

func NewInquiryResponseFormatter() ResponseFormatter {
	return &InquiryResponseFormatter{}
}

func (f *InquiryResponseFormatter) Format(responseBody []byte) (interface{}, error) {
	var response map[string]interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}

	responseCode, _ := response["response_code"].(string)

	// Jika response_code == "00", return full response tanpa reformat
	if responseCode == "00" || responseCode == "01" {
		return response, nil
	}

	// Jika response_code != "00", filter hanya ke fields yang diperlukan
	filtered := map[string]interface{}{
		"client_number": response["client_number"],
		"message":       response["message"],
		"product_code":  response["product_code"],
		"response_code": response["response_code"],
		"timestamp":     response["timestamp"],
	}

	return filtered, nil
}

// PaymentResponseFormatter - template untuk format response payment
type PaymentResponseFormatter struct {
	// Tambahkan dependency di sini jika diperlukan
}

func NewPaymentResponseFormatter() ResponseFormatter {
	return &PaymentResponseFormatter{}
}

func (f *PaymentResponseFormatter) Format(responseBody []byte) (interface{}, error) {
	var response map[string]interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}

	responseCode, _ := response["response_code"].(string)
	if responseCode == "00" || responseCode == "01" {
		return response, nil
	}

	filtered := map[string]interface{}{
		"ref_id":        response["ref_id"],
		"client_number": response["client_number"],
		"product_code":  response["product_code"],
		"response_code": response["response_code"],
		"message":       response["message"],
		"timestamp":     response["timestamp"],
	}

	return filtered, nil
}

// CheckStatusResponseFormatter - template untuk format response check-status
type CheckStatusResponseFormatter struct {
	// Tambahkan dependency di sini jika diperlukan
}

func NewCheckStatusResponseFormatter() ResponseFormatter {
	return &CheckStatusResponseFormatter{}
}

func (f *CheckStatusResponseFormatter) Format(responseBody []byte) (interface{}, error) {
	var response map[string]interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}

	responseCode, _ := response["response_code"].(string)
	if responseCode == "00" || responseCode == "01" {
		return response, nil
	}

	filtered := map[string]interface{}{
		"ref_id":        response["ref_id"],
		"client_number": response["client_number"],
		"product_code":  response["product_code"],
		"response_code": response["response_code"],
		"message":       response["message"],
		"timestamp":     response["timestamp"],
	}

	return filtered, nil
}

// TokenResponseFormatter - template untuk format response token
type TokenResponseFormatter struct {
	// Tambahkan dependency di sini jika diperlukan
}

func NewTokenResponseFormatter() ResponseFormatter {
	return &TokenResponseFormatter{}
}

func (f *TokenResponseFormatter) Format(responseBody []byte) (interface{}, error) {
	var response map[string]interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}

	responseCode, _ := response["response_code"].(string)

	// Jika response_code == "00", return full response (token tetap ada)
	if responseCode == "00" {
		return response, nil
	}

	// Jika response_code selain "00", hilangkan key "token" jika ada
	delete(response, "token")
	return response, nil
}

// HealthCheckResponseFormatter - template untuk format response health check
type HealthCheckResponseFormatter struct {
	// Tambahkan dependency di sini jika diperlukan
}

func NewHealthCheckResponseFormatter() ResponseFormatter {
	return &HealthCheckResponseFormatter{}
}

func (f *HealthCheckResponseFormatter) Format(responseBody []byte) (interface{}, error) {
	// TODO: Implementasi logic formatting health check response

	var response interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}

	return response, nil
}

// ============================================================================
// UTILITY FUNCTIONS UNTUK FORMATTING
// ============================================================================

// FormatCallbackResponse formats callback response data before sending to Tokopedia
// Uses same logic as CheckStatusResponseFormatter
// Returns formatted map ready to be marshaled to JSON
func FormatCallbackResponse(data map[string]interface{}) map[string]interface{} {
	responseCode, _ := data["response_code"].(string)

	// Jika response_code == "00" atau "01", return full response
	if responseCode == "00" || responseCode == "01" {
		return data
	}

	// Jika response_code != "00" dan != "01", filter hanya ke fields yang diperlukan
	filtered := map[string]interface{}{
		"ref_id":        data["ref_id"],
		"client_number": data["client_number"],
		"product_code":  data["product_code"],
		"response_code": data["response_code"],
		"message":       data["message"],
		"timestamp":     data["timestamp"],
	}

	return filtered
}

// formatNumber memformat angka dengan jumlah decimal places tertentu
func formatNumber(value float64, decimalPlaces int) float64 {
	// TODO: Implementasi formatting number jika diperlukan
	return value
}

// sortMapFields mengurutkan fields dari map
func sortMapFields(data map[string]interface{}, fieldOrder []string) map[string]interface{} {
	// TODO: Implementasi sorting fields jika diperlukan
	return data
}

// filterFields menghapus fields yang tidak perlu
func filterFields(data map[string]interface{}, allowedFields []string) map[string]interface{} {
	// TODO: Implementasi filtering fields jika diperlukan
	return data
}
