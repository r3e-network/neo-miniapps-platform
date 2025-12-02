package sandbox

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// =============================================================================
// Integration Tests - End-to-End Sandbox Scenarios
// =============================================================================

// TestIntegration_ServiceIsolation tests that services are properly isolated.
func TestIntegration_ServiceIsolation(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	// Create two sandboxes for different services
	sandbox1, err := manager.CreateSandbox(ctx, CreateSandboxRequest{
		ServiceID:     "service_a",
		PackageID:     "package_a",
		SecurityLevel: SecurityLevelNormal,
		RequestedCapabilities: []Capability{
			CapStorageRead,
			CapStorageWrite,
		},
		StorageQuota: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox1: %v", err)
	}

	sandbox2, err := manager.CreateSandbox(ctx, CreateSandboxRequest{
		ServiceID:     "service_b",
		PackageID:     "package_b",
		SecurityLevel: SecurityLevelNormal,
		RequestedCapabilities: []Capability{
			CapStorageRead,
			CapStorageWrite,
		},
		StorageQuota: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox2: %v", err)
	}

	// Service A writes data
	err = sandbox1.Storage.Set(ctx, "secret_key", []byte("service_a_secret"))
	if err != nil {
		t.Fatalf("service_a failed to write: %v", err)
	}

	// Service B writes data with same key
	err = sandbox2.Storage.Set(ctx, "secret_key", []byte("service_b_secret"))
	if err != nil {
		t.Fatalf("service_b failed to write: %v", err)
	}

	// Verify isolation - each service sees only its own data
	dataA, err := sandbox1.Storage.Get(ctx, "secret_key")
	if err != nil {
		t.Fatalf("service_a failed to read: %v", err)
	}
	if string(dataA) != "service_a_secret" {
		t.Errorf("service_a got wrong data: %s", string(dataA))
	}

	dataB, err := sandbox2.Storage.Get(ctx, "secret_key")
	if err != nil {
		t.Fatalf("service_b failed to read: %v", err)
	}
	if string(dataB) != "service_b_secret" {
		t.Errorf("service_b got wrong data: %s", string(dataB))
	}

	// Cleanup
	manager.DestroySandbox(ctx, "service_a")
	manager.DestroySandbox(ctx, "service_b")
}

// TestIntegration_CapabilityEnforcement tests capability-based access control.
func TestIntegration_CapabilityEnforcement(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	// Create sandbox with limited capabilities
	sandbox, err := manager.CreateSandbox(ctx, CreateSandboxRequest{
		ServiceID:     "limited_service",
		PackageID:     "limited_package",
		SecurityLevel: SecurityLevelNormal,
		RequestedCapabilities: []Capability{
			CapStorageRead, // Only read, no write
		},
		StorageQuota: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Check read capability - should succeed
	err = sandbox.Context.CheckCapability(ctx, CapStorageRead)
	if err != nil {
		t.Errorf("read capability check should succeed: %v", err)
	}

	// Check write capability - should fail
	err = sandbox.Context.CheckCapability(ctx, CapStorageWrite)
	if err == nil {
		t.Error("write capability check should fail")
	}

	// Verify error type
	if _, ok := err.(*CapabilityDeniedError); !ok {
		t.Errorf("expected CapabilityDeniedError, got %T", err)
	}

	manager.DestroySandbox(ctx, "limited_service")
}

// TestIntegration_SecurityLevelHierarchy tests security level restrictions.
func TestIntegration_SecurityLevelHierarchy(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	tests := []struct {
		name          string
		securityLevel SecurityLevel
		capability    Capability
		shouldGrant   bool
	}{
		// System level can have admin
		{"system_admin", SecurityLevelSystem, CapSystemAdmin, true},
		// Privileged cannot have admin
		{"privileged_admin", SecurityLevelPrivileged, CapSystemAdmin, false},
		// Normal cannot have admin
		{"normal_admin", SecurityLevelNormal, CapSystemAdmin, false},
		// Normal can have storage
		{"normal_storage", SecurityLevelNormal, CapStorageRead, true},
		// Untrusted can have basic storage
		{"untrusted_storage", SecurityLevelUntrusted, CapStorageRead, true},
		// Normal cannot access other services' storage
		{"normal_other_storage", SecurityLevelNormal, CapStorageOther, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sandbox, err := manager.CreateSandbox(ctx, CreateSandboxRequest{
				ServiceID:             "test_" + tt.name,
				PackageID:             "test_package",
				SecurityLevel:         tt.securityLevel,
				RequestedCapabilities: []Capability{tt.capability},
			})
			if err != nil {
				t.Fatalf("failed to create sandbox: %v", err)
			}

			hasCapability := sandbox.Caps.Has(tt.capability)
			if hasCapability != tt.shouldGrant {
				t.Errorf("capability %s: got %v, want %v", tt.capability, hasCapability, tt.shouldGrant)
			}

			manager.DestroySandbox(ctx, "test_"+tt.name)
		})
	}
}

// TestIntegration_BusSecurityWithRateLimiting tests bus security and rate limiting.
func TestIntegration_BusSecurityWithRateLimiting(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	// Create sandbox with bus capabilities
	_, err := manager.CreateSandbox(ctx, CreateSandboxRequest{
		ServiceID:     "bus_service",
		PackageID:     "bus_package",
		SecurityLevel: SecurityLevelNormal,
		RequestedCapabilities: []Capability{
			CapBusPublish,
			CapBusSubscribe,
		},
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Create secure bus with low rate limit for testing
	config := SecureBusConfig{
		MaxEventsPerMinute: 3,
	}
	secureBus := NewSecureBus(manager, config)

	// First 3 events should succeed
	for i := 0; i < 3; i++ {
		err := secureBus.SecurePublishEvent(ctx, "bus_service", "test.event", nil)
		if err != nil {
			t.Errorf("event %d should succeed: %v", i+1, err)
		}
	}

	// 4th event should be rate limited
	err = secureBus.SecurePublishEvent(ctx, "bus_service", "test.event", nil)
	if err == nil {
		t.Error("4th event should be rate limited")
	}

	manager.DestroySandbox(ctx, "bus_service")
}

// TestIntegration_AuditLogging tests audit logging integration.
func TestIntegration_AuditLogging(t *testing.T) {
	// Create a buffer to capture logs
	var logBuffer bytes.Buffer
	logger := NewJSONLoggerAdapter(&logBuffer)

	auditor := NewEnhancedAuditor(EnhancedAuditorConfig{
		MaxEvents:       100,
		Logger:          logger,
		DenialThreshold: 3,
		DenialWindow:    time.Minute,
		AlertCooldown:   time.Second,
	})

	ctx := context.Background()
	identity := &ServiceIdentity{
		ServiceID:     "audit_test_service",
		SecurityLevel: SecurityLevelNormal,
	}

	// Log some events
	auditor.LogCapabilityCheck(ctx, identity, CapStorageRead, true)
	auditor.LogCapabilityCheck(ctx, identity, CapStorageWrite, false)
	auditor.LogResourceAccess(ctx, "audit_test_service", "storage:test", "read", true)

	// Verify events were logged
	events := auditor.GetEvents(10)
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// Verify JSON output was written
	if logBuffer.Len() == 0 {
		t.Error("expected log output")
	}

	// Test denial threshold alerting
	for i := 0; i < 5; i++ {
		auditor.LogCapabilityCheck(ctx, identity, CapSystemAdmin, false)
	}

	// Check that alert was triggered (should be in log buffer)
	logOutput := logBuffer.String()
	if len(logOutput) == 0 {
		t.Error("expected alert in log output")
	}
}

// TestIntegration_PolicyEvaluation tests policy evaluation with context.
func TestIntegration_PolicyEvaluation(t *testing.T) {
	policy := NewSecurityPolicy()

	// Add custom rules with glob patterns
	policy.AddRule(PolicyRule{
		Subject:  "com.r3e.services.*",
		Object:   "storage:*",
		Action:   "read",
		Effect:   PolicyEffectAllow,
		Priority: 200,
	})

	policy.AddRule(PolicyRule{
		Subject:  "com.r3e.services.*",
		Object:   "storage:*",
		Action:   "write",
		Effect:   PolicyEffectAllow,
		Priority: 200,
	})

	tests := []struct {
		name     string
		subject  string
		object   string
		action   string
		expected PolicyEffect
	}{
		{
			name:     "r3e_service_read",
			subject:  "com.r3e.services.accounts",
			object:   "storage:accounts/data",
			action:   "read",
			expected: PolicyEffectAllow,
		},
		{
			name:     "r3e_service_write",
			subject:  "com.r3e.services.secrets",
			object:   "storage:secrets/keys",
			action:   "write",
			expected: PolicyEffectAllow,
		},
		{
			name:     "unknown_service_read",
			subject:  "unknown.service",
			object:   "storage:data",
			action:   "read",
			expected: PolicyEffectDeny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use EvaluateWithContext which supports glob pattern matching
			effect := policy.EvaluateWithContext(tt.subject, tt.subject, tt.object, tt.action)
			if effect != tt.expected {
				t.Errorf("got %s, want %s", effect, tt.expected)
			}
		})
	}
}

// TestIntegration_StorageQuotaEnforcement tests storage quota enforcement.
func TestIntegration_StorageQuotaEnforcement(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	// Create sandbox with small quota
	sandbox, err := manager.CreateSandbox(ctx, CreateSandboxRequest{
		ServiceID:     "quota_service",
		PackageID:     "quota_package",
		SecurityLevel: SecurityLevelNormal,
		RequestedCapabilities: []Capability{
			CapStorageRead,
			CapStorageWrite,
		},
		StorageQuota: 100, // 100 bytes
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Small write should succeed
	err = sandbox.Storage.Set(ctx, "small", []byte("small"))
	if err != nil {
		t.Errorf("small write should succeed: %v", err)
	}

	// Large write should fail
	largeData := make([]byte, 200)
	err = sandbox.Storage.Set(ctx, "large", largeData)
	if err == nil {
		t.Error("large write should fail due to quota")
	}

	manager.DestroySandbox(ctx, "quota_service")
}

// TestIntegration_MultiServiceCommunication tests secure inter-service communication.
func TestIntegration_MultiServiceCommunication(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	// Create service A with publish capability
	_, err := manager.CreateSandbox(ctx, CreateSandboxRequest{
		ServiceID:     "publisher_service",
		PackageID:     "publisher_package",
		SecurityLevel: SecurityLevelNormal,
		RequestedCapabilities: []Capability{
			CapBusPublish,
		},
	})
	if err != nil {
		t.Fatalf("failed to create publisher sandbox: %v", err)
	}

	// Create service B with subscribe capability
	_, err = manager.CreateSandbox(ctx, CreateSandboxRequest{
		ServiceID:     "subscriber_service",
		PackageID:     "subscriber_package",
		SecurityLevel: SecurityLevelNormal,
		RequestedCapabilities: []Capability{
			CapBusSubscribe,
		},
	})
	if err != nil {
		t.Fatalf("failed to create subscriber sandbox: %v", err)
	}

	secureBus := NewSecureBus(manager, DefaultSecureBusConfig())

	// Publisher should be able to publish
	err = secureBus.SecurePublishEvent(ctx, "publisher_service", "test.event", map[string]string{"data": "test"})
	if err != nil {
		t.Errorf("publisher should be able to publish: %v", err)
	}

	// Subscriber should be able to subscribe
	err = secureBus.SecureSubscribe(ctx, "subscriber_service", "test.event")
	if err != nil {
		t.Errorf("subscriber should be able to subscribe: %v", err)
	}

	// Publisher should NOT be able to subscribe (no capability)
	err = secureBus.SecureSubscribe(ctx, "publisher_service", "test.event")
	if err == nil {
		t.Error("publisher should not be able to subscribe without capability")
	}

	// Subscriber should NOT be able to publish (no capability)
	err = secureBus.SecurePublishEvent(ctx, "subscriber_service", "test.event", nil)
	if err == nil {
		t.Error("subscriber should not be able to publish without capability")
	}

	manager.DestroySandbox(ctx, "publisher_service")
	manager.DestroySandbox(ctx, "subscriber_service")
}

// TestIntegration_SandboxLifecycle tests complete sandbox lifecycle.
func TestIntegration_SandboxLifecycle(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	// 1. Create sandbox
	sandbox, err := manager.CreateSandbox(ctx, CreateSandboxRequest{
		ServiceID:     "lifecycle_service",
		PackageID:     "lifecycle_package",
		SecurityLevel: SecurityLevelNormal,
		RequestedCapabilities: []Capability{
			CapStorageRead,
			CapStorageWrite,
		},
		StorageQuota: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// 2. Verify sandbox is active
	retrieved, err := manager.GetSandbox("lifecycle_service")
	if err != nil {
		t.Fatalf("failed to get sandbox: %v", err)
	}
	if retrieved.Identity.ServiceID != sandbox.Identity.ServiceID {
		t.Error("retrieved sandbox doesn't match")
	}

	// 3. Use sandbox resources
	err = sandbox.Storage.Set(ctx, "test_key", []byte("test_value"))
	if err != nil {
		t.Errorf("failed to write to storage: %v", err)
	}

	// 4. Verify data persists
	data, err := sandbox.Storage.Get(ctx, "test_key")
	if err != nil {
		t.Errorf("failed to read from storage: %v", err)
	}
	if string(data) != "test_value" {
		t.Errorf("data mismatch: got %s", string(data))
	}

	// 5. Destroy sandbox
	err = manager.DestroySandbox(ctx, "lifecycle_service")
	if err != nil {
		t.Errorf("failed to destroy sandbox: %v", err)
	}

	// 6. Verify sandbox is gone
	_, err = manager.GetSandbox("lifecycle_service")
	if err == nil {
		t.Error("sandbox should be destroyed")
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkIntegration_SandboxCreation(b *testing.B) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		serviceID := "bench_service_" + string(rune('a'+i%26))
		sandbox, _ := manager.CreateSandbox(ctx, CreateSandboxRequest{
			ServiceID:     serviceID,
			PackageID:     "bench_package",
			SecurityLevel: SecurityLevelNormal,
			RequestedCapabilities: []Capability{
				CapStorageRead,
				CapStorageWrite,
			},
		})
		if sandbox != nil {
			manager.DestroySandbox(ctx, serviceID)
		}
	}
}

func BenchmarkIntegration_StorageOperations(b *testing.B) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	sandbox, _ := manager.CreateSandbox(ctx, CreateSandboxRequest{
		ServiceID:     "bench_storage_service",
		PackageID:     "bench_package",
		SecurityLevel: SecurityLevelNormal,
		RequestedCapabilities: []Capability{
			CapStorageRead,
			CapStorageWrite,
		},
		StorageQuota: 100 * 1024 * 1024,
	})

	value := []byte("benchmark_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key_" + string(rune('a'+i%26))
		sandbox.Storage.Set(ctx, key, value)
		sandbox.Storage.Get(ctx, key)
	}

	b.StopTimer()
	manager.DestroySandbox(ctx, "bench_storage_service")
}

func BenchmarkIntegration_CapabilityCheck(b *testing.B) {
	manager := NewManager(nil, DefaultManagerConfig())
	ctx := context.Background()

	sandbox, _ := manager.CreateSandbox(ctx, CreateSandboxRequest{
		ServiceID:     "bench_cap_service",
		PackageID:     "bench_package",
		SecurityLevel: SecurityLevelNormal,
		RequestedCapabilities: []Capability{
			CapStorageRead,
			CapStorageWrite,
			CapBusPublish,
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sandbox.Context.CheckCapability(ctx, CapStorageRead)
	}

	b.StopTimer()
	manager.DestroySandbox(ctx, "bench_cap_service")
}
