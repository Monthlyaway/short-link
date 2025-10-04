package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// ShortCodePrefix is the prefix for short code keys in Redis
	ShortCodePrefix = "short:code:"
	// DefaultTTL is the default TTL for cached items (24 hours)
	DefaultTTL = 24 * time.Hour
)

// RedisCache wraps the Redis client
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(addr, password string, db, poolSize int) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
		PoolSize: poolSize,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{client: client}, nil
}

// Get retrieves the original URL for a given short code
func (r *RedisCache) Get(ctx context.Context, shortCode string) (string, error) {
	key := ShortCodePrefix + shortCode
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Cache miss
	}
	if err != nil {
		return "", fmt.Errorf("failed to get from Redis: %w", err)
	}
	return val, nil
}

// Set stores the original URL for a given short code with default TTL
func (r *RedisCache) Set(ctx context.Context, shortCode, originalURL string) error {
	return r.SetWithTTL(ctx, shortCode, originalURL, DefaultTTL)
}

// SetWithTTL stores the original URL for a given short code with custom TTL
func (r *RedisCache) SetWithTTL(ctx context.Context, shortCode, originalURL string, ttl time.Duration) error {
	key := ShortCodePrefix + shortCode
	if err := r.client.Set(ctx, key, originalURL, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set in Redis: %w", err)
	}
	return nil
}

// Delete removes a short code from cache
func (r *RedisCache) Delete(ctx context.Context, shortCode string) error {
	key := ShortCodePrefix + shortCode
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete from Redis: %w", err)
	}
	return nil
}

// Close closes the Redis connection
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// GetClient returns the underlying Redis client
func (r *RedisCache) GetClient() *redis.Client {
	return r.client
}
