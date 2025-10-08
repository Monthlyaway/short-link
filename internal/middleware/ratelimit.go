package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// ============================================================================
// RATE LIMITING MIDDLEWARE - EDUCATIONAL IMPLEMENTATION
// ============================================================================
// This middleware demonstrates three popular rate limiting algorithms:
// 1. Fixed Window Counter - Simple but has burst issues at window boundaries
// 2. Sliding Window Log - Precise but memory intensive
// 3. Token Bucket - Allows controlled bursts, most flexible
// ============================================================================

// RateLimitStrategy defines the rate limiting algorithm to use
type RateLimitStrategy string

const (
	// FixedWindow counts requests in fixed time windows
	// Pros: Simple, low memory
	// Cons: Allows 2x burst at window boundaries
	FixedWindow RateLimitStrategy = "fixed_window"

	// SlidingWindow uses sorted sets to track request timestamps
	// Pros: Precise, no boundary issues
	// Cons: Higher memory usage (stores timestamp per request)
	SlidingWindow RateLimitStrategy = "sliding_window"

	// TokenBucket refills tokens at a constant rate
	// Pros: Allows controlled bursts, smooth rate limiting
	// Cons: Slightly more complex logic
	TokenBucket RateLimitStrategy = "token_bucket"
)

// RateLimitConfig holds configuration for the rate limiter
type RateLimitConfig struct {
	// Strategy determines which algorithm to use
	Strategy RateLimitStrategy

	// Limit is the maximum number of requests allowed
	Limit int

	// Window is the time period for the limit (e.g., 1 minute)
	Window time.Duration

	// KeyFunc generates the rate limit key (default: IP-based)
	KeyFunc func(*gin.Context) string

	// ErrorHandler is called when rate limit is exceeded
	ErrorHandler func(*gin.Context)

	// SkipFunc determines if rate limiting should be skipped for this request
	SkipFunc func(*gin.Context) bool
}

// RateLimiter manages rate limiting using Redis
type RateLimiter struct {
	redis  *redis.Client
	config *RateLimitConfig
}

// NewRateLimiter creates a new rate limiter instance
func NewRateLimiter(redisClient *redis.Client, config *RateLimitConfig) *RateLimiter {
	// Set default key function (based on client IP)
	if config.KeyFunc == nil {
		config.KeyFunc = func(c *gin.Context) string {
			return fmt.Sprintf("rate_limit:%s:%s", c.ClientIP(), c.Request.URL.Path)
		}
	}

	// Set default error handler
	if config.ErrorHandler == nil {
		config.ErrorHandler = defaultErrorHandler
	}

	// Set default skip function (don't skip any requests)
	if config.SkipFunc == nil {
		config.SkipFunc = func(c *gin.Context) bool {
			return false
		}
	}

	return &RateLimiter{
		redis:  redisClient,
		config: config,
	}
}

// Middleware returns a Gin middleware function
// This is the main entry point that will be used in router.Use()
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// ====================================================================
		// STEP 1: Check if we should skip rate limiting for this request
		// ====================================================================
		if rl.config.SkipFunc(c) {
			c.Next() // Continue to the next handler without rate limiting
			return
		}

		// ====================================================================
		// STEP 2: Generate a unique key for this client/endpoint combination
		// ====================================================================
		// Example key: "rate_limit:192.168.1.100:/api/v1/shorten"
		key := rl.config.KeyFunc(c)

		// ====================================================================
		// STEP 3: Check rate limit based on configured strategy
		// ====================================================================
		allowed, remaining, resetTime, err := rl.checkRateLimit(c.Request.Context(), key)

		// ====================================================================
		// STEP 4: Handle Redis errors gracefully (fail open)
		// ====================================================================
		// If Redis is down, we allow the request to prevent total service outage
		if err != nil {
			// Log the error (in production, use proper logger)
			fmt.Printf("Rate limiter error: %v (failing open)\n", err)
			c.Next()
			return
		}

		// ====================================================================
		// STEP 5: Set rate limit headers (RFC 6585 compliant)
		// ====================================================================
		// These headers inform the client about their rate limit status
		c.Header("X-RateLimit-Limit", strconv.Itoa(rl.config.Limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))

		// ====================================================================
		// STEP 6: Either allow the request or return 429 Too Many Requests
		// ====================================================================
		if !allowed {
			// Calculate retry-after seconds
			retryAfter := resetTime - time.Now().Unix()
			if retryAfter < 0 {
				retryAfter = 0
			}
			c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))

			// Call custom error handler
			rl.config.ErrorHandler(c)

			// Abort prevents calling subsequent handlers
			c.Abort()
			return
		}

		// ====================================================================
		// STEP 7: Request is allowed, continue to the next handler
		// ====================================================================
		c.Next()
	}
}

