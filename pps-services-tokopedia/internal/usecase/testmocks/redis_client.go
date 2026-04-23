package testmocks

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type MockRedisClient struct {
	GetFunc    func(ctx context.Context, key string) *redis.StringCmd
	SetFunc    func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	DelFunc    func(ctx context.Context, keys ...string) *redis.IntCmd
	ExpireFunc func(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	IncrFunc   func(ctx context.Context, key string) *redis.IntCmd
	PingFunc   func(ctx context.Context) *redis.StatusCmd
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key)
	}
	return new(redis.StringCmd)
}
func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	if m.SetFunc != nil {
		return m.SetFunc(ctx, key, value, expiration)
	}
	return new(redis.StatusCmd)
}
func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	if m.DelFunc != nil {
		return m.DelFunc(ctx, keys...)
	}
	return new(redis.IntCmd)
}
func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	if m.ExpireFunc != nil {
		return m.ExpireFunc(ctx, key, expiration)
	}
	return new(redis.BoolCmd)
}
func (m *MockRedisClient) Incr(ctx context.Context, key string) *redis.IntCmd {
	if m.IncrFunc != nil {
		return m.IncrFunc(ctx, key)
	}
	return new(redis.IntCmd)
}
func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return new(redis.StatusCmd)
}

// Add methods as needed for tests
// Example:
// func (m *MockRedisClient) SomeMethod(...) {...}
