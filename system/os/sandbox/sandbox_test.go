package sandbox

import (
	"context"
	"testing"
	"time"
)

// =============================================================================
// Service Identity Tests
// =============================================================================

func TestGenerateServiceID(t *testing.T) {
	id1 := GenerateServiceID("com.r3e.services.accounts", "v1.0.0")
	id2 := GenerateServiceID("com.r3e.services.accounts", "v1.0.0")
	id3 := GenerateServiceID("com.r3e.services.secrets", "v1.0.0")

	// Same inputs should produce same ID
	if id1 != id2 {
		t.Errorf("same inputs should produce same ID: %s != %s", id1, id2)
	}

	// Different inputs should produce different ID
	if id1 == id3 {
		t.Errorf("different inputs should produce different ID: %s == %s", id1, id3)
	}

	// ID should not be empty
	if id1 == "" {
		t.Error("generated ID should not be empty")
	}
}

func TestGenerateProcessID(t *testing.T) {
	pid1 := GenerateProcessID()
	pid2 := GenerateProcessID()

	// Each call should produce unique ID
	if pid1 == pid2 {
		t.Errorf("process IDs should be unique: %s == %s", pid1, pid2)
	}

	// ID should not be empty
	if pid1 == "" {
		t.Error("process ID should not be empty")
	}
}

// =============================================================================
// Capability Tests
// =============================================================================

func TestCapabilitySet(t *testing.T) {
	caps := NewCapabilitySet()

	// Initially empty
	if caps.Has(CapStorageRead) {
		t.Error("new capability set should not have any capabilities")
	}

	// Grant capability
	caps.Grant(CapStorageRead, "test")
	if !caps.Has(CapStorageRead) {
		t.Error("capability should be granted")
	}

	// Revoke capability
	caps.Revoke(CapStorageRead)
	if caps.Has(CapStorageRead) {
		t.Error("capability should be revoked")
	}

	// List capabilities
	caps.Grant(CapStorageRead, "test")
	caps.Grant(CapStorageWrite, "test")
	list := caps.List()
	if len(list) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(list))
	}
}

func TestCapabilitySetMultiple(t *testing.T) {
	caps := NewCapabilitySet()

	// Grant multiple capabilities
	caps.Grant(CapStorageRead, "test")
	caps.Grant(CapStorageWrite, "test")
	caps.Grant(CapBusPublish, "test")

	// Check all are present
	if !caps.Has(CapStorageRead) || !caps.Has(CapStorageWrite) || !caps.Has(CapBusPublish) {
		t.Error("all granted capabilities should be present")
	}

	// Check non-granted capability
	if caps.Has(CapSystemAdmin) {
		t.Error("non-granted capability should not be present")
	}
}

// =============================================================================
// Security Policy Tests
// =============================================================================

func TestSecurityPolicy(t *testing.T) {
	policy := NewSecurityPolicy()

	// Test default deny behavior
	effect := policy.Evaluate("service_a", "storage:service_a/config", "read")
	if effect != PolicyEffectDeny {
		t.Errorf("default should be deny, got %s", effect)
	}
}

// =============================================================================
// Security Auditor Tests
// =============================================================================

func TestSecurityAuditor(t *testing.T) {
	auditor := NewSecurityAuditor(100)

	ctx := context.Background()
	identity := &ServiceIdentity{
		ServiceID: "test_service",
	}

	// Log some events
	auditor.LogCapabilityCheck(ctx, identity, CapStorageRead, true)
	auditor.LogResourceAccess(ctx, "test_service", "storage:test", "read", true)
	auditor.LogIPCCall(ctx, "service_a", "service_b", "method", true)

	// Get events
	events := auditor.GetEvents(10)
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// Check event types
	eventTypes := make(map[string]bool)
	for _, e := range events {
		eventTypes[e.EventType] = true
	}
	if !eventTypes["capability_check"] || !eventTypes["resource_access"] || !eventTypes["ipc_call"] {
		t.Error("missing expected event types")
	}
}

func TestSecurityAuditorMaxEvents(t *testing.T) {
	maxEvents := 5
	auditor := NewSecurityAuditor(maxEvents)

	ctx := context.Background()
	identity := &ServiceIdentity{ServiceID: "test"}

	// Log more events than max
	for i := 0; i < 10; i++ {
		auditor.LogCapabilityCheck(ctx, identity, CapStorageRead, true)
	}

	// Should only keep max events
	events := auditor.GetEvents(100)
	if len(events) > maxEvents {
		t.Errorf("expected at most %d events, got %d", maxEvents, len(events))
	}
}

