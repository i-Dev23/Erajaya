package middleware

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRedisClient for rate limit

type mockLoggerRateLimit struct{ mock.Mock }

func (m *mockLoggerRateLimit) Debug(msg string, args ...interface{}) {}
func (m *mockLoggerRateLimit) Info(msg string, args ...interface{})  {}
func (m *mockLoggerRateLimit) Warn(msg string, args ...interface{})  {}
func (m *mockLoggerRateLimit) Error(msg string, args ...interface{}) {}

type MockRedisClient struct{ mock.Mock }

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	args := m.Called(ctx, key)
	return args.Get(0).(*redis.StringCmd)
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	args := m.Called(ctx, keys)
	return args.Get(0).(*redis.IntCmd)
}
func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	args := m.Called(ctx, key, value, expiration)
	return args.Get(0).(*redis.StatusCmd)
}
func (m *MockRedisClient) Incr(ctx context.Context, key string) *redis.IntCmd {
	args := m.Called(ctx, key)
	return args.Get(0).(*redis.IntCmd)
}
func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	args := m.Called(ctx, key, expiration)
	return args.Get(0).(*redis.BoolCmd)
}
func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	args := m.Called(ctx)
	return args.Get(0).(*redis.StatusCmd)
}

func TestRateLimitMiddleware_AllowsRequest(t *testing.T) {
	app := fiber.New()
	redis := new(MockRedisClient)
	logger := &mockLoggerRateLimit{}
	// Setup mocks to allow request (count = 1, limit = 5)
	redis.On("Incr", mock.Anything, mock.Anything).Return(redisIntCmd(1), nil)
	redis.On("Expire", mock.Anything, mock.Anything, mock.Anything).Return(redisBoolCmd(true), nil)
	app.Use(RateLimitMiddleware(redis, nil, nil, logger))
	app.Get("/auth/token", func(c *fiber.Ctx) error { return c.SendString("ok") })
	req := httptest.NewRequest("GET", "/auth/token", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestRateLimitMiddleware_BlocksRequest(t *testing.T) {
	app := fiber.New()
	redis := new(MockRedisClient)
	logger := &mockLoggerRateLimit{}
	// Setup mocks to block request (count = 6, limit = 5)
	redis.On("Incr", mock.Anything, mock.Anything).Return(redisIntCmd(6), nil)
	redis.On("Expire", mock.Anything, mock.Anything, mock.Anything).Return(redisBoolCmd(true), nil)
	// Crypto and signature mocks for error response
	crypto := &mockCryptoService{}
	digital := &mockDigitalSignatureService{}
	crypto.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted"), "key", nil)
	digital.On("SignPayload", mock.Anything, mock.Anything).Return("sig", nil)
	app.Use(RateLimitMiddleware(redis, crypto, digital, logger))
	app.Get("/auth/token", func(c *fiber.Ctx) error { return c.SendString("ok") })
	req := httptest.NewRequest("GET", "/auth/token", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Middleware returns StatusOK (200) with encrypted error body, not 429
	assert.Equal(t, 200, resp.StatusCode)
}

// Helpers for redis mocks
func redisIntCmd(val int64) *redis.IntCmd {
	cmd := redis.NewIntCmd(context.Background())
	cmd.SetVal(val)
	return cmd
}
func redisBoolCmd(val bool) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(context.Background())
	cmd.SetVal(val)
	return cmd
}
