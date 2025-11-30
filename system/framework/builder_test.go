package framework

import (
	"context"
	"errors"
	"testing"

	service "github.com/R3E-Network/service_layer/system/framework/core"
)

func TestServiceBuilder_MinimalBuild(t *testing.T) {
	svc, err := NewService("test-service", "test").Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svc.Name() != "test-service" {
		t.Errorf("expected name 'test-service', got %q", svc.Name())
	}
	if svc.Domain() != "test" {
		t.Errorf("expected domain 'test', got %q", svc.Domain())
	}
}

func TestServiceBuilder_MissingName(t *testing.T) {
	_, err := NewService("", "domain").Build()
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !errors.Is(err, service.ErrInvalidManifest) {
		t.Errorf("expected ErrInvalidManifest, got %v", err)
	}
}

func TestServiceBuilder_MissingDomain(t *testing.T) {
	_, err := NewService("name", "").Build()
	if err == nil {
		t.Fatal("expected error for missing domain")
	}
	if !errors.Is(err, service.ErrInvalidManifest) {
		t.Errorf("expected ErrInvalidManifest, got %v", err)
	}
}

func TestServiceBuilder_WithOptions(t *testing.T) {
	svc, err := NewService("my-service", "my-domain").
		WithDescription("Test service").
		WithLayer("service").
		WithCapabilities("cap1", "cap2").
		DependsOn("dep1", "dep2").
		RequiresAPI("store", "compute").
		WithQuotas(map[string]string{"gas": "1000"}).
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := svc.Manifest()
	if m.Description != "Test service" {
		t.Errorf("expected description 'Test service', got %q", m.Description)
	}
	if m.Layer != "service" {
		t.Errorf("expected layer 'service', got %q", m.Layer)
	}
	if len(m.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(m.Capabilities))
	}
	if len(m.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(m.DependsOn))
	}
	if len(m.RequiresAPIs) != 2 {
		t.Errorf("expected 2 required APIs, got %d", len(m.RequiresAPIs))
	}
	if m.Quotas["gas"] != "1000" {
		t.Errorf("expected quota gas=1000, got %s", m.Quotas["gas"])
	}
}

func TestServiceBuilder_Lifecycle(t *testing.T) {
	var order []string

	svc, err := NewService("lifecycle-test", "test").
		OnPreStart(func(ctx context.Context) error {
			order = append(order, "pre-start")
			return nil
		}).
		OnStart(func(ctx context.Context) error {
			order = append(order, "start")
			return nil
		}).
		OnPostStart(func(ctx context.Context) error {
			order = append(order, "post-start")
			return nil
		}).
		OnPreStop(func(ctx context.Context) error {
			order = append(order, "pre-stop")
			return nil
		}).
		OnStop(func(ctx context.Context) error {
			order = append(order, "stop")
			return nil
		}).
		OnPostStop(func(ctx context.Context) error {
			order = append(order, "post-stop")
			return nil
		}).
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	if !svc.IsStarted() {
		t.Error("expected service to be started")
	}

	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	if svc.IsStarted() {
		t.Error("expected service to be stopped")
	}

	expected := []string{"pre-start", "start", "post-start", "pre-stop", "stop", "post-stop"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d lifecycle events, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected order[%d]=%q, got %q", i, v, order[i])
		}
	}
}

func TestServiceBuilder_StartError(t *testing.T) {
	expectedErr := errors.New("start failed")

	svc, _ := NewService("error-test", "test").
		OnStart(func(ctx context.Context) error {
			return expectedErr
		}).
		Build()

	err := svc.Start(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServiceBuilder_PreStartHookError(t *testing.T) {
	expectedErr := errors.New("pre-start failed")
	startCalled := false

	svc, _ := NewService("hook-error-test", "test").
		OnPreStart(func(ctx context.Context) error {
			return expectedErr
		}).
		OnStart(func(ctx context.Context) error {
			startCalled = true
			return nil
		}).
		Build()

	err := svc.Start(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if startCalled {
		t.Error("start should not be called when pre-start fails")
	}
}

func TestServiceBuilder_ReadyCheck(t *testing.T) {
	customCheckCalled := false

	svc, _ := NewService("ready-test", "test").
		WithReadyCheck(func(ctx context.Context) error {
			customCheckCalled = true
			return nil
		}).
		Build()

	// Not started yet - base check fails
	err := svc.Ready(context.Background())
	if err == nil {
		t.Error("expected not-ready error when not started")
	}

	// Start the service
	_ = svc.Start(context.Background())

	// Now should be ready and custom check should run
	err = svc.Ready(context.Background())
	if err != nil {
		t.Errorf("unexpected ready error: %v", err)
	}
	if !customCheckCalled {
		t.Error("expected custom ready check to be called")
	}
}

func TestServiceBuilder_DoubleStart(t *testing.T) {
	svc, _ := NewService("double-start", "test").Build()

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("first start failed: %v", err)
	}

	err := svc.Start(context.Background())
	if !errors.Is(err, service.ErrServiceAlreadyStarted) {
		t.Errorf("expected service.ErrServiceAlreadyStarted, got %v", err)
	}
}

func TestServiceBuilder_StopWithoutStart(t *testing.T) {
	svc, _ := NewService("stop-test", "test").Build()

	// Stop without start should not error
	err := svc.Stop(context.Background())
	if err != nil {
		t.Errorf("stop without start should not error: %v", err)
	}
}

func TestServiceBuilder_MustBuild(t *testing.T) {
	// Should not panic
	svc := NewService("must-build", "test").MustBuild()
	if svc.Name() != "must-build" {
		t.Error("MustBuild failed")
	}
}

func TestServiceBuilder_MustBuildPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid service")
		}
	}()

	NewService("", "test").MustBuild()
}