// =============================================================================
// Sandbox Context Tests
// =============================================================================

func TestSandboxContext(t *testing.T) {
	identity := &ServiceIdentity{
		ServiceID:     "test_service",
		SecurityLevel: SecurityLevelNormal,
	}
	caps := NewCapabilitySet()
	caps.Grant(CapStorageRead, "test")

	policy := NewSecurityPolicy()
	auditor := NewSecurityAuditor(100)

	ctx := NewSandboxContext(identity, caps, policy, auditor)

	// Check capability that exists
	err := ctx.CheckCapability(context.Background(), CapStorageRead)
	if err != nil {
		t.Errorf("should have storage.read capability: %v", err)
	}

	// Check capability that doesn't exist
	err = ctx.CheckCapability(context.Background(), CapStorageWrite)
	if err == nil {
		t.Error("should not have storage.write capability")
	}
}

// =============================================================================
// Capability Denied Error Tests
// =============================================================================

func TestCapabilityDeniedError(t *testing.T) {
	err := &CapabilityDeniedError{
		ServiceID:  "test_service",
		Capability: CapStorageWrite,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}

// =============================================================================
// Manager Tests
// =============================================================================

func TestManagerCreateSandbox(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())

	ctx := context.Background()
	req := CreateSandboxRequest{
		ServiceID:     "com.r3e.services.test",
		PackageID:     "com.r3e.services.test",
		SecurityLevel: SecurityLevelNormal,
		RequestedCapabilities: []Capability{
			CapStorageRead,
			CapStorageWrite,
			CapBusPublish,
		},
		StorageQuota: 10 * 1024 * 1024, // 10MB
	}

	sandbox, err := manager.CreateSandbox(ctx, req)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Verify sandbox properties
	if sandbox.Identity.ServiceID != req.ServiceID {
		t.Errorf("service ID mismatch: %s != %s", sandbox.Identity.ServiceID, req.ServiceID)
	}

	if sandbox.Identity.SecurityLevel != SecurityLevelNormal {
		t.Errorf("security level mismatch: %v != %v", sandbox.Identity.SecurityLevel, SecurityLevelNormal)
	}

	// Verify capabilities were granted
	if !sandbox.Caps.Has(CapStorageRead) || !sandbox.Caps.Has(CapStorageWrite) {
		t.Error("expected capabilities not granted")
	}

	// Verify storage was created
	if sandbox.Storage == nil {
		t.Error("storage should be created")
	}
}

func TestManagerGetSandbox(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	// Create sandbox
	req := CreateSandboxRequest{
		ServiceID:     "test_service",
		PackageID:     "test_package",
		SecurityLevel: SecurityLevelNormal,
	}
	_, err := manager.CreateSandbox(ctx, req)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Get existing sandbox
	sandbox, err := manager.GetSandbox("test_service")
	if err != nil {
		t.Errorf("failed to get sandbox: %v", err)
	}
	if sandbox == nil {
		t.Error("sandbox should not be nil")
	}

	// Get non-existing sandbox
	_, err = manager.GetSandbox("non_existing")
	if err == nil {
		t.Error("should fail for non-existing sandbox")
	}
}

func TestManagerDestroySandbox(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	// Create sandbox
	req := CreateSandboxRequest{
		ServiceID:     "test_service",
		PackageID:     "test_package",
		SecurityLevel: SecurityLevelNormal,
	}
	_, err := manager.CreateSandbox(ctx, req)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Destroy sandbox
	err = manager.DestroySandbox(ctx, "test_service")
	if err != nil {
		t.Errorf("failed to destroy sandbox: %v", err)
	}

	// Verify sandbox is gone
	_, err = manager.GetSandbox("test_service")
	if err == nil {
		t.Error("sandbox should be destroyed")
	}
}

func TestManagerDuplicateSandbox(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	req := CreateSandboxRequest{
		ServiceID:     "test_service",
		PackageID:     "test_package",
		SecurityLevel: SecurityLevelNormal,
	}

	// Create first sandbox
	_, err := manager.CreateSandbox(ctx, req)
	if err != nil {
		t.Fatalf("failed to create first sandbox: %v", err)
	}

	// Try to create duplicate
	_, err = manager.CreateSandbox(ctx, req)
	if err == nil {
		t.Error("should fail for duplicate sandbox")
	}
}

