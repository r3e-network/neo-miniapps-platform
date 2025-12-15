package txsubmitter

import (
	"sync"
	"time"
)

// =============================================================================
// Token Bucket Rate Limiter
// =============================================================================

// TokenBucket implements a token bucket rate limiter.
type TokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewTokenBucket creates a new token bucket.
func NewTokenBucket(tokensPerSecond float64, burstMultiplier float64) *TokenBucket {
	maxTokens := tokensPerSecond * burstMultiplier
	return &TokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: tokensPerSecond,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed and consumes a token if so.
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// AllowN checks if n requests are allowed and consumes n tokens if so.
func (tb *TokenBucket) AllowN(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return true
	}
	return false
}

// Available returns the number of available tokens.
func (tb *TokenBucket) Available() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	return tb.tokens
}

// refill adds tokens based on elapsed time (must be called with lock held).
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now
}

// =============================================================================
// Rate Limiter Manager
// =============================================================================

// RateLimiter manages rate limiting for multiple services.
type RateLimiter struct {
	mu             sync.RWMutex
	globalBucket   *TokenBucket
	serviceBuckets map[string]*TokenBucket
	config         *RateLimitConfig
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	rl := &RateLimiter{
		globalBucket:   NewTokenBucket(float64(config.GlobalTPS), config.BurstMultiplier),
		serviceBuckets: make(map[string]*TokenBucket),
		config:         config,
	}

	// Pre-create buckets for known services
	for service, tps := range config.PerServiceLimits {
		rl.serviceBuckets[service] = NewTokenBucket(float64(tps), config.BurstMultiplier)
	}

	return rl
}

// Allow checks if a request from a service is allowed.
func (rl *RateLimiter) Allow(service string) bool {
	// Check global limit first
	if !rl.globalBucket.Allow() {
		return false
	}

	// Check per-service limit
	rl.mu.RLock()
	bucket, ok := rl.serviceBuckets[service]
	rl.mu.RUnlock()

	if !ok {
		// Create bucket for unknown service with default limit
		rl.mu.Lock()
		bucket, ok = rl.serviceBuckets[service]
		if !ok {
			defaultTPS := rl.config.GlobalTPS / 10 // Default to 10% of global
			if defaultTPS < 1 {
				defaultTPS = 1
			}
			bucket = NewTokenBucket(float64(defaultTPS), rl.config.BurstMultiplier)
			rl.serviceBuckets[service] = bucket
		}
		rl.mu.Unlock()
	}

	return bucket.Allow()
}

// Status returns the current rate limit status.
func (rl *RateLimiter) Status() map[string]any {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	status := map[string]any{
		"global_available": rl.globalBucket.Available(),
		"global_tps":       rl.config.GlobalTPS,
	}

	services := make(map[string]float64)
	for service, bucket := range rl.serviceBuckets {
		services[service] = bucket.Available()
	}
	status["services"] = services

	return status
}

// SetServiceLimit updates the rate limit for a service.
func (rl *RateLimiter) SetServiceLimit(service string, tps int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.serviceBuckets[service] = NewTokenBucket(float64(tps), rl.config.BurstMultiplier)
	rl.config.PerServiceLimits[service] = tps
}
