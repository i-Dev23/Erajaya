// Package test - File ini berisi test untuk HTTP middleware.
// Menguji API Key authentication dan request logging middleware.
package test

import (
	"io"                // Read response body
	"net/http"          // HTTP constants
	"net/http/httptest" // HTTP test utilities
	"testing"           // Standard testing

	"github.com/gofiber/fiber/v3"        // Fiber v3
	"github.com/spf13/viper"             // Konfigurasi
	"github.com/stretchr/testify/assert" // Assertion

	"pps-services-publisher-database/internal/delivery/http/middleware" // Middleware yang ditest
)

// setupTestApp membuat Fiber app untuk testing dengan API key middleware.
func setupTestApp() (*fiber.App, *viper.Viper) {
	app := fiber.New()
	config := viper.New()
	config.Set("security.api_key", "test-api-key-123")

	return app, config
}

// TestAPIKeyAuth_Positive menguji autentikasi berhasil dengan API key yang valid.
func TestAPIKeyAuth_Positive(t *testing.T) {
	app, config := setupTestApp()
	log := newTestLogger()

	// Setup route dengan middleware
	app.Use(middleware.NewAPIKeyAuth(config, log))
	app.Get("/test", func(ctx fiber.Ctx) error {
		return ctx.SendString("OK")
	})

	// Buat request dengan API key yang valid
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "test-api-key-123")

	resp, err := app.Test(req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "OK", string(body))
}

// TestAPIKeyAuth_Negative_MissingKey menguji autentikasi gagal tanpa API key.
func TestAPIKeyAuth_Negative_MissingKey(t *testing.T) {
	app, config := setupTestApp()
	log := newTestLogger()

	app.Use(middleware.NewAPIKeyAuth(config, log))
	app.Get("/test", func(ctx fiber.Ctx) error {
		return ctx.SendString("OK")
	})

	// Request tanpa header X-API-Key
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	resp, err := app.Test(req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestAPIKeyAuth_Negative_WrongKey menguji autentikasi gagal dengan API key salah.
func TestAPIKeyAuth_Negative_WrongKey(t *testing.T) {
	app, config := setupTestApp()
	log := newTestLogger()

	app.Use(middleware.NewAPIKeyAuth(config, log))
	app.Get("/test", func(ctx fiber.Ctx) error {
		return ctx.SendString("OK")
	})

	// Request dengan API key yang salah
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "wrong-key")

	resp, err := app.Test(req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestAPIKeyAuth_Negative_EmptyKey menguji autentikasi gagal dengan API key kosong.
func TestAPIKeyAuth_Negative_EmptyKey(t *testing.T) {
	app, config := setupTestApp()
	log := newTestLogger()

	app.Use(middleware.NewAPIKeyAuth(config, log))
	app.Get("/test", func(ctx fiber.Ctx) error {
		return ctx.SendString("OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "")

	resp, err := app.Test(req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestRequestLogger menguji bahwa request logger middleware tidak mengganggu request.
func TestRequestLogger(t *testing.T) {
	app := fiber.New()
	log := newTestLogger()

	app.Use(middleware.NewRequestLogger(log))
	app.Get("/test", func(ctx fiber.Ctx) error {
		return ctx.SendString("OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	resp, err := app.Test(req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestRequestLogger_PostRequest menguji logger dengan POST request.
func TestRequestLogger_PostRequest(t *testing.T) {
	app := fiber.New()
	log := newTestLogger()

	app.Use(middleware.NewRequestLogger(log))
	app.Post("/test", func(ctx fiber.Ctx) error {
		return ctx.SendString("Created")
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)

	resp, err := app.Test(req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestRequestLogger_NotFound menguji logger dengan route yang tidak ada.
func TestRequestLogger_NotFound(t *testing.T) {
	app := fiber.New()
	log := newTestLogger()

	app.Use(middleware.NewRequestLogger(log))

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)

	resp, err := app.Test(req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
