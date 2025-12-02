package framework

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/system/os/sandbox"
)

// =============================================================================
// SandboxContext Tests
// =============================================================================

func TestNilSandboxContext(t *testing.T) {
	ctx := NilSandboxContext()

	// Should return default identity
	identity := ctx.Identity()
	if identity == nil {
		t.Error("identity should not be nil")
	}
	if identity.ServiceID != "unknown" {
		t.Errorf("expected service ID 'unknown', got %s", identity.ServiceID)
	}

	// Should always allow capabilities (backward compatibility)
	if !ctx.HasCapability(sandbox.CapStorageRead) {
		t.Error("nil context should allow all capabilities")
	}

	// CheckCapability should not return error
	err := ctx.CheckCapability(context.Background(), sandbox.CapStorageWrite)
	if err != nil {
		t.Errorf("nil context CheckCapability should not error: %v", err)
	}

	// Security level should be normal
	if ctx.SecurityLevel() != sandbox.SecurityLevelNormal {
		t.Errorf("expected normal security level, got %v", ctx.SecurityLevel())
	}
}

func TestSandboxContextFromServiceSandbox(t *testing.T) {
	// Create a sandbox manager and sandbox
	manager := sandbox.NewManager(nil, sandbox.DefaultManagerConfig())
	ctx := context.Background()

	req := sandbox.CreateSandboxRequest{
		ServiceID:     "test_service",
		PackageID:     "test_package",
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
			sandbox.CapBusPublish,
		},
	}

	sb, err := manager.CreateSandbox(ctx, req)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Create sandbox context
	sandboxCtx := NewSandboxContext(sb)

	// Verify identity
	identity := sandboxCtx.Identity()
	if identity.ServiceID != "test_service" {
		t.Errorf("expected service ID 'test_service', got %s", identity.ServiceID)
	}

	// Verify capabilities
	if !sandboxCtx.HasCapability(sandbox.CapStorageRead) {
		t.Error("should have storage.read capability")
	}
	if !sandboxCtx.HasCapability(sandbox.CapBusPublish) {
		t.Error("should have bus.publish capability")
	}
	if sandboxCtx.HasCapability(sandbox.CapSystemAdmin) {
		t.Error("should not have system.admin capability")
	}

	// Verify security level
	if sandboxCtx.SecurityLevel() != sandbox.SecurityLevelNormal {
		t.Errorf("expected normal security level, got %v", sandboxCtx.SecurityLevel())
	}

	// Verify service ID
	if sandboxCtx.ServiceID() != "test_service" {
		t.Errorf("expected service ID 'test_service', got %s", sandboxCtx.ServiceID())
	}
}

// =============================================================================
// SandboxedServiceBase Tests
// =============================================================================

func TestSandboxedServiceBase(t *testing.T) {
	base := NewSandboxedServiceBase("test_service", "test_domain")

	// Verify basic properties
	if base.Name() != "test_service" {
		t.Errorf("expected name 'test_service', got %s", base.Name())
	}
	if base.Domain() != "test_domain" {
		t.Errorf("expected domain 'test_domain', got %s", base.Domain())
	}

	// Default sandbox context should be nil context
	ctx := base.SandboxContext()
	if ctx == nil {
		t.Error("sandbox context should not be nil")
	}

	// Should have all capabilities by default (nil context)
	if !base.HasCapability(sandbox.CapStorageRead) {
		t.Error("should have capability with nil context")
	}
}

