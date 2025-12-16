// Package smoke provides smoke tests for quick verification of service health.
// Smoke tests are designed to quickly verify that services can start and respond
// to basic health checks without requiring external dependencies.
package smoke

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	neoaccounts "github.com/R3E-Network/service_layer/infrastructure/accountpool/marble"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
	neocompute "github.com/R3E-Network/service_layer/services/confcompute/marble"
)

// TestNeoAccountsSmoke performs basic smoke tests on the NeoAccounts service.
func TestNeoAccountsSmoke(t *testing.T) {
	t.Run("service creates successfully", func(t *testing.T) {
		m, err := marble.New(marble.Config{MarbleType: "neoaccounts"})
		if err != nil {
			t.Fatalf("marble.New: %v", err)
		}
		m.SetTestSecret("POOL_MASTER_KEY", []byte("smoke-test-pool-key-32-bytes!!!!"))

		svc, err := neoaccounts.New(neoaccounts.Config{Marble: m})
		if err != nil {
			t.Fatalf("neoaccounts.New: %v", err)
		}
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})

	t.Run("health endpoint responds", func(t *testing.T) {
		m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
		m.SetTestSecret("POOL_MASTER_KEY", []byte("smoke-test-pool-key-32-bytes!!!!"))
		svc, _ := neoaccounts.New(neoaccounts.Config{Marble: m})

		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		svc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("health check failed: status %d", w.Code)
		}
	})

	t.Run("info endpoint responds", func(t *testing.T) {
		t.Skip("requires database connection")
	})

	t.Run("service metadata correct", func(t *testing.T) {
		m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
		m.SetTestSecret("POOL_MASTER_KEY", []byte("smoke-test-pool-key-32-bytes!!!!"))
		svc, _ := neoaccounts.New(neoaccounts.Config{Marble: m})

		if svc.ID() != "neoaccounts" {
			t.Errorf("expected ID 'neoaccounts', got '%s'", svc.ID())
		}
		if svc.Name() != "Account Pool Service" {
			t.Errorf("expected name 'Account Pool Service', got '%s'", svc.Name())
		}
	})
}

// TestNeoComputeSmoke performs basic smoke tests on the NeoCompute service.
func TestNeoComputeSmoke(t *testing.T) {
	t.Run("service creates successfully", func(t *testing.T) {
		m, err := marble.New(marble.Config{MarbleType: "neocompute"})
		if err != nil {
			t.Fatalf("marble.New: %v", err)
		}
		m.SetTestSecret("COMPUTE_MASTER_KEY", []byte("smoke-test-compute-master-key-32b!!"))

		svc, err := neocompute.New(neocompute.Config{Marble: m})
		if err != nil {
			t.Fatalf("neocompute.New: %v", err)
		}
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})

	t.Run("health endpoint responds", func(t *testing.T) {
		m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
		m.SetTestSecret("COMPUTE_MASTER_KEY", []byte("smoke-test-compute-master-key-32b!!"))
		svc, _ := neocompute.New(neocompute.Config{Marble: m})

		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		svc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("health check failed: status %d", w.Code)
		}
	})

	t.Run("service metadata correct", func(t *testing.T) {
		m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
		m.SetTestSecret("COMPUTE_MASTER_KEY", []byte("smoke-test-compute-master-key-32b!!"))
		svc, _ := neocompute.New(neocompute.Config{Marble: m})

		if svc.ID() != "neocompute" {
			t.Errorf("expected ID 'neocompute', got '%s'", svc.ID())
		}
		if svc.Name() != "NeoCompute Service" {
			t.Errorf("expected name 'NeoCompute Service', got '%s'", svc.Name())
		}
	})
}

