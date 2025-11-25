package system

import (
	"context"
	"errors"
	"testing"

	core "github.com/R3E-Network/service_layer/internal/services/core"
)

// mockDeps implements ServiceDeps for testing
type mockDeps struct{}

func (m mockDeps) Logger() any { return nil }
func (m mockDeps) Stores() any { return nil }

// testService is a minimal Service implementation for testing
type testService struct {
	name    string
	started bool
	stopped bool
	startFn func() error
	stopFn  func() error
}

func (s *testService) Name() string { return s.name }

func (s *testService) Start(ctx context.Context) error {
	if s.startFn != nil {
		if err := s.startFn(); err != nil {
			return err
		}
	}
	s.started = true
	return nil
}

func (s *testService) Stop(ctx context.Context) error {
	if s.stopFn != nil {
		if err := s.stopFn(); err != nil {
			return err
		}
	}
	s.stopped = true
	return nil
}

func TestServiceRegistry_Register(t *testing.T) {
	r := NewServiceRegistry()

	entry := ServiceEntry{
		Name:   "test-service",
		Domain: "test",
		Factory: func(deps ServiceDeps) (Service, error) {
			return &testService{name: "test-service"}, nil
		},
	}

	if err := r.Register(entry); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if r.Count() != 1 {
		t.Errorf("expected 1 service, got %d", r.Count())
	}
}

func TestServiceRegistry_DuplicateRegistration(t *testing.T) {
	r := NewServiceRegistry()

	entry := ServiceEntry{
		Name: "dup",
		Factory: func(deps ServiceDeps) (Service, error) {
			return &testService{name: "dup"}, nil
		},
	}

	_ = r.Register(entry)
	err := r.Register(entry)
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestServiceRegistry_RegisterValidation(t *testing.T) {
	r := NewServiceRegistry()

	// Missing name
	err := r.Register(ServiceEntry{
		Factory: func(deps ServiceDeps) (Service, error) {
			return &testService{}, nil
		},
	})
	if err == nil {
		t.Error("expected error for missing name")
	}

	// Missing factory
	err = r.Register(ServiceEntry{Name: "test"})
	if err == nil {
		t.Error("expected error for missing factory")
	}
}

func TestServiceRegistry_MustRegister(t *testing.T) {
	r := NewServiceRegistry()

	defer func() {
		if recover() == nil {
			t.Error("expected panic for duplicate MustRegister")
		}
	}()

	entry := ServiceEntry{
		Name: "panic-test",
		Factory: func(deps ServiceDeps) (Service, error) {
			return &testService{name: "panic-test"}, nil
		},
	}

	r.MustRegister(entry)
	r.MustRegister(entry) // Should panic
}

func TestServiceRegistry_Get(t *testing.T) {
	r := NewServiceRegistry()

	entry := ServiceEntry{
		Name: "get-test",
		Factory: func(deps ServiceDeps) (Service, error) {
			return &testService{name: "get-test"}, nil
		},
	}
	_ = r.Register(entry)

	got, ok := r.Get("get-test")
	if !ok {
		t.Fatal("service not found")
	}
	if got.Name != "get-test" {
		t.Errorf("expected name get-test, got %s", got.Name)
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent service")
	}
}

func TestServiceRegistry_List(t *testing.T) {
	r := NewServiceRegistry()

	for i := 0; i < 3; i++ {
		name := "svc-" + string(rune('a'+i))
		_ = r.Register(ServiceEntry{
			Name: name,
			Factory: func(deps ServiceDeps) (Service, error) {
				return &testService{name: name}, nil
			},
		})
	}

	entries := r.List()
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Verify registration order
	expected := []string{"svc-a", "svc-b", "svc-c"}
	for i, e := range entries {
		if e.Name != expected[i] {
			t.Errorf("entry %d: expected %s, got %s", i, expected[i], e.Name)
		}
	}
}

func TestServiceRegistry_ListByPriority(t *testing.T) {
	r := NewServiceRegistry()

	// Register in non-priority order
	priorities := []int{30, 10, 20}
	for i, p := range priorities {
		name := "svc-" + string(rune('a'+i))
		_ = r.Register(ServiceEntry{
			Name:     name,
			Priority: p,
			Factory: func(deps ServiceDeps) (Service, error) {
				return &testService{name: name}, nil
			},
		})
	}

	entries := r.ListByPriority()

	// Should be sorted by priority: 10, 20, 30
	expectedOrder := []int{10, 20, 30}
	for i, e := range entries {
		if e.Priority != expectedOrder[i] {
			t.Errorf("entry %d: expected priority %d, got %d", i, expectedOrder[i], e.Priority)
		}
	}
}

func TestServiceRegistry_Unregister(t *testing.T) {
	r := NewServiceRegistry()

	_ = r.Register(ServiceEntry{
		Name: "remove-me",
		Factory: func(deps ServiceDeps) (Service, error) {
			return &testService{name: "remove-me"}, nil
		},
	})

	if err := r.Unregister("remove-me"); err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if r.Count() != 0 {
		t.Error("expected 0 services after unregister")
	}

	// Unregister nonexistent
	if err := r.Unregister("nonexistent"); err == nil {
		t.Error("expected error for nonexistent service")
	}
}

func TestServiceRegistry_CreateServices(t *testing.T) {
	r := NewServiceRegistry()

	created := make([]string, 0)

	for _, name := range []string{"svc-a", "svc-b"} {
		n := name
		_ = r.Register(ServiceEntry{
			Name:      n,
			AutoStart: true,
			Factory: func(deps ServiceDeps) (Service, error) {
				created = append(created, n)
				return &testService{name: n}, nil
			},
		})
	}

	// Add non-autostart service
	_ = r.Register(ServiceEntry{
		Name:      "manual",
		AutoStart: false,
		Factory: func(deps ServiceDeps) (Service, error) {
			created = append(created, "manual")
			return &testService{name: "manual"}, nil
		},
	})

	services, err := r.CreateServices(mockDeps{})
	if err != nil {
		t.Fatalf("CreateServices failed: %v", err)
	}

	if len(services) != 2 {
		t.Errorf("expected 2 auto-start services, got %d", len(services))
	}

	// Manual service should not be created
	for _, c := range created {
		if c == "manual" {
			t.Error("manual service should not be created")
		}
	}
}

func TestServiceRegistry_CreateServicesFailure(t *testing.T) {
	r := NewServiceRegistry()

	// Required service that fails
	_ = r.Register(ServiceEntry{
		Name:      "fail",
		Required:  true,
		AutoStart: true,
		Factory: func(deps ServiceDeps) (Service, error) {
			return nil, errors.New("intentional failure")
		},
	})

	_, err := r.CreateServices(mockDeps{})
	if err == nil {
		t.Fatal("expected error for required service failure")
	}
}

func TestServiceRegistry_CollectDescriptors(t *testing.T) {
	r := NewServiceRegistry()

	desc := core.Descriptor{
		Name:   "with-desc",
		Domain: "test",
		Layer:  core.LayerService,
	}

	_ = r.Register(ServiceEntry{
		Name:       "with-desc",
		Descriptor: &desc,
		Factory: func(deps ServiceDeps) (Service, error) {
			return &testService{name: "with-desc"}, nil
		},
	})

	_ = r.Register(ServiceEntry{
		Name: "no-desc",
		Factory: func(deps ServiceDeps) (Service, error) {
			return &testService{name: "no-desc"}, nil
		},
	})

	descriptors := r.CollectDescriptors()
	if len(descriptors) != 1 {
		t.Errorf("expected 1 descriptor, got %d", len(descriptors))
	}
}

func TestServiceBuilder_Fluent(t *testing.T) {
	entry := NewServiceBuilder("fluent-svc").
		Domain("test").
		Priority(10).
		Required(true).
		AutoStart(true).
		DependsOn("dep1", "dep2").
		Factory(func(deps ServiceDeps) (Service, error) {
			return &testService{name: "fluent-svc"}, nil
		}).
		Build()

	if entry.Name != "fluent-svc" {
		t.Errorf("expected name fluent-svc, got %s", entry.Name)
	}
	if entry.Domain != "test" {
		t.Errorf("expected domain test, got %s", entry.Domain)
	}
	if entry.Priority != 10 {
		t.Errorf("expected priority 10, got %d", entry.Priority)
	}
	if len(entry.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(entry.DependsOn))
	}
}

