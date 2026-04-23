package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"

	"github.com/gofiber/fiber/v2"
)

// TokenHandler handles token-related HTTP requests.
type TokenHandler struct {
	tokenUsecase domain.TokenUsecase
	logger       service.Logger
}

// NewTokenHandler creates a new TokenHandler with the given usecase.
func NewTokenHandler(tokenUsecase domain.TokenUsecase, logger service.Logger) *TokenHandler {
	return &TokenHandler{
		tokenUsecase: tokenUsecase,
		logger:       logger,
	}
}

// RegisterRoutes registers the token handler routes to the given Fiber app/group.
func (h *TokenHandler) RegisterRoutes(router fiber.Router) {
	router.Post("/token", h.GetToken)
}

// GetToken handles the POST /getToken route.
// Now expects the payload to be encrypted, and will pass the raw body to usecase for decryption.
func (h *TokenHandler) GetToken(c *fiber.Ctx) error {
	startTime := time.Now()
	var reqDto dto.TokenRequestDto

	ctx := c.UserContext()
	if ctx == nil {
		ctx = context.Background()
	}

	decryptedVal := c.Locals("decryptedBody")
	decryptedBody, ok := decryptedVal.([]byte)
	if !ok || decryptedBody == nil {
		duration := time.Since(startTime)
		h.logger.Error("decryptedBody missing or invalid", "duration_ms", duration.Milliseconds())
		rc, _ := utils.GetResponseCode("42")
		return c.Status(http.StatusOK).JSON(tokenResponseMap(rc.Code, rc.Message, "", false))
	}
	h.logger.Info("Token API Request Started",
		"endpoint", "/token",
		"method", "POST",
		"timestamp", startTime.Format(timeFormat),
		"request_body", string(decryptedBody))

	if err := json.Unmarshal(decryptedBody, &reqDto); err != nil {
		duration := time.Since(startTime)
		h.logger.Error("failed to parse request body", "error", err, "duration_ms", duration.Milliseconds())
		rc, _ := utils.GetResponseCode("42")
		return c.Status(http.StatusOK).JSON(tokenResponseMap(rc.Code, rc.Message, "", false))
	}

	token, err := h.tokenUsecase.GenerateAndStoreToken(ctx, &domain.GeneratedTokenRequestDomain{
		ClientID:     reqDto.ClientID,
		ClientSecret: reqDto.ClientSecret,
		Timestamp:    reqDto.Timestamp,
	})
	duration := time.Since(startTime)
	if err != nil {
		h.logger.Error("Failed to generate and store token", "error", err, "duration_ms", duration.Milliseconds())
		status, rc := mapErrorToResponseCode(err)
		return c.Status(status).JSON(tokenResponseMap(rc.Code, rc.Message, "", false))
	}
	rc, _ := utils.GetResponseCode("00")
	return c.Status(http.StatusOK).JSON(tokenResponseMap(rc.Code, rc.Message, token, true))
}

// tokenResponseMap returns a map for token response.
// showToken: true = include token key, false = omit token key
func tokenResponseMap(code, message, token string, showToken bool) map[string]interface{} {
	resp := map[string]interface{}{
		"response_code": code,
		"message":       message,
		"timestamp":     time.Now().Format(timeFormat),
	}
	if showToken {
		resp["token"] = token
	}
	return resp
}

// mapErrorToResponseCode mapping error usecase → http status + response code struct
func mapErrorToResponseCode(err error) (int, utils.ResponseCode) {
	switch err {
	case utils.ErrInvalidParameter:
		rc, _ := utils.GetResponseCode("42")
		return http.StatusOK, rc
	case utils.ErrInvalidClientID: // invalid credential
		rc, _ := utils.GetResponseCode("31")
		return http.StatusOK, rc
	case utils.ErrInvalidClientSecret: // invalid credential
		rc, _ := utils.GetResponseCode("31")
		return http.StatusOK, rc
	case utils.ErrInvalidDigitalSignature: // invalid signature
		rc, _ := utils.GetResponseCode("32")
		return http.StatusOK, rc
	default:
		rc, _ := utils.GetResponseCode("62")
		return http.StatusOK, rc
	}
}

// --- TEST/LEGACY COMPATIBILITY HELPERS ---
// Used by tests/benchmarks that expect old helpers
func newTokenResponse(code, message, token string) dto.TokenResponseDto {
	return dto.TokenResponseDto{
		ResponseCode: code,
		Message:      message,
		Token:        token,
		Timestamp:    time.Now().Format(timeFormat),
	}
}
func mapErrorToResponse(err error) (int, dto.TokenResponseDto) {
	switch err {
	case utils.ErrInvalidParameter:
		rc, _ := utils.GetResponseCode("42")
		return http.StatusOK, newTokenResponse(rc.Code, rc.Message, "")
	case utils.ErrInvalidClientID: // invalid credential
		rc, _ := utils.GetResponseCode("31")
		return http.StatusOK, newTokenResponse(rc.Code, rc.Message, "")
	case utils.ErrInvalidClientSecret: // invalid credential
		rc, _ := utils.GetResponseCode("31")
		return http.StatusOK, newTokenResponse(rc.Code, rc.Message, "")
	case utils.ErrInvalidDigitalSignature: // invalid signature
		rc, _ := utils.GetResponseCode("32")
		return http.StatusOK, newTokenResponse(rc.Code, rc.Message, "")
	default:
		rc, _ := utils.GetResponseCode("62")
		return http.StatusOK, newTokenResponse(rc.Code, rc.Message, "")
	}
}
