package middleware

import (
	"context"
	"encoding/json"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// DecryptMiddleware melakukan verifikasi + decrypt payload
func DecryptRequestMiddleware(cryptoService service.CryptoService, digitalSignatureService service.DigitalSignatureService, logger service.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey := c.Get("Key")
		signature := c.Get("Signature")

		if apiKey == "" || signature == "" {
			responseCode, _ := utils.GetResponseCode("42") // missing required headers (Key, Signature)
			return encryptAndSignBearerErrorResponse(c, cryptoService, digitalSignatureService, logger, responseCode.Code, responseCode.Message)
		}

		// Ambil encrypted body
		rawBody := c.Body()
		if len(rawBody) == 0 {
			responseCode, _ := utils.GetResponseCode("42") // request body is empty
			return encryptAndSignBearerErrorResponse(c, cryptoService, digitalSignatureService, logger, responseCode.Code, responseCode.Message)
		}

		// Decrypt body
		decrypted, err := cryptoService.Decrypt(c.Context(), rawBody, apiKey)
		if err != nil {
			logger.Error("failed to decrypt body", "error", err)
			responseCode, _ := utils.GetResponseCode("33") // decryption failed
			return encryptAndSignBearerErrorResponse(c, cryptoService, digitalSignatureService, logger, responseCode.Code, responseCode.Message)
		}

		// Verifikasi signature
		if err := digitalSignatureService.VerifyPayload(c.Context(), string(decrypted), signature); err != nil {
			logger.Error("invalid signature", "error", err)
			responseCode, _ := utils.GetResponseCode("32") // invalid signature
			return encryptAndSignBearerErrorResponse(c, cryptoService, digitalSignatureService, logger, responseCode.Code, responseCode.Message)
		}

		c.Locals("decryptedBody", decrypted)
		return c.Next()
	}
}

type bearerErrorResponseBuilder func(c *fiber.Ctx, cryptoService service.CryptoService, responseCode, message string) interface{}

var bearerErrorResponseBuilders = map[string]bearerErrorResponseBuilder{
	"/auth/token":          buildTokenErrorResponse,
	"/api/v1/inquiry":      buildInquiryErrorResponse,
	"/api/v1/payment":      buildPaymentErrorResponse,
	"/api/v1/check-status": buildCheckStatusErrorResponse,
}

func buildTokenErrorResponse(c *fiber.Ctx, _ service.CryptoService, responseCode, message string) interface{} {
	return dto.TokenResponseDto{
		ResponseCode: responseCode,
		Message:      message,
		Token:        "",
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
	}
}

func buildInquiryErrorResponse(c *fiber.Ctx, cryptoService service.CryptoService, responseCode, message string) interface{} {
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

	return map[string]string{
		"client_number": inquiryReq.ClientNumber,
		"product_code":  inquiryReq.ProductCode,
		"response_code": responseCode,
		"message":       message,
		"timestamp":     time.Now().Format("2006-01-02 15:04:05"),
	}
}

func buildPaymentErrorResponse(c *fiber.Ctx, cryptoService service.CryptoService, responseCode, message string) interface{} {
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

	return map[string]string{
		"ref_id":        paymentReq.RefID,
		"client_number": paymentReq.ClientNumber,
		"product_code":  paymentReq.ProductCode,
		"response_code": responseCode,
		"message":       message,
		"timestamp":     time.Now().Format("2006-01-02 15:04:05"),
	}
}

func buildCheckStatusErrorResponse(c *fiber.Ctx, _ service.CryptoService, responseCode, message string) interface{} {
	var checkStatusReq dto.CheckStatusRequestDto
	_ = c.BodyParser(&checkStatusReq)
	return map[string]string{
		"ref_id":        checkStatusReq.RefID,
		"response_code": responseCode,
		"message":       message,
		"timestamp":     time.Now().Format("2006-01-02 15:04:05"),
	}
}

func buildDefaultErrorResponse(c *fiber.Ctx, _ service.CryptoService, responseCode, message string) interface{} {
	return dto.BaseTokopediaResponseDto{
		ResponseCode: responseCode,
		Message:      message,
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
	}
}

// encryptAndSignBearerErrorResponse helper function khusus untuk Bearer token error responses
func encryptAndSignBearerErrorResponse(c *fiber.Ctx, cryptoService service.CryptoService, digitalSignatureService service.DigitalSignatureService, logger service.Logger, responseCode, message string) error {
	// Create error response
	pathUrlApi := c.Path()
	builder, ok := bearerErrorResponseBuilders[pathUrlApi]
	if !ok {
		builder = buildDefaultErrorResponse
	}
	plainBody := builder(c, cryptoService, responseCode, message)

	// Convert to JSON first
	plainBodyBytes, err := json.Marshal(plainBody)
	if err != nil {
		logger.Error("failed to marshal response", "error", err)
		// Return plain error response to prevent infinite recursion
		errResponse := dto.BaseTokopediaResponseDto{
			ResponseCode: "62",
			Message:      "Internal server error",
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}
		errBodyBytes, _ := json.Marshal(errResponse)
		return c.Status(500).Send(errBodyBytes)
	}

	// Encrypt payload
	encryptedPayload, encryptedKey, err := cryptoService.Encrypt(c.Context(), plainBodyBytes)
	if err != nil {
		logger.Error("failed to encrypt response", "error", err)
		// Return plain error response to prevent infinite recursion
		errResponse := dto.BaseTokopediaResponseDto{
			ResponseCode: "62",
			Message:      "Internal server error",
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}
		errBodyBytes, _ := json.Marshal(errResponse)
		return c.Status(500).Send(errBodyBytes)
	}

	// Sign plaintext (bukan ciphertext)
	signature, err := digitalSignatureService.SignPayload(c.Context(), string(plainBodyBytes))
	if err != nil {
		logger.Error("failed to sign response", "error", err)
		// Return plain error response to prevent infinite recursion
		errResponse := dto.BaseTokopediaResponseDto{
			ResponseCode: "62",
			Message:      "Internal server error",
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}
		errBodyBytes, _ := json.Marshal(errResponse)
		return c.Status(500).Send(errBodyBytes)
	}

	// Replace body dengan payload terenkripsi
	c.Response().Header.Set("Content-Type", "application/octet-stream")
	c.Response().SetBody([]byte(encryptedPayload))

	// Set headers (Api-Key & Signature)
	c.Set("Key", encryptedKey)
	c.Set("Signature", signature)

	return nil
}

