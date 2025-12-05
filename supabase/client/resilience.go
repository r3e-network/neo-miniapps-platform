// Package client provides resilience patterns for Supabase client.
// This file implements connection pooling, retry logic, and circuit breaker.
package client

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// Retry Configuration
// =============================================================================

// RetryConfig configures retry behavior.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int
	// InitialBackoff is the initial backoff duration
	InitialBackoff time.Duration
	// MaxBackoff is the maximum backoff duration
	MaxBackoff time.Duration
	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64
	// Jitter adds randomness to backoff (0.0 to 1.0)
	Jitter float64
	// RetryableStatusCodes are HTTP status codes that should be retried
	RetryableStatusCodes []int
}

// DefaultRetryConfig returns sensible defaults for retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
		RetryableStatusCodes: []int{
			http.StatusTooManyRequests,     // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout,      // 504
		},
	}
}

// =============================================================================
// Circuit Breaker
// =============================================================================

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures circuit breaker behavior.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures before opening the circuit
	FailureThreshold int
	// SuccessThreshold is the number of successes in half-open state to close
	SuccessThreshold int
	// Timeout is how long the circuit stays open before transitioning to half-open
	Timeout time.Duration
	// OnStateChange is called when the circuit state changes
	OnStateChange func(from, to CircuitState)
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	mu sync.RWMutex

	config CircuitBreakerConfig
	state  CircuitState

	failures  int
	successes int
	lastError error
	openedAt  time.Time
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  CircuitClosed,
	}
}

// ErrCircuitOpen is returned when the circuit is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// Allow checks if a request should be allowed.
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return nil
	case CircuitOpen:
		// Check if timeout has passed
		if time.Since(cb.openedAt) > cb.config.Timeout {
			cb.transitionTo(CircuitHalfOpen)
			return nil
		}
		return ErrCircuitOpen
	case CircuitHalfOpen:
		return nil
	}
	return nil
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		cb.failures = 0
	case CircuitHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.transitionTo(CircuitClosed)
		}
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastError = err

	switch cb.state {
	case CircuitClosed:
		cb.failures++
		if cb.failures >= cb.config.FailureThreshold {
			cb.transitionTo(CircuitOpen)
		}
	case CircuitHalfOpen:
		cb.transitionTo(CircuitOpen)
	}
}

func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	oldState := cb.state
	cb.state = newState

	switch newState {
	case CircuitClosed:
		cb.failures = 0
		cb.successes = 0
	case CircuitOpen:
		cb.openedAt = time.Now()
		cb.successes = 0
	case CircuitHalfOpen:
		cb.successes = 0
	}

	if cb.config.OnStateChange != nil {
		go cb.config.OnStateChange(oldState, newState)
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// LastError returns the last recorded error.
func (cb *CircuitBreaker) LastError() error {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.lastError
}

// =============================================================================
// Resilient HTTP Client
// =============================================================================

// ResilientClient wraps an HTTP client with retry and circuit breaker.
type ResilientClient struct {
	client         *http.Client
	retryConfig    RetryConfig
	circuitBreaker *CircuitBreaker

	// Metrics
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	retriedRequests int64
}

// ResilientClientConfig configures the resilient client.
type ResilientClientConfig struct {
	// BaseClient is the underlying HTTP client
	BaseClient *http.Client
	// RetryConfig configures retry behavior
	RetryConfig RetryConfig
	// CircuitBreakerConfig configures circuit breaker
	CircuitBreakerConfig CircuitBreakerConfig
}

// NewResilientClient creates a new resilient HTTP client.
func NewResilientClient(config ResilientClientConfig) *ResilientClient {
	if config.BaseClient == nil {
		config.BaseClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		}
	}

	return &ResilientClient{
		client:         config.BaseClient,
		retryConfig:    config.RetryConfig,
		circuitBreaker: NewCircuitBreaker(config.CircuitBreakerConfig),
	}
}

