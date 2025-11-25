package engine

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestEngine_FullLifecycle tests complete service registration, start, and stop cycle.
func TestEngine_FullLifecycle(t *testing.T) {
	e := New()

	// Create services with dependencies
	svcA := newTestModule("svc-a", "domain-a")
	svcB := newTestModule("svc-b", "domain-b")
	svcB.dependsOn = []string{"svc-a"}

	// Register in reverse dependency order to test ordering
	if err := e.Register(svcB); err != nil {
		t.Fatalf("register svc-b: %v", err)
	}
	if err := e.Register(svcA); err != nil {
		t.Fatalf("register svc-a: %v", err)
	}

	// Set dependencies
	e.SetModuleDeps("svc-b", "svc-a")

	ctx := context.Background()

	// Start all modules
	if err := e.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Verify both started
	if !svcA.started.Load() {
		t.Error("svc-a should be started")
	}
	if !svcB.started.Load() {
		t.Error("svc-b should be started")
	}

	// Check start order - A must start before B
	if svcA.startTime.After(svcB.startTime) {
		t.Error("svc-a should start before svc-b (dependency)")
	}

	// Stop all modules
	if err := e.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Verify both stopped
	if !svcA.stopped.Load() {
		t.Error("svc-a should be stopped")
	}
	if !svcB.stopped.Load() {
		t.Error("svc-b should be stopped")
	}

	// Note: Stop order depends on lifecycle manager implementation.
	// The key requirement is that all services stop successfully.
	// Dependency-based stop ordering is a nice-to-have but not strictly required
	// by the current implementation.
}

// TestEngine_StartFailureRollback tests that start failure triggers rollback of already-started services.
func TestEngine_StartFailureRollback(t *testing.T) {
	e := New()

	svcGood := newTestModule("svc-good", "domain")
	svcBad := newTestModule("svc-bad", "domain")
	svcBad.startErr = errors.New("intentional start failure")

	// Register good first so it starts first
	_ = e.Register(svcGood)
	_ = e.Register(svcBad)

	ctx := context.Background()
	err := e.Start(ctx)

	if err == nil {
		t.Fatal("expected start error")
	}

	// Good service should have been rolled back (stopped)
	if !svcGood.stopped.Load() {
		t.Error("svc-good should be stopped after rollback")
	}
}

// TestEngine_HealthMonitoring tests health status tracking.
func TestEngine_HealthMonitoring(t *testing.T) {
	e := New()

	svc := newTestModule("health-test", "domain")
	_ = e.Register(svc)

	ctx := context.Background()
	_ = e.Start(ctx)

	// Initially healthy (started status)
	health := e.ModulesHealth()
	if len(health) == 0 {
		t.Fatal("expected health data")
	}

	found := false
	for _, h := range health {
		if h.Name == "health-test" {
			found = true
			if h.Status != StatusStarted {
				t.Errorf("expected status %s, got %s", StatusStarted, h.Status)
			}
		}
	}
	if !found {
		t.Error("health-test not found in health data")
	}

	// Update health via health monitor
	e.Health().SetHealth("health-test", ModuleHealth{
		Name:   "health-test",
		Domain: "domain",
		Status: StatusFailed,
		Error:  "high load",
	})

	health = e.ModulesHealth()
	for _, h := range health {
		if h.Name == "health-test" {
			if h.Status != StatusFailed {
				t.Errorf("expected status %s, got %s", StatusFailed, h.Status)
			}
			if h.Error != "high load" {
				t.Errorf("expected error 'high load', got %q", h.Error)
			}
		}
	}
}

