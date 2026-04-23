package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RateLimitConfig defines the rate limiting configuration for different endpoints
type RateLimitConfig struct {
	Requests int           // Number of requests allowed
	Window   time.Duration // Time window for the requests
}

// RateLimitRule defines rate limiting rules for specific paths
type RateLimitRule struct {
	Path   string
	Config RateLimitConfig
}

// Default rate limiting rules based on requirements
var DefaultRateLimitRules = []RateLimitRule{
	{Path: "/auth/token", Config: RateLimitConfig{Requests: utils.GetEnvAsInt("RATE_LIMIT_TOKEN", 5), Window: time.Hour}},
	{Path: "/api/v1/health", Config: RateLimitConfig{Requests: utils.GetEnvAsInt("RATE_LIMIT_HEALTH_CHECK", 1), Window: time.Minute}},
	{Path: "/api/v1/inquiry", Config: RateLimitConfig{Requests: utils.GetEnvAsInt("RATE_LIMIT_INQUIRY", 40), Window: time.Second}},
	{Path: "/api/v1/payment", Config: RateLimitConfig{Requests: utils.GetEnvAsInt("RATE_LIMIT_PAYMENT", 20), Window: time.Second}},
	{Path: "/api/v1/check-status", Config: RateLimitConfig{Requests: utils.GetEnvAsInt("RATE_LIMIT_CHECK_STATUS", 20), Window: time.Second}},
}

// RateLimitMiddleware creates a rate limiting middleware using Redis
func RateLimitMiddleware(redisClient service.RedisClient, cryptoService service.CryptoService, digitalSignatureService service.DigitalSignatureService, logger service.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get client identifier (IP address or user ID)
		clientID := getClientIdentifier(c)

		// Get the current path
		path := c.Path()

		// Find matching rate limit rule
		rule := findRateLimitRule(path)
		if rule == nil {
			// No rate limiting for this path
			return c.Next()
		}

		// Check rate limit
		allowed, err := checkRateLimit(c.Context(), redisClient, clientID, path, rule.Config, logger)
		if err != nil {
			logger.Error("Rate limit check failed", "error", err, "path", path, "client_id", clientID)
			// On Redis error, allow the request but log the error
			return c.Next()
		}

		if !allowed {
			logger.Warn("Rate limit exceeded", "path", path, "client_id", clientID, "limit", rule.Config.Requests, "window", rule.Config.Window)
			return handleRateLimitExceeded(c, cryptoService, digitalSignatureService, logger)
		}

		return c.Next()
	}
}

// RateLimitMiddlewareWithConfig creates a rate limiting middleware with custom rules
func RateLimitMiddlewareWithConfig(redisClient service.RedisClient, cryptoService service.CryptoService, digitalSignatureService service.DigitalSignatureService, logger service.Logger, rules []RateLimitRule) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get client identifier (IP address or user ID)
		clientID := getClientIdentifier(c)

		// Get the current path
		path := c.Path()

		// Find matching rate limit rule
		rule := findRateLimitRuleFromList(path, rules)
		if rule == nil {
			// No rate limiting for this path
			return c.Next()
		}

		// Check rate limit
		allowed, err := checkRateLimit(c.Context(), redisClient, clientID, path, rule.Config, logger)
		if err != nil {
			logger.Error("Rate limit check failed", "error", err, "path", path, "client_id", clientID)
			// On Redis error, allow the request but log the error
			return c.Next()
		}

		if !allowed {
			logger.Warn("Rate limit exceeded", "path", path, "client_id", clientID, "limit", rule.Config.Requests, "window", rule.Config.Window)
			return handleRateLimitExceeded(c, cryptoService, digitalSignatureService, logger)
		}

		return c.Next()
	}
}

// getClientIdentifier returns a unique identifier for the client
func getClientIdentifier(c *fiber.Ctx) string {
	// Try to get client IP from X-Real-IP header first
	clientIP := c.Get("X-Real-IP")
	if clientIP == "" {
		clientIP = c.IP()
	}
	return clientIP
}

// findRateLimitRule finds the rate limit rule for the given path
func findRateLimitRule(path string) *RateLimitRule {
	return findRateLimitRuleFromList(path, DefaultRateLimitRules)
}

