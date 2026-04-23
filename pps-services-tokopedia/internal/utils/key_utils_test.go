package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func writeTempPEMFile(t *testing.T, blockType string, derBytes []byte) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "key.pem")
	f, err := os.Create(filePath)
	assert.NoError(t, err)
	defer f.Close()
	err = pem.Encode(f, &pem.Block{Type: blockType, Bytes: derBytes})
	assert.NoError(t, err)
	return filePath
}

func TestLoadPrivateKey(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	// PKCS#1
	pkcs1Bytes := x509.MarshalPKCS1PrivateKey(privKey)
	pkcs1Path := writeTempPEMFile(t, "RSA PRIVATE KEY", pkcs1Bytes)

	loadedKey, err := LoadPrivateKey(pkcs1Path)
	assert.NoError(t, err)
	assert.Equal(t, privKey.N.Cmp(loadedKey.N), 0)

	// PKCS#8
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	assert.NoError(t, err)
	pkcs8Path := writeTempPEMFile(t, "PRIVATE KEY", pkcs8Bytes)

	loadedKey2, err := LoadPrivateKey(pkcs8Path)
	assert.NoError(t, err)
	assert.Equal(t, privKey.N.Cmp(loadedKey2.N), 0)

	// Unsupported type
	unsupportedPath := writeTempPEMFile(t, "EC PRIVATE KEY", pkcs1Bytes)
	_, err = LoadPrivateKey(unsupportedPath)
	assert.Error(t, err)

	// Invalid file
	invalidPath := filepath.Join(t.TempDir(), "nonexistent.pem")
	_, err = LoadPrivateKey(invalidPath)
	assert.Error(t, err)
}

func TestLoadPublicKey(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	pubKey := &privKey.PublicKey

	// PKCS#1
	pkcs1Bytes := x509.MarshalPKCS1PublicKey(pubKey)
	pkcs1Path := writeTempPEMFile(t, "RSA PUBLIC KEY", pkcs1Bytes)

	loadedPub, err := LoadPublicKey(pkcs1Path)
	assert.NoError(t, err)
	assert.Equal(t, pubKey.N.Cmp(loadedPub.N), 0)

	// PKIX
	pkixBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	assert.NoError(t, err)
	pkixPath := writeTempPEMFile(t, "PUBLIC KEY", pkixBytes)

	loadedPub2, err := LoadPublicKey(pkixPath)
	assert.NoError(t, err)
	assert.Equal(t, pubKey.N.Cmp(loadedPub2.N), 0)

	// Unsupported type
	unsupportedPath := writeTempPEMFile(t, "EC PUBLIC KEY", pkcs1Bytes)
	_, err = LoadPublicKey(unsupportedPath)
	assert.Error(t, err)

	// Invalid file
	invalidPath := filepath.Join(t.TempDir(), "nonexistent_pub.pem")
	_, err = LoadPublicKey(invalidPath)
	assert.Error(t, err)
}

func TestLoadPrivateKeyFromString(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	// PKCS#1
	pkcs1Bytes := x509.MarshalPKCS1PrivateKey(privKey)
	pkcs1PEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pkcs1Bytes})
	pkcs1Base64 := base64.StdEncoding.EncodeToString(pkcs1PEM)

	loadedKey, err := LoadPrivateKeyFromString(pkcs1Base64)
	assert.NoError(t, err)
	assert.Equal(t, privKey.N.Cmp(loadedKey.N), 0)

	// PKCS#8
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	assert.NoError(t, err)
	pkcs8PEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes})
	pkcs8Base64 := base64.StdEncoding.EncodeToString(pkcs8PEM)

	loadedKey2, err := LoadPrivateKeyFromString(pkcs8Base64)
	assert.NoError(t, err)
	assert.Equal(t, privKey.N.Cmp(loadedKey2.N), 0)

	// Empty string
	_, err = LoadPrivateKeyFromString("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "private key string is empty")

	// Invalid base64
	_, err = LoadPrivateKeyFromString("invalid base64 content")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "illegal base64 data")

	// Unsupported type
	unsupportedPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: pkcs1Bytes})
	unsupportedBase64 := base64.StdEncoding.EncodeToString(unsupportedPEM)
	_, err = LoadPrivateKeyFromString(unsupportedBase64)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported private key type")
}