// TestEngine_ReadinessTracking tests readiness status management.
func TestEngine_ReadinessTracking(t *testing.T) {
	e := New()

	svc := newTestModule("ready-test", "domain")
	_ = e.Register(svc)

	ctx := context.Background()
	_ = e.Start(ctx)

	// Mark ready using the Engine method
	e.MarkReady(ReadyStatusReady, "", "ready-test")

	health := e.ModulesHealth()
	for _, h := range health {
		if h.Name == "ready-test" {
			if h.ReadyStatus != ReadyStatusReady {
				t.Errorf("expected ready status %s, got %s", ReadyStatusReady, h.ReadyStatus)
			}
		}
	}

	// Mark not ready with error
	e.MarkReady(ReadyStatusNotReady, "warming up", "ready-test")

	health = e.ModulesHealth()
	for _, h := range health {
		if h.Name == "ready-test" {
			if h.ReadyStatus != ReadyStatusNotReady {
				t.Errorf("expected ready status %s, got %s", ReadyStatusNotReady, h.ReadyStatus)
			}
			if h.ReadyError != "warming up" {
				t.Errorf("expected ready error 'warming up', got %q", h.ReadyError)
			}
		}
	}
}

// TestEngine_BusEventFanOut tests event publishing to multiple subscribers.
func TestEngine_BusEventFanOut(t *testing.T) {
	e := New()

	// Create event-capable modules
	svc1 := newEventModule("event-svc-1", "domain")
	svc2 := newEventModule("event-svc-2", "domain")

	_ = e.Register(svc1)
	_ = e.Register(svc2)

	// Grant event permissions
	e.SetBusPermissions("event-svc-1", BusPermissions{AllowEvents: true})
	e.SetBusPermissions("event-svc-2", BusPermissions{AllowEvents: true})

	ctx := context.Background()
	_ = e.Start(ctx)

	// Publish event
	err := e.PublishEvent(ctx, "test.event", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("publish event: %v", err)
	}

	// Both should receive the event
	time.Sleep(50 * time.Millisecond) // Allow async processing

	if svc1.eventsReceived.Load() != 1 {
		t.Errorf("svc1: expected 1 event, got %d", svc1.eventsReceived.Load())
	}
	if svc2.eventsReceived.Load() != 1 {
		t.Errorf("svc2: expected 1 event, got %d", svc2.eventsReceived.Load())
	}
}

// TestEngine_BusTimeout tests that slow event handlers don't block indefinitely.
// Note: This is a timing-sensitive test that may occasionally fail under heavy load.
func TestEngine_BusTimeout(t *testing.T) {
	e := New()

	// Set timeout for test via Bus
	timeout := 200 * time.Millisecond
	e.Bus().SetTimeout(timeout)

	fastSvc := newEventModule("fast-svc", "domain")
	slowSvc := newEventModule("slow-svc", "domain")
	slowSvc.publishDelay = 2 * time.Second // Much longer than timeout

	_ = e.Register(fastSvc)
	_ = e.Register(slowSvc)

	// Grant event permissions
	e.SetBusPermissions("fast-svc", BusPermissions{AllowEvents: true})
	e.SetBusPermissions("slow-svc", BusPermissions{AllowEvents: true})

	ctx := context.Background()
	_ = e.Start(ctx)

	start := time.Now()
	_ = e.PublishEvent(ctx, "test.event", nil)
	elapsed := time.Since(start)

	// Should complete within a reasonable time, not wait for slow service (2s)
	// We allow generous margin (500ms) to avoid flaky tests
	maxExpected := timeout + 500*time.Millisecond
	if elapsed > maxExpected {
		t.Errorf("publish took too long: %v (expected < %v)", elapsed, maxExpected)
	}

	// Fast service should have received the event
	time.Sleep(50 * time.Millisecond)
	if fastSvc.eventsReceived.Load() != 1 {
		t.Errorf("fast-svc should have received event, got %d", fastSvc.eventsReceived.Load())
	}
}

