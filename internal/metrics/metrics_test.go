package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNew(t *testing.T) {
	// Use a custom registry for testing to avoid conflicts
	reg := prometheus.NewRegistry()
	m := NewWithRegistry("test-service", reg)

	if m == nil {
		t.Fatal("Expected metrics instance, got nil")
	}

	if m.RequestsTotal == nil {
		t.Error("RequestsTotal should not be nil")
	}
	if m.RequestDuration == nil {
		t.Error("RequestDuration should not be nil")
	}
	if m.ErrorsTotal == nil {
		t.Error("ErrorsTotal should not be nil")
	}
}

func TestRecordHTTPRequest(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewWithRegistry("test-service", reg)

	// Should not panic
	m.RecordHTTPRequest("test-service", "GET", "/api/test", "200", 100*time.Millisecond)
	m.RecordHTTPRequest("test-service", "POST", "/api/test", "201", 200*time.Millisecond)
	m.RecordHTTPRequest("test-service", "GET", "/api/test", "404", 50*time.Millisecond)
}

func TestRecordError(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewWithRegistry("test-service", reg)

	// Should not panic
	m.RecordError("test-service", "validation", "create_user")
	m.RecordError("test-service", "database", "query")
}

func TestRecordBlockchainTx(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewWithRegistry("test-service", reg)

	// Should not panic
	m.RecordBlockchainTx("test-service", "neo", "invoke", "success", 2*time.Second)
	m.RecordBlockchainTx("test-service", "neo", "invoke", "failed", 1*time.Second)
}

func TestRecordVRFRequest(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewWithRegistry("test-service", reg)

	// Should not panic
	m.RecordVRFRequest("test-service", "success")
	m.RecordVRFRequest("test-service", "failed")
}

func TestRecordNeoVaultOperation(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewWithRegistry("test-service", reg)

	// Should not panic
	m.RecordNeoVaultOperation("test-service", "deposit", "success")
	m.RecordNeoVaultOperation("test-service", "withdraw", "failed")
}

func TestRecordDatabaseQuery(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewWithRegistry("test-service", reg)

	// Should not panic
	m.RecordDatabaseQuery("test-service", "select", "success", 10*time.Millisecond)
	m.RecordDatabaseQuery("test-service", "insert", "failed", 5*time.Millisecond)
}

func TestSetDatabaseConnections(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewWithRegistry("test-service", reg)

	// Should not panic
	m.SetDatabaseConnections(10)
	m.SetDatabaseConnections(0)
}

func TestUpdateUptime(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewWithRegistry("test-service", reg)
	startTime := time.Now().Add(-1 * time.Hour)

	// Should not panic
	m.UpdateUptime(startTime)
}

func TestInFlightCounters(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewWithRegistry("test-service", reg)

	// Should not panic
	m.IncrementInFlight()
	m.IncrementInFlight()
	m.DecrementInFlight()
	m.DecrementInFlight()
}

func TestNewWithRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewWithRegistry("test-service", reg)

	if m == nil {
		t.Fatal("Expected metrics instance, got nil")
	}

	// Verify metrics are registered
	metricFamilies, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(metricFamilies) == 0 {
		t.Error("Expected metrics to be registered")
	}
}