func TestManagerListSandboxes(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	// Create multiple sandboxes
	for i := 0; i < 3; i++ {
		req := CreateSandboxRequest{
			ServiceID:     "service_" + string(rune('a'+i)),
			PackageID:     "package_" + string(rune('a'+i)),
			SecurityLevel: SecurityLevelNormal,
		}
		_, err := manager.CreateSandbox(ctx, req)
		if err != nil {
			t.Fatalf("failed to create sandbox %d: %v", i, err)
		}
	}

	// List sandboxes
	list := manager.ListSandboxes()
	if len(list) != 3 {
		t.Errorf("expected 3 sandboxes, got %d", len(list))
	}
}

// =============================================================================
// Isolated Storage Tests
// =============================================================================

func TestIsolatedStorage(t *testing.T) {
	backend := NewMemoryStorageBackend()
	auditor := NewSecurityAuditor(100)
	storage := NewIsolatedStorage("test_service", backend, 1024*1024, auditor)

	ctx := context.Background()

	// Set value
	err := storage.Set(ctx, "key1", []byte("value1"))
	if err != nil {
		t.Errorf("failed to set value: %v", err)
	}

	// Get value
	value, err := storage.Get(ctx, "key1")
	if err != nil {
		t.Errorf("failed to get value: %v", err)
	}
	if string(value) != "value1" {
		t.Errorf("value mismatch: %s != value1", string(value))
	}

	// Delete value
	err = storage.Delete(ctx, "key1")
	if err != nil {
		t.Errorf("failed to delete value: %v", err)
	}

	// Verify deleted
	_, err = storage.Get(ctx, "key1")
	if err == nil {
		t.Error("should fail for deleted key")
	}
}

func TestIsolatedStorageNamespace(t *testing.T) {
	backend := NewMemoryStorageBackend()
	auditor := NewSecurityAuditor(100)

	storage1 := NewIsolatedStorage("service_a", backend, 1024*1024, auditor)
	storage2 := NewIsolatedStorage("service_b", backend, 1024*1024, auditor)

	ctx := context.Background()

	// Set value in storage1
	err := storage1.Set(ctx, "shared_key", []byte("value_a"))
	if err != nil {
		t.Fatalf("failed to set value: %v", err)
	}

	// Set value in storage2 with same key
	err = storage2.Set(ctx, "shared_key", []byte("value_b"))
	if err != nil {
		t.Fatalf("failed to set value: %v", err)
	}

	// Values should be isolated
	value1, _ := storage1.Get(ctx, "shared_key")
	value2, _ := storage2.Get(ctx, "shared_key")

	if string(value1) != "value_a" {
		t.Errorf("storage1 value mismatch: %s != value_a", string(value1))
	}
	if string(value2) != "value_b" {
		t.Errorf("storage2 value mismatch: %s != value_b", string(value2))
	}
}

func TestIsolatedStorageQuota(t *testing.T) {
	backend := NewMemoryStorageBackend()
	auditor := NewSecurityAuditor(100)
	storage := NewIsolatedStorage("test_service", backend, 100, auditor) // 100 bytes quota

	ctx := context.Background()

	// Set small value - should succeed
	err := storage.Set(ctx, "small", []byte("small"))
	if err != nil {
		t.Errorf("small value should succeed: %v", err)
	}

	// Set large value - should fail
	largeValue := make([]byte, 200)
	err = storage.Set(ctx, "large", largeValue)
	if err == nil {
		t.Error("large value should fail due to quota")
	}
}

func TestIsolatedStorageList(t *testing.T) {
	backend := NewMemoryStorageBackend()
	auditor := NewSecurityAuditor(100)
	storage := NewIsolatedStorage("test_service", backend, 1024*1024, auditor)

	ctx := context.Background()

	// Set multiple values
	storage.Set(ctx, "config/a", []byte("a"))
	storage.Set(ctx, "config/b", []byte("b"))
	storage.Set(ctx, "data/c", []byte("c"))

	// List with prefix
	keys, err := storage.List(ctx, "config/")
	if err != nil {
		t.Errorf("failed to list: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys with prefix 'config/', got %d", len(keys))
	}
}

// =============================================================================
// Bus Integration Tests
// =============================================================================

func TestSecureBus(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	// Create sandbox with bus capabilities
	req := CreateSandboxRequest{
		ServiceID:     "test_service",
		PackageID:     "test_package",
		SecurityLevel: SecurityLevelNormal,
		RequestedCapabilities: []Capability{
			CapBusPublish,
			CapBusSubscribe,
		},
	}
	_, err := manager.CreateSandbox(ctx, req)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	secureBus := NewSecureBus(manager, DefaultSecureBusConfig())

	// Test publish (should succeed)
	err = secureBus.SecurePublishEvent(ctx, "test_service", "test.event", nil)
	if err != nil {
		t.Errorf("publish should succeed: %v", err)
	}

	// Test subscribe (should succeed)
	err = secureBus.SecureSubscribe(ctx, "test_service", "test.event")
	if err != nil {
		t.Errorf("subscribe should succeed: %v", err)
	}
}

