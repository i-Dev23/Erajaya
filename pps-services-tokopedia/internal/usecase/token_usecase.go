package usecase

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
)

type tokenUsecaseImpl struct {
	redisClient service.RedisClient
	logger      service.Logger
}

func NewTokenUsecase(redisClient service.RedisClient, logger service.Logger) domain.TokenUsecase {
	return &tokenUsecaseImpl{
		redisClient: redisClient,
		logger:      logger,
	}
}

func (u *tokenUsecaseImpl) validateClientCredentials(clientID, clientSecret string) error {
	expectedClientID := utils.GetEnv("TP_CLIENT_ID", "")
	expectedClientSecret := utils.GetEnv("TP_CLIENT_SECRET", "")

	if clientID != expectedClientID {
		u.logger.Warn("Invalid client_id provided", "provided", clientID)
		return utils.ErrInvalidClientID
	}
	if clientSecret != expectedClientSecret {
		u.logger.Warn("Invalid client_secret provided")
		return utils.ErrInvalidClientSecret
	}
	return nil
}

// GenerateAndStoreToken implements domain.TokenUsecase interface
func (u *tokenUsecaseImpl) GenerateAndStoreToken(ctx context.Context, request *domain.GeneratedTokenRequestDomain) (string, error) {
	// Validate mandatory parameters
	if request.ClientID == "" {
		u.logger.Warn("Missing client_id in token request")
		return "", utils.ErrInvalidParameter
	}

	if request.ClientSecret == "" {
		u.logger.Warn("Missing client_secret in token request")
		return "", utils.ErrInvalidParameter
	}

	if request.Timestamp == "" {
		u.logger.Warn("Missing timestamp in token request")
		return "", utils.ErrInvalidParameter
	}

	// Convert from domain format to DTO format
	reqDto := dto.TokenRequestDto{
		ClientID:     request.ClientID,
		ClientSecret: request.ClientSecret,
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
	}
	return u.generateAndStoreToken(ctx, reqDto)
}

// generateAndStoreToken is the internal implementation
func (u *tokenUsecaseImpl) generateAndStoreToken(ctx context.Context, tokenRequestDto dto.TokenRequestDto) (string, error) {

	//validate the client credentials
	err := u.validateClientCredentials(tokenRequestDto.ClientID, tokenRequestDto.ClientSecret)
	if err != nil {
		u.logger.Error("Failed to validate client credentials", "error", err)
		return "", err
	}

	// Read expiration from environment variable (default to 3600 seconds if not set)
	expirationSeconds := utils.GetEnvAsInt("TOKEN_EXPIRATION_SECONDS", 3600)

	// Build JWT claims (stateless)
	now := time.Now()
	jti, _ := generateRandomToken() // reuse random generator for jti
	claims := jwt.RegisteredClaims{
		Issuer:    "pps-services-tokopedia",
		Subject:   tokenRequestDto.ClientID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expirationSeconds) * time.Second)),
		ID:        jti,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Use JWT secret from environment
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		u.logger.Error("JWT secret is not configured")
		return "", fmt.Errorf("jwt secret is not configured")
	}

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		u.logger.Error("Failed to sign JWT", "error", err)
		return "", err
	}

	u.logger.Info("JWT generated successfully", "subject", tokenRequestDto.ClientID, "expires_in_seconds", expirationSeconds)
	return signed, nil
}

func generateRandomToken() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 64) // 64 characters is a good length for a token
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

// ValidateToken implements domain.TokenUsecase interface
func (u *tokenUsecaseImpl) ValidateToken(ctx context.Context, tokenValue string) error {
	// Parse and validate JWT using secret from environment
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		u.logger.Error("JWT secret is not configured")
		return utils.ErrInvalidDigitalSignature
	}

	rc := &jwt.RegisteredClaims{}
	parsedToken, err := jwt.ParseWithClaims(tokenValue, rc, func(t *jwt.Token) (interface{}, error) {
		// Ensure signing method is HMAC
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil || !parsedToken.Valid {
		// Check if error is specifically token expiration
		if err != nil {
			errMsg := err.Error()
			// Check if it's an expiration error (case-insensitive check)
			if strings.Contains(strings.ToLower(errMsg), "expired") {
				u.logger.Error("Expired JWT token - Security Alert", "error", err)
				// Return expired token error to use response code "30"
				return utils.ErrExpiredToken
			} else {
				u.logger.Error("Invalid JWT token - Security Alert", "error", err)
			}
		} else {
			u.logger.Error("Invalid JWT token - Token validation failed", "error", "parsedToken.Valid is false")
		}
		return utils.ErrInvalidDigitalSignature
	}

	// Optional: validate subject against configured client ID
	expectedClientID := utils.GetEnv("TP_CLIENT_ID", "")
	if expectedClientID != "" && rc.Subject != expectedClientID {
		u.logger.Warn("JWT subject does not match expected client ID", "sub", rc.Subject)
		return utils.ErrInvalidDigitalSignature
	}

	return nil
}

// RevokeToken implements domain.TokenUsecase interface
func (u *tokenUsecaseImpl) RevokeToken(ctx context.Context, tokenID string) error {
	// Delete token from Redis
	result := u.redisClient.Del(ctx, tokenID)
	if err := result.Err(); err != nil {
		u.logger.Error("Failed to revoke token", "token", tokenID, "error", err)
		return err
	}

	u.logger.Info("Token revoked successfully", "token", tokenID)
	return nil
}
