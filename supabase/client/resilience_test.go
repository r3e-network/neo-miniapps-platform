// Package client provides resilience pattern tests.
package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// RetryConfig Tests
// =============================================================================

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.InitialBackoff != 100*time.Millisecond {
		t.Errorf("InitialBackoff = %v, want 100ms", cfg.InitialBackoff)
	}
	if cfg.MaxBackoff != 10*time.Second {
		t.Errorf("MaxBackoff = %v, want 10s", cfg.MaxBackoff)
	}
	if cfg.BackoffMultiplier != 2.0 {
		t.Errorf("BackoffMultiplier = %f, want 2.0", cfg.BackoffMultiplier)
	}
	if len(cfg.RetryableStatusCodes) == 0 {
		t.Error("RetryableStatusCodes should not be empty")
	}
}

// =============================================================================
// CircuitBreaker Tests
// =============================================================================

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()

	if cfg.FailureThreshold != 5 {
		t.Errorf("FailureThreshold = %d, want 5", cfg.FailureThreshold)
	}
	if cfg.SuccessThreshold != 2 {
		t.Errorf("SuccessThreshold = %d, want 2", cfg.SuccessThreshold)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
}

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	if cb == nil {
		t.Fatal("NewCircuitBreaker() returned nil")
	}
	if cb.State() != CircuitClosed {
		t.Errorf("initial State() = %v, want %v", cb.State(), CircuitClosed)
	}
}

func TestCircuitBreaker_Allow_Closed(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	err := cb.Allow()
	if err != nil {
		t.Errorf("Allow() error in closed state: %v", err)
	}
}

func TestCircuitBreaker_RecordSuccess(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	// Record some failures
	for i := 0; i < 3; i++ {
		cb.RecordFailure(errors.New("test error"))
	}

	// Record success should reset failures
	cb.RecordSuccess()

	// Should still be closed
	if cb.State() != CircuitClosed {
		t.Errorf("State() = %v, want %v", cb.State(), CircuitClosed)
	}
}

func TestCircuitBreaker_OpenOnFailures(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}
	cb := NewCircuitBreaker(cfg)

	// Record failures to open circuit
	for i := 0; i < 3; i++ {
		cb.RecordFailure(errors.New("test error"))
	}

	if cb.State() != CircuitOpen {
		t.Errorf("State() = %v, want %v", cb.State(), CircuitOpen)
	}

	// Allow should return error when open
	err := cb.Allow()
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("Allow() error = %v, want ErrCircuitOpen", err)
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
	}
	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure(errors.New("error 1"))
	cb.RecordFailure(errors.New("error 2"))

	if cb.State() != CircuitOpen {
		t.Fatalf("State() = %v, want %v", cb.State(), CircuitOpen)
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Allow should transition to half-open
	err := cb.Allow()
	if err != nil {
		t.Errorf("Allow() error after timeout: %v", err)
	}
	if cb.State() != CircuitHalfOpen {
		t.Errorf("State() = %v, want %v", cb.State(), CircuitHalfOpen)
	}
}

func TestCircuitBreaker_CloseFromHalfOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          10 * time.Millisecond,
	}
	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure(errors.New("error 1"))
	cb.RecordFailure(errors.New("error 2"))

	// Wait for timeout to transition to half-open
	time.Sleep(20 * time.Millisecond)
	cb.Allow()

	// Record successes to close
	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != CircuitClosed {
		t.Errorf("State() = %v, want %v", cb.State(), CircuitClosed)
	}
}

func TestCircuitBreaker_ReopenFromHalfOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          10 * time.Millisecond,
	}
	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure(errors.New("error 1"))
	cb.RecordFailure(errors.New("error 2"))

	// Wait for timeout to transition to half-open
	time.Sleep(20 * time.Millisecond)
	cb.Allow()

	// Record failure in half-open should reopen
	cb.RecordFailure(errors.New("error in half-open"))

	if cb.State() != CircuitOpen {
		t.Errorf("State() = %v, want %v", cb.State(), CircuitOpen)
	}
}

func TestCircuitBreaker_LastError(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	if cb.LastError() != nil {
		t.Error("LastError() should be nil initially")
	}

	testErr := errors.New("test error")
	cb.RecordFailure(testErr)

	if cb.LastError() != testErr {
		t.Errorf("LastError() = %v, want %v", cb.LastError(), testErr)
	}
}

func TestCircuitState_String(t *testing.T) {
	testCases := []struct {
		state CircuitState
		want  string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.state.String(); got != tc.want {
				t.Errorf("String() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestCircuitBreaker_OnStateChange(t *testing.T) {
	var stateChanges []struct{ from, to CircuitState }
	var mu sync.Mutex

	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          10 * time.Millisecond,
		OnStateChange: func(from, to CircuitState) {
			mu.Lock()
			stateChanges = append(stateChanges, struct{ from, to CircuitState }{from, to})
			mu.Unlock()
		},
	}
	cb := NewCircuitBreaker(cfg)

	// Trigger state changes
	cb.RecordFailure(errors.New("error 1"))
	cb.RecordFailure(errors.New("error 2"))

	// Wait for callback
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	if len(stateChanges) == 0 {
		t.Error("OnStateChange should have been called")
	}
	mu.Unlock()
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			cb.Allow()
		}()
		go func() {
			defer wg.Done()
			cb.RecordSuccess()
		}()
		go func() {
			defer wg.Done()
			cb.RecordFailure(errors.New("test"))
		}()
	}

	wg.Wait()
}

