package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// setupTestRedis creates a Redis client for testing
// Make sure Redis is running on localhost:6379
func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use DB 15 for testing to avoid conflicts
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	// Clean up test keys
	client.FlushDB(ctx)

	return client
}

// setupTestRouter creates a Gin router with rate limiting
func setupTestRouter(limiter *RateLimiter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Apply rate limiting middleware
	router.Use(limiter.Middleware())

	// Test endpoint
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	return router
}

// TestFixedWindowStrategy tests the fixed window rate limiting algorithm
func TestFixedWindowStrategy(t *testing.T) {
	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	limiter := NewRateLimiter(redisClient, &RateLimitConfig{
		Strategy: FixedWindow,
		Limit:    5,
		Window:   1 * time.Second,
	})

	router := setupTestRouter(limiter)

	// Send 5 requests (should all succeed)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
		assert.Equal(t, "5", w.Header().Get("X-RateLimit-Limit"))
	}

	// 6th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "0", w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
}

// TestSlidingWindowStrategy tests the sliding window rate limiting algorithm
func TestSlidingWindowStrategy(t *testing.T) {
	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	limiter := NewRateLimiter(redisClient, &RateLimitConfig{
		Strategy: SlidingWindow,
		Limit:    3,
		Window:   2 * time.Second,
	})

	router := setupTestRouter(limiter)

	// Send 3 requests (should all succeed)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
	}

	// 4th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Wait for window to partially slide
	time.Sleep(1 * time.Second)

	// Should still be limited (only 1 second passed, window is 2 seconds)
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Wait for full window to pass
	time.Sleep(1500 * time.Millisecond)

	// Now should succeed (old requests expired)
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestTokenBucketStrategy tests the token bucket rate limiting algorithm
func TestTokenBucketStrategy(t *testing.T) {
	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	limiter := NewRateLimiter(redisClient, &RateLimitConfig{
		Strategy: TokenBucket,
		Limit:    5,
		Window:   5 * time.Second, // Refill rate: 1 token/second
	})

	router := setupTestRouter(limiter)

	// Consume all 5 tokens
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
	}

	// 6th request should fail (no tokens)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Wait for 1 token to refill (1 second)
	time.Sleep(1100 * time.Millisecond)

	// Should succeed now (1 token refilled)
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCustomKeyFunc tests custom key generation
func TestCustomKeyFunc(t *testing.T) {
	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	// IP-only key (all paths share the same limit)
	limiter := NewRateLimiter(redisClient, &RateLimitConfig{
		Strategy: FixedWindow,
		Limit:    3,
		Window:   10 * time.Second,
		KeyFunc:  IPBasedKey,
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(limiter.Middleware())
	router.GET("/path1", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "path1"})
	})
	router.GET("/path2", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "path2"})
	})

	// Send 2 requests to path1
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/path1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Send 1 request to path2 (should succeed)
	req := httptest.NewRequest("GET", "/path2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 4th request to any path should fail (shared limit of 3)
	req = httptest.NewRequest("GET", "/path1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

// TestSkipFunc tests skipping rate limiting for certain requests
func TestSkipFunc(t *testing.T) {
	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	limiter := NewRateLimiter(redisClient, &RateLimitConfig{
		Strategy: FixedWindow,
		Limit:    2,
		Window:   10 * time.Second,
		SkipFunc: func(c *gin.Context) bool {
			// Skip rate limiting if X-Admin header is present
			return c.GetHeader("X-Admin") == "true"
		},
	})

	router := setupTestRouter(limiter)

	// Send 2 regular requests (consume limit)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// 3rd regular request should fail
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Admin request should bypass rate limiting
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Admin", "true")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRateLimitHeaders tests that proper headers are set
func TestRateLimitHeaders(t *testing.T) {
	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	limiter := NewRateLimiter(redisClient, &RateLimitConfig{
		Strategy: FixedWindow,
		Limit:    10,
		Window:   60 * time.Second,
	})

	router := setupTestRouter(limiter)

	// First request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "10", w.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "9", w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))

	// Second request
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "8", w.Header().Get("X-RateLimit-Remaining"))
}

// TestConcurrentRequests tests thread safety
func TestConcurrentRequests(t *testing.T) {
	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	limiter := NewRateLimiter(redisClient, &RateLimitConfig{
		Strategy: FixedWindow,
		Limit:    100,
		Window:   10 * time.Second,
	})

	router := setupTestRouter(limiter)

	// Send 50 concurrent requests
	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < 50; i++ {
		<-done
	}
}

// BenchmarkFixedWindow benchmarks the fixed window algorithm
func BenchmarkFixedWindow(b *testing.B) {
	redisClient := setupTestRedis(&testing.T{})
	defer redisClient.Close()

	limiter := NewRateLimiter(redisClient, &RateLimitConfig{
		Strategy: FixedWindow,
		Limit:    1000000,
		Window:   60 * time.Second,
	})

	router := setupTestRouter(limiter)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkSlidingWindow benchmarks the sliding window algorithm
func BenchmarkSlidingWindow(b *testing.B) {
	redisClient := setupTestRedis(&testing.T{})
	defer redisClient.Close()

	limiter := NewRateLimiter(redisClient, &RateLimitConfig{
		Strategy: SlidingWindow,
		Limit:    1000000,
		Window:   60 * time.Second,
	})

	router := setupTestRouter(limiter)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkTokenBucket benchmarks the token bucket algorithm
func BenchmarkTokenBucket(b *testing.B) {
	redisClient := setupTestRedis(&testing.T{})
	defer redisClient.Close()

	limiter := NewRateLimiter(redisClient, &RateLimitConfig{
		Strategy: TokenBucket,
		Limit:    1000000,
		Window:   60 * time.Second,
	})

	router := setupTestRouter(limiter)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
