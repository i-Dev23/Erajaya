package utils

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
)

// LoadPrivateKey loads an RSA private key from a PEM file (PKCS#1 or PKCS#8).
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing private key")
	}

	// Handle RSA PRIVATE KEY (PKCS#1)
	if block.Type == "RSA PRIVATE KEY" {
		priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return priv, nil
	}

	// Handle PRIVATE KEY (PKCS#8)
	if block.Type == "PRIVATE KEY" {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		priv, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("not an RSA private key")
		}
		return priv, nil
	}

	return nil, errors.New("unsupported private key type: " + block.Type)
}

// LoadPublicKey loads an RSA public key from a PEM file (PKCS#1 or PKIX).
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing public key")
	}

	// Handle PKIX (-----BEGIN PUBLIC KEY-----)
	if block.Type == "PUBLIC KEY" {
		pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		pub, ok := pubInterface.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("not an RSA public key")
		}
		return pub, nil
	}

	// Handle PKCS#1 (-----BEGIN RSA PUBLIC KEY-----)
	if block.Type == "RSA PUBLIC KEY" {
		pub, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return pub, nil
	}

	return nil, errors.New("unsupported public key type: " + block.Type)
}

// LoadPrivateKeyFromString loads an RSA private key from a PEM string (PKCS#1 or PKCS#8).
func LoadPrivateKeyFromString(keyString string) (*rsa.PrivateKey, error) {
	if keyString == "" {
		return nil, errors.New("private key string is empty")
	}

	pemBytes, err := base64.StdEncoding.DecodeString(keyString)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode([]byte(pemBytes))
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing private key")
	}

	// Handle RSA PRIVATE KEY (PKCS#1)
	if block.Type == "RSA PRIVATE KEY" {
		priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return priv, nil
	}

	// Handle PRIVATE KEY (PKCS#8)
	if block.Type == "PRIVATE KEY" {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		priv, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("not an RSA private key")
		}
		return priv, nil
	}

	return nil, errors.New("unsupported private key type: " + block.Type)
}

// LoadPublicKeyFromString loads an RSA public key from a PEM string (PKCS#1 or PKIX).
func LoadPublicKeyFromString(keyString string) (*rsa.PublicKey, error) {
	if keyString == "" {
		return nil, errors.New("public key string is empty")
	}

	pemBytes, err := base64.StdEncoding.DecodeString(keyString)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode([]byte(pemBytes))
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing public key")
	}

	// Handle PKIX (-----BEGIN PUBLIC KEY-----)
	if block.Type == "PUBLIC KEY" {
		pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		pub, ok := pubInterface.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("not an RSA public key")
		}
		return pub, nil
	}

	// Handle PKCS#1 (-----BEGIN RSA PUBLIC KEY-----)
	if block.Type == "RSA PUBLIC KEY" {
		pub, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return pub, nil
	}

	return nil, errors.New("unsupported public key type: " + block.Type)
}

// LoadKeysFromEnv loads private and public keys from environment variables.
// It tries to load from string env vars first, then falls back to file paths.
func LoadKeysFromEnv(privateKeyEnv, publicKeyEnv, privateKeyPathEnv, publicKeyPathEnv string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	var privKey *rsa.PrivateKey
	var pubKey *rsa.PublicKey
	var err error

	// Try to load private key from string first
	if privateKeyEnv != "" {
		privKeyString := GetEnv(privateKeyEnv, "")
		if privKeyString != "" {
			privKey, err = LoadPrivateKeyFromString(privKeyString)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	// Try to load public key from string first
	if publicKeyEnv != "" {
		pubKeyString := GetEnv(publicKeyEnv, "")
		if pubKeyString != "" {
			pubKey, err = LoadPublicKeyFromString(pubKeyString)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	// Fallback to file paths if string loading failed
	if privKey == nil && privateKeyPathEnv != "" {
		privKeyPath := GetEnv(privateKeyPathEnv, "")
		if privKeyPath != "" {
			privKey, err = LoadPrivateKey(privKeyPath)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	if pubKey == nil && publicKeyPathEnv != "" {
		pubKeyPath := GetEnv(publicKeyPathEnv, "")
		if pubKeyPath != "" {
			pubKey, err = LoadPublicKey(pubKeyPath)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	if privKey == nil {
		return nil, nil, errors.New("failed to load private key from both string and file")
	}

	if pubKey == nil {
		return nil, nil, errors.New("failed to load public key from both string and file")
	}

	return privKey, pubKey, nil
}
