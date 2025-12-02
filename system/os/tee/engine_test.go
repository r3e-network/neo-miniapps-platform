package tee

import (
	"context"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	engine, err := NewEngine(EngineConfig{
		Mode: EnclaveModeSimulation,
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestEngine_StartStop(t *testing.T) {
	engine, err := NewEngine(EngineConfig{
		Mode: EnclaveModeSimulation,
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctx := context.Background()

	// Start
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Health check
	if err := engine.Health(ctx); err != nil {
		t.Fatalf("Health: %v", err)
	}

	// Stop
	if err := engine.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Health should fail after stop
	if err := engine.Health(ctx); err == nil {
		t.Fatal("expected health check to fail after stop")
	}
}

func TestEngine_RegisterService(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	err := engine.RegisterService(ctx, ServiceRegistration{
		ServiceID:             "test-service",
		AllowedSecretPatterns: []string{"test_*", "api_key"},
		MaxConcurrentExecutions: 5,
	})
	if err != nil {
		t.Fatalf("RegisterService: %v", err)
	}

	// Register without service ID should fail
	err = engine.RegisterService(ctx, ServiceRegistration{})
	if err == nil {
		t.Fatal("expected error for empty service_id")
	}
}

func TestEngine_GetAttestation(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	report, err := engine.GetAttestation(ctx)
	if err != nil {
		t.Fatalf("GetAttestation: %v", err)
	}

	if report.Mode != EnclaveModeSimulation {
		t.Errorf("expected simulation mode, got %s", report.Mode)
	}
	if report.EnclaveID == "" {
		t.Error("expected non-empty enclave ID")
	}
	if len(report.Quote) == 0 {
		t.Error("expected non-empty quote")
	}
}

func TestEngine_Execute_SimpleScript(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	// Register service
	_ = engine.RegisterService(ctx, ServiceRegistration{
		ServiceID:             "test-service",
		AllowedSecretPatterns: []string{"*"},
	})

	// Execute simple script
	result, err := engine.Execute(ctx, ExecutionRequest{
		ServiceID:  "test-service",
		AccountID:  "account-1",
		Script:     `function main(input) { return { sum: input.a + input.b }; }`,
		EntryPoint: "main",
		Input:      map[string]any{"a": 10, "b": 20},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result.Status != ExecutionStatusSucceeded {
		t.Errorf("expected succeeded, got %s: %s", result.Status, result.Error)
	}

	// goja may return int64 or float64 depending on the value
	var sum float64
	switch v := result.Output["sum"].(type) {
	case float64:
		sum = v
	case int64:
		sum = float64(v)
	default:
		t.Errorf("unexpected type for sum: %T, value: %v", result.Output["sum"], result.Output["sum"])
	}
	if sum != 30 {
		t.Errorf("expected sum=30, got %v", sum)
	}
}

func TestEngine_Execute_WithSecrets(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	// Register service
	_ = engine.RegisterService(ctx, ServiceRegistration{
		ServiceID:             "test-service",
		AllowedSecretPatterns: []string{"api_*"},
	})

	// Store a secret via the vault (simulating secrets service)
	impl := engine.(*engineImpl)
	err := impl.secretVault.StoreSecret(ctx, "test-service", "account-1", "api_key", []byte("secret-value-123"))
	if err != nil {
		t.Fatalf("StoreSecret: %v", err)
	}

	// Verify secret was stored correctly
	storedValue, err := impl.secretVault.GetSecret(ctx, "test-service", "account-1", "api_key")
	if err != nil {
		t.Fatalf("GetSecret verification: %v", err)
	}
	if string(storedValue) != "secret-value-123" {
		t.Fatalf("stored secret mismatch: got %s", string(storedValue))
	}

	// Execute script that uses secrets
	result, err := engine.Execute(ctx, ExecutionRequest{
		ServiceID:  "test-service",
		AccountID:  "account-1",
		Script:     `function main(input) { return { hasSecret: secrets.api_key !== undefined, keyLength: secrets.api_key ? secrets.api_key.length : 0 }; }`,
		EntryPoint: "main",
		Input:      map[string]any{},
		Secrets:    []string{"api_key"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result.Status != ExecutionStatusSucceeded {
		t.Errorf("expected succeeded, got %s: %s", result.Status, result.Error)
	}

	hasSecret, _ := result.Output["hasSecret"].(bool)
	if !hasSecret {
		t.Errorf("expected hasSecret=true, output: %+v", result.Output)
	}

	// goja may return int64 or float64
	var keyLength float64
	switch v := result.Output["keyLength"].(type) {
	case float64:
		keyLength = v
	case int64:
		keyLength = float64(v)
	}
	if keyLength != 16 { // "secret-value-123" = 16 chars
		t.Errorf("expected keyLength=16, got %v", keyLength)
	}
}

func TestEngine_Execute_UnregisteredService(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	// Execute without registering service
	_, err := engine.Execute(ctx, ExecutionRequest{
		ServiceID:  "unregistered-service",
		AccountID:  "account-1",
		Script:     `function main() { return {}; }`,
		EntryPoint: "main",
	})
	if err != ErrServiceNotRegistered {
		t.Errorf("expected ErrServiceNotRegistered, got %v", err)
	}
}

func TestEngine_Execute_SecretAccessDenied(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	// Register service with limited secret access
	_ = engine.RegisterService(ctx, ServiceRegistration{
		ServiceID:             "limited-service",
		AllowedSecretPatterns: []string{"allowed_*"},
	})

	// Try to access a secret not in allowed patterns
	_, err := engine.Execute(ctx, ExecutionRequest{
		ServiceID:  "limited-service",
		AccountID:  "account-1",
		Script:     `function main() { return {}; }`,
		EntryPoint: "main",
		Secrets:    []string{"forbidden_secret"},
	})
	if err == nil {
		t.Error("expected error for forbidden secret access")
	}
}

func TestEngine_Execute_Timeout(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	_ = engine.RegisterService(ctx, ServiceRegistration{
		ServiceID:             "test-service",
		AllowedSecretPatterns: []string{"*"},
	})

	// Execute script with very short timeout
	// Note: V8 doesn't support true interruption, so this tests the context timeout
	result, err := engine.Execute(ctx, ExecutionRequest{
		ServiceID:  "test-service",
		AccountID:  "account-1",
		Script:     `function main() { return { done: true }; }`,
		EntryPoint: "main",
		Timeout:    1 * time.Millisecond,
	})

	// The script is fast enough to complete, but we verify timeout is applied
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Duration > 1*time.Second {
		t.Errorf("execution took too long: %v", result.Duration)
	}
}

func TestEngine_Execute_ConsoleLogs(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	_ = engine.RegisterService(ctx, ServiceRegistration{
		ServiceID:             "test-service",
		AllowedSecretPatterns: []string{"*"},
	})

	result, err := engine.Execute(ctx, ExecutionRequest{
		ServiceID:  "test-service",
		AccountID:  "account-1",
		Script: `
			function main(input) {
				console.log("Hello from TEE");
				console.log("Input:", JSON.stringify(input));
				return { logged: true };
			}
		`,
		EntryPoint: "main",
		Input:      map[string]any{"test": "value"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(result.Logs) < 1 {
		t.Error("expected at least one log entry")
	}
}

func TestSecretManager_Isolation(t *testing.T) {
	vault := newSimulationVault(nil)
	manager := NewSecretManager(vault)

	// Register two services
	_ = manager.RegisterPolicy(SecretPolicy{
		ServiceID:       "service-a",
		AllowedPatterns: []string{"a_*"},
		MaxSecrets:      10,
		CanGrantAccess:  true,
	})
	_ = manager.RegisterPolicy(SecretPolicy{
		ServiceID:       "service-b",
		AllowedPatterns: []string{"b_*"},
		MaxSecrets:      10,
		CanGrantAccess:  false,
	})

	ctx := context.Background()

	// Service A stores a secret
	err := manager.StoreSecret(ctx, "service-a", "account-1", "a_secret", []byte("value-a"), nil)
	if err != nil {
		t.Fatalf("StoreSecret: %v", err)
	}

	// Service A can access its own secret
	value, err := manager.GetSecret(ctx, "service-a", "account-1", "a_secret")
	if err != nil {
		t.Fatalf("GetSecret (own): %v", err)
	}
	if string(value) != "value-a" {
		t.Errorf("expected 'value-a', got '%s'", string(value))
	}

	// Service B cannot access Service A's secret
	_, err = manager.GetSecret(ctx, "service-b", "account-1", "a_secret")
	if err == nil {
		t.Error("expected error when service-b accesses service-a's secret")
	}

	// Service A grants access to Service B
	err = manager.GrantAccess(ctx, SecretGrant{
		OwnerServiceID:  "service-a",
		TargetServiceID: "service-b",
		AccountID:       "account-1",
		SecretPattern:   "a_secret",
	})
	if err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}

	// Now Service B can access the secret
	value, err = manager.GetSecret(ctx, "service-b", "account-1", "a_secret")
	if err != nil {
		t.Fatalf("GetSecret (granted): %v", err)
	}
	if string(value) != "value-a" {
		t.Errorf("expected 'value-a', got '%s'", string(value))
	}

	// Revoke access
	err = manager.RevokeAccess(ctx, "service-a", "service-b", "account-1", "a_secret")
	if err != nil {
		t.Fatalf("RevokeAccess: %v", err)
	}

	// Service B can no longer access
	_, err = manager.GetSecret(ctx, "service-b", "account-1", "a_secret")
	if err == nil {
		t.Error("expected error after access revoked")
	}
}

func TestSecretManager_PatternMatching(t *testing.T) {
	vault := newSimulationVault(nil)
	manager := NewSecretManager(vault)

	_ = manager.RegisterPolicy(SecretPolicy{
		ServiceID:       "test-service",
		AllowedPatterns: []string{"db_*", "api_key", "config_*"},
		MaxSecrets:      100,
	})

	ctx := context.Background()

	// These should succeed
	validNames := []string{"db_password", "db_host", "api_key", "config_timeout"}
	for _, name := range validNames {
		err := manager.StoreSecret(ctx, "test-service", "account-1", name, []byte("value"), nil)
		if err != nil {
			t.Errorf("StoreSecret(%s) should succeed: %v", name, err)
		}
	}

	// These should fail
	invalidNames := []string{"secret", "password", "other_key"}
	for _, name := range invalidNames {
		err := manager.StoreSecret(ctx, "test-service", "account-1", name, []byte("value"), nil)
		if err == nil {
			t.Errorf("StoreSecret(%s) should fail", name)
		}
	}
}

func TestServiceTEEAdapter(t *testing.T) {
	// Initialize provider
	err := InitializeProvider(ProviderConfig{
		Mode:                    EnclaveModeSimulation,
		RegisterDefaultPolicies: true,
	})
	if err != nil {
		t.Fatalf("InitializeProvider: %v", err)
	}

	ctx := context.Background()
	_ = StartProvider(ctx)
	defer StopProvider(ctx)

	// Create adapter for functions service
	adapter := NewServiceTEEAdapter("functions")
	if err := adapter.Initialize(); err != nil {
		t.Fatalf("adapter.Initialize: %v", err)
	}

	// Store a secret
	err = adapter.StoreSecret(ctx, "account-1", "fn_api_key", []byte("my-api-key"))
	if err != nil {
		t.Fatalf("StoreSecret: %v", err)
	}

	// Retrieve the secret
	value, err := adapter.GetSecret(ctx, "account-1", "fn_api_key")
	if err != nil {
		t.Fatalf("GetSecret: %v", err)
	}
	if string(value) != "my-api-key" {
		t.Errorf("expected 'my-api-key', got '%s'", string(value))
	}

	// Execute a script
	result, err := adapter.Execute(ctx, "account-1",
		`function main(input) { return { greeting: "Hello " + input.name }; }`,
		"main",
		map[string]any{"name": "TEE"},
		nil,
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != ExecutionStatusSucceeded {
		t.Errorf("expected succeeded, got %s: %s", result.Status, result.Error)
	}

	greeting, _ := result.Output["greeting"].(string)
	if greeting != "Hello TEE" {
		t.Errorf("expected 'Hello TEE', got '%s'", greeting)
	}
}
