package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
)

// GenerateSignature generates signature using formula:
// MD5(mdn + produk + notrx + MD5(password))
//
// Parameters:
//   - mdn: Mobile number (MSISDN)
//   - produk: Product code
//   - notrx: Transaction number
//   - password: Password/secret key
//
// Returns:
//   - Signature string in hexadecimal format
//
// Example:
//
//	signature := GenerateSignature("081234567890", "PLN_PREPAID", "TRX-001", "mypassword")
//	// Returns: "a1b2c3d4e5f6..."
func GenerateSignature(mdn, produk, notrx, password string) string {

	// Step 2: Concatenate: mdn + produk + notrx + MD5(password)
	combined := mdn + produk + notrx + password
	log.Println("Combined string:", combined)
	// Step 3: Hash the combined string with MD5
	finalHash := md5.Sum([]byte(combined))
	log.Println("Final hash:", finalHash)
	signature := hex.EncodeToString(finalHash[:])
	log.Println("Signature:", signature)
	return signature
}

// GenerateSignatureWithLogging generates signature with logging for debugging
// Same as GenerateSignature but logs intermediate steps
func GenerateSignatureWithLogging(mdn, produk, notrx, password string) (string, map[string]string) {
	// Step 1: Hash password with MD5
	passwordHash := md5.Sum([]byte(password))
	passwordHashStr := hex.EncodeToString(passwordHash[:])

	// Step 2: Concatenate: mdn + produk + notrx + MD5(password)
	combined := mdn + produk + notrx + passwordHashStr

	// Step 3: Hash the combined string with MD5
	finalHash := md5.Sum([]byte(combined))
	signature := hex.EncodeToString(finalHash[:])

	// Return signature and debug info
	debugInfo := map[string]string{
		"mdn":             mdn,
		"produk":          produk,
		"notrx":           notrx,
		"password_hash":   passwordHashStr,
		"combined_string": combined,
		"final_signature": signature,
	}

	return signature, debugInfo
}

// ValidateSignature validates if a signature matches the expected value
// using the same formula: MD5(mdn + produk + notrx + MD5(password))
func ValidateSignature(mdn, produk, notrx, password, expectedSignature string) bool {
	generatedSignature := GenerateSignature(mdn, produk, notrx, password)
	return generatedSignature == expectedSignature
}

// GenerateSignatureUpperCase generates signature in uppercase hexadecimal
// Some systems may require uppercase hex format
func GenerateSignatureUpperCase(mdn, produk, notrx, password string) string {
	// Step 1: Hash password with MD5
	passwordHash := md5.Sum([]byte(password))
	passwordHashStr := hex.EncodeToString(passwordHash[:])

	// Step 2: Concatenate: mdn + produk + notrx + MD5(password)
	combined := mdn + produk + notrx + passwordHashStr

	// Step 3: Hash the combined string with MD5
	finalHash := md5.Sum([]byte(combined))

	// Step 4: Convert to uppercase hexadecimal
	signature := fmt.Sprintf("%X", finalHash)

	return signature
}
