package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// IPWhitelistMiddleware creates a middleware to check IP whitelist from Redis with Oracle fallback
func IPWhitelistMiddleware(redisClient service.RedisClient, productRepo domain.ProductRepository, cryptoService service.CryptoService, digitalSignatureService service.DigitalSignatureService, logger service.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get client IP
		clientIP := getClientIP(c)

		// Check if IP is whitelisted (with Oracle fallback)
		isWhitelisted, err := checkIPWhitelistWithFallback(c.Context(), redisClient, productRepo, clientIP, logger)
		if err != nil {
			logger.Error("IP whitelist check failed - Oracle exception occurred", "error", err, "client_ip", clientIP)
			// Oracle exception (database error) - return code 62 (Internal Server Error)
			return handleIPWhitelistError(c, cryptoService, digitalSignatureService, logger, "Failed to validate IP whitelist due to Oracle exception")
		}

		if !isWhitelisted {
			logger.Warn("IP not whitelisted", "client_ip", clientIP)
			// IP not in whitelist (business logic) - return code 60 (Access is not allowed)
			return handleIPNotWhitelisted(c, cryptoService, digitalSignatureService, logger)
		}

		return c.Next()
	}
}

// getClientIP returns the client IP address
func getClientIP(c *fiber.Ctx) string {
	// Try to get client IP from X-Real-IP header first (for load balancer)
	clientIP := c.Get("X-Real-IP")
	if clientIP == "" {
		clientIP = c.IP()
	}
	return clientIP
}

// checkIPWhitelistWithFallback checks if the given IP is in the whitelist from Redis, with Oracle fallback
func checkIPWhitelistWithFallback(ctx context.Context, redisClient service.RedisClient, productRepo domain.ProductRepository, clientIP string, logger service.Logger) (bool, error) {
	// Try to get whitelisted IPs from Redis first
	whitelistData, err := redisClient.Get(ctx, utils.RedisKeyWhitelistedIP).Result()

	// If Redis error (connection issue, not just key not found), fallback to Oracle
	if err != nil && err != redis.Nil {
		logger.Warn("Redis error when checking IP whitelist, falling back to Oracle",
			"error", err,
			"client_ip", clientIP)

		// Fallback to Oracle
		return checkIPWhitelistFromOracle(ctx, productRepo, redisClient, clientIP, logger)
	}

	// If key not found in Redis (redis.Nil), also fallback to Oracle
	if err == redis.Nil || strings.TrimSpace(whitelistData) == "" {
		logger.Info("IP whitelist not found in Redis, falling back to Oracle",
			"client_ip", clientIP)

		return checkIPWhitelistFromOracle(ctx, productRepo, redisClient, clientIP, logger)
	}

	// Redis has data, validate IP
	return validateIPInList(whitelistData, clientIP), nil
}

// checkIPWhitelistFromOracle fetches IP whitelist from Oracle and validates
func checkIPWhitelistFromOracle(ctx context.Context, productRepo domain.ProductRepository, redisClient service.RedisClient, clientIP string, logger service.Logger) (bool, error) {
	// Get username from environment
	username := os.Getenv("TP_CLIENT_ID")
	if username == "" {
		username = "ALFA-DEV"
	}

	logger.Info("Fetching IP whitelist from Oracle",
		"username", username,
		"client_ip", clientIP)

	// Call Oracle to get whitelisted IPs
	ipResponse, err := productRepo.GetIpByUser(ctx, username)
	if err != nil {
		logger.Error("Oracle exception: Failed to get IP whitelist from Oracle database",
			"error", err,
			"username", username,
			"error_type", "oracle_exception")
		// Propagate error untuk trigger response code 62 (Internal Server Error)
		return false, err
	}

	// Check if Oracle response is valid (business logic validation)
	if ipResponse.OuterRCode != "0" || ipResponse.OutIp == "" {
		logger.Warn("Oracle returned invalid IP whitelist data (business logic error, not exception)",
			"outerrcode", ipResponse.OuterRCode,
			"outerrmsg", ipResponse.OuterRMsg,
			"username", username)
		// Return false WITHOUT error - trigger response code 60 (Access denied)
		return false, nil
	}

	logger.Info("Successfully fetched IP whitelist from Oracle",
		"ip_list", ipResponse.OutIp,
		"username", username)

	// Try to save to Redis asynchronously (best effort, don't block on error)
	go func() {
		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := redisClient.Set(saveCtx, utils.RedisKeyWhitelistedIP, ipResponse.OutIp, 0).Err()
		if err != nil {
			logger.Error("Failed to save IP whitelist to Redis cache (async)",
				"error", err)
		} else {
			logger.Info("Successfully cached IP whitelist to Redis (async)",
				"ip_list", ipResponse.OutIp)
		}
	}()

	// Validate IP against Oracle data
	return validateIPInList(ipResponse.OutIp, clientIP), nil
}