func TestLoadPublicKeyFromString(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	pubKey := &privKey.PublicKey

	// PKCS#1
	pkcs1Bytes := x509.MarshalPKCS1PublicKey(pubKey)
	pkcs1PEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: pkcs1Bytes})
	pkcs1Base64 := base64.StdEncoding.EncodeToString(pkcs1PEM)

	loadedPub, err := LoadPublicKeyFromString(pkcs1Base64)
	assert.NoError(t, err)
	assert.Equal(t, pubKey.N.Cmp(loadedPub.N), 0)

	// PKIX
	pkixBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	assert.NoError(t, err)
	pkixPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkixBytes})
	pkixBase64 := base64.StdEncoding.EncodeToString(pkixPEM)

	loadedPub2, err := LoadPublicKeyFromString(pkixBase64)
	assert.NoError(t, err)
	assert.Equal(t, pubKey.N.Cmp(loadedPub2.N), 0)

	// Empty string
	_, err = LoadPublicKeyFromString("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "public key string is empty")

	// Invalid base64
	_, err = LoadPublicKeyFromString("invalid base64 content")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "illegal base64 data")

	// Unsupported type
	unsupportedPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PUBLIC KEY", Bytes: pkcs1Bytes})
	unsupportedBase64 := base64.StdEncoding.EncodeToString(unsupportedPEM)
	_, err = LoadPublicKeyFromString(unsupportedBase64)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported public key type")
}

func TestLoadKeysFromEnv(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	pubKey := &privKey.PublicKey

	// Prepare base64 encoded PEM strings
	pkcs1PrivBytes := x509.MarshalPKCS1PrivateKey(privKey)
	pkcs1PrivPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pkcs1PrivBytes})
	pkcs1PrivBase64 := base64.StdEncoding.EncodeToString(pkcs1PrivPEM)

	pkixPubBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	assert.NoError(t, err)
	pkixPubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkixPubBytes})
	pkixPubBase64 := base64.StdEncoding.EncodeToString(pkixPubPEM)

	// Test loading from string env vars
	t.Setenv("TEST_PRIVATE_KEY", pkcs1PrivBase64)
	t.Setenv("TEST_PUBLIC_KEY", pkixPubBase64)

	loadedPriv, loadedPub, err := LoadKeysFromEnv("TEST_PRIVATE_KEY", "TEST_PUBLIC_KEY", "", "")
	assert.NoError(t, err)
	assert.Equal(t, privKey.N.Cmp(loadedPriv.N), 0)
	assert.Equal(t, pubKey.N.Cmp(loadedPub.N), 0)

	// Test fallback to file paths
	// Create temporary files
	pkcs8PrivBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	assert.NoError(t, err)
	privFilePath := writeTempPEMFile(t, "PRIVATE KEY", pkcs8PrivBytes)

	pkcs1PubBytes := x509.MarshalPKCS1PublicKey(pubKey)
	pubFilePath := writeTempPEMFile(t, "RSA PUBLIC KEY", pkcs1PubBytes)

	// Clear string env vars
	t.Setenv("TEST_PRIVATE_KEY", "")
	t.Setenv("TEST_PUBLIC_KEY", "")
	t.Setenv("TEST_PRIVATE_KEY_PATH", privFilePath)
	t.Setenv("TEST_PUBLIC_KEY_PATH", pubFilePath)

	loadedPriv2, loadedPub2, err := LoadKeysFromEnv("", "", "TEST_PRIVATE_KEY_PATH", "TEST_PUBLIC_KEY_PATH")
	assert.NoError(t, err)
	assert.Equal(t, privKey.N.Cmp(loadedPriv2.N), 0)
	assert.Equal(t, pubKey.N.Cmp(loadedPub2.N), 0)

	// Test error when both string and file fail
	t.Setenv("TEST_PRIVATE_KEY", "")
	t.Setenv("TEST_PUBLIC_KEY", "")
	t.Setenv("TEST_PRIVATE_KEY_PATH", "")
	t.Setenv("TEST_PUBLIC_KEY_PATH", "")

	_, _, err = LoadKeysFromEnv("", "", "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load private key")
}
