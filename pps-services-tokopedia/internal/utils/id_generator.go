package utils

import (
	"crypto/rand"
	"fmt"
	"time"
)

// GeneratePPSRequestID generates a unique PPS request ID
// Format: PPS-YYYYMMDD-HHMMSS-XXXXXX
// Where XXXXXX is a random 6-character string
func GeneratePPSRequestID() string {
	now := time.Now()

	// Format: PPS-YYYYMMDD-HHMMSS
	timestamp := now.Format("PPS-20060102-150405")

	// Generate random 6-character string
	randomStr := generateRandomString(6)

	return fmt.Sprintf("%s-%s", timestamp, randomStr)
}

// GeneratePPSRequestIDWithPrefix generates a unique PPS request ID with custom prefix
// Format: {prefix}-YYYYMMDD-HHMMSS-XXXXXX
func GeneratePPSRequestIDWithPrefix(prefix string) string {
	now := time.Now()

	// Format: {prefix}-YYYYMMDD-HHMMSS
	timestamp := now.Format(fmt.Sprintf("%s-20060102-150405", prefix))

	// Generate random 6-character string
	randomStr := generateRandomString(6)

	return fmt.Sprintf("%s-%s", timestamp, randomStr)
}

// generateRandomString generates a random string of specified length
func generateRandomString(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)

	for i := range b {
		randomBytes := make([]byte, 1)
		rand.Read(randomBytes)
		b[i] = charset[randomBytes[0]%byte(len(charset))]
	}

	return string(b)
}

// GenerateInquiryID generates a unique inquiry ID for Tokopedia
// Format: INQ-YYYYMMDD-HHMMSS-XXXXXX
func GenerateInquiryID() string {
	return GeneratePPSRequestIDWithPrefix("INQ")
}

// GenerateTransactionID generates a unique transaction ID
// Format: TXN-YYYYMMDD-HHMMSS-XXXXXX
func GenerateTransactionID() string {
	return GeneratePPSRequestIDWithPrefix("TXN")
}

// GenerateCallbackID generates a unique callback ID
// Format: CBK-YYYYMMDD-HHMMSS-XXXXXX
func GenerateCallbackID() string {
	return GeneratePPSRequestIDWithPrefix("CBK")
}
