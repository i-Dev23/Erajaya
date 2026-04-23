package middleware

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HTTPLoggingMiddleware creates a middleware for logging HTTP requests and responses to PostgreSQL
func HTTPLoggingMiddleware(logger service.Logger, httpLoggingRepo domain.HTTPLoggingRepository, cryptoService service.CryptoService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		startTime := time.Now()
		requestID := utils.GeneratePPSRequestID()

		// Store request ID in context for later use
		c.Locals("http_logging_request_id", requestID)
		c.Locals("http_logging_start_time", startTime)

		// Process the request
		err := c.Next()

		// Capture request and response details after processing
		endTime := time.Now()
		requestDetails := captureRequestDetailsAfterDecrypt(c, requestID, startTime, cryptoService, logger)
		responseDetails := captureResponseDetailsAfterEncrypt(c, requestID, endTime, err, cryptoService, logger)

		// Calculate response time
		responseTime := endTime.Sub(startTime).Milliseconds()

		// Log request start
		logger.Info("HTTP Request Started",
			"request_id", requestID,
			"method", requestDetails.Method,
			"path", requestDetails.Path,
			"client_ip", requestDetails.ClientIP,
			"user_agent", requestDetails.UserAgent)

		// Log response
		logger.Info("HTTP Request Completed",
			"request_id", requestID,
			"status_code", responseDetails.StatusCode,
			"response_time_ms", responseTime,
			"error", responseDetails.Error)

		// Insert log to PostgreSQL asynchronously
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			requestHeadersJSON, _ := json.Marshal(requestDetails.Headers)
			responseHeadersJSON, _ := json.Marshal(responseDetails.Headers)

			logRequest := &domain.HTTPLogInsertRequest{
				RequestID:       requestID,
				Method:          requestDetails.Method,
				Path:            requestDetails.Path,
				QueryParams:     requestDetails.QueryParams,
				RequestHeaders:  string(requestHeadersJSON),
				RequestBody:     requestDetails.Body,
				StatusCode:      responseDetails.StatusCode,
				ResponseHeaders: string(responseHeadersJSON),
				ResponseBody:    responseDetails.Body,
				ClientIP:        requestDetails.ClientIP,
				UserAgent:       requestDetails.UserAgent,
				ResponseTimeMs:  responseTime,
				RequestTime:     startTime,
				ResponseTime:    endTime,
				Error:           responseDetails.Error,
			}

			_, insertErr := httpLoggingRepo.InsertHTTPLog(ctx, logRequest)
			if insertErr != nil {
				logger.Error("Failed to insert HTTP log to database",
					"request_id", requestID,
					"error", insertErr)
			}
		}()

		return err
	}
}

// captureRequestDetailsAfterDecrypt captures request details after decryption middleware runs
func captureRequestDetailsAfterDecrypt(c *fiber.Ctx, requestID string, timestamp time.Time, cryptoService service.CryptoService, logger service.Logger) *domain.HTTPLogRequest {
	// Read request body - this should be plain text after decryption
	var body string
	if c.Body() != nil {
		body = string(c.Body())
	}

	// Capture headers
	headers := make(map[string]string)
	c.Request().Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})

	// Get query parameters
	queryParams := string(c.Request().URI().QueryString())

	// Get client IP from X-Real-IP header, fallback to c.IP()
	clientIP := c.Get("X-Real-IP")
	if clientIP == "" {
		clientIP = c.IP()
	}

	// Check if body is encrypted/encoded and attempt to decrypt
	var loggableBody string

	// If body appears encrypted, try to decrypt it first
	if isEncryptedOrEncoded(body) {
		encryptedKey := getEncryptedKeyFromHeaders(headers)
		logger.Debug("Request body appears encrypted",
			"body_length", len(body),
			"available_headers", headers,
			"found_encrypted_key", encryptedKey != "")

		if encryptedKey != "" {
			decryptedBody := decryptPayloadIfEncrypted(cryptoService, logger, body, encryptedKey)
			if decryptedBody != "[DECRYPT_FAILED]" {
				loggableBody = decryptedBody
			} else {
				loggableBody = "[ENCRYPTED_REQUEST_BODY]"
			}
		} else {
			logger.Debug("No encrypted key found in request headers", "headers", headers)
			loggableBody = "[ENCRYPTED_REQUEST_BODY]"
		}
	} else {
		// Use normal loggable body logic for non-encrypted content
		loggableBody = getLoggableRequestBody(c, body)
	}

	return &domain.HTTPLogRequest{
		RequestID:     requestID,
		Method:        c.Method(),
		Path:          c.Path(),
		QueryParams:   queryParams,
		Headers:       headers,
		Body:          loggableBody,
		ClientIP:      clientIP,
		UserAgent:     c.Get("User-Agent"),
		Timestamp:     timestamp,
		ContentType:   c.Get("Content-Type"),
		ContentLength: int64(len(loggableBody)),
	}
}

