package framework

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/system/os/sandbox"
)

// =============================================================================
// SandboxedServiceEngine Tests
// =============================================================================

func TestNewSandboxedServiceEngine(t *testing.T) {
	eng := NewSandboxedServiceEngine(SandboxedServiceConfig{
		ServiceConfig: ServiceConfig{
			Name:        "test_service",
			Description: "Test service",
		},
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
			sandbox.CapStorageWrite,
		},
		StorageQuota: 1024 * 1024,
	})

	if eng.Name() != "test_service" {
		t.Errorf("expected name 'test_service', got %s", eng.Name())
	}

	if eng.SecurityLevel() != sandbox.SecurityLevelNormal {
		t.Errorf("expected security level normal, got %v", eng.SecurityLevel())
	}

	caps := eng.RequestedCapabilities()
	if len(caps) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(caps))
	}

	if eng.StorageQuota() != 1024*1024 {
		t.Errorf("expected storage quota 1MB, got %d", eng.StorageQuota())
	}
}

func TestSandboxedServiceEngineDefaultSecurityLevel(t *testing.T) {
	eng := NewSandboxedServiceEngine(SandboxedServiceConfig{
		ServiceConfig: ServiceConfig{
			Name: "test_service",
		},
		// SecurityLevel not set
	})

	if eng.SecurityLevel() != sandbox.SecurityLevelNormal {
		t.Errorf("expected default security level normal, got %v", eng.SecurityLevel())
	}
}

func TestSandboxedServiceEngineSimple(t *testing.T) {
	eng := NewSandboxedServiceEngineSimple("simple_service", "Simple test service", nil)

	if eng.Name() != "simple_service" {
		t.Errorf("expected name 'simple_service', got %s", eng.Name())
	}

	caps := eng.RequestedCapabilities()
	if len(caps) != 4 {
		t.Errorf("expected 4 default capabilities, got %d", len(caps))
	}
}

func TestSandboxedServiceEngineSandboxContext(t *testing.T) {
	eng := NewSandboxedServiceEngine(SandboxedServiceConfig{
		ServiceConfig: ServiceConfig{
			Name: "test_service",
		},
	})

	// Default should be nil context
	ctx := eng.SandboxContext()
	if ctx == nil {
		t.Error("sandbox context should not be nil")
	}

	// Nil context should allow all capabilities
	if !ctx.HasCapability(sandbox.CapStorageRead) {
		t.Error("nil context should allow all capabilities")
	}
}