// Do executes an HTTP request with retry and circuit breaker.
func (rc *ResilientClient) Do(req *http.Request) (*http.Response, error) {
	atomic.AddInt64(&rc.totalRequests, 1)

	// Check circuit breaker
	if err := rc.circuitBreaker.Allow(); err != nil {
		atomic.AddInt64(&rc.failedRequests, 1)
		return nil, err
	}

	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt <= rc.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			atomic.AddInt64(&rc.retriedRequests, 1)

			// Calculate backoff
			backoff := rc.calculateBackoff(attempt)

			// Wait with context cancellation support
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(backoff):
			}

			// Clone request for retry (body needs to be re-readable)
			req = req.Clone(req.Context())
		}

		resp, lastErr = rc.client.Do(req)

		if lastErr != nil {
			// Network error - might be retryable
			if rc.isRetryableError(lastErr) {
				continue
			}
			rc.circuitBreaker.RecordFailure(lastErr)
			atomic.AddInt64(&rc.failedRequests, 1)
			return nil, lastErr
		}

		// Check if status code is retryable
		if rc.isRetryableStatusCode(resp.StatusCode) {
			lastErr = &HTTPError{StatusCode: resp.StatusCode}
			resp.Body.Close()
			continue
		}

		// Success
		rc.circuitBreaker.RecordSuccess()
		atomic.AddInt64(&rc.successRequests, 1)
		return resp, nil
	}

	// All retries exhausted
	rc.circuitBreaker.RecordFailure(lastErr)
	atomic.AddInt64(&rc.failedRequests, 1)
	return resp, lastErr
}

func (rc *ResilientClient) calculateBackoff(attempt int) time.Duration {
	backoff := float64(rc.retryConfig.InitialBackoff) * math.Pow(rc.retryConfig.BackoffMultiplier, float64(attempt-1))

	// Apply max backoff
	if backoff > float64(rc.retryConfig.MaxBackoff) {
		backoff = float64(rc.retryConfig.MaxBackoff)
	}

	// Apply jitter
	if rc.retryConfig.Jitter > 0 {
		jitter := backoff * rc.retryConfig.Jitter * (rand.Float64()*2 - 1)
		backoff += jitter
	}

	return time.Duration(backoff)
}

func (rc *ResilientClient) isRetryableError(err error) bool {
	// Network errors are generally retryable
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	// Context errors are not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	return false
}

func (rc *ResilientClient) isRetryableStatusCode(code int) bool {
	for _, retryable := range rc.retryConfig.RetryableStatusCodes {
		if code == retryable {
			return true
		}
	}
	return false
}

// HTTPError represents an HTTP error.
type HTTPError struct {
	StatusCode int
}

func (e *HTTPError) Error() string {
	return http.StatusText(e.StatusCode)
}

// Metrics returns client metrics.
func (rc *ResilientClient) Metrics() map[string]int64 {
	return map[string]int64{
		"total_requests":   atomic.LoadInt64(&rc.totalRequests),
		"success_requests": atomic.LoadInt64(&rc.successRequests),
		"failed_requests":  atomic.LoadInt64(&rc.failedRequests),
		"retried_requests": atomic.LoadInt64(&rc.retriedRequests),
	}
}

// CircuitState returns the current circuit breaker state.
func (rc *ResilientClient) CircuitState() CircuitState {
	return rc.circuitBreaker.State()
}

// =============================================================================
// Enhanced Client with Resilience
// =============================================================================

// EnhancedConfig extends Config with resilience options.
type EnhancedConfig struct {
	Config
	RetryConfig          RetryConfig
	CircuitBreakerConfig CircuitBreakerConfig
	EnableResilience     bool
}

// NewEnhanced creates a new Supabase client with resilience features.
func NewEnhanced(cfg EnhancedConfig) (*Client, error) {
	if cfg.URL == "" {
		return nil, errors.New("URL is required")
	}
	if cfg.APIKey == "" {
		return nil, errors.New("APIKey is required")
	}

	var httpClient *http.Client

	if cfg.EnableResilience {
		resilientClient := NewResilientClient(ResilientClientConfig{
			BaseClient:           cfg.HTTPClient,
			RetryConfig:          cfg.RetryConfig,
			CircuitBreakerConfig: cfg.CircuitBreakerConfig,
		})

		// Wrap resilient client in http.Client interface
		httpClient = &http.Client{
			Transport: &resilientTransport{client: resilientClient},
			Timeout:   30 * time.Second,
		}
	} else {
		httpClient = cfg.HTTPClient
		if httpClient == nil {
			httpClient = &http.Client{
				Timeout: 30 * time.Second,
				Transport: &http.Transport{
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 10,
					IdleConnTimeout:     90 * time.Second,
				},
			}
		}
	}

	return &Client{
		baseURL:    cfg.URL,
		apiKey:     cfg.APIKey,
		httpClient: httpClient,
	}, nil
}

// resilientTransport wraps ResilientClient as http.RoundTripper.
type resilientTransport struct {
	client *ResilientClient
}

func (rt *resilientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt.client.Do(req)
}

// =============================================================================
// Request ID and Tracing
// =============================================================================

// RequestIDKey is the context key for request ID.
type requestIDKey struct{}

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// GenerateRequestID generates a unique request ID.
func GenerateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