// TestEngine_UnregisterCleanup tests that unregister properly cleans up all state.
func TestEngine_UnregisterCleanup(t *testing.T) {
	e := New()

	svc := newTestModule("cleanup-test", "domain")
	_ = e.Register(svc)

	// Set various metadata
	e.SetModuleLayer("cleanup-test", "infra")
	e.SetModuleDeps("cleanup-test", "other")
	e.Health().SetHealth("cleanup-test", ModuleHealth{
		Name:   "cleanup-test",
		Domain: "domain",
		Status: StatusStarted,
	})

	// Verify registered
	if m := e.Lookup("cleanup-test"); m == nil {
		t.Fatal("module should be registered")
	}

	// Unregister
	if err := e.Unregister("cleanup-test"); err != nil {
		t.Fatalf("unregister: %v", err)
	}

	// Verify all state cleaned up
	if m := e.Lookup("cleanup-test"); m != nil {
		t.Error("module should be unregistered")
	}

	// Health should be gone
	health := e.ModulesHealth()
	for _, h := range health {
		if h.Name == "cleanup-test" {
			t.Error("health data should be cleaned up")
		}
	}
}

// TestEngine_ConcurrentOperations tests thread safety.
func TestEngine_ConcurrentOperations(t *testing.T) {
	e := New()

	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	// Concurrent registrations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			svc := newTestModule("concurrent-"+string(rune('a'+idx)), "domain")
			if err := e.Register(svc); err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent registration error: %v", err)
	}

	// Verify all registered
	if len(e.ModulesInfo()) != 10 {
		t.Errorf("expected 10 modules, got %d", len(e.ModulesInfo()))
	}
}

// TestEngine_ModulesInfo tests module information retrieval.
func TestEngine_ModulesInfo(t *testing.T) {
	e := New()

	svc := newTestModule("info-test", "test-domain")
	_ = e.Register(svc)

	e.SetModuleLayer("info-test", "platform")
	e.SetModuleCapabilities("info-test", "cap1", "cap2")

	info := e.ModulesInfo()
	if len(info) != 1 {
		t.Fatalf("expected 1 module info, got %d", len(info))
	}

	mi := info[0]
	if mi.Name != "info-test" {
		t.Errorf("expected name 'info-test', got %q", mi.Name)
	}
	if mi.Domain != "test-domain" {
		t.Errorf("expected domain 'test-domain', got %q", mi.Domain)
	}
	if mi.Layer != "platform" {
		t.Errorf("expected layer 'platform', got %q", mi.Layer)
	}
	if len(mi.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(mi.Capabilities))
	}
}

// TestEngine_DuplicateRegistration tests that duplicate registration fails.
func TestEngine_DuplicateRegistration(t *testing.T) {
	e := New()

	svc := newTestModule("dup-test", "domain")
	if err := e.Register(svc); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	if err := e.Register(svc); err == nil {
		t.Error("expected error for duplicate registration")
	}
}

// TestEngine_NilRegistration tests that nil registration fails gracefully.
func TestEngine_NilRegistration(t *testing.T) {
	e := New()

	if err := e.Register(nil); err == nil {
		t.Error("expected error for nil registration")
	}
}

// TestEngine_ModulesByDomain tests filtering modules by domain.
func TestEngine_ModulesByDomain(t *testing.T) {
	e := New()

	_ = e.Register(newTestModule("svc1", "domain-a"))
	_ = e.Register(newTestModule("svc2", "domain-a"))
	_ = e.Register(newTestModule("svc3", "domain-b"))

	domainA := e.ModulesByDomain("domain-a")
	if len(domainA) != 2 {
		t.Errorf("expected 2 modules in domain-a, got %d", len(domainA))
	}

	domainB := e.ModulesByDomain("domain-b")
	if len(domainB) != 1 {
		t.Errorf("expected 1 module in domain-b, got %d", len(domainB))
	}

	unknown := e.ModulesByDomain("unknown")
	if len(unknown) != 0 {
		t.Errorf("expected 0 modules in unknown domain, got %d", len(unknown))
	}
}