func TestSandboxedServiceEngineWithSandbox(t *testing.T) {
	// Create sandbox manager and sandbox
	manager := sandbox.NewManager(nil, sandbox.DefaultManagerConfig())
	ctx := context.Background()

	sb, err := manager.CreateSandbox(ctx, sandbox.CreateSandboxRequest{
		ServiceID:     "sandboxed_service",
		PackageID:     "test_package",
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
			sandbox.CapBusPublish,
		},
		StorageQuota: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Create engine and inject sandbox
	eng := NewSandboxedServiceEngine(SandboxedServiceConfig{
		ServiceConfig: ServiceConfig{
			Name: "sandboxed_service",
		},
	})
	eng.InjectSandbox(sb)

	// Verify sandbox context
	sandboxCtx := eng.SandboxContext()
	if sandboxCtx.ServiceID() != "sandboxed_service" {
		t.Errorf("expected service ID 'sandboxed_service', got %s", sandboxCtx.ServiceID())
	}

	// Verify capabilities
	if !eng.HasCapability(sandbox.CapStorageRead) {
		t.Error("should have storage.read capability")
	}
	if !eng.HasCapability(sandbox.CapBusPublish) {
		t.Error("should have bus.publish capability")
	}
	if eng.HasCapability(sandbox.CapStorageWrite) {
		t.Error("should not have storage.write capability")
	}

	// Verify IPC and storage are set
	if eng.IPC() == nil {
		t.Error("IPC should be set")
	}
	if eng.IsolatedStorage() == nil {
		t.Error("isolated storage should be set")
	}
}

func TestSandboxedServiceEngineRequireCapability(t *testing.T) {
	manager := sandbox.NewManager(nil, sandbox.DefaultManagerConfig())
	ctx := context.Background()

	sb, err := manager.CreateSandbox(ctx, sandbox.CreateSandboxRequest{
		ServiceID:     "limited_service",
		PackageID:     "test_package",
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
		},
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	eng := NewSandboxedServiceEngine(SandboxedServiceConfig{
		ServiceConfig: ServiceConfig{
			Name: "limited_service",
		},
	})
	eng.InjectSandbox(sb)

	// RequireCapability should succeed for granted capability
	err = eng.RequireCapability(ctx, sandbox.CapStorageRead)
	if err != nil {
		t.Errorf("RequireCapability should succeed: %v", err)
	}

	// RequireCapability should fail for non-granted capability
	err = eng.RequireCapability(ctx, sandbox.CapStorageWrite)
	if err == nil {
		t.Error("RequireCapability should fail for non-granted capability")
	}

	// Error should be CapabilityDeniedError
	if _, ok := err.(*sandbox.CapabilityDeniedError); !ok {
		t.Errorf("expected CapabilityDeniedError, got %T", err)
	}
}

func TestSandboxedServiceEngineRequireCapabilities(t *testing.T) {
	manager := sandbox.NewManager(nil, sandbox.DefaultManagerConfig())
	ctx := context.Background()

	sb, err := manager.CreateSandbox(ctx, sandbox.CreateSandboxRequest{
		ServiceID:     "multi_cap_service",
		PackageID:     "test_package",
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
			sandbox.CapStorageWrite,
		},
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	eng := NewSandboxedServiceEngine(SandboxedServiceConfig{
		ServiceConfig: ServiceConfig{
			Name: "multi_cap_service",
		},
	})
	eng.InjectSandbox(sb)

	// Should succeed when all capabilities are granted
	err = eng.RequireCapabilities(ctx, sandbox.CapStorageRead, sandbox.CapStorageWrite)
	if err != nil {
		t.Errorf("RequireCapabilities should succeed: %v", err)
	}

	// Should fail when any capability is missing
	err = eng.RequireCapabilities(ctx, sandbox.CapStorageRead, sandbox.CapBusPublish)
	if err == nil {
		t.Error("RequireCapabilities should fail when any capability is missing")
	}
}

func TestSandboxedServiceEngineCreateSandboxRequest(t *testing.T) {
	eng := NewSandboxedServiceEngine(SandboxedServiceConfig{
		ServiceConfig: ServiceConfig{
			Name:   "request_service",
			Domain: "test_domain",
		},
		SecurityLevel: sandbox.SecurityLevelPrivileged,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
			sandbox.CapDatabaseRead,
		},
		StorageQuota: 2 * 1024 * 1024,
	})

	req := eng.CreateSandboxRequest()

	if req.ServiceID != "request_service" {
		t.Errorf("expected service ID 'request_service', got %s", req.ServiceID)
	}
	if req.PackageID != "test_domain" {
		t.Errorf("expected package ID 'test_domain', got %s", req.PackageID)
	}
	if req.SecurityLevel != sandbox.SecurityLevelPrivileged {
		t.Errorf("expected privileged security level, got %v", req.SecurityLevel)
	}
	if len(req.RequestedCapabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(req.RequestedCapabilities))
	}
	if req.StorageQuota != 2*1024*1024 {
		t.Errorf("expected 2MB quota, got %d", req.StorageQuota)
	}
}

func TestSandboxedServiceEngineInjectSandboxNil(t *testing.T) {
	eng := NewSandboxedServiceEngine(SandboxedServiceConfig{
		ServiceConfig: ServiceConfig{
			Name: "nil_sandbox_service",
		},
	})

	// Inject nil sandbox
	eng.InjectSandbox(nil)

	// Should have nil context (allows all)
	ctx := eng.SandboxContext()
	if ctx == nil {
		t.Error("sandbox context should not be nil")
	}

	// IPC and storage should be nil
	if eng.IPC() != nil {
		t.Error("IPC should be nil")
	}
	if eng.IsolatedStorage() != nil {
		t.Error("isolated storage should be nil")
	}
}

func TestSandboxedServiceEngineSetSandboxContext(t *testing.T) {
	eng := NewSandboxedServiceEngine(SandboxedServiceConfig{
		ServiceConfig: ServiceConfig{
			Name: "context_service",
		},
	})

	// Create a sandbox and context
	manager := sandbox.NewManager(nil, sandbox.DefaultManagerConfig())
	ctx := context.Background()

	sb, err := manager.CreateSandbox(ctx, sandbox.CreateSandboxRequest{
		ServiceID:     "context_service",
		PackageID:     "test_package",
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
		},
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	sandboxCtx := NewSandboxContext(sb)
	eng.SetSandboxContext(sandboxCtx)

	if eng.SandboxContext().ServiceID() != "context_service" {
		t.Errorf("expected service ID 'context_service', got %s", eng.SandboxContext().ServiceID())
	}
}

func TestSandboxedServiceEngineSetSandboxContextNil(t *testing.T) {
	eng := NewSandboxedServiceEngine(SandboxedServiceConfig{
		ServiceConfig: ServiceConfig{
			Name: "nil_context_service",
		},
	})

	// Set nil context
	eng.SetSandboxContext(nil)

	// Should fall back to nil context
	ctx := eng.SandboxContext()
	if ctx == nil {
		t.Error("sandbox context should not be nil")
	}

	// Nil context allows all capabilities
	if !ctx.HasCapability(sandbox.CapSystemAdmin) {
		t.Error("nil context should allow all capabilities")
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkSandboxedServiceEngineHasCapability(b *testing.B) {
	manager := sandbox.NewManager(nil, sandbox.DefaultManagerConfig())
	ctx := context.Background()

	sb, _ := manager.CreateSandbox(ctx, sandbox.CreateSandboxRequest{
		ServiceID:     "bench_service",
		PackageID:     "bench_package",
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
			sandbox.CapStorageWrite,
			sandbox.CapBusPublish,
		},
	})

	eng := NewSandboxedServiceEngine(SandboxedServiceConfig{
		ServiceConfig: ServiceConfig{
			Name: "bench_service",
		},
	})
	eng.InjectSandbox(sb)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng.HasCapability(sandbox.CapStorageRead)
	}
}

func BenchmarkSandboxedServiceEngineRequireCapability(b *testing.B) {
	manager := sandbox.NewManager(nil, sandbox.DefaultManagerConfig())
	ctx := context.Background()

	sb, _ := manager.CreateSandbox(ctx, sandbox.CreateSandboxRequest{
		ServiceID:     "bench_service",
		PackageID:     "bench_package",
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
		},
	})

	eng := NewSandboxedServiceEngine(SandboxedServiceConfig{
		ServiceConfig: ServiceConfig{
			Name: "bench_service",
		},
	})
	eng.InjectSandbox(sb)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = eng.RequireCapability(ctx, sandbox.CapStorageRead)
	}
}