// =============================================================================
// ResilientClient Tests
// =============================================================================

func TestNewResilientClient(t *testing.T) {
	client := NewResilientClient(ResilientClientConfig{
		RetryConfig:          DefaultRetryConfig(),
		CircuitBreakerConfig: DefaultCircuitBreakerConfig(),
	})

	if client == nil {
		t.Fatal("NewResilientClient() returned nil")
	}
}

func TestResilientClient_Do_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	client := NewResilientClient(ResilientClientConfig{
		RetryConfig:          DefaultRetryConfig(),
		CircuitBreakerConfig: DefaultCircuitBreakerConfig(),
	})

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	resp.Body.Close()
}

func TestResilientClient_Do_RetryOnServerError(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewResilientClient(ResilientClientConfig{
		RetryConfig: RetryConfig{
			MaxRetries:           3,
			InitialBackoff:       10 * time.Millisecond,
			MaxBackoff:           100 * time.Millisecond,
			BackoffMultiplier:    2.0,
			RetryableStatusCodes: []int{http.StatusServiceUnavailable},
		},
		CircuitBreakerConfig: DefaultCircuitBreakerConfig(),
	})

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if atomic.LoadInt32(&attempts) < 3 {
		t.Errorf("attempts = %d, want >= 3", attempts)
	}
	resp.Body.Close()
}

func TestResilientClient_Do_CircuitOpen(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewResilientClient(ResilientClientConfig{
		RetryConfig: RetryConfig{
			MaxRetries:           0,
			RetryableStatusCodes: []int{http.StatusServiceUnavailable},
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 2,
			SuccessThreshold: 1,
			Timeout:          1 * time.Second,
		},
	})

	// Trigger failures to open circuit
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", server.URL, nil)
		client.Do(req)
	}

	// Next request should fail with circuit open
	req, _ := http.NewRequest("GET", server.URL, nil)
	_, err := client.Do(req)

	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("Do() error = %v, want ErrCircuitOpen", err)
	}
}

func TestResilientClient_Metrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewResilientClient(ResilientClientConfig{
		RetryConfig:          DefaultRetryConfig(),
		CircuitBreakerConfig: DefaultCircuitBreakerConfig(),
	})

	// Make some requests
	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}

	metrics := client.Metrics()
	if metrics["total_requests"] != 5 {
		t.Errorf("total_requests = %d, want 5", metrics["total_requests"])
	}
	if metrics["success_requests"] != 5 {
		t.Errorf("success_requests = %d, want 5", metrics["success_requests"])
	}
}

func TestResilientClient_CircuitState(t *testing.T) {
	client := NewResilientClient(ResilientClientConfig{
		RetryConfig:          DefaultRetryConfig(),
		CircuitBreakerConfig: DefaultCircuitBreakerConfig(),
	})

	if client.CircuitState() != CircuitClosed {
		t.Errorf("CircuitState() = %v, want %v", client.CircuitState(), CircuitClosed)
	}
}

func TestResilientClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewResilientClient(ResilientClientConfig{
		RetryConfig:          DefaultRetryConfig(),
		CircuitBreakerConfig: DefaultCircuitBreakerConfig(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	_, err := client.Do(req)

	if err == nil {
		t.Error("Do() should error on context cancellation")
	}
}

// =============================================================================
// Request ID Tests
// =============================================================================

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "test-request-id")

	id := GetRequestID(ctx)
	if id != "test-request-id" {
		t.Errorf("GetRequestID() = %s, want test-request-id", id)
	}
}

func TestGetRequestID_Empty(t *testing.T) {
	ctx := context.Background()
	id := GetRequestID(ctx)
	if id != "" {
		t.Errorf("GetRequestID() = %s, want empty", id)
	}
}

func TestGenerateRequestID(t *testing.T) {
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	if id1 == "" {
		t.Error("GenerateRequestID() returned empty string")
	}
	if id1 == id2 {
		t.Error("GenerateRequestID() should return unique IDs")
	}
}

// =============================================================================
// HTTPError Tests
// =============================================================================

func TestHTTPError_Error(t *testing.T) {
	err := &HTTPError{StatusCode: http.StatusNotFound}
	if err.Error() != "Not Found" {
		t.Errorf("Error() = %s, want Not Found", err.Error())
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkCircuitBreaker_Allow(b *testing.B) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Allow()
	}
}

func BenchmarkCircuitBreaker_RecordSuccess(b *testing.B) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.RecordSuccess()
	}
}

func BenchmarkResilientClient_Do(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewResilientClient(ResilientClientConfig{
		RetryConfig:          DefaultRetryConfig(),
		CircuitBreakerConfig: DefaultCircuitBreakerConfig(),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}
}
