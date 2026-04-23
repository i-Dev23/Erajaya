package usecase

import (
	"context"
	"pps-services-tokopedia/internal/service"
)

type DecryptSignatureGenerateUsecase interface {
	Encrypt(ctx context.Context, plainPayload []byte) (string, string, error)
	GenerateSignature(ctx context.Context, plainPayload string) (string, error)
	Decrypt(ctx context.Context, encryptedPayload []byte, encryptedKey string) (string, error)
	ValidateDigitalSignature(ctx context.Context, payload, signature string) error
}

type decryptSignatureGenerateUsecaseImpl struct {
	cryptoService           service.CryptoService
	digitalSignatureService service.DigitalSignatureService
	logger                  service.Logger
}

func NewDecryptSignatureGenerateUsecase(cryptoService service.CryptoService, digitalSignatureService service.DigitalSignatureService, logger service.Logger) DecryptSignatureGenerateUsecase {
	return &decryptSignatureGenerateUsecaseImpl{
		cryptoService:           cryptoService,
		digitalSignatureService: digitalSignatureService,
		logger:                  logger,
	}
}

func (u *decryptSignatureGenerateUsecaseImpl) encrypt(ctx context.Context, encryptedPayload []byte) (string, string, error) {
	decryptedBytes, encryptedKey, err := u.cryptoService.Encrypt(ctx, encryptedPayload)
	if err != nil {
		u.logger.Error("Failed to encrypt payload", "error", err)
		return "", "", err
	}
	return encryptedKey, string(decryptedBytes), nil
}

func (u *decryptSignatureGenerateUsecaseImpl) generateSignature(ctx context.Context, plainText string) (string, error) {
	signature, err := u.digitalSignatureService.SignPayload(ctx, plainText)
	if err != nil {
		u.logger.Error("Failed to generate signature", "error", err)
		return "", err
	}
	return signature, nil
}

func (u *decryptSignatureGenerateUsecaseImpl) decrypt(ctx context.Context, encryptedPayload []byte, encryptedKey string) (string, error) {
	decryptedBytes, err := u.cryptoService.Decrypt(ctx, encryptedPayload, encryptedKey)
	if err != nil {
		u.logger.Error("Failed to decrypt payload", "error", err)
		return "", err
	}
	return string(decryptedBytes), nil
}

func (u *decryptSignatureGenerateUsecaseImpl) Encrypt(ctx context.Context, encryptedPayload []byte) (string, string, error) {
	return u.encrypt(ctx, encryptedPayload)
}

func (u *decryptSignatureGenerateUsecaseImpl) GenerateSignature(ctx context.Context, encryptedPayload string) (string, error) {
	return u.generateSignature(ctx, encryptedPayload)
}

func (u *decryptSignatureGenerateUsecaseImpl) Decrypt(ctx context.Context, encryptedPayload []byte, encryptedKey string) (string, error) {
	return u.decrypt(ctx, encryptedPayload, encryptedKey)
}

func (u *decryptSignatureGenerateUsecaseImpl) ValidateDigitalSignature(ctx context.Context, payload, signature string) error {
	return u.digitalSignatureService.VerifyPayload(ctx, payload, signature)
}