func TestSandboxedServiceBaseWithSandbox(t *testing.T) {
	// Create sandbox
	manager := sandbox.NewManager(nil, sandbox.DefaultManagerConfig())
	ctx := context.Background()

	req := sandbox.CreateSandboxRequest{
		ServiceID:     "sandboxed_service",
		PackageID:     "sandboxed_package",
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
		},
	}

	sb, err := manager.CreateSandbox(ctx, req)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Create service base and inject sandbox
	base := NewSandboxedServiceBase("sandboxed_service", "test_domain")
	sandboxCtx := NewSandboxContext(sb)
	base.SetSandboxContext(sandboxCtx)

	// Verify sandbox context is set
	if base.SandboxContext().ServiceID() != "sandboxed_service" {
		t.Errorf("expected service ID 'sandboxed_service', got %s", base.SandboxContext().ServiceID())
	}

	// Verify capability checks work
	if !base.HasCapability(sandbox.CapStorageRead) {
		t.Error("should have storage.read capability")
	}
	if base.HasCapability(sandbox.CapStorageWrite) {
		t.Error("should not have storage.write capability")
	}

	// Verify CheckCapability
	err = base.CheckCapability(ctx, sandbox.CapStorageRead)
	if err != nil {
		t.Errorf("CheckCapability should succeed for granted capability: %v", err)
	}

	err = base.CheckCapability(ctx, sandbox.CapStorageWrite)
	if err == nil {
		t.Error("CheckCapability should fail for non-granted capability")
	}
}

func TestSandboxedServiceBaseRequireCapability(t *testing.T) {
	// Create sandbox with limited capabilities
	manager := sandbox.NewManager(nil, sandbox.DefaultManagerConfig())
	ctx := context.Background()

	req := sandbox.CreateSandboxRequest{
		ServiceID:     "limited_service",
		PackageID:     "limited_package",
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
		},
	}

	sb, err := manager.CreateSandbox(ctx, req)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	base := NewSandboxedServiceBase("limited_service", "test_domain")
	base.SetSandboxContext(NewSandboxContext(sb))

	// RequireCapability should succeed for granted capability
	err = base.RequireCapability(ctx, sandbox.CapStorageRead)
	if err != nil {
		t.Errorf("RequireCapability should succeed: %v", err)
	}

	// RequireCapability should fail for non-granted capability
	err = base.RequireCapability(ctx, sandbox.CapStorageWrite)
	if err == nil {
		t.Error("RequireCapability should fail for non-granted capability")
	}

	// Error should be CapabilityDeniedError
	if _, ok := err.(*sandbox.CapabilityDeniedError); !ok {
		t.Errorf("expected CapabilityDeniedError, got %T", err)
	}
}

// =============================================================================
// InjectSandboxContext Tests
// =============================================================================

type mockSandboxAwareService struct {
	ServiceBase
	sandboxCtx      SandboxContext
	ipc             *sandbox.IPCProxy
	isolatedStorage *sandbox.IsolatedStorage
}

func (s *mockSandboxAwareService) SetSandboxContext(ctx SandboxContext) {
	s.sandboxCtx = ctx
}

func (s *mockSandboxAwareService) SetIPCProxy(proxy *sandbox.IPCProxy) {
	s.ipc = proxy
}

func (s *mockSandboxAwareService) SetIsolatedStorage(storage *sandbox.IsolatedStorage) {
	s.isolatedStorage = storage
}

func TestInjectSandboxContext(t *testing.T) {
	// Create sandbox
	manager := sandbox.NewManager(nil, sandbox.DefaultManagerConfig())
	ctx := context.Background()

	req := sandbox.CreateSandboxRequest{
		ServiceID:     "injectable_service",
		PackageID:     "injectable_package",
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
			sandbox.CapStorageWrite,
		},
		StorageQuota: 1024 * 1024,
	}

	sb, err := manager.CreateSandbox(ctx, req)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Create mock service
	service := &mockSandboxAwareService{}

	// Inject sandbox context
	InjectSandboxContext(service, sb)

	// Verify sandbox context was injected
	if service.sandboxCtx == nil {
		t.Error("sandbox context should be injected")
	}
	if service.sandboxCtx.ServiceID() != "injectable_service" {
		t.Errorf("expected service ID 'injectable_service', got %s", service.sandboxCtx.ServiceID())
	}

	// Verify IPC was injected
	if service.ipc == nil {
		t.Error("IPC proxy should be injected")
	}

	// Verify isolated storage was injected
	if service.isolatedStorage == nil {
		t.Error("isolated storage should be injected")
	}
}