// validateIPInList checks if clientIP exists in comma-separated IP list
func validateIPInList(ipList string, clientIP string) bool {
	if strings.TrimSpace(ipList) == "" {
		return false
	}

	// Split comma-separated IPs
	whitelistedIPs := strings.Split(ipList, ",")

	// Check if client IP is in the whitelist
	for _, whitelistedIP := range whitelistedIPs {
		if strings.TrimSpace(whitelistedIP) == clientIP {
			return true
		}
	}

	return false
}

// checkIPWhitelist checks if the given IP is in the whitelist (legacy function, kept for compatibility)
func checkIPWhitelist(ctx context.Context, redisClient service.RedisClient, clientIP string, logger service.Logger) (bool, error) {
	// Get whitelisted IPs from Redis
	whitelistData, err := redisClient.Get(ctx, "WHITELISTED_IP").Result()
	if err != nil {
		return false, err
	}

	// Handle empty whitelist data
	if strings.TrimSpace(whitelistData) == "" {
		return false, nil
	}

	return validateIPInList(whitelistData, clientIP), nil
}

// handleIPNotWhitelisted handles the case when IP is not whitelisted
func handleIPNotWhitelisted(c *fiber.Ctx, cryptoService service.CryptoService, digitalSignatureService service.DigitalSignatureService, logger service.Logger) error {
	// Get response code 60 (Access is not allowed) from mapping
	responseCode, found := utils.GetResponseCode("60")
	if !found {
		// Fallback to a generic access denied response
		responseCode = utils.ResponseCode{
			Code:        "60",
			Message:     "Access is not allowed",
			Description: "Tokopedia's IP is blocked or no API access is allowed",
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
	} else if path == "/api/v1/inquiry" { // inquiry
		var inquiryReq dto.InquiryRequestDto
		// Prefer decrypted body if available; else attempt decrypt via header Key
		if v := c.Locals("decryptedBody"); v != nil {
			if b, ok := v.([]byte); ok {
				_ = json.Unmarshal(b, &inquiryReq)
			}
		} else {
			apiKey := c.Get("Key")
			if apiKey != "" {
				if dec, err := cryptoService.Decrypt(c.Context(), c.Body(), apiKey); err == nil {
					_ = json.Unmarshal(dec, &inquiryReq)
				}
			}
		}
		plainBody = map[string]string{
			"client_number": inquiryReq.ClientNumber,
			"product_code":  inquiryReq.ProductCode,
			"response_code": responseCode.Code,
			"message":       responseCode.Message,
			"timestamp":     time.Now().Format("2006-01-02 15:04:05"),
		}
	} else if path == "/api/v1/payment" { // payment
		var paymentReq dto.PaymentRequestDto
		if v := c.Locals("decryptedBody"); v != nil {
			if b, ok := v.([]byte); ok {
				_ = json.Unmarshal(b, &paymentReq)
			}
		} else {
			apiKey := c.Get("Key")
			if apiKey != "" {
				if dec, err := cryptoService.Decrypt(c.Context(), c.Body(), apiKey); err == nil {
					_ = json.Unmarshal(dec, &paymentReq)
				}
			}
		}
		plainBody = map[string]string{
			"ref_id":        paymentReq.RefID,
			"client_number": paymentReq.ClientNumber,
			"product_code":  paymentReq.ProductCode,
			"response_code": responseCode.Code,
			"message":       responseCode.Message,
			"timestamp":     time.Now().Format("2006-01-02 15:04:05"),
		}
	} else if path == "/api/v1/check-status" { // check status
		var checkReq dto.CheckStatusRequestDto
		if v := c.Locals("decryptedBody"); v != nil {
			if b, ok := v.([]byte); ok {
				_ = json.Unmarshal(b, &checkReq)
			}
		} else {
			apiKey := c.Get("Key")
			if apiKey != "" {
				if dec, err := cryptoService.Decrypt(c.Context(), c.Body(), apiKey); err == nil {
					_ = json.Unmarshal(dec, &checkReq)
				}
			}
		}
		plainBody = map[string]string{
			"ref_id":        checkReq.RefID,
			"response_code": responseCode.Code,
			"message":       responseCode.Message,
			"timestamp":     time.Now().Format("2006-01-02 15:04:05"),
		}
	} else if strings.HasPrefix(path, "/api/v1/") {
		// Fallback generic response for other API v1 endpoints
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
		logger.Error("Failed to marshal IP whitelist response", "error", err)
		return handleIPWhitelistError(c, cryptoService, digitalSignatureService, logger, "Failed to marshal response")
	}

	// Encrypt payload
	encryptedPayload, encryptedKey, err := cryptoService.Encrypt(c.Context(), plainBodyBytes)
	if err != nil {
		logger.Error("Failed to encrypt IP whitelist response", "error", err)
		return handleIPWhitelistError(c, cryptoService, digitalSignatureService, logger, "Failed to encrypt response")
	}

	// Sign plaintext (not ciphertext)
	signature, err := digitalSignatureService.SignPayload(c.Context(), string(plainBodyBytes))
	if err != nil {
		logger.Error("Failed to sign IP whitelist response", "error", err)
		return handleIPWhitelistError(c, cryptoService, digitalSignatureService, logger, "Failed to sign response")
	}

	// Set response headers
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Key", encryptedKey)
	c.Set("Signature", signature)

	// Return the encrypted response with HTTP 200
	c.Response().SetBody([]byte(encryptedPayload))
	return c.Status(http.StatusOK).Send([]byte(encryptedPayload))
}

// handleIPWhitelistError handles errors during IP whitelist processing with response code 62
func handleIPWhitelistError(c *fiber.Ctx, cryptoService service.CryptoService, digitalSignatureService service.DigitalSignatureService, logger service.Logger, errorMessage string) error {
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
	} else if path == "/api/v1/inquiry" { // inquiry
		var inquiryReq dto.InquiryRequestDto
		if v := c.Locals("decryptedBody"); v != nil {
			if b, ok := v.([]byte); ok {
				_ = json.Unmarshal(b, &inquiryReq)
			}
		} else {
			apiKey := c.Get("Key")
			if apiKey != "" {
				if dec, err := cryptoService.Decrypt(c.Context(), c.Body(), apiKey); err == nil {
					_ = json.Unmarshal(dec, &inquiryReq)
				}
			}
		}
		plainBody = map[string]string{
			"client_number": inquiryReq.ClientNumber,
			"product_code":  inquiryReq.ProductCode,
			"response_code": responseCode.Code,
			"message":       responseCode.Message,
			"timestamp":     time.Now().Format("2006-01-02 15:04:05"),
		}
	} else if path == "/api/v1/payment" { // payment
		var paymentReq dto.PaymentRequestDto
		if v := c.Locals("decryptedBody"); v != nil {
			if b, ok := v.([]byte); ok {
				_ = json.Unmarshal(b, &paymentReq)
			}
		} else {
			apiKey := c.Get("Key")
			if apiKey != "" {
				if dec, err := cryptoService.Decrypt(c.Context(), c.Body(), apiKey); err == nil {
					_ = json.Unmarshal(dec, &paymentReq)
				}
			}
		}
		plainBody = map[string]string{
			"ref_id":        paymentReq.RefID,
			"client_number": paymentReq.ClientNumber,
			"product_code":  paymentReq.ProductCode,
			"response_code": responseCode.Code,
			"message":       responseCode.Message,
			"timestamp":     time.Now().Format("2006-01-02 15:04:05"),
		}
	} else if path == "/api/v1/check-status" { // check status
		var checkReq dto.CheckStatusRequestDto
		if v := c.Locals("decryptedBody"); v != nil {
			if b, ok := v.([]byte); ok {
				_ = json.Unmarshal(b, &checkReq)
			}
		} else {
			apiKey := c.Get("Key")
			if apiKey != "" {
				if dec, err := cryptoService.Decrypt(c.Context(), c.Body(), apiKey); err == nil {
					_ = json.Unmarshal(dec, &checkReq)
				}
			}
		}
		plainBody = map[string]string{
			"ref_id":        checkReq.RefID,
			"response_code": responseCode.Code,
			"message":       responseCode.Message,
			"timestamp":     time.Now().Format("2006-01-02 15:04:05"),
		}
	} else if strings.HasPrefix(path, "/api/v1/") {
		// Fallback generic response for other API v1 endpoints
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
		logger.Error("Failed to marshal IP whitelist error response", "error", err)
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}

	// Try to encrypt payload, but if it fails, return a simple JSON response
	encryptedPayload, encryptedKey, encryptErr := cryptoService.Encrypt(c.Context(), plainBodyBytes)
	if encryptErr != nil {
		logger.Error("Failed to encrypt IP whitelist error response", "error", encryptErr)
		return c.Status(http.StatusOK).JSON(plainBody)
	}

	// Try to sign payload, but if it fails, return without signature
	signature, signErr := digitalSignatureService.SignPayload(c.Context(), string(plainBodyBytes))
	if signErr != nil {
		logger.Error("Failed to sign IP whitelist error response", "error", signErr)
		// Return encrypted response without signature
		c.Set("Content-Type", "application/octet-stream")
		c.Set("Key", encryptedKey)
		c.Response().SetBody([]byte(encryptedPayload))
		return c.Status(http.StatusOK).Send([]byte(encryptedPayload))
	}

	// Set response headers
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Key", encryptedKey)
	c.Set("Signature", signature)

	// Return the encrypted response with HTTP 200
	c.Response().SetBody([]byte(encryptedPayload))
	return c.Status(http.StatusOK).Send([]byte(encryptedPayload))
}