// findRateLimitRuleFromList finds the rate limit rule from a list of rules
func findRateLimitRuleFromList(path string, rules []RateLimitRule) *RateLimitRule {
	for _, rule := range rules {
		if strings.HasPrefix(path, rule.Path) {
			return &rule
		}
	}
	return nil
}

// checkRateLimit checks if the client has exceeded the rate limit
func checkRateLimit(ctx context.Context, redisClient service.RedisClient, clientID, path string, config RateLimitConfig, logger service.Logger) (bool, error) {
	// Create a unique key for this client and path
	key := fmt.Sprintf("%s%s:%s", utils.RedisKeyRateLimitPrefix, clientID, strings.ReplaceAll(path, "/", "_"))

	// Get current count
	count, err := redisClient.Incr(ctx, key).Result()
	if err != nil {
		// If Redis is down or error, allow unlimited (return true with error)
		// The caller will log the error but allow the request
		return true, err
	}

	// Set expiration on first request
	if count == 1 {
		_, err = redisClient.Expire(ctx, key, config.Window).Result()
		if err != nil {
			logger.Warn("Failed to set rate limit expiration", "error", err, "key", key)
		}
	}

	// Check if limit exceeded
	return count <= int64(config.Requests), nil
}

// handleRateLimitExceeded handles the case when rate limit is exceeded
func handleRateLimitExceeded(c *fiber.Ctx, cryptoService service.CryptoService, digitalSignatureService service.DigitalSignatureService, logger service.Logger) error {
	// Get response code 23 (Limit exceeded) from mapping
	responseCode, found := utils.GetResponseCode("23")
	if !found {
		// Fallback to a generic rate limit response
		responseCode = utils.ResponseCode{
			Code:        "23",
			Message:     "Limit exceeded",
			Description: "User had exceeded top-up or payment limit",
			Behavior:    "Failed and Refund",
			Expected:    "Failed and Refund",
		}
	}

	// Create error response based on the path
	path := c.Path()
	var plainBody interface{}

	if path == "/auth/token" {
		plainBody = dto.TokenResponseDto{
			ResponseCode: responseCode.Code,
			Message:      responseCode.Message,
			Token:        "",
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}
	} else if strings.HasPrefix(path, "/api/v1/") {
		// For API v1 endpoints, create a generic response
		plainBody = dto.BaseTokopediaResponseDto{
			ResponseCode: responseCode.Code,
			Message:      responseCode.Message,
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}
	} else {
		// Default response
		plainBody = dto.BaseTokopediaResponseDto{
			ResponseCode: responseCode.Code,
			Message:      responseCode.Message,
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}
	}

	// Convert to JSON
	plainBodyBytes, err := json.Marshal(plainBody)
	if err != nil {
		logger.Error("Failed to marshal rate limit response", "error", err)
		return handleRateLimitError(c, cryptoService, digitalSignatureService, logger, "Failed to marshal response")
	}

	// Encrypt payload
	encryptedPayload, encryptedKey, err := cryptoService.Encrypt(c.Context(), plainBodyBytes)
	if err != nil {
		logger.Error("Failed to encrypt rate limit response", "error", err)
		return handleRateLimitError(c, cryptoService, digitalSignatureService, logger, "Failed to encrypt response")
	}

	// Sign plaintext (not ciphertext)
	signature, err := digitalSignatureService.SignPayload(c.Context(), string(plainBodyBytes))
	if err != nil {
		logger.Error("Failed to sign rate limit response", "error", err)
		return handleRateLimitError(c, cryptoService, digitalSignatureService, logger, "Failed to sign response")
	}

	// Set response headers
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Key", encryptedKey)
	c.Set("Signature", signature)
	c.Set("X-RateLimit-Limit", strconv.Itoa(getRateLimitForPath(path)))
	c.Set("X-RateLimit-Remaining", "0")
	c.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))

	// Return the encrypted response
	c.Response().SetBody([]byte(encryptedPayload))
	return c.Status(http.StatusOK).Send([]byte(encryptedPayload))
}