// TestMarbleSmoke performs basic smoke tests on the Marble framework.
func TestMarbleSmoke(t *testing.T) {
	t.Run("marble creates successfully", func(t *testing.T) {
		m, err := marble.New(marble.Config{MarbleType: "test"})
		if err != nil {
			t.Fatalf("marble.New: %v", err)
		}
		if m == nil {
			t.Fatal("marble should not be nil")
		}
	})

	t.Run("marble type correct", func(t *testing.T) {
		m, _ := marble.New(marble.Config{MarbleType: "test-type"})
		if m.MarbleType() != "test-type" {
			t.Errorf("expected type 'test-type', got '%s'", m.MarbleType())
		}
	})

	t.Run("secrets can be set and retrieved", func(t *testing.T) {
		m, _ := marble.New(marble.Config{MarbleType: "test"})
		m.SetTestSecret("TEST_KEY", []byte("test-value"))

		val, ok := m.Secret("TEST_KEY")
		if !ok {
			t.Error("secret should be retrievable")
		}
		if string(val) != "test-value" {
			t.Errorf("expected 'test-value', got '%s'", string(val))
		}
	})

	t.Run("missing secret returns false", func(t *testing.T) {
		m, _ := marble.New(marble.Config{MarbleType: "test"})
		_, ok := m.Secret("NONEXISTENT")
		if ok {
			t.Error("missing secret should return false")
		}
	})
}

// TestServiceFrameworkSmoke tests the service framework.
func TestServiceFrameworkSmoke(t *testing.T) {
	t.Run("service lifecycle", func(t *testing.T) {
		m, _ := marble.New(marble.Config{MarbleType: "test"})
		svc := marble.NewService(marble.ServiceConfig{
			ID:      "smoke-test",
			Name:    "Smoke Test Service",
			Version: "1.0.0",
			Marble:  m,
			DB:      nil,
		})

		if svc.ID() != "smoke-test" {
			t.Errorf("expected ID 'smoke-test', got '%s'", svc.ID())
		}

		ctx := context.Background()
		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Start: %v", err)
		}

		if !svc.IsRunning() {
			t.Error("service should be running after Start")
		}

		if err := svc.Stop(); err != nil {
			t.Fatalf("Stop: %v", err)
		}

		if svc.IsRunning() {
			t.Error("service should not be running after Stop")
		}
	})

	t.Run("router available", func(t *testing.T) {
		m, _ := marble.New(marble.Config{MarbleType: "test"})
		svc := marble.NewService(marble.ServiceConfig{
			ID:      "smoke-test",
			Name:    "Smoke Test Service",
			Version: "1.0.0",
			Marble:  m,
			DB:      nil,
		})

		router := svc.Router()
		if router == nil {
			t.Error("router should not be nil")
		}
	})
}

// TestConcurrencySmoke tests basic concurrent access.
func TestConcurrencySmoke(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("smoke-test-pool-key-32-bytes!!!!"))
	svc, _ := neoaccounts.New(neoaccounts.Config{Marble: m})

	done := make(chan bool, 20)

	for i := 0; i < 20; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()
			svc.Router().ServeHTTP(w, req)
			done <- (w.Code == http.StatusOK)
		}()
	}

	timeout := time.After(5 * time.Second)
	success := 0
	for i := 0; i < 20; i++ {
		select {
		case ok := <-done:
			if ok {
				success++
			}
		case <-timeout:
			t.Fatal("concurrent requests timed out")
		}
	}

	if success != 20 {
		t.Errorf("expected 20 successful requests, got %d", success)
	}
}

// TestEndpointResponsivenessSmoke tests that endpoints respond within expected time.
func TestEndpointResponsivenessSmoke(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("smoke-test-pool-key-32-bytes!!!!"))
	svc, _ := neoaccounts.New(neoaccounts.Config{Marble: m})

	maxDuration := 100 * time.Millisecond

	t.Run("health endpoint responsive", func(t *testing.T) {
		start := time.Now()
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		svc.Router().ServeHTTP(w, req)
		duration := time.Since(start)

		if duration > maxDuration {
			t.Errorf("health endpoint too slow: %v > %v", duration, maxDuration)
		}
	})
}
