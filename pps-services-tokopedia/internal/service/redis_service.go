package service

import (
	"context"
	"errors"
	"pps-services-tokopedia/internal/utils"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient interface {
	Ping(ctx context.Context) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	// Add more methods as needed
}

type redisClientImpl struct {
	client *redis.Client
}

func (r *redisClientImpl) Ping(ctx context.Context) *redis.StatusCmd {
	return r.client.Ping(ctx)
}

func (r *redisClientImpl) Get(ctx context.Context, key string) *redis.StringCmd {
	return r.client.Get(ctx, key)
}

func (r *redisClientImpl) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	return r.client.Set(ctx, key, value, expiration)
}

func (r *redisClientImpl) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return r.client.Del(ctx, keys...)
}

func (r *redisClientImpl) Incr(ctx context.Context, key string) *redis.IntCmd {
	return r.client.Incr(ctx, key)
}

func (r *redisClientImpl) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	return r.client.Expire(ctx, key, expiration)
}

// mockRedisClient is a mock implementation for development when Redis is not available
type mockRedisClient struct{}

func (m *mockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	return redis.NewStatusCmd(ctx, "PING", "OK")
}

func (m *mockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx, "GET", key)
	cmd.SetErr(errors.New("Redis not available - using mock"))
	return cmd
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx, "SET", key, value)
	cmd.SetErr(errors.New("Redis not available - using mock"))
	return cmd
}

func (m *mockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx, "DEL", keys)
	cmd.SetErr(errors.New("Redis not available - using mock"))
	return cmd
}

func (m *mockRedisClient) Incr(ctx context.Context, key string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx, "INCR", key)
	cmd.SetErr(errors.New("Redis not available - using mock"))
	return cmd
}

func (m *mockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(ctx, "EXPIRE", key, expiration.Seconds())
	cmd.SetErr(errors.New("Redis not available - using mock"))
	return cmd
}

var (
	redisOnce   sync.Once
	redisClient RedisClient
)

// NewRedisClient returns a singleton RedisClient instance, configured from environment variables.
func NewRedisClient() RedisClient {
	redisOnce.Do(func() {
		addr := utils.GetEnv("REDIS_ADDR", "localhost:6379")
		password := utils.GetEnv("REDIS_PASSWORD", "")
		db := utils.GetEnvAsInt("REDIS_DB", 0)

		client := redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		})

		// Test connection on startup, but don't panic in development
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := client.Ping(ctx).Err(); err != nil {
			// In development, we can use a mock Redis client
			// In production, this should be handled properly
			redisClient = &mockRedisClient{}
			return
		}

		redisClient = &redisClientImpl{client: client}
	})
	return redisClient
}