func TestInjectSandboxContextNilSandbox(t *testing.T) {
	service := &mockSandboxAwareService{}

	// Inject with nil sandbox
	InjectSandboxContext(service, nil)

	// Should get nil sandbox context
	if service.sandboxCtx == nil {
		t.Error("sandbox context should be injected (nil context)")
	}

	// IPC and storage should remain nil
	if service.ipc != nil {
		t.Error("IPC should be nil for nil sandbox")
	}
	if service.isolatedStorage != nil {
		t.Error("isolated storage should be nil for nil sandbox")
	}
}

// =============================================================================
// SandboxedEnvironment Tests
// =============================================================================

func TestNewSandboxedEnvironment(t *testing.T) {
	// Create sandbox
	manager := sandbox.NewManager(nil, sandbox.DefaultManagerConfig())
	ctx := context.Background()

	req := sandbox.CreateSandboxRequest{
		ServiceID:     "env_service",
		PackageID:     "env_package",
		SecurityLevel: sandbox.SecurityLevelPrivileged,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
			sandbox.CapBusPublish,
		},
		StorageQuota: 1024 * 1024,
	}

	sb, err := manager.CreateSandbox(ctx, req)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Create base environment
	baseEnv := Environment{
		Config: ConfigMap{"key": "value"},
	}

	// Create sandboxed environment
	sandboxedEnv := NewSandboxedEnvironment(baseEnv, sb)

	// Verify base environment is preserved
	if val, ok := sandboxedEnv.Config.Get("key"); !ok || val != "value" {
		t.Error("base environment config should be preserved")
	}

	// Verify sandbox context
	if sandboxedEnv.SandboxCtx == nil {
		t.Error("sandbox context should be set")
	}
	if sandboxedEnv.SandboxCtx.ServiceID() != "env_service" {
		t.Errorf("expected service ID 'env_service', got %s", sandboxedEnv.SandboxCtx.ServiceID())
	}

	// Verify IPC
	if sandboxedEnv.IPC == nil {
		t.Error("IPC should be set")
	}

	// Verify isolated storage
	if sandboxedEnv.IsolatedStorage == nil {
		t.Error("isolated storage should be set")
	}
}

func TestNewSandboxedEnvironmentNilSandbox(t *testing.T) {
	baseEnv := Environment{
		Config: ConfigMap{"key": "value"},
	}

	sandboxedEnv := NewSandboxedEnvironment(baseEnv, nil)

	// Base environment should be preserved
	if val, ok := sandboxedEnv.Config.Get("key"); !ok || val != "value" {
		t.Error("base environment config should be preserved")
	}

	// Sandbox context should be nil context
	if sandboxedEnv.SandboxCtx == nil {
		t.Error("sandbox context should be set (nil context)")
	}

	// IPC and storage should be nil
	if sandboxedEnv.IPC != nil {
		t.Error("IPC should be nil for nil sandbox")
	}
	if sandboxedEnv.IsolatedStorage != nil {
		t.Error("isolated storage should be nil for nil sandbox")
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkHasCapability(b *testing.B) {
	manager := sandbox.NewManager(nil, sandbox.DefaultManagerConfig())
	ctx := context.Background()

	req := sandbox.CreateSandboxRequest{
		ServiceID:     "bench_service",
		PackageID:     "bench_package",
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
			sandbox.CapStorageWrite,
			sandbox.CapBusPublish,
		},
	}

	sb, _ := manager.CreateSandbox(ctx, req)
	sandboxCtx := NewSandboxContext(sb)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sandboxCtx.HasCapability(sandbox.CapStorageRead)
	}
}

func BenchmarkCheckCapability(b *testing.B) {
	manager := sandbox.NewManager(nil, sandbox.DefaultManagerConfig())
	ctx := context.Background()

	req := sandbox.CreateSandboxRequest{
		ServiceID:     "bench_service",
		PackageID:     "bench_package",
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
		},
	}

	sb, _ := manager.CreateSandbox(ctx, req)
	sandboxCtx := NewSandboxContext(sb)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sandboxCtx.CheckCapability(ctx, sandbox.CapStorageRead)
	}
}
