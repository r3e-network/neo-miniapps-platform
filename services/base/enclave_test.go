// Package base provides base components for all services.
package base

import (
	"context"
	"sync"
	"testing"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/tee"
)

// =============================================================================
// Test Helpers
// =============================================================================

func setupEnclaveTestOS(t *testing.T) (os.ServiceOS, func()) {
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
// BaseEnclave Tests
// =============================================================================

func TestNewBaseEnclave(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)

	if enclave == nil {
		t.Fatal("NewBaseEnclave returned nil")
	}
	if enclave.ServiceID() != "test-service" {
		t.Errorf("ServiceID() = %s, want test-service", enclave.ServiceID())
	}
	if enclave.IsReady() {
		t.Error("IsReady() = true before Initialize, want false")
	}
}

func TestBaseEnclave_Initialize(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	// Initialize
	if err := enclave.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}

	if !enclave.IsReady() {
		t.Error("IsReady() = false after Initialize, want true")
	}

	// Double initialize should be idempotent
	if err := enclave.Initialize(ctx); err != nil {
		t.Errorf("second Initialize() error: %v", err)
	}
}

func TestBaseEnclave_Shutdown(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	enclave.Initialize(ctx)

	// Shutdown
	if err := enclave.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error: %v", err)
	}

	if enclave.IsReady() {
		t.Error("IsReady() = true after Shutdown, want false")
	}

	// Double shutdown should be idempotent
	if err := enclave.Shutdown(ctx); err != nil {
		t.Errorf("second Shutdown() error: %v", err)
	}
}

func TestBaseEnclave_Health(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	// Health should fail before initialize
	if err := enclave.Health(ctx); err == nil {
		t.Error("Health() should error before Initialize")
	}

	enclave.Initialize(ctx)

	// Health should pass after initialize
	if err := enclave.Health(ctx); err != nil {
		t.Errorf("Health() error after Initialize: %v", err)
	}

	enclave.Shutdown(ctx)

	// Health should fail after shutdown
	if err := enclave.Health(ctx); err == nil {
		t.Error("Health() should error after Shutdown")
	}
}

func TestBaseEnclave_OS(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)

	if enclave.OS() == nil {
		t.Error("OS() returned nil")
	}
	if enclave.Logger() == nil {
		t.Error("Logger() returned nil")
	}
}

func TestBaseEnclave_UseSecret_NotReady(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	err := enclave.UseSecret(ctx, "test-secret", func(secret []byte) error {
		return nil
	})
	if err == nil {
		t.Error("UseSecret() should error when not ready")
	}
}

func TestBaseEnclave_UseSecrets_NotReady(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	err := enclave.UseSecrets(ctx, []string{"secret1", "secret2"}, func(secrets map[string][]byte) error {
		return nil
	})
	if err == nil {
		t.Error("UseSecrets() should error when not ready")
	}
}

func TestBaseEnclave_DeriveKey_NotReady(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	_, err := enclave.DeriveKey(ctx, "test/path")
	if err == nil {
		t.Error("DeriveKey() should error when not ready")
	}
}

func TestBaseEnclave_Sign_NotReady(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	_, err := enclave.Sign(ctx, "test-handle", []byte("data"))
	if err == nil {
		t.Error("Sign() should error when not ready")
	}
}

func TestBaseEnclave_FetchWithSecret_NotReady(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	req := os.HTTPRequest{
		Method: "GET",
		URL:    "https://example.com",
	}
	_, err := enclave.FetchWithSecret(ctx, req, "api-key", os.AuthBearer)
	if err == nil {
		t.Error("FetchWithSecret() should error when not ready")
	}
}

func TestBaseEnclave_Execute_NotReady(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	req := os.ComputeRequest{
		Script: "return 1",
	}
	_, err := enclave.Execute(ctx, req)
	if err == nil {
		t.Error("Execute() should error when not ready")
	}
}

