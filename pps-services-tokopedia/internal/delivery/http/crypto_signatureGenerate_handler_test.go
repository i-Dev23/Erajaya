package http

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

// mockDecryptSignatureGenerateUsecase is a mock implementation for testing
type mockDecryptSignatureGenerateUsecase struct {
	EncryptFunc                  func(ctx context.Context, payload []byte) (string, string, error)
	GenerateSignatureFunc        func(ctx context.Context, payload string) (string, error)
	DecryptFunc                  func(ctx context.Context, payload []byte, apiKey string) (string, error)
	ValidateDigitalSignatureFunc func(ctx context.Context, payload string, signature string) error
}

func (m *mockDecryptSignatureGenerateUsecase) Encrypt(ctx context.Context, payload []byte) (string, string, error) {
	if m.EncryptFunc != nil {
		return m.EncryptFunc(ctx, payload)
	}
	return "encryptedKey123", "cipherText456", nil
}

func (m *mockDecryptSignatureGenerateUsecase) GenerateSignature(ctx context.Context, payload string) (string, error) {
	if m.GenerateSignatureFunc != nil {
		return m.GenerateSignatureFunc(ctx, payload)
	}
	return "signature789", nil
}

func (m *mockDecryptSignatureGenerateUsecase) Decrypt(ctx context.Context, payload []byte, apiKey string) (string, error) {
	if m.DecryptFunc != nil {
		return m.DecryptFunc(ctx, payload, apiKey)
	}
	return `{"data":"decrypted"}`, nil
}

func (m *mockDecryptSignatureGenerateUsecase) ValidateDigitalSignature(ctx context.Context, payload string, signature string) error {
	if m.ValidateDigitalSignatureFunc != nil {
		return m.ValidateDigitalSignatureFunc(ctx, payload, signature)
	}
	return nil
}

func TestDecryptSignatureGenerateHandler_Encrypt(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*mockDecryptSignatureGenerateUsecase)
		expectedStatus int
		validateResp   func(*testing.T, string)
	}{
		{
			name:        "success encrypt",
			requestBody: `{"payload":"test data"}`,
			mockSetup: func(m *mockDecryptSignatureGenerateUsecase) {
				m.EncryptFunc = func(ctx context.Context, payload []byte) (string, string, error) {
					return "key123", "cipher456", nil
				}
			},
			expectedStatus: 200,
			validateResp: func(t *testing.T, body string) {
				var resp EncryptResponse
				err := json.Unmarshal([]byte(body), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "key123", resp.EncryptedKey)
				assert.Equal(t, "cipher456", resp.CipherText)
			},
		},
		{
			name:        "encrypt fails",
			requestBody: `{"payload":"test"}`,
			mockSetup: func(m *mockDecryptSignatureGenerateUsecase) {
				m.EncryptFunc = func(ctx context.Context, payload []byte) (string, string, error) {
					return "", "", assert.AnError
				}
			},
			expectedStatus: 500,
			validateResp: func(t *testing.T, body string) {
				assert.Contains(t, body, "failed to encrypt payload")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockUsecase := &mockDecryptSignatureGenerateUsecase{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockUsecase)
			}

			handler := NewDecryptSignatureGenerateHandler(mockUsecase)
			handler.RegisterRoutes(app)

			req := httptest.NewRequest("POST", "/encrypt", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			body := make([]byte, resp.ContentLength)
			resp.Body.Read(body)
			tt.validateResp(t, string(body))
		})
	}
}

