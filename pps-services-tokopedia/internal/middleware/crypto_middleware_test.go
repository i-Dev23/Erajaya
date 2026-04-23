package middleware

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"pps-services-tokopedia/internal/domain"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mocks ---

type mockLogger struct{ mock.Mock }

func (m *mockLogger) Debug(msg string, args ...interface{}) { m.Called(msg, args) }
func (m *mockLogger) Info(msg string, args ...interface{})  { m.Called(msg, args) }
func (m *mockLogger) Warn(msg string, args ...interface{})  { m.Called(msg, args) }
func (m *mockLogger) Error(msg string, args ...interface{}) { m.Called(msg, args) }

type mockCryptoService struct{ mock.Mock }

func (m *mockCryptoService) Encrypt(ctx context.Context, payload []byte) ([]byte, string, error) {
	args := m.Called(ctx, payload)
	return args.Get(0).([]byte), args.String(1), args.Error(2)
}
func (m *mockCryptoService) Decrypt(ctx context.Context, encryptedPayload []byte, encryptedKey string) ([]byte, error) {
	args := m.Called(ctx, encryptedPayload, encryptedKey)
	return args.Get(0).([]byte), args.Error(1)
}

type mockDigitalSignatureService struct{ mock.Mock }

func (m *mockDigitalSignatureService) SignPayload(ctx context.Context, payload string) (string, error) {
	args := m.Called(ctx, payload)
	return args.String(0), args.Error(1)
}

// Only one definition of VerifyPayload should exist
func (m *mockDigitalSignatureService) VerifyPayload(ctx context.Context, payload, signature string) error {
	args := m.Called(ctx, payload, signature)
	return args.Error(0)
}

type mockTokenUsecase struct{ mock.Mock }

func (m *mockTokenUsecase) GenerateAndStoreToken(ctx context.Context, request *domain.GeneratedTokenRequestDomain) (string, error) {
	args := m.Called(ctx, request)
	return args.String(0), args.Error(1)
}

func (m *mockTokenUsecase) ValidateToken(ctx context.Context, tokenValue string) error {
	args := m.Called(ctx, tokenValue)
	return args.Error(0)
}

func (m *mockTokenUsecase) RevokeToken(ctx context.Context, tokenID string) error {
	args := m.Called(ctx, tokenID)
	return args.Error(0)
}

// --- Tests ---

// Moved test functions below package/imports

func TestDecryptRequestMiddleware_Success(t *testing.T) {
	// Test implementation...
}

// --- Additional black box tests for DecryptRequestMiddleware ---

func TestDecryptRequestMiddleware_EmptyBody(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}

	// Setup mocks for error response encryption
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted-error"), "encrypted-key", nil)
	digitalSignatureService.On("SignPayload", mock.Anything, mock.Anything).Return("signature", nil)
	logger.On("Error", mock.Anything, mock.Anything).Return().Maybe()

	app.Use(DecryptRequestMiddleware(cryptoService, digitalSignatureService, logger))
	app.Post("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "success"})
	})

	req := httptest.NewRequest("POST", "/test", nil) // empty body
	req.Header.Set("Key", "test-key")
	req.Header.Set("Signature", "test-signature")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestDecryptRequestMiddleware_SignatureVerificationFailure(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}

	plainText := `{"test": "data"}`
	cryptoService.On("Decrypt", mock.Anything, mock.Anything, mock.Anything).Return([]byte(plainText), nil)
	digitalSignatureService.On("VerifyPayload", mock.Anything, plainText, mock.Anything).Return(errors.New("invalid signature"))
	logger.On("Error", "invalid signature", mock.Anything).Return()
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted-error"), "encrypted-key", nil)
	digitalSignatureService.On("SignPayload", mock.Anything, mock.Anything).Return("signature", nil)

	app.Use(DecryptRequestMiddleware(cryptoService, digitalSignatureService, logger))
	app.Post("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "success"})
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader("encrypted-data"))
	req.Header.Set("Key", "test-key")
	req.Header.Set("Signature", "test-signature")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestDecryptRequestMiddleware_MissingBothHeaders(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}

	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted-error"), "encrypted-key", nil)
	digitalSignatureService.On("SignPayload", mock.Anything, mock.Anything).Return("signature", nil)
	logger.On("Error", mock.Anything, mock.Anything).Return().Maybe()

	app.Use(DecryptRequestMiddleware(cryptoService, digitalSignatureService, logger))
	app.Post("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "success"})
	})

	req := httptest.NewRequest("POST", "/test", nil) // empty body, no headers

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestDecryptRequestMiddleware_MissingHeaders(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}

	// Setup mocks for error response encryption
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted-error"), "encrypted-key", nil)
	digitalSignatureService.On("SignPayload", mock.Anything, mock.Anything).Return("signature", nil)
	logger.On("Error", mock.Anything, mock.Anything).Return()

	app.Use(DecryptRequestMiddleware(cryptoService, digitalSignatureService, logger))
	app.Post("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "success"})
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader("encrypted-data"))
	// No headers set

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Should not call decrypt/verify but should call encrypt/sign for error response
	cryptoService.AssertNotCalled(t, "Decrypt")
	digitalSignatureService.AssertNotCalled(t, "VerifyPayload")
	cryptoService.AssertCalled(t, "Encrypt", mock.Anything, mock.Anything)
	digitalSignatureService.AssertCalled(t, "SignPayload", mock.Anything, mock.Anything)
}