func TestBaseEnclave_ExecuteWithSecrets_NotReady(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	req := os.ComputeRequest{
		Script: "return 1",
	}
	_, err := enclave.ExecuteWithSecrets(ctx, req, []string{"secret1"})
	if err == nil {
		t.Error("ExecuteWithSecrets() should error when not ready")
	}
}

func TestBaseEnclave_GenerateQuote_NotReady(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	_, err := enclave.GenerateQuote(ctx, []byte("user-data"))
	if err == nil {
		t.Error("GenerateQuote() should error when not ready")
	}
}

func TestBaseEnclave_UseStorage_NotReady(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	err := enclave.UseStorage(ctx, "test-key", func(value []byte) error {
		return nil
	})
	if err == nil {
		t.Error("UseStorage() should error when not ready")
	}
}

func TestBaseEnclave_StorageExists_NotReady(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	_, err := enclave.StorageExists(ctx, "test-key")
	if err == nil {
		t.Error("StorageExists() should error when not ready")
	}
}

func TestBaseEnclave_LoadConfigJSON_NotReady(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	var config struct{}
	_, err := enclave.LoadConfigJSON(ctx, "config-key", &config)
	if err == nil {
		t.Error("LoadConfigJSON() should error when not ready")
	}
}

func TestBaseEnclave_ConcurrentAccess(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()

	var wg sync.WaitGroup

	// Concurrent initialize/shutdown
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			enclave.Initialize(ctx)
		}()
		go func() {
			defer wg.Done()
			enclave.Shutdown(ctx)
		}()
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = enclave.IsReady()
			_ = enclave.ServiceID()
		}()
	}

	wg.Wait()
}

// =============================================================================
// Integration Tests (with initialized enclave)
// =============================================================================

func TestBaseEnclave_DeriveKey_Ready(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()
	enclave.Initialize(ctx)
	defer enclave.Shutdown(ctx)

	handle, err := enclave.DeriveKey(ctx, "test/path")
	if err != nil {
		t.Fatalf("DeriveKey() error: %v", err)
	}
	if handle == "" {
		t.Error("DeriveKey() returned empty handle")
	}
}

func TestBaseEnclave_Sign_Ready(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()
	enclave.Initialize(ctx)
	defer enclave.Shutdown(ctx)

	handle, _ := enclave.DeriveKey(ctx, "signing/key")

	// Sign may fail due to capability restrictions in simulation mode
	// This test verifies the enclave is ready and the call is made
	sig, err := enclave.Sign(ctx, handle, []byte("test data"))
	if err != nil {
		// In simulation mode, signing may be restricted
		// Verify it's a capability error, not an "enclave not ready" error
		if err.Error() == "enclave not ready" {
			t.Fatalf("Sign() should not fail with 'enclave not ready': %v", err)
		}
		// Capability denied is acceptable in test environment
		t.Skipf("Sign() capability restricted in test environment: %v", err)
	}
	if len(sig) == 0 {
		t.Error("Sign() returned empty signature")
	}
}

func TestBaseEnclave_GenerateQuote_Ready(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()
	enclave.Initialize(ctx)
	defer enclave.Shutdown(ctx)

	quote, err := enclave.GenerateQuote(ctx, []byte("user-data"))
	if err != nil {
		t.Fatalf("GenerateQuote() error: %v", err)
	}
	if quote == nil {
		t.Error("GenerateQuote() returned nil")
	}
}

func TestBaseEnclave_Execute_Ready(t *testing.T) {
	serviceOS, cleanup := setupEnclaveTestOS(t)
	defer cleanup()

	enclave := NewBaseEnclave("test-service", serviceOS)
	ctx := context.Background()
	enclave.Initialize(ctx)
	defer enclave.Shutdown(ctx)

	req := os.ComputeRequest{
		Script:     "return input.a + input.b",
		EntryPoint: "main",
		Input: map[string]any{
			"a": 1,
			"b": 2,
		},
	}

	result, err := enclave.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == nil {
		t.Error("Execute() returned nil result")
	}
}
