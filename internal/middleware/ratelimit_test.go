package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/R3E-Network/service_layer/internal/logging"
)

func TestNewRateLimiter(t *testing.T) {
	logger := logging.New("test", "info", "json")
	rl := NewRateLimiter(10, 20, logger)

	if rl == nil {
		t.Fatal("NewRateLimiter() returned nil")
	}

	if rl.rate != rate.Limit(10) {
		t.Errorf("rate = %v, want %v", rl.rate, rate.Limit(10))
	}

	if rl.burst != 20 {
		t.Errorf("burst = %d, want 20", rl.burst)
	}

	if rl.logger != logger {
		t.Error("logger not set correctly")
	}

	if rl.limiters == nil {
		t.Error("limiters map not initialized")
	}
}

func TestRateLimiter_getLimiter(t *testing.T) {
	logger := logging.New("test", "info", "json")
	rl := NewRateLimiter(10, 20, logger)

	// Get limiter for first time
	limiter1 := rl.getLimiter("key1")
	if limiter1 == nil {
		t.Fatal("getLimiter() returned nil")
	}

	// Get same limiter again
	limiter2 := rl.getLimiter("key1")
	if limiter1 != limiter2 {
		t.Error("getLimiter() returned different limiter for same key")
	}

	// Get limiter for different key
	limiter3 := rl.getLimiter("key2")
	if limiter1 == limiter3 {
		t.Error("getLimiter() returned same limiter for different keys")
	}

	// Check limiters map size
	if len(rl.limiters) != 2 {
		t.Errorf("limiters map size = %d, want 2", len(rl.limiters))
	}
}

func TestRateLimiter_Handler_AllowsRequests(t *testing.T) {
	logger := logging.New("test", "info", "json")
	rl := NewRateLimiter(100, 100, logger) // High limit

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRateLimiter_Handler_BlocksExcessiveRequests(t *testing.T) {
	logger := logging.New("test", "info", "json")
	rl := NewRateLimiter(1, 1, logger) // Very low limit

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should succeed
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("First request status = %d, want %d", rec1.Code, http.StatusOK)
	}

	// Second immediate request should be rate limited
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request status = %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiter_Handler_UsesUserID(t *testing.T) {
	logger := logging.New("test", "info", "json")
	rl := NewRateLimiter(1, 1, logger)

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with user ID
	ctx := logging.WithUserID(context.Background(), "user-123")
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	req1 = req1.WithContext(ctx)
	req1.RemoteAddr = "192.168.1.1:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("First request status = %d, want %d", rec1.Code, http.StatusOK)
	}

	// Second request with same user ID should be rate limited
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2 = req2.WithContext(ctx)
	req2.RemoteAddr = "192.168.1.1:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request status = %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiter_Handler_DifferentIPsIndependent(t *testing.T) {
	logger := logging.New("test", "info", "json")
	rl := NewRateLimiter(1, 1, logger)

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request from IP 1
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("IP1 first request status = %d, want %d", rec1.Code, http.StatusOK)
	}

	// Request from IP 2 should still succeed
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.RemoteAddr = "192.168.1.2:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("IP2 first request status = %d, want %d", rec2.Code, http.StatusOK)
	}
}

func TestRateLimiter_Handler_BurstAllowance(t *testing.T) {
	logger := logging.New("test", "info", "json")
	rl := NewRateLimiter(1, 3, logger) // Allow burst of 3

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 3 requests should succeed (burst)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Request %d status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	// 4th request should be rate limited
	req4 := httptest.NewRequest("GET", "/api/test", nil)
	req4.RemoteAddr = "192.168.1.1:12345"
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4)

	if rec4.Code != http.StatusTooManyRequests {
		t.Errorf("4th request status = %d, want %d", rec4.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	logger := logging.New("test", "info", "json")
	rl := NewRateLimiter(10, 20, logger)

	// Add many limiters
	for i := 0; i < 15000; i++ {
		rl.getLimiter(string(rune(i)))
	}

	initialSize := len(rl.limiters)
	if initialSize < 10000 {
		t.Errorf("Initial size = %d, expected > 10000", initialSize)
	}

	// Cleanup should reset if size > 10000
	rl.Cleanup()

	finalSize := rl.LimiterCount()
	if finalSize != 0 {
		t.Errorf("Final size = %d, want 0", finalSize)
	}
}

func TestRateLimiter_Cleanup_NoResetIfSmall(t *testing.T) {
	logger := logging.New("test", "info", "json")
	rl := NewRateLimiter(10, 20, logger)

	// Add few limiters
	for i := 0; i < 100; i++ {
		rl.getLimiter(string(rune(i)))
	}

	initialSize := rl.LimiterCount()

	// Cleanup should not reset if size <= 10000
	rl.Cleanup()

	finalSize := rl.LimiterCount()
	if finalSize != initialSize {
		t.Errorf("Size changed from %d to %d, should remain unchanged", initialSize, finalSize)
	}
}

func TestRateLimiter_StartCleanup(t *testing.T) {
	logger := logging.New("test", "info", "json")
	rl := NewRateLimiter(10, 20, logger)

	// Add many limiters
	for i := 0; i < 15000; i++ {
		rl.getLimiter(string(rune(i)))
	}

	// Start cleanup with very short interval
	stop := rl.StartCleanup(10 * time.Millisecond)
	t.Cleanup(stop)

	// Wait for cleanup to run
	time.Sleep(50 * time.Millisecond)

	// Limiters should be cleaned up
	finalSize := rl.LimiterCount()
	if finalSize > 10000 {
		t.Errorf("Final size = %d, expected cleanup to have run", finalSize)
	}
}

func TestRateLimiter_Handler_ContentType(t *testing.T) {
	logger := logging.New("test", "info", "json")
	rl := NewRateLimiter(1, 1, logger)

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request to exhaust limit
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	// Second request should be rate limited with JSON content type
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	contentType := rec2.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %v, want application/json", contentType)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	logger := logging.New("test", "info", "json")
	rl := NewRateLimiter(100, 100, logger)

	// Test concurrent access to getLimiter
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				rl.getLimiter(string(rune(id)))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have 10 limiters
	if rl.LimiterCount() != 10 {
		t.Errorf("limiters size = %d, want 10", rl.LimiterCount())
	}
}

func TestRateLimiter_Handler_PreservesContext(t *testing.T) {
	logger := logging.New("test", "info", "json")
	rl := NewRateLimiter(100, 100, logger)

	var capturedTraceID string
	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTraceID = logging.GetTraceID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	ctx := logging.WithTraceID(context.Background(), "trace-789")
	req := httptest.NewRequest("GET", "/api/test", nil)
	req = req.WithContext(ctx)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedTraceID != "trace-789" {
		t.Errorf("Trace ID = %v, want trace-789", capturedTraceID)
	}
}