// captureResponseDetailsAfterEncrypt captures response details after encryption middleware runs
func captureResponseDetailsAfterEncrypt(c *fiber.Ctx, requestID string, timestamp time.Time, err error, cryptoService service.CryptoService, logger service.Logger) *domain.HTTPLogResponse {
	// Capture response headers
	headers := make(map[string]string)
	c.Response().Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})

	// Get response body - this should be encrypted after encryption middleware
	var body string
	if c.Response().Body() != nil {
		body = string(c.Response().Body())
	}

	// Determine status code
	statusCode := c.Response().StatusCode()
	if err != nil {
		// If there's an error, try to get the status code from the error
		if fiberErr, ok := err.(*fiber.Error); ok {
			statusCode = fiberErr.Code
		} else {
			statusCode = http.StatusInternalServerError
		}
	}

	// Format error message
	var errorMsg string
	if err != nil {
		errorMsg = err.Error()
	}

	// Check if body is encrypted/encoded and attempt to decrypt
	var loggableBody string

	// Check if we have the original response body stored before encryption
	if originalBody, exists := c.Locals("original_response_body").(string); exists && originalBody != "" {
		// Use the original response body (before encryption)
		loggableBody = originalBody
		logger.Debug("Using original response body (before encryption)",
			"original_length", len(originalBody),
			"encrypted_length", len(body),
			"original_preview", originalBody[:min(50, len(originalBody))])
	} else {
		// Fallback to encrypted body if original not available
		loggableBody = body
		logger.Debug("Using encrypted response body (original not available)",
			"body_length", len(body),
			"body_preview", body[:min(50, len(body))])
	}

	return &domain.HTTPLogResponse{
		RequestID:     requestID,
		StatusCode:    statusCode,
		Headers:       headers,
		Body:          loggableBody,
		ResponseTime:  0, // Will be calculated by caller
		Timestamp:     timestamp,
		ContentType:   c.Get("Content-Type"),
		ContentLength: int64(len(loggableBody)),
		Error:         errorMsg,
	}
}

// captureResponseDetails captures all relevant response information
func captureResponseDetails(c *fiber.Ctx, requestID string, timestamp time.Time, err error) *domain.HTTPLogResponse {
	// Capture response headers
	headers := make(map[string]string)
	c.Response().Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})

	// Get response body
	var body string
	if c.Response().Body() != nil {
		body = string(c.Response().Body())
	}

	// Determine status code
	statusCode := c.Response().StatusCode()
	if err != nil {
		// If there's an error, try to get the status code from the error
		if fiberErr, ok := err.(*fiber.Error); ok {
			statusCode = fiberErr.Code
		} else {
			statusCode = http.StatusInternalServerError
		}
	}

	// Format error message
	var errorMsg string
	if err != nil {
		errorMsg = err.Error()
	}

	// Check if body is encrypted/encoded - only log plain text bodies
	loggableBody := body

	return &domain.HTTPLogResponse{
		RequestID:     requestID,
		StatusCode:    statusCode,
		Headers:       headers,
		Body:          loggableBody,
		ResponseTime:  0, // Will be calculated by caller
		Timestamp:     timestamp,
		ContentType:   c.Get("Content-Type"),
		ContentLength: int64(len(loggableBody)),
		Error:         errorMsg,
	}
}

