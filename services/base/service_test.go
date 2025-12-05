// Package base provides base components for all services.
package base

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/tee"
)

// =============================================================================
// Test Helpers
// =============================================================================

func setupTestServiceOS(t *testing.T) (os.ServiceOS, func()) {
	trustRoot, err := tee.NewSimulation("test-enclave")
	if err != nil {
		t.Fatalf("failed to create trust root: %v", err)
	}

	ctx := context.Background()
	if err := trustRoot.Start(ctx); err != nil {
		t.Fatalf("failed to start trust root: %v", err)
	}

	manifest := &os.LegacyManifest{
		ServiceID: "test-service",
		RequiredCapabilities: []os.Capability{
			os.CapSecrets, os.CapNetwork, os.CapKeys,
			os.CapCompute, os.CapStorage, os.CapAttestation,
		},
	}

	svcCtx, err := os.NewServiceContext(manifest, trustRoot, nil)
	if err != nil {
		trustRoot.Stop(ctx)
		t.Fatalf("failed to create context: %v", err)
	}

	cleanup := func() {
		svcCtx.Close()
		trustRoot.Stop(ctx)
	}

	return svcCtx, cleanup
}

// =============================================================================
// BaseService Tests
// =============================================================================

func TestNewBaseService(t *testing.T) {
	serviceOS, cleanup := setupTestServiceOS(t)
	defer cleanup()

	svc := NewBaseService("test-id", "Test Service", "1.0.0", serviceOS)

	if svc == nil {
		t.Fatal("NewBaseService returned nil")
	}
	if svc.ID() != "test-id" {
		t.Errorf("ID() = %s, want test-id", svc.ID())
	}
	if svc.Name() != "Test Service" {
		t.Errorf("Name() = %s, want Test Service", svc.Name())
	}
	if svc.Version() != "1.0.0" {
		t.Errorf("Version() = %s, want 1.0.0", svc.Version())
	}
	if svc.State() != StateCreated {
		t.Errorf("State() = %s, want %s", svc.State(), StateCreated)
	}
}

func TestBaseService_Lifecycle(t *testing.T) {
	serviceOS, cleanup := setupTestServiceOS(t)
	defer cleanup()

	svc := NewBaseService("test-id", "Test Service", "1.0.0", serviceOS)
	ctx := context.Background()

	// Initial state
	if svc.State() != StateCreated {
		t.Errorf("initial State() = %s, want %s", svc.State(), StateCreated)
	}

	// Start
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if svc.State() != StateRunning {
		t.Errorf("after Start() State() = %s, want %s", svc.State(), StateRunning)
	}

	// Health should pass when running
	if err := svc.Health(ctx); err != nil {
		t.Errorf("Health() error when running: %v", err)
	}

	// Stop
	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if svc.State() != StateStopped {
		t.Errorf("after Stop() State() = %s, want %s", svc.State(), StateStopped)
	}

	// Health should fail when stopped
	if err := svc.Health(ctx); err == nil {
		t.Error("Health() should error when stopped")
	}
}

func TestBaseService_SetState(t *testing.T) {
	serviceOS, cleanup := setupTestServiceOS(t)
	defer cleanup()

	svc := NewBaseService("test-id", "Test Service", "1.0.0", serviceOS)

	states := []ServiceState{
		StateCreated,
		StateStarting,
		StateRunning,
		StateStopping,
		StateStopped,
		StateFailed,
	}

	for _, state := range states {
		svc.SetState(state)
		if svc.State() != state {
			t.Errorf("SetState(%s): State() = %s", state, svc.State())
		}
	}
}

func TestBaseService_OS(t *testing.T) {
	serviceOS, cleanup := setupTestServiceOS(t)
	defer cleanup()

	svc := NewBaseService("test-id", "Test Service", "1.0.0", serviceOS)

	if svc.OS() == nil {
		t.Error("OS() returned nil")
	}
	if svc.Logger() == nil {
		t.Error("Logger() returned nil")
	}
}

func TestBaseService_ConcurrentStateAccess(t *testing.T) {
	serviceOS, cleanup := setupTestServiceOS(t)
	defer cleanup()

	svc := NewBaseService("test-id", "Test Service", "1.0.0", serviceOS)

	var wg sync.WaitGroup
	states := []ServiceState{StateCreated, StateStarting, StateRunning, StateStopping, StateStopped}

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			svc.SetState(states[idx%len(states)])
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.State()
		}()
	}

	wg.Wait()
}

