package http

import (
	"pps-services-tokopedia/internal/usecase"

	"github.com/gofiber/fiber/v2"
)

// DecryptSignatureGenerateHandler handles the decrypt, signature, and generate routes.
type DecryptSignatureGenerateHandler struct {
	usecase usecase.DecryptSignatureGenerateUsecase
}

// NewDecryptSignatureGenerateHandler creates a new DecryptSignatureGenerateHandler with the given usecase.
func NewDecryptSignatureGenerateHandler(usecase usecase.DecryptSignatureGenerateUsecase) *DecryptSignatureGenerateHandler {
	return &DecryptSignatureGenerateHandler{
		usecase: usecase,
	}
}

// RegisterRoutes registers the decrypt, signature, and generate routes to the given Fiber app/group.
func (h *DecryptSignatureGenerateHandler) RegisterRoutes(router fiber.Router) {
	router.Post("/encrypt", h.Encrypt)
	router.Post("/generate-signature", h.GenerateSignature)
	router.Post("/decrypt", h.Decrypt)
	router.Post("/validate-digital-signature", h.ValidateDigitalSignature)
}

// EncryptRequest is the request body for the encrypt route.
type EncryptRequest struct {
	Payload []byte `json:"payload"`
}

// EncryptResponse is the response body for the encrypt route.
type EncryptResponse struct {
	EncryptedKey string `json:"encrypted_key"`
	CipherText   string `json:"cipher_text"`
}

// GenerateSignatureRequest is the request body for the generate signature route.
type GenerateSignatureRequest struct {
	Payload string `json:"payload"`
}

// GenerateSignatureResponse is the response body for the generate signature route.
type GenerateSignatureResponse struct {
	Signature string `json:"signature"`
}

// Encrypt handles the POST /encrypt route.
func (h *DecryptSignatureGenerateHandler) Encrypt(c *fiber.Ctx) error {
	encryptedKey, cipherText, err := h.usecase.Encrypt(c.Context(), c.Body())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to encrypt payload",
		})
	}

	resp := EncryptResponse{
		EncryptedKey: encryptedKey,
		CipherText:   cipherText,
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// GenerateSignature handles the POST /generate-signature route.
func (h *DecryptSignatureGenerateHandler) GenerateSignature(c *fiber.Ctx) error {
	signature, err := h.usecase.GenerateSignature(c.Context(), string(c.Body()))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate signature",
		})
	}

	resp := GenerateSignatureResponse{
		Signature: signature,
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// Decrypt handles the POST /decrypt route.
func (h *DecryptSignatureGenerateHandler) Decrypt(c *fiber.Ctx) error {
	decryptedPayload, err := h.usecase.Decrypt(c.Context(), c.Body(), c.Get("Api-Key"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to decrypt payload",
		})
	}
	return c.Status(fiber.StatusOK).JSON(decryptedPayload)
}

// ValidateDigitalSignature handles the POST /validate-digital-signature route.
func (h *DecryptSignatureGenerateHandler) ValidateDigitalSignature(c *fiber.Ctx) error {
	err := h.usecase.ValidateDigitalSignature(c.Context(), string(c.Body()), c.Get("Signature"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to validate digital signature",
		})
	}
	return c.Status(fiber.StatusOK).JSON("Success Validate Digital Signature")
}