// captureResponseDetailsBeforeEncryption captures response details before encryption middleware runs
func captureResponseDetailsBeforeEncryption(c *fiber.Ctx, requestID string, timestamp time.Time, err error) *domain.HTTPLogResponse {
	// Capture response headers
	headers := make(map[string]string)
	c.Response().Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})

	// Get response body - this should be plain text before encryption
	var body string
	if c.Response().Body() != nil {
		body = string(c.Response().Body())
	}

	// Determine status code
	statusCode := c.Response().StatusCode()
	if err != nil {
		// If there's an error, try to get the status code from the error
		if fiberErr, ok := err.(*fiber.Error); ok {
			statusCode = fiberErr.Code
		} else {
			statusCode = http.StatusInternalServerError
		}
	}

	// Format error message
	var errorMsg string
	if err != nil {
		errorMsg = err.Error()
	}

	// Check if body is encrypted/encoded - only log plain text bodies
	loggableBody := body

	return &domain.HTTPLogResponse{
		RequestID:     requestID,
		StatusCode:    statusCode,
		Headers:       headers,
		Body:          loggableBody,
		ResponseTime:  0, // Will be calculated by caller
		Timestamp:     timestamp,
		ContentType:   c.Get("Content-Type"),
		ContentLength: int64(len(loggableBody)),
		Error:         errorMsg,
	}
}

// HTTPLoggingMiddlewareWithConfig creates a middleware with custom configuration
func HTTPLoggingMiddlewareWithConfig(config HTTPLoggingConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip logging for excluded paths
		if config.ExcludePaths != nil {
			for _, path := range config.ExcludePaths {
				if strings.HasPrefix(c.Path(), path) {
					return c.Next()
				}
			}
		}

		// Skip logging for excluded methods
		if config.ExcludeMethods != nil {
			for _, method := range config.ExcludeMethods {
				if c.Method() == method {
					return c.Next()
				}
			}
		}

		// Use the main middleware
		return HTTPLoggingMiddleware(config.Logger, config.HTTPLoggingRepo, config.CryptoService)(c)
	}
}

// HTTPLoggingConfig defines configuration for HTTP logging middleware
type HTTPLoggingConfig struct {
	Logger          service.Logger
	HTTPLoggingRepo domain.HTTPLoggingRepository
	CryptoService   service.CryptoService
	ExcludePaths    []string // Paths to exclude from logging
	ExcludeMethods  []string // HTTP methods to exclude from logging
	MaxBodySize     int      // Maximum body size to log (0 = no limit)
}

// DefaultHTTPLoggingConfig returns default configuration for HTTP logging
func DefaultHTTPLoggingConfig(logger service.Logger, httpLoggingRepo domain.HTTPLoggingRepository, cryptoService service.CryptoService) HTTPLoggingConfig {
	return HTTPLoggingConfig{
		Logger:          logger,
		HTTPLoggingRepo: httpLoggingRepo,
		CryptoService:   cryptoService,
		ExcludePaths:    []string{"/health", "/metrics"},
		ExcludeMethods:  []string{"OPTIONS"},
		MaxBodySize:     1024 * 1024, // 1MB
	}
}

// getLoggableRequestBody determines if request body should be logged (only plain text)
func getLoggableRequestBody(c *fiber.Ctx, body string) string {
	// Check if body is encrypted or encoded
	if isEncryptedOrEncoded(body) {
		return "[ENCRYPTED_REQUEST_BODY]"
	}

	// Check if body contains sensitive data patterns
	if containsSensitiveData(body) {
		return "[SENSITIVE_REQUEST_BODY]"
	}

	// Check if body is too large
	if len(body) > 1024*1024 { // 1MB limit
		return "[LARGE_REQUEST_BODY]"
	}

	return body
}

// isEncryptedOrEncoded checks if the body appears to be encrypted or encoded
func isEncryptedOrEncoded(body string) bool {
	if body == "" {
		return false
	}

	// Check for common encryption/encoding patterns
	bodyLower := strings.ToLower(body)

	// Check for base64-like patterns (high ratio of base64 characters)
	if isBase64Like(body) {
		return true
	}

	// Check for hex-encoded patterns
	if isHexEncoded(body) {
		return true
	}

	// Check for encrypted data patterns (high entropy, non-printable characters)
	if hasHighEntropy(body) {
		return true
	}

	// Check for specific encryption indicators
	encryptionIndicators := []string{
		"encrypted",
		"cipher",
		"aes",
		"rsa",
		"des",
		"3des",
		"blowfish",
		"twofish",
		"serpent",
		"camellia",
	}

	for _, indicator := range encryptionIndicators {
		if strings.Contains(bodyLower, indicator) {
			return true
		}
	}

	return false
}

