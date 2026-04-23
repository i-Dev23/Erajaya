package service

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
)

// DigitalSignatureService defines the interface for signing and verifying payloads.
type DigitalSignatureService interface {
	SignPayload(ctx context.Context, payload string) (string, error)
	VerifyPayload(ctx context.Context, payload, signature string) error
}

// digitalSignatureServiceImpl implements DigitalSignatureService using PSS.
type digitalSignatureServiceImpl struct {
	signer  *SignatureTypePSS
	privKey *rsa.PrivateKey
	pubKey  *rsa.PublicKey
}

// NewDigitalSignatureService creates a new DigitalSignatureService.
// Load private & public key once during initialization.
func NewDigitalSignatureService(privKey *rsa.PrivateKey, pubKey *rsa.PublicKey) DigitalSignatureService {
	return &digitalSignatureServiceImpl{
		signer:  &SignatureTypePSS{},
		privKey: privKey,
		pubKey:  pubKey,
	}
}

// SignPayload signs the payload using the service's RSA private key.
func (s *digitalSignatureServiceImpl) SignPayload(ctx context.Context, payload string) (string, error) {
	if s.privKey == nil {
		return "", errors.New("private key not initialized")
	}
	// Context is not used here, but included for future extensibility.
	return s.signer.Sign(s.privKey, payload)
}

// VerifyPayload verifies the payload and signature using the service's RSA public key.
func (s *digitalSignatureServiceImpl) VerifyPayload(ctx context.Context, payload, signature string) error {
	if s.pubKey == nil {
		return errors.New("public key not initialized")
	}
	return s.signer.Verify(s.pubKey, payload, signature)
}

type SignatureTypePSS struct{}
type SignatureTypePKCS struct{}

func (s *SignatureTypePSS) Sign(privKey *rsa.PrivateKey, msg string) (string, error) {
	hashed := sha256.Sum256([]byte(msg))
	signature, err := rsa.SignPSS(rand.Reader, privKey, crypto.SHA256, hashed[:], nil)
	if err != nil {
		fmt.Println("error signing", err)
		return "", err
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

func (s *SignatureTypePSS) Verify(pubKey *rsa.PublicKey, msg, signature string) error {
	if signature == "" {
		return errors.New("signature is empty")
	}

	message := []byte(msg)
	bSignature, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		log.Println("error decoding", err)
		return err
	}

	hashed := sha256.Sum256(message)
	err = rsa.VerifyPSS(pubKey, crypto.SHA256, hashed[:], bSignature, nil)
	if err != nil {
		log.Println("error verifying", err)
		return err
	}

	return nil
}

func (s *SignatureTypePKCS) Sign(privKey *rsa.PrivateKey, msg string) (string, error) {
	rng := rand.Reader
	message := []byte(msg)
	hashed := sha256.Sum256(message)
	signature, err := rsa.SignPKCS1v15(rng, privKey, crypto.SHA256, hashed[:])
	if err != nil {
		fmt.Println("error signing", err)
		return "", err
	}

	sEnc := base64.StdEncoding.EncodeToString(signature)
	return sEnc, nil
}

func (s *SignatureTypePKCS) Verify(pubKey *rsa.PublicKey, msg string, base64Signature string) error {
	message := []byte(msg)
	bSignature, err := base64.StdEncoding.DecodeString(base64Signature)

	if err != nil {
		fmt.Println("Failed to decode signature")
		return err
	}
	hashed := sha256.Sum256(message)

	err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hashed[:], bSignature)
	if err != nil {
		fmt.Println("error verifying", err)
		return err
	}

	return nil
}
