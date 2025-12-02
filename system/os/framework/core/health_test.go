package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewHealthCheck(t *testing.T) {
	hc := NewHealthCheck("test-service")

	if hc.Status != HealthStatusHealthy {
		t.Errorf("expected healthy status, got %s", hc.Status)
	}
	if hc.Service != "test-service" {
		t.Errorf("expected service name 'test-service', got %s", hc.Service)
	}
	if hc.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestHealthCheck_WithStatus(t *testing.T) {
	hc := NewHealthCheck("test").WithStatus(HealthStatusDegraded)
	if hc.Status != HealthStatusDegraded {
		t.Errorf("expected degraded status, got %s", hc.Status)
	}
}

func TestHealthCheck_WithLatency(t *testing.T) {
	hc := NewHealthCheck("test").WithLatency(50 * time.Millisecond)
	if hc.Latency != 50*time.Millisecond {
		t.Errorf("expected 50ms latency, got %v", hc.Latency)
	}
}

func TestHealthCheck_WithDetail(t *testing.T) {
	hc := NewHealthCheck("test").
		WithDetail("version", "1.0.0").
		WithDetail("uptime", "24h")

	if hc.Details["version"] != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", hc.Details["version"])
	}
	if hc.Details["uptime"] != "24h" {
		t.Errorf("expected uptime '24h', got %s", hc.Details["uptime"])
	}
}

func TestHealthCheck_WithComponent(t *testing.T) {
	hc := NewHealthCheck("test").
		WithComponent(ComponentCheck{
			Name:   "database",
			Status: HealthStatusHealthy,
		}).
		WithComponent(ComponentCheck{
			Name:   "cache",
			Status: HealthStatusHealthy,
		})

	if len(hc.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(hc.Checks))
	}
	if hc.Status != HealthStatusHealthy {
		t.Errorf("expected healthy status, got %s", hc.Status)
	}
}

func TestHealthCheck_WithComponent_StatusPropagation(t *testing.T) {
	// Unhealthy component should make overall unhealthy
	hc := NewHealthCheck("test").
		WithComponent(ComponentCheck{Name: "ok", Status: HealthStatusHealthy}).
		WithComponent(ComponentCheck{Name: "bad", Status: HealthStatusUnhealthy})

	if hc.Status != HealthStatusUnhealthy {
		t.Errorf("expected unhealthy status, got %s", hc.Status)
	}

	// Degraded component should make overall degraded
	hc2 := NewHealthCheck("test").
		WithComponent(ComponentCheck{Name: "ok", Status: HealthStatusHealthy}).
		WithComponent(ComponentCheck{Name: "slow", Status: HealthStatusDegraded})

	if hc2.Status != HealthStatusDegraded {
		t.Errorf("expected degraded status, got %s", hc2.Status)
	}
}

func TestHealthCheck_IsHealthy(t *testing.T) {
	healthy := NewHealthCheck("test")
	if !healthy.IsHealthy() {
		t.Error("new health check should be healthy")
	}

	unhealthy := NewHealthCheck("test").WithStatus(HealthStatusUnhealthy)
	if unhealthy.IsHealthy() {
		t.Error("unhealthy check should not be healthy")
	}

	degraded := NewHealthCheck("test").WithStatus(HealthStatusDegraded)
	if degraded.IsHealthy() {
		t.Error("degraded check should not be healthy")
	}
}

func TestCheckStore_Success(t *testing.T) {
	ctx := context.Background()
	check := CheckStore(ctx, "postgres", func(ctx context.Context) error {
		return nil
	})

	if check.Status != HealthStatusHealthy {
		t.Errorf("expected healthy status, got %s", check.Status)
	}
	if check.Name != "postgres" {
		t.Errorf("expected name 'postgres', got %s", check.Name)
	}
}

func TestCheckStore_Failure(t *testing.T) {
	ctx := context.Background()
	check := CheckStore(ctx, "postgres", func(ctx context.Context) error {
		return errors.New("connection refused")
	})

	if check.Status != HealthStatusUnhealthy {
		t.Errorf("expected unhealthy status, got %s", check.Status)
	}
	if check.Message != "connection refused" {
		t.Errorf("expected error message, got %s", check.Message)
	}
}

func TestCheckStore_SlowResponse(t *testing.T) {
	ctx := context.Background()
	check := CheckStore(ctx, "postgres", func(ctx context.Context) error {
		time.Sleep(150 * time.Millisecond)
		return nil
	})

	if check.Status != HealthStatusDegraded {
		t.Errorf("expected degraded status for slow response, got %s", check.Status)
	}
}

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.err
}

func TestCheckDependency_Success(t *testing.T) {
	ctx := context.Background()
	check := CheckDependency(ctx, "cache", &mockPinger{err: nil})

	if check.Status != HealthStatusHealthy {
		t.Errorf("expected healthy status, got %s", check.Status)
	}
}

func TestCheckDependency_Failure(t *testing.T) {
	ctx := context.Background()
	check := CheckDependency(ctx, "cache", &mockPinger{err: errors.New("timeout")})

	if check.Status != HealthStatusUnhealthy {
		t.Errorf("expected unhealthy status, got %s", check.Status)
	}
}

func TestCheckDependency_Nil(t *testing.T) {
	ctx := context.Background()
	check := CheckDependency(ctx, "cache", nil)

	if check.Status != HealthStatusUnhealthy {
		t.Errorf("expected unhealthy status for nil checker, got %s", check.Status)
	}
	if check.Message != "dependency not configured" {
		t.Errorf("expected 'dependency not configured' message, got %s", check.Message)
	}
}

func TestAggregateStatus(t *testing.T) {
	tests := []struct {
		name     string
		statuses []HealthStatus
		expected HealthStatus
	}{
		{"all healthy", []HealthStatus{HealthStatusHealthy, HealthStatusHealthy}, HealthStatusHealthy},
		{"one degraded", []HealthStatus{HealthStatusHealthy, HealthStatusDegraded}, HealthStatusDegraded},
		{"one unhealthy", []HealthStatus{HealthStatusHealthy, HealthStatusUnhealthy}, HealthStatusUnhealthy},
		{"degraded and unhealthy", []HealthStatus{HealthStatusDegraded, HealthStatusUnhealthy}, HealthStatusUnhealthy},
		{"empty", []HealthStatus{}, HealthStatusHealthy},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := AggregateStatus(tc.statuses...)
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}
