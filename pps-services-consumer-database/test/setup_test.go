// Package test berisi semua test untuk aplikasi pps-services-database.
// Test menggunakan mock untuk semua external dependency (database, RabbitMQ)
// sehingga bisa dijalankan tanpa infrastruktur nyata.
package test

import (
	"os"

	"github.com/go-playground/validator/v10" // Validator
	"github.com/rs/zerolog"                  // Logger
)

// newTestLogger membuat logger untuk testing dengan level Debug.
func newTestLogger() zerolog.Logger {
	return zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.DebugLevel)
}

// newTestValidator membuat validator untuk testing.
func newTestValidator() *validator.Validate {
	return validator.New()
}