func TestSecureBusWithoutCapability(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	// Create sandbox WITHOUT bus capabilities
	req := CreateSandboxRequest{
		ServiceID:     "test_service",
		PackageID:     "test_package",
		SecurityLevel: SecurityLevelNormal,
		RequestedCapabilities: []Capability{
			CapStorageRead, // Only storage, no bus
		},
	}
	_, err := manager.CreateSandbox(ctx, req)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	secureBus := NewSecureBus(manager, DefaultSecureBusConfig())

	// Test publish (should fail)
	err = secureBus.SecurePublishEvent(ctx, "test_service", "test.event", nil)
	if err == nil {
		t.Error("publish should fail without capability")
	}
}

func TestBusRateLimiter(t *testing.T) {
	config := SecureBusConfig{
		MaxEventsPerMinute: 3,
	}
	limiter := NewBusRateLimiter(config)

	// First 3 should succeed
	for i := 0; i < 3; i++ {
		if !limiter.AllowEvent("service_a") {
			t.Errorf("event %d should be allowed", i+1)
		}
	}

	// 4th should fail
	if limiter.AllowEvent("service_a") {
		t.Error("4th event should be rate limited")
	}

	// Different service should still work
	if !limiter.AllowEvent("service_b") {
		t.Error("different service should not be rate limited")
	}
}

// =============================================================================
// Security Level Tests
// =============================================================================

func TestSecurityLevelCapabilityGrants(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	tests := []struct {
		name          string
		securityLevel SecurityLevel
		capability    Capability
		shouldGrant   bool
	}{
		{"system can have admin", SecurityLevelSystem, CapSystemAdmin, true},
		{"privileged cannot have admin", SecurityLevelPrivileged, CapSystemAdmin, false},
		{"normal cannot have admin", SecurityLevelNormal, CapSystemAdmin, false},
		{"untrusted cannot have admin", SecurityLevelUntrusted, CapSystemAdmin, false},
		{"normal can have storage", SecurityLevelNormal, CapStorageRead, true},
		{"untrusted can have storage", SecurityLevelUntrusted, CapStorageRead, true},
		{"normal cannot have storage.other", SecurityLevelNormal, CapStorageOther, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := CreateSandboxRequest{
				ServiceID:             "test_" + tt.name,
				PackageID:             "test_package",
				SecurityLevel:         tt.securityLevel,
				RequestedCapabilities: []Capability{tt.capability},
			}

			sandbox, err := manager.CreateSandbox(ctx, req)
			if err != nil {
				t.Fatalf("failed to create sandbox: %v", err)
			}

			hasCapability := sandbox.Caps.Has(tt.capability)
			if hasCapability != tt.shouldGrant {
				t.Errorf("capability %s grant mismatch: got %v, want %v",
					tt.capability, hasCapability, tt.shouldGrant)
			}

			// Cleanup
			manager.DestroySandbox(ctx, req.ServiceID)
		})
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkCapabilityCheck(b *testing.B) {
	caps := NewCapabilitySet()
	caps.Grant(CapStorageRead, "test")
	caps.Grant(CapStorageWrite, "test")
	caps.Grant(CapBusPublish, "test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		caps.Has(CapStorageRead)
	}
}

func BenchmarkPolicyEvaluate(b *testing.B) {
	policy := NewSecurityPolicy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.Evaluate("service_a", "storage:test", "read")
	}
}

func BenchmarkStorageSet(b *testing.B) {
	backend := NewMemoryStorageBackend()
	auditor := NewSecurityAuditor(100)
	storage := NewIsolatedStorage("test", backend, 100*1024*1024, auditor)
	ctx := context.Background()
	value := []byte("test value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.Set(ctx, "key", value)
	}
}

func BenchmarkCreateSandbox(b *testing.B) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := CreateSandboxRequest{
			ServiceID:     "service_" + time.Now().String(),
			PackageID:     "package",
			SecurityLevel: SecurityLevelNormal,
			RequestedCapabilities: []Capability{
				CapStorageRead,
				CapStorageWrite,
			},
		}
		manager.CreateSandbox(ctx, req)
	}
}
