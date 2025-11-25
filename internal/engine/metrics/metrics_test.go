package metrics

import (
	"errors"
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector("test")
	if c == nil {
		t.Fatal("NewCollector returned nil")
	}
	if c.registry == nil {
		t.Error("registry should not be nil")
	}
}

func TestNewCollector_DefaultNamespace(t *testing.T) {
	c := NewCollector("")
	if c == nil {
		t.Fatal("NewCollector returned nil")
	}
	// Should use default namespace "engine"
	if c.registry == nil {
		t.Error("registry should not be nil")
	}
}

func TestCollector_ModuleMetrics(t *testing.T) {
	c := NewCollector("test")

	// Should not panic
	c.RecordModuleStatus("mod1", "domain1", 3) // Running
	c.RecordModuleReadiness("mod1", "domain1", 1) // Ready
	c.RecordModuleStart("mod1", "domain1", 100*time.Millisecond, nil)
	c.RecordModuleStart("mod2", "domain1", 50*time.Millisecond, errors.New("start failed"))
	c.RecordModuleStop("mod1", "domain1", 10*time.Millisecond, nil)
	c.RecordModuleStop("mod2", "domain1", 5*time.Millisecond, errors.New("stop failed"))
	c.RecordModuleRestart("mod1", "domain1")
	c.RecordModuleFailure("mod1", "domain1", "runtime")
}

func TestCollector_BusMetrics(t *testing.T) {
	c := NewCollector("test")

	// Should not panic
	c.RecordBusPublish("events.topic1", 5*time.Millisecond, nil)
	c.RecordBusPublish("events.topic2", 10*time.Millisecond, errors.New("publish failed"))
	c.RecordBusPush("data.topic1", 2*time.Millisecond, nil)
	c.RecordBusPush("data.topic2", 3*time.Millisecond, errors.New("push failed"))
	c.RecordBusInvoke("compute.func1", 100*time.Millisecond, nil)
	c.RecordBusInvoke("compute.func2", 200*time.Millisecond, errors.New("invoke failed"))
	c.RecordBusInFlight("event", 5)
	c.RecordBusInFlight("data", 3)
	c.RecordBusInFlight("compute", 2)
	c.RecordBusQueueDepth("event", 10)
	c.RecordBusQueueDepth("data", 5)
}

func TestCollector_RecoveryMetrics(t *testing.T) {
	c := NewCollector("test")

	// Should not panic
	c.RecordRecoveryAttempt("mod1", "restart")
	c.RecordRecoveryResult("mod1", "restart", 500*time.Millisecond, nil)
	c.RecordRecoveryAttempt("mod2", "restart")
	c.RecordRecoveryResult("mod2", "restart", 1*time.Second, errors.New("recovery failed"))
}

func TestCollector_DependencyMetrics(t *testing.T) {
	c := NewCollector("test")

	// Should not panic
	c.RecordDependencyWait("mod1", "mod2", 100*time.Millisecond)
	c.RecordDependencyCycle()
	c.RecordDependencyMissing()
}

func TestCollector_ResourceMetrics(t *testing.T) {
	c := NewCollector("test")

	// Should not panic
	c.RecordGoroutines(100)
	c.UpdateUptime()
}

func TestCollector_Reset(t *testing.T) {
	c := NewCollector("test")

	c.RecordModuleStatus("mod1", "domain1", 3)
	c.RecordBusInFlight("event", 10)

	// Should not panic
	c.Reset()
}

func TestCollector_Registry(t *testing.T) {
	c := NewCollector("test")

	reg := c.Registry()
	if reg == nil {
		t.Error("Registry() should not return nil")
	}
}

func TestNoOpCollector(t *testing.T) {
	c := NewNoOpCollector()

	// All these should not panic
	c.RecordModuleStatus("mod1", "domain1", 3)
	c.RecordModuleReadiness("mod1", "domain1", 1)
	c.RecordModuleStart("mod1", "domain1", 100*time.Millisecond, nil)
	c.RecordModuleStop("mod1", "domain1", 10*time.Millisecond, nil)
	c.RecordModuleRestart("mod1", "domain1")
	c.RecordModuleFailure("mod1", "domain1", "runtime")
	c.RecordBusPublish("topic1", 5*time.Millisecond, nil)
	c.RecordBusPush("topic1", 2*time.Millisecond, nil)
	c.RecordBusInvoke("func1", 100*time.Millisecond, nil)
	c.RecordBusInFlight("event", 5)
	c.RecordBusQueueDepth("event", 10)
	c.RecordRecoveryAttempt("mod1", "restart")
	c.RecordRecoveryResult("mod1", "restart", 500*time.Millisecond, nil)
	c.RecordDependencyWait("mod1", "mod2", 100*time.Millisecond)
	c.RecordDependencyCycle()
	c.RecordDependencyMissing()
	c.RecordGoroutines(100)
	c.UpdateUptime()
	c.Reset()
}

func TestMetricsCollectorInterface(t *testing.T) {
	// Verify both types implement the interface
	var _ MetricsCollector = (*Collector)(nil)
	var _ MetricsCollector = (*NoOpCollector)(nil)
}

func BenchmarkCollector_RecordModuleStatus(b *testing.B) {
	c := NewCollector("bench")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RecordModuleStatus("mod1", "domain1", 3)
	}
}

func BenchmarkCollector_RecordBusPublish(b *testing.B) {
	c := NewCollector("bench")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RecordBusPublish("topic1", 5*time.Millisecond, nil)
	}
}

func BenchmarkNoOpCollector_RecordModuleStatus(b *testing.B) {
	c := NewNoOpCollector()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RecordModuleStatus("mod1", "domain1", 3)
	}
}
