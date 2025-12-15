// Package middleware provides HTTP middleware for the service layer
package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/R3E-Network/service_layer/internal/errors"
	internalhttputil "github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/internal/logging"
)

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	limit    int
	window   time.Duration
	logger   *logging.Logger
}

// LimiterCount returns the number of active limiters.
func (rl *RateLimiter) LimiterCount() int {
	if rl == nil {
		return 0
	}
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return len(rl.limiters)
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerSecond, burst int, logger *logging.Logger) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
		limit:    requestsPerSecond,
		window:   time.Second,
		logger:   logger,
	}
}

// NewRateLimiterWithWindow creates a rate limiter configured by a fixed window
// and request budget, e.g. 100 requests per 1 minute.
func NewRateLimiterWithWindow(limit int, window time.Duration, burst int, logger *logging.Logger) *RateLimiter {
	if window <= 0 {
		window = time.Second
	}
	requestsPerSecond := float64(limit) / window.Seconds()
	if requestsPerSecond < 0 {
		requestsPerSecond = 0
	}

	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
		limit:    limit,
		window:   window,
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
			key = internalhttputil.ClientIP(r)
		}
		if key == "" {
			key = "unknown"
		}

		limiter := rl.getLimiter(key)

		if !limiter.Allow() {
			if rl.logger != nil {
				rl.logger.LogSecurityEvent(r.Context(), "rate_limit_exceeded", map[string]interface{}{
					"key":    key,
					"path":   r.URL.Path,
					"method": r.Method,
				})
			}

			window := rl.window
			if window <= 0 {
				window = time.Second
			}
			serviceErr := errors.RateLimitExceeded(rl.limit, window.String())
			if seconds := int(math.Ceil(window.Seconds())); seconds > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
			}
			internalhttputil.WriteErrorResponse(w, r, serviceErr.HTTPStatus, string(serviceErr.Code), serviceErr.Message, serviceErr.Details)
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
func (rl *RateLimiter) StartCleanup(interval time.Duration) (stop func()) {
	if interval <= 0 {
		interval = time.Minute
	}

	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	var once sync.Once

	go func() {
		for {
			select {
			case <-ticker.C:
				rl.Cleanup()
			case <-done:
				return
			}
		}
	}()

	return func() {
		once.Do(func() {
			ticker.Stop()
			close(done)
		})
	}
}
