package middleware

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

type mockLoggerDB struct{}

func (m *mockLoggerDB) Info(msg string, args ...interface{})  {}
func (m *mockLoggerDB) Warn(msg string, args ...interface{})  {}
func (m *mockLoggerDB) Error(msg string, args ...interface{}) {}
func (m *mockLoggerDB) Debug(msg string, args ...interface{}) {}

// fake database error type for test
var errFakeDB = errors.New("fake db error")

func TestDatabaseErrorHandlingMiddleware_ReturnsServerError(t *testing.T) {
	app := fiber.New()
	logger := &mockLoggerDB{}
	app.Use(DatabaseErrorHandlingMiddleware(logger))
	app.Get("/api/v1/token", func(c *fiber.Ctx) error {
		return errFakeDB
	})

	req := httptest.NewRequest("GET", "/api/v1/token", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode) // Should return 500 with error response body
}

func TestDatabaseErrorHandlingMiddleware_PassThroughOnNoError(t *testing.T) {
	app := fiber.New()
	logger := &mockLoggerDB{}
	app.Use(DatabaseErrorHandlingMiddleware(logger))
	app.Get("/api/v1/token", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/token", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}
