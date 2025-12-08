// Package marble provides the core Marble SDK for MarbleRun integration.
package marble

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// =============================================================================
// Marble Tests
// =============================================================================

func TestNewMarble(t *testing.T) {
	m, err := New(Config{
		MarbleType: "test-marble",
		DNSNames:   []string{"localhost"},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if m.MarbleType() != "test-marble" {
		t.Errorf("MarbleType() = %s, want test-marble", m.MarbleType())
	}
}

func TestMarbleType(t *testing.T) {
	tests := []struct {
		name       string
		marbleType string
	}{
		{"vrf", "vrf"},
		{"mixer", "mixer"},
		{"datafeeds", "datafeeds"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := New(Config{MarbleType: tt.marbleType})
			if m.MarbleType() != tt.marbleType {
				t.Errorf("MarbleType() = %s, want %s", m.MarbleType(), tt.marbleType)
			}
		})
	}
}

func TestMarbleIsEnclave(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})

	// Outside enclave, report should be nil
	// This test runs outside SGX, so IsEnclave should return false
	if m.IsEnclave() {
		t.Log("Running inside enclave (unexpected in test environment)")
	} else {
		t.Log("Running outside enclave (expected in test environment)")
	}
}

func TestMarbleSecret(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})

	// Manually inject a secret for testing
	m.secrets["test-secret"] = []byte("secret-value")

	secret, ok := m.Secret("test-secret")
	if !ok {
		t.Error("Secret() should return true for existing secret")
	}
	if string(secret) != "secret-value" {
		t.Errorf("Secret() = %s, want secret-value", string(secret))
	}

	_, ok = m.Secret("nonexistent")
	if ok {
		t.Error("Secret() should return false for nonexistent secret")
	}
}

func TestMarbleUseSecret(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})
	m.secrets["test-secret"] = []byte("secret-value")

	var capturedSecret string
	err := m.UseSecret("test-secret", func(secret []byte) error {
		capturedSecret = string(secret)
		return nil
	})

	if err != nil {
		t.Errorf("UseSecret() error = %v", err)
	}
	if capturedSecret != "secret-value" {
		t.Errorf("UseSecret() captured = %s, want secret-value", capturedSecret)
	}
}

func TestMarbleUseSecretNotFound(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})

	err := m.UseSecret("nonexistent", func(secret []byte) error {
		return nil
	})

	if err == nil {
		t.Error("UseSecret() should return error for nonexistent secret")
	}
}

func TestMarbleInitialize(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})

	// Set environment variables for testing
	os.Setenv("MARBLE_UUID", "test-uuid-123")
	defer os.Unsetenv("MARBLE_UUID")

	ctx := context.Background()
	err := m.Initialize(ctx)
	if err != nil {
		t.Errorf("Initialize() error = %v", err)
	}

	if m.UUID() != "test-uuid-123" {
		t.Errorf("UUID() = %s, want test-uuid-123", m.UUID())
	}
}

func TestMarbleInitializeIdempotent(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})

	ctx := context.Background()
	_ = m.Initialize(ctx)
	err := m.Initialize(ctx)

	if err != nil {
		t.Errorf("Initialize() should be idempotent, got error = %v", err)
	}
}

func TestMarbleHTTPClient(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})

	client := m.HTTPClient()
	if client == nil {
		t.Error("HTTPClient() should not return nil")
	}
}

func TestMarbleTLSConfig(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})

	// Before initialization, TLS config may be nil
	tlsConfig := m.TLSConfig()
	// This is expected to be nil without proper initialization
	_ = tlsConfig
}

// =============================================================================
// Service Tests
// =============================================================================

func TestNewService(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})

	svc := NewService(ServiceConfig{
		ID:      "test-service",
		Name:    "Test Service",
		Version: "1.0.0",
		Marble:  m,
		DB:      nil,
	})

	if svc.ID() != "test-service" {
		t.Errorf("ID() = %s, want test-service", svc.ID())
	}
	if svc.Name() != "Test Service" {
		t.Errorf("Name() = %s, want Test Service", svc.Name())
	}
	if svc.Version() != "1.0.0" {
		t.Errorf("Version() = %s, want 1.0.0", svc.Version())
	}
}

func TestServiceStartStop(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})
	svc := NewService(ServiceConfig{
		ID:      "test-service",
		Name:    "Test Service",
		Version: "1.0.0",
		Marble:  m,
	})

	ctx := context.Background()

	// Initially not running
	if svc.IsRunning() {
		t.Error("Service should not be running initially")
	}

	// Start service
	if err := svc.Start(ctx); err != nil {
		t.Errorf("Start() error = %v", err)
	}

	if !svc.IsRunning() {
		t.Error("Service should be running after Start()")
	}

	// Stop service
	if err := svc.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	if svc.IsRunning() {
		t.Error("Service should not be running after Stop()")
	}
}