// handleRateLimitError handles errors during rate limit processing with response code 62
func handleRateLimitError(c *fiber.Ctx, cryptoService service.CryptoService, digitalSignatureService service.DigitalSignatureService, logger service.Logger, errorMessage string) error {
	// Get response code 62 (Server error) from mapping
	responseCode, found := utils.GetResponseCode("62")
	if !found {
		// Fallback to a generic server error response
		responseCode = utils.ResponseCode{
			Code:        "62",
			Message:     "Server error",
			Description: "Internal error on Partner's server",
			Behavior:    "Failed and Retry",
			Expected:    "Pending",
		}
	}

	// Create error response based on the path
	path := c.Path()
	var plainBody interface{}

	if path == "/auth/token" {
		plainBody = dto.TokenResponseDto{
			ResponseCode: responseCode.Code,
			Message:      responseCode.Message,
			Token:        "",
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}
	} else if strings.HasPrefix(path, "/api/v1/") {
		// For API v1 endpoints, create a generic response
		plainBody = dto.BaseTokopediaResponseDto{
			ResponseCode: responseCode.Code,
			Message:      responseCode.Message,
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}
	} else {
		// Default response
		plainBody = dto.BaseTokopediaResponseDto{
			ResponseCode: responseCode.Code,
			Message:      responseCode.Message,
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}
	}

	// Convert to JSON
	plainBodyBytes, err := json.Marshal(plainBody)
	if err != nil {
		logger.Error("Failed to marshal rate limit error response", "error", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}

	// Try to encrypt payload, but if it fails, return a simple JSON response
	encryptedPayload, encryptedKey, encryptErr := cryptoService.Encrypt(c.Context(), plainBodyBytes)
	if encryptErr != nil {
		logger.Error("Failed to encrypt rate limit error response", "error", encryptErr)
		return c.Status(http.StatusInternalServerError).JSON(plainBody)
	}

	// Try to sign payload, but if it fails, return without signature
	signature, signErr := digitalSignatureService.SignPayload(c.Context(), string(plainBodyBytes))
	if signErr != nil {
		logger.Error("Failed to sign rate limit error response", "error", signErr)
		// Return encrypted response without signature
		c.Set("Content-Type", "application/octet-stream")
		c.Set("Key", encryptedKey)
		c.Response().SetBody([]byte(encryptedPayload))
		return c.Status(http.StatusInternalServerError).Send([]byte(encryptedPayload))
	}

	// Set response headers
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Key", encryptedKey)
	c.Set("Signature", signature)

	// Return the encrypted response
	c.Response().SetBody([]byte(encryptedPayload))
	return c.Status(http.StatusOK).Send([]byte(encryptedPayload))
}

// getRateLimitForPath returns the rate limit for a given path
func getRateLimitForPath(path string) int {
	rule := findRateLimitRule(path)
	if rule != nil {
		return rule.Config.Requests
	}
	return 0
}

// RateLimitConfigBuilder helps build rate limit configurations
type RateLimitConfigBuilder struct {
	rules []RateLimitRule
}

// NewRateLimitConfigBuilder creates a new rate limit configuration builder
func NewRateLimitConfigBuilder() *RateLimitConfigBuilder {
	return &RateLimitConfigBuilder{
		rules: make([]RateLimitRule, 0),
	}
}

// AddRule adds a rate limit rule
func (b *RateLimitConfigBuilder) AddRule(path string, requests int, window time.Duration) *RateLimitConfigBuilder {
	b.rules = append(b.rules, RateLimitRule{
		Path: path,
		Config: RateLimitConfig{
			Requests: requests,
			Window:   window,
		},
	})
	return b
}

// Build returns the configured rate limit rules
func (b *RateLimitConfigBuilder) Build() []RateLimitRule {
	return b.rules
}

// DefaultRateLimitConfigBuilder returns a builder with default rules
func DefaultRateLimitConfigBuilder() *RateLimitConfigBuilder {
	builder := NewRateLimitConfigBuilder()

	for _, rule := range DefaultRateLimitRules {
		builder.AddRule(rule.Path, rule.Config.Requests, rule.Config.Window)
	}

	return builder
}