// checkRateLimit implements the actual rate limiting logic
// Returns: (allowed bool, remaining int, resetTime int64, error)
func (rl *RateLimiter) checkRateLimit(ctx context.Context, key string) (bool, int, int64, error) {
	switch rl.config.Strategy {
	case FixedWindow:
		return rl.fixedWindowCheck(ctx, key)
	case SlidingWindow:
		return rl.slidingWindowCheck(ctx, key)
	case TokenBucket:
		return rl.tokenBucketCheck(ctx, key)
	default:
		return rl.fixedWindowCheck(ctx, key)
	}
}

// ============================================================================
// ALGORITHM 1: FIXED WINDOW COUNTER
// ============================================================================
// How it works:
// - Each time window (e.g., 1 minute) gets a counter in Redis
// - Increment counter on each request
// - Reset counter when window expires
//
// Example timeline (limit=5, window=1min):
// 10:00:00 - Request 1 (count=1) ✅
// 10:00:30 - Request 2 (count=2) ✅
// 10:00:59 - Request 5 (count=5) ✅
// 10:01:00 - Window resets (count=0)
// 10:01:00 - Request 6 (count=1) ✅
//
// ⚠️ Boundary Problem:
// 10:00:59 - 5 requests ✅
// 10:01:00 - 5 requests ✅ (window reset)
// → User sent 10 requests in 1 second!
// ============================================================================
func (rl *RateLimiter) fixedWindowCheck(ctx context.Context, key string) (bool, int, int64, error) {
	// Calculate current window start time
	now := time.Now()
	windowStart := now.Truncate(rl.config.Window).Unix()

	// Redis key includes the window timestamp
	// Example: "rate_limit:192.168.1.100:/api/v1/shorten:1696780800"
	windowKey := fmt.Sprintf("%s:%d", key, windowStart)

	// Use Redis pipeline for atomic operations
	pipe := rl.redis.Pipeline()

	// INCR command: atomically increment the counter
	incrCmd := pipe.Incr(ctx, windowKey)

	// Set expiration to prevent memory leak
	// TTL = 2x window to handle clock skew
	pipe.Expire(ctx, windowKey, rl.config.Window*2)

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, 0, err
	}

	// Get the current count
	count := int(incrCmd.Val())

	// Calculate when the window resets
	resetTime := windowStart + int64(rl.config.Window.Seconds())

	// Check if limit exceeded
	allowed := count <= rl.config.Limit
	remaining := rl.config.Limit - count
	if remaining < 0 {
		remaining = 0
	}

	return allowed, remaining, resetTime, nil
}

// ============================================================================
// ALGORITHM 2: SLIDING WINDOW LOG
// ============================================================================
// How it works:
// - Store timestamp of each request in a Redis Sorted Set
// - Score = timestamp (for range queries)
// - Remove old timestamps outside the window
// - Count remaining timestamps
//
// Example (limit=5, window=60s):
// Redis Sorted Set: "rate_limit:IP:path"
// ┌─────────────────────────────┐
// │ Score (timestamp) │ Member  │
// ├─────────────────────────────┤
// │ 1696780810        │ req1    │ ← 50s ago (keep)
// │ 1696780820        │ req2    │ ← 40s ago (keep)
// │ 1696780830        │ req3    │ ← 30s ago (keep)
// │ 1696780840        │ req4    │ ← 20s ago (keep)
// │ 1696780850        │ req5    │ ← 10s ago (keep)
// └─────────────────────────────┘
//
// Pros: Precise, no boundary issues
// Cons: Memory usage O(limit) per key
// ============================================================================
func (rl *RateLimiter) slidingWindowCheck(ctx context.Context, key string) (bool, int, int64, error) {
	now := time.Now()
	windowStart := now.Add(-rl.config.Window).UnixNano()
	nowNano := now.UnixNano()

	pipe := rl.redis.Pipeline()

	// Remove timestamps older than the window
	// ZREMRANGEBYSCORE key -inf (now - window)
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))

	// Add current request timestamp
	// ZADD key timestamp timestamp
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(nowNano),
		Member: nowNano, // Use timestamp as member for uniqueness
	})

	// Count total requests in the window
	// ZCARD key
	zcardCmd := pipe.ZCard(ctx, key)

	// Set expiration
	pipe.Expire(ctx, key, rl.config.Window*2)

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, 0, err
	}

	count := int(zcardCmd.Val())

	// Calculate reset time (when oldest request expires)
	resetTime := now.Add(rl.config.Window).Unix()

	allowed := count <= rl.config.Limit
	remaining := rl.config.Limit - count
	if remaining < 0 {
		remaining = 0
	}

	return allowed, remaining, resetTime, nil
}

