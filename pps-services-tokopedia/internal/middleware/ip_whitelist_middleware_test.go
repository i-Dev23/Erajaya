package middleware

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"pps-services-tokopedia/internal/domain"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestIPWhitelistMiddleware_IPNotWhitelisted(t *testing.T) {
	app := fiber.New()
	mockRedis := new(MockRedisClientForIP)
	mockProductRepo := new(MockProductRepositoryForIP)
	mockCrypto := new(MockCryptoService)
	mockSignature := new(MockDigitalSignatureServiceForIP)
	mockLogger := new(MockLoggerForIP)

	cmd := redis.NewStringCmd(context.Background())
	cmd.SetVal("192.168.1.1,10.0.0.1")
	mockRedis.On("Get", mock.Anything, "WHITELISTED_IP").Return(cmd)
	mockCrypto.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted"), "key123", nil)
	mockSignature.On("SignPayload", mock.Anything, mock.Anything).Return("signature123", nil)
	mockLogger.On("Info", mock.Anything).Return()
	mockLogger.On("Warn", mock.Anything).Return()
	mockLogger.On("Error", mock.Anything).Return()

	app.Use(IPWhitelistMiddleware(mockRedis, mockProductRepo, mockCrypto, mockSignature, mockLogger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	mockRedis.AssertExpectations(t)
	mockCrypto.AssertExpectations(t)
	mockSignature.AssertExpectations(t)
}

// TestIPWhitelistMiddleware_RedisError tests when Redis returns an error (not redis.Nil)
func TestIPWhitelistMiddleware_RedisError(t *testing.T) {
	app := fiber.New()
	mockRedis := new(MockRedisClientForIP)
	mockProductRepo := new(MockProductRepositoryForIP)
	mockCrypto := new(MockCryptoService)
	mockSignature := new(MockDigitalSignatureServiceForIP)
	mockLogger := new(MockLoggerForIP)

	cmdErr := redis.NewStringCmd(context.Background())
	cmdErr.SetErr(errors.New("redis connection error"))
	mockRedis.On("Get", mock.Anything, "WHITELISTED_IP").Return(cmdErr)
	mockCrypto.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted"), "key123", nil)
	mockSignature.On("SignPayload", mock.Anything, mock.Anything).Return("signature123", nil)
	mockLogger.On("Info", mock.Anything).Return()
	mockLogger.On("Warn", mock.Anything).Return()
	mockLogger.On("Error", mock.Anything).Return()

	// Setup GetIpByUser to return nil and error for fallback path
	mockProductRepo.On("GetIpByUser", mock.Anything, mock.Anything).Return(nil, errors.New("oracle error"))

	app.Use(IPWhitelistMiddleware(mockRedis, mockProductRepo, mockCrypto, mockSignature, mockLogger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.1")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	mockRedis.AssertExpectations(t)
	mockCrypto.AssertExpectations(t)
	mockSignature.AssertExpectations(t)
}

// TestIPWhitelistMiddleware_MissingHeader tests when X-Real-IP header is missing
func TestIPWhitelistMiddleware_MissingHeader(t *testing.T) {
	app := fiber.New()
	mockRedis := new(MockRedisClientForIP)
	mockProductRepo := new(MockProductRepositoryForIP)
	mockCrypto := new(MockCryptoService)
	mockSignature := new(MockDigitalSignatureServiceForIP)
	mockLogger := new(MockLoggerForIP)

	cmd := redis.NewStringCmd(context.Background())
	cmd.SetVal("192.168.1.1,10.0.0.1")
	mockRedis.On("Get", mock.Anything, "WHITELISTED_IP").Return(cmd)
	mockCrypto.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted"), "key123", nil)
	mockSignature.On("SignPayload", mock.Anything, mock.Anything).Return("signature123", nil)
	mockLogger.On("Info", mock.Anything).Return()
	mockLogger.On("Warn", mock.Anything).Return()
	mockLogger.On("Error", mock.Anything).Return()

	app.Use(IPWhitelistMiddleware(mockRedis, mockProductRepo, mockCrypto, mockSignature, mockLogger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// No X-Real-IP header
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	mockRedis.AssertExpectations(t)
	mockCrypto.AssertExpectations(t)
	mockSignature.AssertExpectations(t)
}

// TestIPWhitelistMiddleware_EncryptError tests error when encrypting error response
func TestIPWhitelistMiddleware_EncryptError(t *testing.T) {
	app := fiber.New()
	mockRedis := new(MockRedisClientForIP)
	mockProductRepo := new(MockProductRepositoryForIP)
	mockCrypto := new(MockCryptoService)
	mockSignature := new(MockDigitalSignatureServiceForIP)
	mockLogger := new(MockLoggerForIP)

	cmd := redis.NewStringCmd(context.Background())
	cmd.SetVal("")
	mockRedis.On("Get", mock.Anything, "WHITELISTED_IP").Return(cmd)
	mockCrypto.On("Encrypt", mock.Anything, mock.Anything).Return([]byte{}, "", errors.New("encrypt fail"))
	mockLogger.On("Info", mock.Anything).Return()
	mockLogger.On("Warn", mock.Anything).Return()
	mockLogger.On("Error", mock.Anything).Return()

	// Setup GetIpByUser to return nil and error for fallback path
	mockProductRepo.On("GetIpByUser", mock.Anything, mock.Anything).Return(nil, errors.New("oracle error"))

	app.Use(IPWhitelistMiddleware(mockRedis, mockProductRepo, mockCrypto, mockSignature, mockLogger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	mockRedis.AssertExpectations(t)
	mockCrypto.AssertExpectations(t)
}

// TestIPWhitelistMiddleware_SignError tests error when signing error response
func TestIPWhitelistMiddleware_SignError(t *testing.T) {
	app := fiber.New()
	mockRedis := new(MockRedisClientForIP)
	mockProductRepo := new(MockProductRepositoryForIP)
	mockCrypto := new(MockCryptoService)
	mockSignature := new(MockDigitalSignatureServiceForIP)
	mockLogger := new(MockLoggerForIP)

	cmd := redis.NewStringCmd(context.Background())
	cmd.SetVal("")
	mockRedis.On("Get", mock.Anything, "WHITELISTED_IP").Return(cmd)
	mockCrypto.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted"), "key123", nil)
	mockSignature.On("SignPayload", mock.Anything, mock.Anything).Return("", errors.New("sign fail"))
	mockLogger.On("Info", mock.Anything).Return()
	mockLogger.On("Warn", mock.Anything).Return()
	mockLogger.On("Error", mock.Anything).Return()

	// Setup GetIpByUser to return nil and error for fallback path
	mockProductRepo.On("GetIpByUser", mock.Anything, mock.Anything).Return(nil, errors.New("oracle error"))

	app.Use(IPWhitelistMiddleware(mockRedis, mockProductRepo, mockCrypto, mockSignature, mockLogger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	mockRedis.AssertExpectations(t)
	mockCrypto.AssertExpectations(t)
	mockSignature.AssertExpectations(t)
}

// MockRedisClientForIP is a mock implementation of RedisClient
type MockRedisClientForIP struct {
	mock.Mock
}

func (m *MockRedisClientForIP) Get(ctx context.Context, key string) *redis.StringCmd {
	args := m.Called(ctx, key)
	return args.Get(0).(*redis.StringCmd)
}

func (m *MockRedisClientForIP) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	args := m.Called(ctx, key, value, expiration)
	return args.Get(0).(*redis.StatusCmd)
}

func (m *MockRedisClientForIP) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	args := m.Called(ctx, keys)
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClientForIP) Incr(ctx context.Context, key string) *redis.IntCmd {
	args := m.Called(ctx, key)
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClientForIP) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	args := m.Called(ctx, key, expiration)
	return args.Get(0).(*redis.BoolCmd)
}

func (m *MockRedisClientForIP) Ping(ctx context.Context) *redis.StatusCmd {
	args := m.Called(ctx)
	return args.Get(0).(*redis.StatusCmd)
}

// MockProductRepositoryForIP is a mock implementation of ProductRepository
type MockProductRepositoryForIP struct {
	mock.Mock
}

func (m *MockProductRepositoryForIP) GetPriceByUser(ctx context.Context, username string, provider string) (*[]domain.ProductPriceResponseDomain, error) {
	args := m.Called(ctx, username, provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*[]domain.ProductPriceResponseDomain), args.Error(1)
}

func (m *MockProductRepositoryForIP) GetPriceByUserAndProductCode(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
	args := m.Called(ctx, username, productCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProductPriceResponseDomain), args.Error(1)
}

func (m *MockProductRepositoryForIP) GetProductByUser(ctx context.Context, username string) (*[]domain.ProductPriceResponseDomain, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*[]domain.ProductPriceResponseDomain), args.Error(1)
}

func (m *MockProductRepositoryForIP) GetProductByUserAndProductCode(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
	args := m.Called(ctx, username, productCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProductPriceResponseDomain), args.Error(1)
}

func (m *MockProductRepositoryForIP) GetIpByUser(ctx context.Context, username string) (*domain.WhitelistedIpResponseDomain, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.WhitelistedIpResponseDomain), args.Error(1)
}

func (m *MockProductRepositoryForIP) GetCutOff(ctx context.Context) (*domain.CutOffDataResponseDomain, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CutOffDataResponseDomain), args.Error(1)
}

// MockDigitalSignatureServiceForIP is a mock implementation of DigitalSignatureService
type MockDigitalSignatureServiceForIP struct {
	mock.Mock
}

func (m *MockDigitalSignatureServiceForIP) SignPayload(ctx context.Context, payload string) (string, error) {
	args := m.Called(ctx, payload)
	return args.String(0), args.Error(1)
}

func (m *MockDigitalSignatureServiceForIP) VerifyPayload(ctx context.Context, payload string, signature string) error {
	args := m.Called(ctx, payload, signature)
	return args.Error(0)
}

// MockLoggerForIP is a mock logger for IP whitelist tests
type MockLoggerForIP struct {
	mock.Mock
}

func (m *MockLoggerForIP) Info(msg string, keysAndValues ...interface{}) {
	m.Called(msg)
}

func (m *MockLoggerForIP) Error(msg string, keysAndValues ...interface{}) {
	m.Called(msg)
}

func (m *MockLoggerForIP) Warn(msg string, keysAndValues ...interface{}) {
	m.Called(msg)
}

func (m *MockLoggerForIP) Debug(msg string, keysAndValues ...interface{}) {
	m.Called(msg)
}

func (m *MockLoggerForIP) Fatal(msg string, keysAndValues ...interface{}) {
	m.Called(msg)
}

func (m *MockLoggerForIP) Sync() error {
	args := m.Called()
	return args.Error(0)
}

// TestIPWhitelistMiddleware_Success_FromRedis tests successful IP validation from Redis cache
func TestIPWhitelistMiddleware_Success_FromRedis(t *testing.T) {
	app := fiber.New()
	mockRedis := new(MockRedisClientForIP)
	mockProductRepo := new(MockProductRepositoryForIP)
	mockCrypto := new(MockCryptoService)
	mockSignature := new(MockDigitalSignatureServiceForIP)
	mockLogger := new(MockLoggerForIP)

	cmd := redis.NewStringCmd(context.Background())
	cmd.SetVal("192.168.1.1,10.0.0.1,127.0.0.1")
	mockRedis.On("Get", mock.Anything, "WHITELISTED_IP").Return(cmd)
	mockLogger.On("Info", mock.Anything).Return()

	app.Use(IPWhitelistMiddleware(mockRedis, mockProductRepo, mockCrypto, mockSignature, mockLogger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.1")
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	mockRedis.AssertExpectations(t)
}

// TestIPWhitelistMiddleware_OracleException_Code62 tests Oracle exception returns code 62
func TestIPWhitelistMiddleware_OracleException_Code62(t *testing.T) {
	app := fiber.New()
	mockRedis := new(MockRedisClientForIP)
	mockProductRepo := new(MockProductRepositoryForIP)
	mockCrypto := new(MockCryptoService)
	mockSignature := new(MockDigitalSignatureServiceForIP)
	mockLogger := new(MockLoggerForIP)

	cmdErr := redis.NewStringCmd(context.Background())
	cmdErr.SetErr(redis.Nil)
	mockRedis.On("Get", mock.Anything, "WHITELISTED_IP").Return(cmdErr)

	mockProductRepo.On("GetIpByUser", mock.Anything, mock.Anything).Return(
		nil,
		errors.New("ORA-12541: TNS:no listener"),
	)

	mockCrypto.On("Encrypt", mock.Anything, mock.Anything).Return([]byte("encrypted"), "key123", nil)
	mockSignature.On("SignPayload", mock.Anything, mock.Anything).Return("signature123", nil)

	mockLogger.On("Info", mock.Anything).Return()
	mockLogger.On("Warn", mock.Anything).Return()
	mockLogger.On("Error", mock.Anything).Return()

	app.Use(IPWhitelistMiddleware(mockRedis, mockProductRepo, mockCrypto, mockSignature, mockLogger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.1")
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	mockRedis.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
	mockCrypto.AssertExpectations(t)
	mockSignature.AssertExpectations(t)
	mockLogger.AssertCalled(t, "Error", "IP whitelist check failed - Oracle exception occurred")
}

// TestValidateIPInList tests the IP validation logic
func TestValidateIPInList(t *testing.T) {
	tests := []struct {
		name     string
		ipList   string
		clientIP string
		expected bool
	}{
		{
			name:     "IP found in list",
			ipList:   "192.168.1.1,10.0.0.1,127.0.0.1",
			clientIP: "192.168.1.1",
			expected: true,
		},
		{
			name:     "IP not found in list",
			ipList:   "192.168.1.1,10.0.0.1",
			clientIP: "99.99.99.99",
			expected: false,
		},
		{
			name:     "Empty IP list",
			ipList:   "",
			clientIP: "192.168.1.1",
			expected: false,
		},
		{
			name:     "IP with whitespace",
			ipList:   "192.168.1.1, 10.0.0.1 , 127.0.0.1",
			clientIP: "10.0.0.1",
			expected: true,
		},
		{
			name:     "Single IP match",
			ipList:   "192.168.1.1",
			clientIP: "192.168.1.1",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateIPInList(tt.ipList, tt.clientIP)
			assert.Equal(t, tt.expected, result)
		})
	}
}