// isBase64Like checks if string looks like base64 encoding
func isBase64Like(s string) bool {
	if len(s) < 8 { // Minimum length for base64
		return false
	}

	base64Chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/="
	base64Count := 0

	for _, char := range s {
		if strings.ContainsRune(base64Chars, char) {
			base64Count++
		}
	}

	// If more than 90% of characters are base64-like, consider it encoded
	// Base64 can have padding or not, so we don't require exact multiple of 4
	ratio := float64(base64Count) / float64(len(s))
	return ratio > 0.9
}

// isHexEncoded checks if string looks like hex encoding
func isHexEncoded(s string) bool {
	if len(s) < 4 || len(s)%2 != 0 { // Minimum length and must be even
		return false
	}

	hexChars := "0123456789ABCDEFabcdef"
	hexCount := 0

	for _, char := range s {
		if strings.ContainsRune(hexChars, char) {
			hexCount++
		}
	}

	// If more than 95% of characters are hex-like, consider it hex encoded
	return float64(hexCount)/float64(len(s)) > 0.95
}

// hasHighEntropy checks if string has high entropy (likely encrypted)
func hasHighEntropy(s string) bool {
	if len(s) < 10 {
		return false
	}

	// Count character frequency
	charCount := make(map[rune]int)
	for _, char := range s {
		charCount[char]++
	}

	// Calculate entropy
	entropy := 0.0
	length := float64(len(s))

	for _, count := range charCount {
		if count > 0 {
			p := float64(count) / length
			entropy -= p * math.Log2(p)
		}
	}

	// High entropy (> 4.5) suggests encrypted data
	return entropy > 4.5
}

// containsSensitiveData checks if body contains sensitive data patterns
func containsSensitiveData(body string) bool {
	if body == "" {
		return false
	}

	bodyLower := strings.ToLower(body)

	// Common sensitive data patterns
	sensitivePatterns := []string{
		"password",
		"passwd",
		"pwd",
		"secret",
		"token",
		"key",
		"private",
		"credential",
		"auth",
		"session",
		"cookie",
		"ssn",
		"social security",
		"credit card",
		"card number",
		"cvv",
		"cvc",
		"pin",
		"otp",
		"verification code",
		"api_key",
		"access_token",
		"refresh_token",
		"bearer",
		"authorization",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(bodyLower, pattern) {
			return true
		}
	}

	// Check for credit card number patterns (16 digits)
	if matched, _ := regexp.MatchString(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`, body); matched {
		return true
	}

	// Check for SSN patterns (XXX-XX-XXXX)
	if matched, _ := regexp.MatchString(`\b\d{3}[\s-]?\d{2}[\s-]?\d{4}\b`, body); matched {
		return true
	}

	return false
}

// decryptPayloadIfEncrypted attempts to decrypt payload if it appears to be encrypted
func decryptPayloadIfEncrypted(cryptoService service.CryptoService, logger service.Logger, body string, encryptedKey string) string {
	if body == "" || encryptedKey == "" {
		return body
	}

	// Check if body appears to be encrypted
	if !isEncryptedOrEncoded(body) {
		return body
	}

	// Log debug information for troubleshooting
	logger.Debug("Attempting to decrypt payload",
		"body_length", len(body),
		"body_preview", body[:min(50, len(body))],
		"encrypted_key_length", len(encryptedKey),
		"encrypted_key_preview", encryptedKey[:min(20, len(encryptedKey))])

	// Attempt to decrypt
	decryptedBytes, err := cryptoService.Decrypt(context.Background(), []byte(body), encryptedKey)
	if err != nil {
		logger.Warn("Failed to decrypt payload for logging",
			"error", err,
			"body_length", len(body),
			"encrypted_key_length", len(encryptedKey),
			"encrypted_key_preview", encryptedKey[:min(20, len(encryptedKey))])
		return "[DECRYPT_FAILED]"
	}

	logger.Debug("Successfully decrypted payload",
		"decrypted_length", len(decryptedBytes),
		"decrypted_preview", string(decryptedBytes[:min(100, len(decryptedBytes))]))

	return string(decryptedBytes)
}

// getEncryptedKeyFromHeaders extracts the encrypted key from request/response headers
func getEncryptedKeyFromHeaders(headers map[string]string) string {
	// Check for common header names that might contain the encrypted key
	keyHeaders := []string{"Key", "Api-Key", "X-Key", "X-Api-Key", "Encryption-Key"}

	for _, headerName := range keyHeaders {
		if key, exists := headers[headerName]; exists && key != "" {
			return key
		}
	}

	return ""
}
