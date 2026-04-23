package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateSignature(t *testing.T) {
	tests := []struct {
		name     string
		mdn      string
		produk   string
		notrx    string
		password string
		expected string // Will be calculated
	}{
		{
			name:     "Valid signature generation",
			mdn:      "081234567890",
			produk:   "PLN_PREPAID",
			notrx:    "TRX-001",
			password: "mypassword",
		},
		{
			name:     "Empty values",
			mdn:      "",
			produk:   "",
			notrx:    "",
			password: "",
		},
		{
			name:     "Special characters",
			mdn:      "08123-456-7890",
			produk:   "PLN@PREPAID",
			notrx:    "TRX#001",
			password: "p@ssw0rd!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature := GenerateSignature(tt.mdn, tt.produk, tt.notrx, tt.password)

			// Signature should be non-empty
			assert.NotEmpty(t, signature)

			// Signature should be 32 characters (MD5 hex = 32 chars)
			assert.Equal(t, 32, len(signature))

			// Signature should be hexadecimal (only contains 0-9 and a-f)
			assert.Regexp(t, "^[0-9a-f]{32}$", signature)

			// Same inputs should produce same signature (deterministic)
			signature2 := GenerateSignature(tt.mdn, tt.produk, tt.notrx, tt.password)
			assert.Equal(t, signature, signature2)
		})
	}
}

func TestGenerateSignature_Deterministic(t *testing.T) {
	mdn := "081234567890"
	produk := "PLN_PREPAID"
	notrx := "TRX-001"
	password := "secret"

	// Generate signature multiple times
	sig1 := GenerateSignature(mdn, produk, notrx, password)
	sig2 := GenerateSignature(mdn, produk, notrx, password)
	sig3 := GenerateSignature(mdn, produk, notrx, password)

	// All should be identical
	assert.Equal(t, sig1, sig2)
	assert.Equal(t, sig2, sig3)
	assert.Equal(t, 32, len(sig1))
}

func TestGenerateSignature_DifferentInputs(t *testing.T) {
	password := "secret"

	sig1 := GenerateSignature("081234567890", "PROD1", "TRX-001", password)
	sig2 := GenerateSignature("081234567890", "PROD2", "TRX-001", password) // Different produk
	sig3 := GenerateSignature("081234567890", "PROD1", "TRX-002", password) // Different notrx
	sig4 := GenerateSignature("089876543210", "PROD1", "TRX-001", password) // Different mdn

	// All signatures should be different
	assert.NotEqual(t, sig1, sig2)
	assert.NotEqual(t, sig1, sig3)
	assert.NotEqual(t, sig1, sig4)
	assert.NotEqual(t, sig2, sig3)
}

func TestGenerateSignatureWithLogging(t *testing.T) {
	mdn := "081234567890"
	produk := "PLN_PREPAID"
	notrx := "TRX-001"
	password := "mypassword"

	signature, debugInfo := GenerateSignatureWithLogging(mdn, produk, notrx, password)

	// Check signature
	assert.NotEmpty(t, signature)
	assert.Equal(t, 32, len(signature))

	// Check debug info
	assert.Equal(t, mdn, debugInfo["mdn"])
	assert.Equal(t, produk, debugInfo["produk"])
	assert.Equal(t, notrx, debugInfo["notrx"])
	assert.NotEmpty(t, debugInfo["password_hash"])
	assert.Equal(t, 32, len(debugInfo["password_hash"])) // MD5 hash length
	assert.NotEmpty(t, debugInfo["combined_string"])
	assert.Equal(t, signature, debugInfo["final_signature"])

	// Combined string should contain all parts
	combined := debugInfo["combined_string"]
	assert.Contains(t, combined, mdn)
	assert.Contains(t, combined, produk)
	assert.Contains(t, combined, notrx)
	assert.Contains(t, combined, debugInfo["password_hash"])
}

func TestValidateSignature(t *testing.T) {
	mdn := "081234567890"
	produk := "PLN_PREPAID"
	notrx := "TRX-001"
	password := "secret"

	// Generate signature
	correctSignature := GenerateSignature(mdn, produk, notrx, password)

	tests := []struct {
		name              string
		mdn               string
		produk            string
		notrx             string
		password          string
		expectedSignature string
		shouldBeValid     bool
	}{
		{
			name:              "Valid signature",
			mdn:               mdn,
			produk:            produk,
			notrx:             notrx,
			password:          password,
			expectedSignature: correctSignature,
			shouldBeValid:     true,
		},
		{
			name:              "Invalid signature - wrong password",
			mdn:               mdn,
			produk:            produk,
			notrx:             notrx,
			password:          "wrongpassword",
			expectedSignature: correctSignature,
			shouldBeValid:     false,
		},
		{
			name:              "Invalid signature - wrong mdn",
			mdn:               "089999999999",
			produk:            produk,
			notrx:             notrx,
			password:          password,
			expectedSignature: correctSignature,
			shouldBeValid:     false,
		},
		{
			name:              "Invalid signature - tampered",
			mdn:               mdn,
			produk:            produk,
			notrx:             notrx,
			password:          password,
			expectedSignature: "invalidhash12345678901234567890",
			shouldBeValid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := ValidateSignature(tt.mdn, tt.produk, tt.notrx, tt.password, tt.expectedSignature)
			assert.Equal(t, tt.shouldBeValid, isValid)
		})
	}
}

func TestGenerateSignatureUpperCase(t *testing.T) {
	mdn := "081234567890"
	produk := "PLN_PREPAID"
	notrx := "TRX-001"
	password := "secret"

	signatureLower := GenerateSignature(mdn, produk, notrx, password)
	signatureUpper := GenerateSignatureUpperCase(mdn, produk, notrx, password)

	// Should be uppercase
	assert.NotEmpty(t, signatureUpper)
	assert.Equal(t, 32, len(signatureUpper))

	// Should be all uppercase
	assert.Regexp(t, "^[0-9A-F]{32}$", signatureUpper)

	// Lowercase and uppercase signature should not be equal
	assert.NotEqual(t, signatureLower, signatureUpper)

	// Lowercase signature should be all lowercase hex
	assert.Regexp(t, "^[0-9a-f]{32}$", signatureLower)

	// Uppercase signature should be all uppercase hex
	assert.Regexp(t, "^[0-9A-F]{32}$", signatureUpper)
}

func TestGenerateSignature_RealExample(t *testing.T) {
	// Real example for documentation
	mdn := "1234567890"
	produk := "product5627"
	notrx := "TRX-20251008-152522-ABC123"
	password := "ASIKBOJONG"

	signature := GenerateSignature(mdn, produk, notrx, password)

	t.Logf("MDN: %s", mdn)
	t.Logf("Produk: %s", produk)
	t.Logf("NoTrx: %s", notrx)
	t.Logf("Password: %s", password)
	t.Logf("Generated Signature: %s", signature)

	// Verify signature properties
	assert.NotEmpty(t, signature)
	assert.Equal(t, 32, len(signature))
	assert.Regexp(t, "^[0-9a-f]{32}$", signature)
}