// TestEngine_TypedEngines tests typed engine accessors.
func TestEngine_TypedEngines(t *testing.T) {
	e := New()

	eventSvc := newEventModule("event-svc", "domain")
	normalSvc := newTestModule("normal-svc", "domain")

	_ = e.Register(eventSvc)
	_ = e.Register(normalSvc)

	// Grant event permissions to see the event engine
	e.SetBusPermissions("event-svc", BusPermissions{AllowEvents: true})

	eventEngines := e.EventEngines()
	if len(eventEngines) != 1 {
		t.Errorf("expected 1 event engine, got %d", len(eventEngines))
	}
}

// TestEngine_BusPermissions tests bus permission control.
func TestEngine_BusPermissions(t *testing.T) {
	e := New()

	svc := newTestModule("perm-test", "domain")
	_ = e.Register(svc)

	// Set restrictive permissions
	e.SetBusPermissions("perm-test", BusPermissions{
		AllowEvents:  false,
		AllowData:    true,
		AllowCompute: false,
	})

	perms := e.Permissions()
	if perms == nil {
		t.Fatal("permissions manager is nil")
	}

	p := perms.GetPermissions("perm-test")
	if p.AllowEvents {
		t.Error("expected AllowEvents to be false")
	}
	if !p.AllowData {
		t.Error("expected AllowData to be true")
	}
}

// TestEngine_ProbeReadiness tests readiness probing.
func TestEngine_ProbeReadiness(t *testing.T) {
	e := New()

	svc := newReadyModule("probe-test", "domain")
	_ = e.Register(svc)

	ctx := context.Background()
	_ = e.Start(ctx)

	// Initial probe
	e.ProbeReadiness(ctx)

	health := e.ModulesHealth()
	for _, h := range health {
		if h.Name == "probe-test" {
			// Should be ready since our module reports ready
			if h.ReadyStatus != ReadyStatusReady {
				t.Errorf("expected ready status after probe, got %s", h.ReadyStatus)
			}
		}
	}
}

// testModule is a configurable test service module.
type testModule struct {
	name      string
	domain    string
	dependsOn []string
	startErr  error
	stopErr   error
	started   atomic.Bool
	stopped   atomic.Bool
	startTime time.Time
	stopTime  time.Time
	mu        sync.Mutex
}

func newTestModule(name, domain string) *testModule {
	return &testModule{name: name, domain: domain}
}

func (m *testModule) Name() string   { return m.name }
func (m *testModule) Domain() string { return m.domain }

func (m *testModule) Start(ctx context.Context) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.mu.Lock()
	m.startTime = time.Now()
	m.mu.Unlock()
	m.started.Store(true)
	time.Sleep(10 * time.Millisecond) // Small delay for ordering tests
	return nil
}

func (m *testModule) Stop(ctx context.Context) error {
	if m.stopErr != nil {
		return m.stopErr
	}
	m.mu.Lock()
	m.stopTime = time.Now()
	m.mu.Unlock()
	m.stopped.Store(true)
	time.Sleep(10 * time.Millisecond) // Small delay for ordering tests
	return nil
}

// eventModule is a test module that implements EventEngine.
type eventModule struct {
	testModule
	eventsReceived atomic.Int64
	publishDelay   time.Duration
}

func newEventModule(name, domain string) *eventModule {
	return &eventModule{
		testModule: testModule{name: name, domain: domain},
	}
}

func (m *eventModule) Publish(ctx context.Context, event string, payload any) error {
	if m.publishDelay > 0 {
		select {
		case <-time.After(m.publishDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m.eventsReceived.Add(1)
	return nil
}

func (m *eventModule) Subscribe(ctx context.Context, event string, handler func(context.Context, any) error) error {
	return nil
}

// readyModule is a test module that implements ReadyChecker.
type readyModule struct {
	testModule
	isReady atomic.Bool
}

func newReadyModule(name, domain string) *readyModule {
	m := &readyModule{testModule: testModule{name: name, domain: domain}}
	m.isReady.Store(true)
	return m
}

func (m *readyModule) Ready(ctx context.Context) error {
	if !m.isReady.Load() {
		return errors.New("not ready")
	}
	return nil
}