// ============================================================================
// ALGORITHM 3: TOKEN BUCKET
// ============================================================================
// How it works:
// - Bucket has a capacity of tokens (= limit)
// - Tokens refill at a constant rate (limit / window)
// - Each request consumes 1 token
// - If no tokens available, request is rejected
//
// Example (limit=10, window=60s, refill_rate=10/60=0.167 tokens/sec):
// Time    Tokens  Action
// 0s      10      Initial state
// 1s      10      5 requests → 5 tokens remain
// 2s      5.17    Refilled 0.17 tokens
// 10s     6.5     Refilled 1.33 tokens
// 10s     0.5     6 requests → exceeded by 5.5 (reject)
//
// Pros: Allows bursts up to capacity, smooth refilling
// Cons: More complex logic
// ============================================================================
func (rl *RateLimiter) tokenBucketCheck(ctx context.Context, key string) (bool, int, int64, error) {
	now := time.Now()

	// Token bucket uses two Redis keys:
	tokensKey := key + ":tokens"         // Current token count
	lastRefillKey := key + ":last_refill" // Last refill timestamp

	// Refill rate: tokens per second
	refillRate := float64(rl.config.Limit) / rl.config.Window.Seconds()

	// Get current state
	pipe := rl.redis.Pipeline()
	getTokensCmd := pipe.Get(ctx, tokensKey)
	getLastRefillCmd := pipe.Get(ctx, lastRefillKey)
	_, _ = pipe.Exec(ctx)

	// Parse current tokens (default to full capacity)
	tokens := float64(rl.config.Limit)
	if getTokensCmd.Err() == nil {
		if val, err := strconv.ParseFloat(getTokensCmd.Val(), 64); err == nil {
			tokens = val
		}
	}

	// Parse last refill time (default to now)
	lastRefill := now.Unix()
	if getLastRefillCmd.Err() == nil {
		if val, err := strconv.ParseInt(getLastRefillCmd.Val(), 10, 64); err == nil {
			lastRefill = val
		}
	}

	// Calculate tokens to add based on time elapsed
	elapsed := now.Unix() - lastRefill
	tokensToAdd := float64(elapsed) * refillRate

	// Refill tokens (capped at limit)
	tokens += tokensToAdd
	if tokens > float64(rl.config.Limit) {
		tokens = float64(rl.config.Limit)
	}

	// Try to consume 1 token
	allowed := tokens >= 1.0
	if allowed {
		tokens -= 1.0
	}

	// Update Redis
	pipe = rl.redis.Pipeline()
	pipe.Set(ctx, tokensKey, fmt.Sprintf("%.2f", tokens), rl.config.Window*2)
	pipe.Set(ctx, lastRefillKey, now.Unix(), rl.config.Window*2)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, 0, err
	}

	// Calculate reset time (when bucket refills to 1 token)
	resetTime := now.Unix()
	if tokens < 1.0 {
		secondsUntilRefill := int64((1.0 - tokens) / refillRate)
		resetTime += secondsUntilRefill
	}

	remaining := int(tokens)
	if remaining < 0 {
		remaining = 0
	}

	return allowed, remaining, resetTime, nil
}

// ============================================================================
// DEFAULT ERROR HANDLER
// ============================================================================
// Returns a standard 429 Too Many Requests response
func defaultErrorHandler(c *gin.Context) {
	c.JSON(http.StatusTooManyRequests, gin.H{
		"code":    http.StatusTooManyRequests,
		"message": "Rate limit exceeded. Please try again later.",
		"error":   "too_many_requests",
	})
}

// ============================================================================
// HELPER FUNCTIONS FOR CUSTOM CONFIGURATIONS
// ============================================================================

// IPBasedKey generates a rate limit key based on client IP only
func IPBasedKey(c *gin.Context) string {
	return fmt.Sprintf("rate_limit:ip:%s", c.ClientIP())
}

// PathBasedKey generates a rate limit key based on path only (global per endpoint)
func PathBasedKey(c *gin.Context) string {
	return fmt.Sprintf("rate_limit:path:%s", c.Request.URL.Path)
}

// IPAndPathKey generates a rate limit key based on both IP and path (default)
func IPAndPathKey(c *gin.Context) string {
	return fmt.Sprintf("rate_limit:%s:%s", c.ClientIP(), c.Request.URL.Path)
}

// SkipHealthCheck skips rate limiting for health check endpoints
func SkipHealthCheck(c *gin.Context) bool {
	return c.Request.URL.Path == "/health" || c.Request.URL.Path == "/metrics"
}