// =============================================================================
// Registry Tests
// =============================================================================

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}
}

func TestRegistry_Register(t *testing.T) {
	serviceOS, cleanup := setupTestServiceOS(t)
	defer cleanup()

	registry := NewRegistry()
	svc := NewBaseService("test-id", "Test Service", "1.0.0", serviceOS)

	// First registration should succeed
	if err := registry.Register(svc); err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	// Duplicate registration should fail
	if err := registry.Register(svc); err == nil {
		t.Error("duplicate Register() should error")
	}
}

func TestRegistry_Get(t *testing.T) {
	serviceOS, cleanup := setupTestServiceOS(t)
	defer cleanup()

	registry := NewRegistry()
	svc := NewBaseService("test-id", "Test Service", "1.0.0", serviceOS)
	registry.Register(svc)

	// Get existing service
	got, ok := registry.Get("test-id")
	if !ok {
		t.Error("Get() should find registered service")
	}
	if got.ID() != "test-id" {
		t.Errorf("Get() returned service with ID %s, want test-id", got.ID())
	}

	// Get non-existent service
	_, ok = registry.Get("non-existent")
	if ok {
		t.Error("Get() should not find non-existent service")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	serviceOS, cleanup := setupTestServiceOS(t)
	defer cleanup()

	registry := NewRegistry()
	svc := NewBaseService("test-id", "Test Service", "1.0.0", serviceOS)
	registry.Register(svc)

	// Unregister
	if err := registry.Unregister("test-id"); err != nil {
		t.Fatalf("Unregister() error: %v", err)
	}

	// Should not find after unregister
	_, ok := registry.Get("test-id")
	if ok {
		t.Error("Get() should not find unregistered service")
	}
}

func TestRegistry_List(t *testing.T) {
	serviceOS, cleanup := setupTestServiceOS(t)
	defer cleanup()

	registry := NewRegistry()

	// Empty registry
	if len(registry.List()) != 0 {
		t.Error("List() should return empty slice for empty registry")
	}

	// Add services
	for i := 0; i < 3; i++ {
		svc := NewBaseService(
			"test-id-"+string(rune('a'+i)),
			"Test Service",
			"1.0.0",
			serviceOS,
		)
		registry.Register(svc)
	}

	services := registry.List()
	if len(services) != 3 {
		t.Errorf("List() returned %d services, want 3", len(services))
	}
}

func TestRegistry_StartAll(t *testing.T) {
	serviceOS, cleanup := setupTestServiceOS(t)
	defer cleanup()

	registry := NewRegistry()
	ctx := context.Background()

	// Add services
	for i := 0; i < 3; i++ {
		svc := NewBaseService(
			"test-id-"+string(rune('a'+i)),
			"Test Service",
			"1.0.0",
			serviceOS,
		)
		registry.Register(svc)
	}

	// Start all
	if err := registry.StartAll(ctx); err != nil {
		t.Fatalf("StartAll() error: %v", err)
	}

	// Verify all running
	for _, svc := range registry.List() {
		if svc.State() != StateRunning {
			t.Errorf("service %s State() = %s, want %s", svc.ID(), svc.State(), StateRunning)
		}
	}
}

func TestRegistry_StopAll(t *testing.T) {
	serviceOS, cleanup := setupTestServiceOS(t)
	defer cleanup()

	registry := NewRegistry()
	ctx := context.Background()

	// Add and start services
	for i := 0; i < 3; i++ {
		svc := NewBaseService(
			"test-id-"+string(rune('a'+i)),
			"Test Service",
			"1.0.0",
			serviceOS,
		)
		registry.Register(svc)
	}
	registry.StartAll(ctx)

	// Stop all
	if err := registry.StopAll(ctx); err != nil {
		t.Fatalf("StopAll() error: %v", err)
	}

	// Verify all stopped
	for _, svc := range registry.List() {
		if svc.State() != StateStopped {
			t.Errorf("service %s State() = %s, want %s", svc.ID(), svc.State(), StateStopped)
		}
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	serviceOS, cleanup := setupTestServiceOS(t)
	defer cleanup()

	registry := NewRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			svc := NewBaseService(
				"concurrent-"+string(rune('a'+idx%26))+"-"+time.Now().String(),
				"Test Service",
				"1.0.0",
				serviceOS,
			)
			registry.Register(svc)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.List()
		}()
	}

	wg.Wait()
}
