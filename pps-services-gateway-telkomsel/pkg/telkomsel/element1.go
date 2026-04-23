package telkomsel

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

const (
	element1Cipher = "AES-128-CBC"
	// Same as PHP example: hex("31323334353630303030303030303030") => bytes("1234560000000000")
	element1IVHex = "31323334353630303030303030303030"
)

// EncryptElement1FromEnv encrypts env PIN using env ENCRYPTION_KEY.
//
// Formula (as per provided PHP snippet):
// element1 = base64( AES-128-CBC( PKCS5Pad(PIN,16), key=base64Decode(ENCRYPTION_KEY), iv=hex2bin(element1IVHex) ) )
func EncryptElement1FromEnv() (string, error) {
	pin := strings.TrimSpace(os.Getenv("PIN"))
	encryptionKeyB64 := strings.TrimSpace(os.Getenv("ENCRYPTION_KEY"))

	if pin == "" {
		return "", fmt.Errorf("PIN env is required")
	}
	if encryptionKeyB64 == "" {
		return "", fmt.Errorf("ENCRYPTION_KEY env is required")
	}

	return EncryptElement1(pin, encryptionKeyB64)
}

// EncryptElement1 encrypts PIN (plaintext) using ENCRYPTION_KEY (base64) per vendor formula.
func EncryptElement1(pin string, encryptionKeyB64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(encryptionKeyB64)
	if err != nil {
		return "", fmt.Errorf("base64 decode ENCRYPTION_KEY: %w", err)
	}
	if len(key) != 16 {
		return "", fmt.Errorf("ENCRYPTION_KEY must decode to 16 bytes for %s (got %d)", element1Cipher, len(key))
	}

	iv, err := hex.DecodeString(element1IVHex)
	if err != nil {
		return "", fmt.Errorf("decode IV hex: %w", err)
	}
	if len(iv) != aes.BlockSize {
		return "", fmt.Errorf("IV must be %d bytes (got %d)", aes.BlockSize, len(iv))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("init cipher: %w", err)
	}

	plaintext := pkcs5Pad([]byte(pin), aes.BlockSize)
	ciphertext := make([]byte, len(plaintext))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func pkcs5Pad(in []byte, blockSize int) []byte {
	pad := blockSize - (len(in) % blockSize)
	if pad == 0 {
		pad = blockSize
	}

	out := make([]byte, 0, len(in)+pad)
	out = append(out, in...)
	for i := 0; i < pad; i++ {
		out = append(out, byte(pad))
	}
	return out
}