func TestServiceContainer_Add(t *testing.T) {
	c := NewServiceContainer()

	svc := &testService{name: "container-test"}
	if err := c.Add(svc); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Duplicate
	if err := c.Add(svc); err == nil {
		t.Error("expected error for duplicate add")
	}

	// Nil
	if err := c.Add(nil); err == nil {
		t.Error("expected error for nil service")
	}
}

func TestServiceContainer_GetService(t *testing.T) {
	c := NewServiceContainer()
	svc := &testService{name: "lookup-test"}
	_ = c.Add(svc)

	got, ok := c.GetService("lookup-test")
	if !ok {
		t.Fatal("service not found")
	}
	if got.Name() != "lookup-test" {
		t.Errorf("expected name lookup-test, got %s", got.Name())
	}

	_, ok = c.GetService("missing")
	if ok {
		t.Error("expected not found for missing service")
	}
}

func TestServiceContainer_StartStopAll(t *testing.T) {
	c := NewServiceContainer()

	services := make([]*testService, 3)
	for i := 0; i < 3; i++ {
		services[i] = &testService{name: "svc-" + string(rune('a'+i))}
		_ = c.Add(services[i])
	}

	ctx := context.Background()

	if err := c.StartAll(ctx); err != nil {
		t.Fatalf("StartAll failed: %v", err)
	}

	for _, s := range services {
		if !s.started {
			t.Errorf("service %s not started", s.name)
		}
	}

	if err := c.StopAll(ctx); err != nil {
		t.Fatalf("StopAll failed: %v", err)
	}

	for _, s := range services {
		if !s.stopped {
			t.Errorf("service %s not stopped", s.name)
		}
	}
}

func TestServiceContainer_StartRollback(t *testing.T) {
	c := NewServiceContainer()

	svc1 := &testService{name: "svc-1"}
	svc2 := &testService{
		name: "svc-2",
		startFn: func() error {
			return errors.New("start failure")
		},
	}

	_ = c.Add(svc1)
	_ = c.Add(svc2)

	ctx := context.Background()
	err := c.StartAll(ctx)
	if err == nil {
		t.Fatal("expected error from StartAll")
	}

	// svc1 should be rolled back (stopped)
	if !svc1.stopped {
		t.Error("svc1 should have been stopped during rollback")
	}
}
