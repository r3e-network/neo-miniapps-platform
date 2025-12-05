// Package engine provides the Service Engine for the Service Layer.
package engine

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// mockService is a simple mock service for testing.
type mockService struct {
	*base.BaseService
	startCalled bool
	stopCalled  bool
}

// createMockServiceFactory creates a factory that uses the manifest's ServiceID.
func createMockServiceFactory(serviceID string) ServiceFactory {
	return func(serviceOS os.ServiceOS) (base.Service, error) {
		return &mockService{
			BaseService: base.NewBaseService(serviceID, "Mock Service "+serviceID, "1.0.0", serviceOS),
		}, nil
	}
}

func newMockService(serviceOS os.ServiceOS) (base.Service, error) {
	// Get service ID from manifest via ServiceOS
	return &mockService{
		BaseService: base.NewBaseService(serviceOS.ServiceID(), "Mock Service", "1.0.0", serviceOS),
	}, nil
}

func (m *mockService) Start(ctx context.Context) error {
	m.startCalled = true
	return m.BaseService.Start(ctx)
}

func (m *mockService) Stop(ctx context.Context) error {
	m.stopCalled = true
	return m.BaseService.Stop(ctx)
}

func TestEngine_New(t *testing.T) {
	cfg := DefaultConfig()
	engine := New(cfg)

	if engine == nil {
		t.Fatal("New() returned nil")
	}
	if engine.State() != StateCreated {
		t.Errorf("State() = %s, want %s", engine.State(), StateCreated)
	}
}

func TestEngine_RegisterService(t *testing.T) {
	cfg := DefaultConfig()
	engine := New(cfg)

	def := ServiceDefinition{
		Manifest: &os.Manifest{
			ServiceID: "test-service",
		},
		Factory: newMockService,
	}

	err := engine.RegisterService(def)
	if err != nil {
		t.Fatalf("RegisterService() error: %v", err)
	}
}

func TestEngine_RegisterService_NilManifest(t *testing.T) {
	cfg := DefaultConfig()
	engine := New(cfg)

	def := ServiceDefinition{
		Manifest: nil,
		Factory:  newMockService,
	}

	err := engine.RegisterService(def)
	if err == nil {
		t.Error("RegisterService() should error with nil manifest")
	}
}

func TestEngine_RegisterService_NilFactory(t *testing.T) {
	cfg := DefaultConfig()
	engine := New(cfg)

	def := ServiceDefinition{
		Manifest: &os.Manifest{
			ServiceID: "test-service",
		},
		Factory: nil,
	}

	err := engine.RegisterService(def)
	if err == nil {
		t.Error("RegisterService() should error with nil factory")
	}
}

func TestEngine_StartAndStop(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnclaveID = "test-enclave"
	cfg.Mode = "simulation"
	cfg.StoragePath = t.TempDir()

	engine := New(cfg)

	// Register a service
	def := ServiceDefinition{
		Manifest: &os.Manifest{
			ServiceID: "mock",
		},
		Factory: newMockService,
	}
	engine.RegisterService(def)

	// Start engine
	ctx := context.Background()
	err := engine.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Verify state
	if engine.State() != StateRunning {
		t.Errorf("State() = %s, want %s", engine.State(), StateRunning)
	}

	// Verify service is registered
	svc, ok := engine.GetService("mock")
	if !ok {
		t.Error("GetService() should find mock service")
	}
	if svc == nil {
		t.Error("GetService() returned nil service")
	}

	// Stop engine
	err = engine.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	// Verify state
	if engine.State() != StateStopped {
		t.Errorf("State() = %s, want %s", engine.State(), StateStopped)
	}
}

func TestEngine_Health(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnclaveID = "test-enclave"
	cfg.Mode = "simulation"
	cfg.StoragePath = t.TempDir()

	engine := New(cfg)

	ctx := context.Background()

	// Health should fail before start
	err := engine.Health(ctx)
	if err == nil {
		t.Error("Health() should error before start")
	}

	// Start engine
	engine.Start(ctx)
	defer engine.Stop(ctx)

	// Health should pass after start
	err = engine.Health(ctx)
	if err != nil {
		t.Errorf("Health() error: %v", err)
	}
}

func TestEngine_Stats(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnclaveID = "test-enclave"
	cfg.Mode = "simulation"
	cfg.StoragePath = t.TempDir()

	engine := New(cfg)

	// Register services
	for i := 0; i < 3; i++ {
		def := ServiceDefinition{
			Manifest: &os.Manifest{
				ServiceID: "mock-" + string(rune('a'+i)),
			},
			Factory: newMockService,
		}
		engine.RegisterService(def)
	}

	ctx := context.Background()
	engine.Start(ctx)
	defer engine.Stop(ctx)

	stats := engine.Stats()
	if stats.State != StateRunning {
		t.Errorf("Stats.State = %s, want %s", stats.State, StateRunning)
	}
	if stats.TotalServices != 3 {
		t.Errorf("Stats.TotalServices = %d, want 3", stats.TotalServices)
	}
}

func TestEngine_OnStateChange(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnclaveID = "test-enclave"
	cfg.Mode = "simulation"
	cfg.StoragePath = t.TempDir()

	engine := New(cfg)

	// Track state changes
	var states []State
	engine.OnStateChange(func(state State) {
		states = append(states, state)
	})

	ctx := context.Background()
	engine.Start(ctx)
	engine.Stop(ctx)

	// Wait a bit for callbacks
	time.Sleep(10 * time.Millisecond)

	// Verify state transitions
	expectedStates := []State{StateStarting, StateRunning, StateStopping, StateStopped}
	if len(states) != len(expectedStates) {
		t.Errorf("got %d state changes, want %d", len(states), len(expectedStates))
	}
	for i, expected := range expectedStates {
		if i < len(states) && states[i] != expected {
			t.Errorf("state[%d] = %s, want %s", i, states[i], expected)
		}
	}
}

func TestEngine_ListServices(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnclaveID = "test-enclave"
	cfg.Mode = "simulation"
	cfg.StoragePath = t.TempDir()

	engine := New(cfg)

	// Register services
	for _, id := range []string{"svc-a", "svc-b", "svc-c"} {
		def := ServiceDefinition{
			Manifest: &os.Manifest{
				ServiceID: id,
			},
			Factory: newMockService,
		}
		engine.RegisterService(def)
	}

	ctx := context.Background()
	engine.Start(ctx)
	defer engine.Stop(ctx)

	services := engine.ListServices()
	if len(services) != 3 {
		t.Errorf("ListServices() returned %d services, want 3", len(services))
	}
}

func TestEngine_DoubleStart(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnclaveID = "test-enclave"
	cfg.Mode = "simulation"
	cfg.StoragePath = t.TempDir()

	engine := New(cfg)

	ctx := context.Background()
	engine.Start(ctx)
	defer engine.Stop(ctx)

	// Second start should error
	err := engine.Start(ctx)
	if err == nil {
		t.Error("second Start() should error")
	}
}