func TestServiceBuilder_WithVersion(t *testing.T) {
	svc, err := NewService("test", "domain").
		WithVersion("1.2.3").
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svc.Version() != "1.2.3" {
		t.Errorf("Version() = %q, want '1.2.3'", svc.Version())
	}
	if svc.Manifest().Version != "1.2.3" {
		t.Errorf("Manifest().Version = %q, want '1.2.3'", svc.Manifest().Version)
	}
}

func TestServiceBuilder_WithTags(t *testing.T) {
	svc, _ := NewService("test", "domain").
		WithTags(map[string]string{"env": "prod"}).
		WithTag("tier", "premium").
		Build()

	m := svc.Manifest()
	if v, ok := m.GetTag("env"); !ok || v != "prod" {
		t.Errorf("GetTag(env) = %q, %v; want 'prod', true", v, ok)
	}
	if v, ok := m.GetTag("tier"); !ok || v != "premium" {
		t.Errorf("GetTag(tier) = %q, %v; want 'premium', true", v, ok)
	}
}

func TestServiceBuilder_WithQuota(t *testing.T) {
	svc, _ := NewService("test", "domain").
		WithQuota("gas", "1000").
		WithQuota("rpc", "500").
		Build()

	m := svc.Manifest()
	if v, ok := m.GetQuota("gas"); !ok || v != "1000" {
		t.Errorf("GetQuota(gas) = %q, %v; want '1000', true", v, ok)
	}
	if v, ok := m.GetQuota("rpc"); !ok || v != "500" {
		t.Errorf("GetQuota(rpc) = %q, %v; want '500', true", v, ok)
	}
}

func TestServiceBuilder_Enabled(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		svc, _ := NewService("test", "domain").
			Enabled(true).
			Build()

		if !svc.IsEnabled() {
			t.Error("service should be enabled")
		}
	})

	t.Run("disabled", func(t *testing.T) {
		svc, _ := NewService("test", "domain").
			Enabled(false).
			Build()

		if svc.IsEnabled() {
			t.Error("service should be disabled")
		}
	})
}

func TestServiceBuilder_MergeManifest(t *testing.T) {
	override := &Manifest{
		Description:  "Overridden description",
		Version:      "2.0.0",
		Capabilities: []string{"extra-cap"},
	}

	svc, _ := NewService("test", "domain").
		WithDescription("Original").
		WithCapabilities("cap1").
		MergeManifest(override).
		Build()

	m := svc.Manifest()
	if m.Description != "Overridden description" {
		t.Errorf("Description = %q, want 'Overridden description'", m.Description)
	}
	if m.Version != "2.0.0" {
		t.Errorf("Version = %q, want '2.0.0'", m.Version)
	}
	// Capabilities should be merged (cap1 + extra-cap)
	if len(m.Capabilities) != 2 {
		t.Errorf("Capabilities len = %d, want 2", len(m.Capabilities))
	}
}

func TestServiceBuilder_MergeManifest_ChangesName(t *testing.T) {
	override := &Manifest{
		Name:   "new-name",
		Domain: "new-domain",
	}

	svc, _ := NewService("old-name", "old-domain").
		MergeManifest(override).
		Build()

	if svc.Name() != "new-name" {
		t.Errorf("Name() = %q, want 'new-name'", svc.Name())
	}
	if svc.Domain() != "new-domain" {
		t.Errorf("Domain() = %q, want 'new-domain'", svc.Domain())
	}
}

func TestServiceBuilder_WithValidator(t *testing.T) {
	validator := ManifestValidatorFunc(func(m *Manifest) error {
		if m.Version == "" {
			return errors.New("version required")
		}
		return nil
	})

	t.Run("validation fails", func(t *testing.T) {
		_, err := NewService("test", "domain").
			WithValidator(validator).
			Build()

		if err == nil {
			t.Fatal("expected validation error")
		}
	})

	t.Run("validation passes", func(t *testing.T) {
		svc, err := NewService("test", "domain").
			WithVersion("1.0.0").
			WithValidator(validator).
			Build()

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if svc.Version() != "1.0.0" {
			t.Error("service should be built")
		}
	})
}

func TestServiceBuilder_WithValidatorFunc(t *testing.T) {
	_, err := NewService("test", "domain").
		WithValidatorFunc(func(m *Manifest) error {
			if m.Description == "" {
				return errors.New("description required")
			}
			return nil
		}).
		Build()

	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestBuiltService_Descriptor(t *testing.T) {
	svc, _ := NewService("test-service", "test-domain").
		WithLayer("runner").
		WithCapabilities("cap1", "cap2").
		DependsOn("dep1").
		RequiresAPI("store").
		Build()

	d := svc.Descriptor()

	if d.Name != "test-service" {
		t.Errorf("Descriptor.Name = %q, want 'test-service'", d.Name)
	}
	if d.Domain != "test-domain" {
		t.Errorf("Descriptor.Domain = %q, want 'test-domain'", d.Domain)
	}
	if string(d.Layer) != "runner" {
		t.Errorf("Descriptor.Layer = %q, want 'runner'", d.Layer)
	}
	if len(d.Capabilities) != 2 {
		t.Errorf("Descriptor.Capabilities len = %d, want 2", len(d.Capabilities))
	}
}

func TestBuiltService_Description(t *testing.T) {
	svc, _ := NewService("test", "domain").
		WithDescription("Test service description").
		Build()

	if svc.Description() != "Test service description" {
		t.Errorf("Description() = %q, want 'Test service description'", svc.Description())
	}
}