func TestDecryptRequestMiddleware_DecryptionFailure(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}

	// Mock decryption failure
	cryptoService.On("Decrypt", mock.Anything, mock.Anything, mock.Anything).Return([]byte{}, errors.New("decryption failed"))
	logger.On("Error", "failed to decrypt body", mock.Anything).Return()

	// Setup mocks for error response encryption
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted-error"), "encrypted-key", nil)
	digitalSignatureService.On("SignPayload", mock.Anything, mock.Anything).Return("signature", nil)

	app.Use(DecryptRequestMiddleware(cryptoService, digitalSignatureService, logger))
	app.Post("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "success"})
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader("encrypted-data"))
	req.Header.Set("Key", "test-key")
	req.Header.Set("Signature", "test-signature")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	cryptoService.AssertExpectations(t)
	logger.AssertExpectations(t)
}

func TestEncryptResponseMiddleware_Success(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}

	// Mock successful encryption and signing
	responseData := fiber.Map{"status": "success"}
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted-data"), "encrypted-key", nil)
	digitalSignatureService.On("SignPayload", mock.Anything, mock.Anything).Return("signature", nil)
	logger.On("Debug", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()

	app.Use(EncryptResponseMiddleware(cryptoService, digitalSignatureService, logger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(responseData)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Check headers
	assert.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "encrypted-key", resp.Header.Get("Key"))
	assert.Equal(t, "signature", resp.Header.Get("Signature"))

	cryptoService.AssertExpectations(t)
	digitalSignatureService.AssertExpectations(t)
}

func TestEncryptResponseMiddleware_EncryptionFailure(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}

	// Mock encryption failure first, then success for error response
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte{}, "", errors.New("encryption failed")).Once()
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted-error"), "encrypted-key", nil).Once()
	logger.On("Error", "failed to encrypt response", mock.Anything).Return()
	logger.On("Debug", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
	digitalSignatureService.On("SignPayload", mock.Anything, mock.Anything).Return("signature", nil)

	app.Use(EncryptResponseMiddleware(cryptoService, digitalSignatureService, logger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	cryptoService.AssertExpectations(t)
	logger.AssertExpectations(t)
}

func TestCheckBearerTokenMiddleware_Success(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}
	tokenUsecase := &mockTokenUsecase{}

	// Mock successful token validation
	tokenUsecase.On("ValidateToken", mock.Anything, "valid-token").Return(nil)

	app.Use(CheckBearerTokenMiddleware(tokenUsecase, cryptoService, digitalSignatureService, logger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	tokenUsecase.AssertExpectations(t)
}

func TestCheckBearerTokenMiddleware_MissingAuthHeader(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}
	tokenUsecase := &mockTokenUsecase{}

	// Mock encryption and signing for error response
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted-error"), "encrypted-key", nil)
	digitalSignatureService.On("SignPayload", mock.Anything, mock.Anything).Return("signature", nil)
	logger.On("Warn", mock.Anything, mock.Anything).Return()

	app.Use(CheckBearerTokenMiddleware(tokenUsecase, cryptoService, digitalSignatureService, logger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// No Authorization header

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Check headers
	assert.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "encrypted-key", resp.Header.Get("Key"))
	assert.Equal(t, "signature", resp.Header.Get("Signature"))

	logger.AssertExpectations(t)
	cryptoService.AssertExpectations(t)
	digitalSignatureService.AssertExpectations(t)
}

func TestCheckBearerTokenMiddleware_InvalidFormat(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}
	tokenUsecase := &mockTokenUsecase{}

	// Mock encryption and signing for error response
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted-error"), "encrypted-key", nil)
	digitalSignatureService.On("SignPayload", mock.Anything, mock.Anything).Return("signature", nil)
	logger.On("Warn", mock.Anything, mock.Anything).Return()

	app.Use(CheckBearerTokenMiddleware(tokenUsecase, cryptoService, digitalSignatureService, logger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "InvalidFormat token")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Check headers
	assert.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "encrypted-key", resp.Header.Get("Key"))
	assert.Equal(t, "signature", resp.Header.Get("Signature"))

	logger.AssertExpectations(t)
	cryptoService.AssertExpectations(t)
	digitalSignatureService.AssertExpectations(t)
}

func TestCheckBearerTokenMiddleware_MissingToken(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}
	tokenUsecase := &mockTokenUsecase{}

	// Mock encryption and signing for error response
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted-error"), "encrypted-key", nil)
	digitalSignatureService.On("SignPayload", mock.Anything, mock.Anything).Return("signature", nil)
	logger.On("Warn", mock.Anything, mock.Anything).Return()

	app.Use(CheckBearerTokenMiddleware(tokenUsecase, cryptoService, digitalSignatureService, logger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer ")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Check headers
	assert.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "encrypted-key", resp.Header.Get("Key"))
	assert.Equal(t, "signature", resp.Header.Get("Signature"))

	logger.AssertExpectations(t)
	cryptoService.AssertExpectations(t)
	digitalSignatureService.AssertExpectations(t)
}

func TestCheckBearerTokenMiddleware_InvalidToken(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}
	tokenUsecase := &mockTokenUsecase{}

	// Mock token validation failure
	tokenUsecase.On("ValidateToken", mock.Anything, "invalid-token").Return(errors.New("invalid token"))

	// Mock encryption and signing for error response
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted-error"), "encrypted-key", nil)
	digitalSignatureService.On("SignPayload", mock.Anything, mock.Anything).Return("signature", nil)
	logger.On("Error", mock.Anything, mock.Anything).Return()

	app.Use(CheckBearerTokenMiddleware(tokenUsecase, cryptoService, digitalSignatureService, logger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Check headers
	assert.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "encrypted-key", resp.Header.Get("Key"))
	assert.Equal(t, "signature", resp.Header.Get("Signature"))

	tokenUsecase.AssertExpectations(t)
	logger.AssertExpectations(t)
	cryptoService.AssertExpectations(t)
	digitalSignatureService.AssertExpectations(t)
}

func TestEncryptAndSignBearerErrorResponse_Success(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}

	// Mock successful encryption and signing
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted-error"), "encrypted-key", nil)
	digitalSignatureService.On("SignPayload", mock.Anything, mock.Anything).Return("signature", nil)

	app.Get("/test", func(c *fiber.Ctx) error {
		return encryptAndSignBearerErrorResponse(c, cryptoService, digitalSignatureService, logger, "31", "Invalid credential")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Check headers
	assert.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "encrypted-key", resp.Header.Get("Key"))
	assert.Equal(t, "signature", resp.Header.Get("Signature"))

	cryptoService.AssertExpectations(t)
	digitalSignatureService.AssertExpectations(t)
}

func TestEncryptAndSignBearerErrorResponse_EncryptionFailure(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}

	// Mock encryption failure
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte{}, "", errors.New("encryption failed"))
	logger.On("Error", "failed to encrypt response", mock.Anything).Return()

	app.Get("/test", func(c *fiber.Ctx) error {
		return encryptAndSignBearerErrorResponse(c, cryptoService, digitalSignatureService, logger, "31", "Invalid credential")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)

	cryptoService.AssertExpectations(t)
	logger.AssertExpectations(t)
}

func TestEncryptAndSignBearerErrorResponse_SigningFailure(t *testing.T) {
	app := fiber.New()
	logger := &mockLogger{}
	cryptoService := &mockCryptoService{}
	digitalSignatureService := &mockDigitalSignatureService{}

	// Mock successful encryption but signing failure
	cryptoService.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted-error"), "encrypted-key", nil)
	digitalSignatureService.On("SignPayload", mock.Anything, mock.Anything).Return("", errors.New("signing failed"))
	logger.On("Error", "failed to sign response", mock.Anything).Return()

	app.Get("/test", func(c *fiber.Ctx) error {
		return encryptAndSignBearerErrorResponse(c, cryptoService, digitalSignatureService, logger, "31", "Invalid credential")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)

	cryptoService.AssertExpectations(t)
	digitalSignatureService.AssertExpectations(t)
	logger.AssertExpectations(t)
}
