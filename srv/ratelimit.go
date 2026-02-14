package srv

import (
	"sync"
	"time"
)

// RateLimitConfig defines per-message-type rate limits.
type RateLimitConfig struct {
	// Rate is the number of tokens added per second.
	Rate float64
	// Burst is the maximum number of tokens (bucket capacity).
	Burst int
}

// Default rate limit configurations per message type.
var defaultRateLimits = map[string]RateLimitConfig{
	// Game actions: relatively strict
	"answer":             {Rate: 1, Burst: 3},
	"vote":               {Rate: 1, Burst: 3},
	"challenge":          {Rate: 0.5, Burst: 2},
	"rebuttal":           {Rate: 0.5, Burst: 2},
	"withdraw_challenge": {Rate: 0.5, Burst: 2},

	// Room management: moderate
	"create_room": {Rate: 0.5, Burst: 2},
	"join":        {Rate: 0.5, Burst: 3},
	"leave_room":  {Rate: 1, Burst: 3},
	"start_game":  {Rate: 0.5, Burst: 2},

	// Read-only / lightweight: generous
	"get_rooms":  {Rate: 2, Burst: 5},
	"get_genres": {Rate: 2, Burst: 5},
	"ping":       {Rate: 2, Burst: 5},
}

// globalRateLimit applies to all messages regardless of type.
var globalRateLimit = RateLimitConfig{Rate: 10, Burst: 20}

// tokenBucket implements the token bucket algorithm.
type tokenBucket struct {
	tokens    float64
	max       float64
	rate      float64
	lastCheck time.Time
}

// newTokenBucket creates a new token bucket starting full.
func newTokenBucket(rate float64, burst int) *tokenBucket {
	return &tokenBucket{
		tokens:    float64(burst),
		max:       float64(burst),
		rate:      rate,
		lastCheck: time.Now(),
	}
}

// allow checks if a token is available and consumes one if so.
func (tb *tokenBucket) allow() bool {
	now := time.Now()
	elapsed := now.Sub(tb.lastCheck).Seconds()
	tb.lastCheck = now

	// Add tokens based on elapsed time
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.max {
		tb.tokens = tb.max
	}

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// ConnectionRateLimiter manages rate limits for a single WebSocket connection.
type ConnectionRateLimiter struct {
	mu      sync.Mutex
	global  *tokenBucket
	buckets map[string]*tokenBucket
	// Track consecutive violations for escalating response
	violations int
}

// NewConnectionRateLimiter creates a rate limiter for one connection.
func NewConnectionRateLimiter() *ConnectionRateLimiter {
	return &ConnectionRateLimiter{
		global:  newTokenBucket(globalRateLimit.Rate, globalRateLimit.Burst),
		buckets: make(map[string]*tokenBucket),
	}
}

// Allow checks if the given message type is allowed under rate limits.
// Returns (allowed bool, shouldDisconnect bool).
func (rl *ConnectionRateLimiter) Allow(msgType string) (bool, bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Check global rate limit first
	if !rl.global.allow() {
		rl.violations++
		return false, rl.violations >= 50
	}

	// Check per-type rate limit
	config, ok := defaultRateLimits[msgType]
	if !ok {
		// Unknown message types get a strict default
		config = RateLimitConfig{Rate: 1, Burst: 2}
	}

	bucket, exists := rl.buckets[msgType]
	if !exists {
		bucket = newTokenBucket(config.Rate, config.Burst)
		rl.buckets[msgType] = bucket
	}

	if !bucket.allow() {
		rl.violations++
		return false, rl.violations >= 50
	}

	// Reset violations on successful request
	if rl.violations > 0 {
		rl.violations--
	}
	return true, false
}