// EncryptResponseMiddleware mengenkripsi response API + menambahkan signature
func EncryptResponseMiddleware(cryptoService service.CryptoService, digitalSignatureService service.DigitalSignatureService, logger service.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Jalankan handler dulu
		if err := c.Next(); err != nil {
			return err
		}

		// Ambil response body plain (hasil handler)
		plainBody := c.Response().Body()
		if len(plainBody) == 0 {
			logger.Debug("EncryptResponseMiddleware: Response body is empty, skipping encryption")
			return nil // skip kalau kosong
		}

		// Store original response body for logging middleware
		c.Locals("original_response_body", string(plainBody))
		logger.Debug("EncryptResponseMiddleware: Stored original response body",
			"body_length", len(plainBody),
			"body_preview", string(plainBody[:min(50, len(plainBody))]))

		// 🔑 Normalisasi payload sebelum sign
		// Kalau payload JSON, decode & re-marshal biar konsisten
		var normalized interface{}
		if err := json.Unmarshal(plainBody, &normalized); err == nil {
			plainBody, _ = json.Marshal(normalized)
		}

		// Encrypt payload
		encryptedPayload, encryptedKey, err := cryptoService.Encrypt(c.Context(), plainBody)
		if err != nil {
			logger.Error("failed to encrypt response", "error", err)
			responseCode, _ := utils.GetResponseCode("62") // internal server error
			return encryptAndSignBearerErrorResponse(c, cryptoService, digitalSignatureService, logger, responseCode.Code, responseCode.Message)
		}

		// Sign plaintext (bukan ciphertext)
		signature, err := digitalSignatureService.SignPayload(c.Context(), string(plainBody))
		if err != nil {
			logger.Error("failed to sign response", "error", err)
			responseCode, _ := utils.GetResponseCode("62") // internal server error
			return encryptAndSignBearerErrorResponse(c, cryptoService, digitalSignatureService, logger, responseCode.Code, responseCode.Message)
		}

		// Replace body dengan payload terenkripsi
		c.Response().Header.Set("Content-Type", "application/octet-stream")
		c.Response().SetBody([]byte(encryptedPayload))

		// Set headers (Api-Key & Signature)
		c.Set("Key", encryptedKey)
		c.Set("Signature", signature)

		return nil
	}
}

// Token validation middleware: check Bearer token in header Authorization, validate in usecase token
func CheckBearerTokenMiddleware(tokenUsecase domain.TokenUsecase, cryptoService service.CryptoService, digitalSignatureService service.DigitalSignatureService, logger service.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")

		// Check if Authorization header is missing
		if authHeader == "" {
			logger.Warn("Authorization header missing")
			responseCode, _ := utils.GetResponseCode("30") //invalid token
			return encryptAndSignBearerErrorResponse(c, cryptoService, digitalSignatureService, logger, responseCode.Code, responseCode.Message)
		}

		// Check if Authorization header format is invalid
		const prefix = "Bearer "
		if len(authHeader) <= len(prefix) || authHeader[:len(prefix)] != prefix {
			logger.Warn("Invalid Authorization header format")
			responseCode, _ := utils.GetResponseCode("30") //invalid token
			return encryptAndSignBearerErrorResponse(c, cryptoService, digitalSignatureService, logger, responseCode.Code, responseCode.Message)
		}

		// Check if Bearer token is missing
		token := authHeader[len(prefix):]
		if token == "" {
			logger.Warn("Bearer token missing")
			responseCode, _ := utils.GetResponseCode("30") //invalid token
			return encryptAndSignBearerErrorResponse(c, cryptoService, digitalSignatureService, logger, responseCode.Code, responseCode.Message)
		}

		// Get context
		ctx := c.UserContext()
		if ctx == nil {
			ctx = context.Background()
		}

		// Validate token in usecase token
		if err := tokenUsecase.ValidateToken(ctx, token); err != nil {
			// Check if error is expired token or other invalid token
			// Both cases use response code "30" (Invalid token) per mapping_response_code_utils.go
			if err == utils.ErrExpiredToken {
				logger.Error("Expired token detected", "token", token, "error", err)
			} else {
				logger.Error("Invalid token detected", "token", token, "error", err)
			}
			responseCode, _ := utils.GetResponseCode("30") // Invalid token (includes expired token)
			return encryptAndSignBearerErrorResponse(c, cryptoService, digitalSignatureService, logger, responseCode.Code, responseCode.Message)
		}

		// Token valid, continue to next handler
		return c.Next()
	}
}
