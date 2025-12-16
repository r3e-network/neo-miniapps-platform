// Package marble provides the core Marble SDK for MarbleRun integration.
package marble

import (
	"context"
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
		{"neofeeds", "neofeeds"},
		{"neocompute", "neocompute"},
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