func TestServiceStartTwice(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})
	svc := NewService(ServiceConfig{
		ID:     "test-service",
		Name:   "Test Service",
		Marble: m,
	})

	ctx := context.Background()
	_ = svc.Start(ctx)

	err := svc.Start(ctx)
	if err == nil {
		t.Error("Start() should return error when already running")
	}
}

func TestServiceStopTwice(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})
	svc := NewService(ServiceConfig{
		ID:     "test-service",
		Name:   "Test Service",
		Marble: m,
	})

	ctx := context.Background()
	_ = svc.Start(ctx)
	_ = svc.Stop()

	// Second stop should not error
	err := svc.Stop()
	if err != nil {
		t.Errorf("Stop() should not error when already stopped, got %v", err)
	}
}

func TestServiceRouter(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})
	svc := NewService(ServiceConfig{
		ID:     "test-service",
		Name:   "Test Service",
		Marble: m,
	})

	router := svc.Router()
	if router == nil {
		t.Error("Router() should not return nil")
	}
}

func TestServiceMarble(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})
	svc := NewService(ServiceConfig{
		ID:     "test-service",
		Name:   "Test Service",
		Marble: m,
	})

	if svc.Marble() != m {
		t.Error("Marble() should return the configured marble")
	}
}

// =============================================================================
// Middleware Tests
// =============================================================================

func TestAuthMiddleware(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})
	middleware := AuthMiddleware(m)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"valid bearer token", "Bearer valid-token", http.StatusOK},
		{"missing header", "", http.StatusUnauthorized},
		{"invalid format", "Basic token", http.StatusUnauthorized},
		{"short header", "Bear", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestRecoveryMiddlewareNoPanic(t *testing.T) {
	handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// =============================================================================
// Health Handler Tests
// =============================================================================

func TestHealthHandler(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})
	svc := NewService(ServiceConfig{
		ID:      "test-service",
		Name:    "Test Service",
		Version: "1.0.0",
		Marble:  m,
	})

	handler := HealthHandler(svc)

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("Status = %s, want healthy", resp.Status)
	}
	if resp.Service != "Test Service" {
		t.Errorf("Service = %s, want Test Service", resp.Service)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("Version = %s, want 1.0.0", resp.Version)
	}
}

// =============================================================================
// Request/Response Tests
// =============================================================================

func TestRequestJSON(t *testing.T) {
	req := Request{
		ID:      "req-123",
		UserID:  "user-456",
		Service: "vrf",
		Method:  "getPrice",
		Payload: json.RawMessage(`{"pair":"BTC/USD"}`),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != req.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, req.ID)
	}
	if decoded.UserID != req.UserID {
		t.Errorf("UserID = %s, want %s", decoded.UserID, req.UserID)
	}
}

func TestResponseJSON(t *testing.T) {
	resp := Response{
		ID:        "resp-123",
		RequestID: "req-456",
		Success:   true,
		Result:    json.RawMessage(`{"price":50000}`),
		GasUsed:   1000,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Success != resp.Success {
		t.Errorf("Success = %v, want %v", decoded.Success, resp.Success)
	}
	if decoded.GasUsed != resp.GasUsed {
		t.Errorf("GasUsed = %d, want %d", decoded.GasUsed, resp.GasUsed)
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestServiceConcurrentAccess(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})
	svc := NewService(ServiceConfig{
		ID:     "test-service",
		Name:   "Test Service",
		Marble: m,
	})

	ctx := context.Background()
	_ = svc.Start(ctx)

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			_ = svc.IsRunning()
			_ = svc.ID()
			_ = svc.Name()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	_ = svc.Stop()
}

func TestMarbleConcurrentSecretAccess(t *testing.T) {
	m, _ := New(Config{MarbleType: "test"})
	m.secrets["test-secret"] = []byte("secret-value")

	done := make(chan bool)

	// Concurrent secret reads
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = m.Secret("test-secret")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkNewMarble(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = New(Config{MarbleType: "benchmark"})
	}
}

func BenchmarkNewService(b *testing.B) {
	m, _ := New(Config{MarbleType: "benchmark"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewService(ServiceConfig{
			ID:     "benchmark-service",
			Name:   "Benchmark Service",
			Marble: m,
		})
	}
}

func BenchmarkServiceStartStop(b *testing.B) {
	m, _ := New(Config{MarbleType: "benchmark"})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc := NewService(ServiceConfig{
			ID:     "benchmark-service",
			Name:   "Benchmark Service",
			Marble: m,
		})
		_ = svc.Start(ctx)
		_ = svc.Stop()
	}
}

func BenchmarkHealthHandler(b *testing.B) {
	m, _ := New(Config{MarbleType: "benchmark"})
	svc := NewService(ServiceConfig{
		ID:      "benchmark-service",
		Name:    "Benchmark Service",
		Version: "1.0.0",
		Marble:  m,
	})
	handler := HealthHandler(svc)

	req := httptest.NewRequest("GET", "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkAuthMiddleware(b *testing.B) {
	m, _ := New(Config{MarbleType: "benchmark"})
	middleware := AuthMiddleware(m)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}
