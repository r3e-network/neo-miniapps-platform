// Package middleware provides HTTP middleware for the service layer
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/errors"
	"github.com/R3E-Network/service_layer/internal/logging"
	"golang.org/x/time/rate"
)

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	logger   *logging.Logger
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerSecond int, burst int, logger *logging.Logger) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
		logger:   logger,
	}
}

// getLimiter returns a rate limiter for the given key (e.g., user ID or IP)
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[key] = limiter
	}

	return limiter
}

// Handler returns the rate limiting middleware handler
func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use user ID if authenticated, otherwise use IP address
		key := GetUserID(r.Context())
		if key == "" {
			key = r.RemoteAddr
		}

		limiter := rl.getLimiter(key)

		if !limiter.Allow() {
			rl.logger.LogSecurityEvent(r.Context(), "rate_limit_exceeded", map[string]interface{}{
				"key":    key,
				"path":   r.URL.Path,
				"method": r.Method,
			})

			serviceErr := errors.RateLimitExceeded(int(rl.rate), "1s")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(serviceErr.HTTPStatus)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Cleanup removes old limiters (should be called periodically)
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Remove limiters that haven't been used recently
	// This is a simple implementation; in production, you might want
	// to track last access time and remove based on that
	if len(rl.limiters) > 10000 {
		rl.limiters = make(map[string]*rate.Limiter)
	}
}

// StartCleanup starts a background goroutine to periodically cleanup old limiters
func (rl *RateLimiter) StartCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			rl.Cleanup()
		}
	}()
}