func TestDecryptSignatureGenerateHandler_GenerateSignature(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*mockDecryptSignatureGenerateUsecase)
		expectedStatus int
		validateResp   func(*testing.T, string)
	}{
		{
			name:        "success generate signature",
			requestBody: `{"payload":"test data"}`,
			mockSetup: func(m *mockDecryptSignatureGenerateUsecase) {
				m.GenerateSignatureFunc = func(ctx context.Context, payload string) (string, error) {
					return "sig123abc", nil
				}
			},
			expectedStatus: 200,
			validateResp: func(t *testing.T, body string) {
				var resp GenerateSignatureResponse
				err := json.Unmarshal([]byte(body), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "sig123abc", resp.Signature)
			},
		},
		{
			name:        "generate signature fails",
			requestBody: `{"payload":"data"}`,
			mockSetup: func(m *mockDecryptSignatureGenerateUsecase) {
				m.GenerateSignatureFunc = func(ctx context.Context, payload string) (string, error) {
					return "", assert.AnError
				}
			},
			expectedStatus: 500,
			validateResp: func(t *testing.T, body string) {
				assert.Contains(t, body, "failed to generate signature")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockUsecase := &mockDecryptSignatureGenerateUsecase{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockUsecase)
			}

			handler := NewDecryptSignatureGenerateHandler(mockUsecase)
			handler.RegisterRoutes(app)

			req := httptest.NewRequest("POST", "/generate-signature", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			body := make([]byte, resp.ContentLength)
			resp.Body.Read(body)
			tt.validateResp(t, string(body))
		})
	}
}

func TestDecryptSignatureGenerateHandler_Decrypt(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		apiKey         string
		mockSetup      func(*mockDecryptSignatureGenerateUsecase)
		expectedStatus int
		validateResp   func(*testing.T, string)
	}{
		{
			name:        "success decrypt",
			requestBody: `{"encrypted":"data"}`,
			apiKey:      "test-api-key",
			mockSetup: func(m *mockDecryptSignatureGenerateUsecase) {
				m.DecryptFunc = func(ctx context.Context, payload []byte, apiKey string) (string, error) {
					return `{"result":"decrypted data"}`, nil
				}
			},
			expectedStatus: 200,
			validateResp: func(t *testing.T, body string) {
				assert.Contains(t, body, "decrypted data")
			},
		},
		{
			name:        "decrypt fails",
			requestBody: `{"encrypted":"bad"}`,
			apiKey:      "test-key",
			mockSetup: func(m *mockDecryptSignatureGenerateUsecase) {
				m.DecryptFunc = func(ctx context.Context, payload []byte, apiKey string) (string, error) {
					return "", assert.AnError
				}
			},
			expectedStatus: 500,
			validateResp: func(t *testing.T, body string) {
				assert.Contains(t, body, "failed to decrypt payload")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockUsecase := &mockDecryptSignatureGenerateUsecase{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockUsecase)
			}

			handler := NewDecryptSignatureGenerateHandler(mockUsecase)
			handler.RegisterRoutes(app)

			req := httptest.NewRequest("POST", "/decrypt", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Api-Key", tt.apiKey)

			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			body := make([]byte, resp.ContentLength)
			resp.Body.Read(body)
			tt.validateResp(t, string(body))
		})
	}
}

func TestDecryptSignatureGenerateHandler_ValidateDigitalSignature(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		signature      string
		mockSetup      func(*mockDecryptSignatureGenerateUsecase)
		expectedStatus int
		validateResp   func(*testing.T, string)
	}{
		{
			name:        "success validate signature",
			requestBody: `{"data":"test"}`,
			signature:   "valid-signature",
			mockSetup: func(m *mockDecryptSignatureGenerateUsecase) {
				m.ValidateDigitalSignatureFunc = func(ctx context.Context, payload string, signature string) error {
					return nil
				}
			},
			expectedStatus: 200,
			validateResp: func(t *testing.T, body string) {
				assert.Contains(t, body, "Success Validate Digital Signature")
			},
		},
		{
			name:        "validate signature fails",
			requestBody: `{"data":"test"}`,
			signature:   "invalid-signature",
			mockSetup: func(m *mockDecryptSignatureGenerateUsecase) {
				m.ValidateDigitalSignatureFunc = func(ctx context.Context, payload string, signature string) error {
					return assert.AnError
				}
			},
			expectedStatus: 500,
			validateResp: func(t *testing.T, body string) {
				assert.Contains(t, body, "failed to validate digital signature")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockUsecase := &mockDecryptSignatureGenerateUsecase{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockUsecase)
			}

			handler := NewDecryptSignatureGenerateHandler(mockUsecase)
			handler.RegisterRoutes(app)

			req := httptest.NewRequest("POST", "/validate-digital-signature", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Signature", tt.signature)

			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			body := make([]byte, resp.ContentLength)
			resp.Body.Read(body)
			tt.validateResp(t, string(body))
		})
	}
}
