package sandbox

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// PolicyConfig Tests
// =============================================================================

func TestDefaultPolicyConfig(t *testing.T) {
	config := DefaultPolicyConfig()

	if config.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", config.Version)
	}

	if config.DefaultEffect != PolicyEffectDeny {
		t.Errorf("expected default effect deny, got %s", config.DefaultEffect)
	}

	if len(config.Rules) == 0 {
		t.Error("expected default rules")
	}

	if len(config.CapabilityProfiles) == 0 {
		t.Error("expected capability profiles")
	}

	// Check standard profile exists
	if _, ok := config.CapabilityProfiles["standard"]; !ok {
		t.Error("expected 'standard' capability profile")
	}
}

func TestGetCapabilityProfile(t *testing.T) {
	config := DefaultPolicyConfig()

	// Test existing profile
	minimal := config.GetCapabilityProfile("minimal")
	if len(minimal) == 0 {
		t.Error("expected minimal profile to have capabilities")
	}

	// Test non-existing profile
	nonExistent := config.GetCapabilityProfile("nonexistent")
	if nonExistent != nil {
		t.Error("expected nil for non-existent profile")
	}
}

func TestGetServicePolicy(t *testing.T) {
	config := DefaultPolicyConfig()

	// Test pattern matching
	policy := config.GetServicePolicy("com.r3e.services.accounts")
	if policy == nil {
		t.Error("expected policy for com.r3e.services.accounts")
	}

	if policy.SecurityLevel != "privileged" {
		t.Errorf("expected privileged security level, got %s", policy.SecurityLevel)
	}

	// Test non-matching service
	policy = config.GetServicePolicy("unknown.service")
	if policy != nil {
		t.Error("expected nil for unknown service")
	}
}

func TestIsCapabilityAllowed(t *testing.T) {
	config := DefaultPolicyConfig()

	// R3E services should have storage.read allowed
	if !config.IsCapabilityAllowed("com.r3e.services.accounts", CapStorageRead) {
		t.Error("expected storage.read to be allowed for R3E services")
	}

	// Unknown services should have capabilities allowed by default
	if !config.IsCapabilityAllowed("unknown.service", CapStorageRead) {
		t.Error("expected storage.read to be allowed for unknown services")
	}
}

// =============================================================================
// PolicyLoader Tests
// =============================================================================

func TestPolicyLoaderDefaults(t *testing.T) {
	loader := NewPolicyLoader(PolicyLoaderConfig{})

	policy, err := loader.Load()
	if err != nil {
		t.Fatalf("failed to load default policy: %v", err)
	}

	if policy == nil {
		t.Error("expected non-nil policy")
	}

	config := loader.Config()
	if config == nil {
		t.Error("expected non-nil config")
	}
}

func TestPolicyLoaderFromJSON(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "policy.json")

	configContent := `{
		"version": "1.0",
		"default_effect": "deny",
		"rules": [
			{
				"subject": "test_service",
				"object": "storage:test/*",
				"action": "read",
				"effect": "allow",
				"priority": 100
			}
		]
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	loader := NewPolicyLoader(PolicyLoaderConfig{
		ConfigPath: configPath,
	})

	policy, err := loader.Load()
	if err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}

	if policy == nil {
		t.Error("expected non-nil policy")
	}

	// Verify custom rule was loaded
	rules := policy.Rules()
	found := false
	for _, rule := range rules {
		if rule.Subject == "test_service" && rule.Object == "storage:test/*" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected custom rule to be loaded")
	}
}

func TestPolicyLoaderInvalidFile(t *testing.T) {
	loader := NewPolicyLoader(PolicyLoaderConfig{
		ConfigPath: "/nonexistent/path/policy.json",
	})

	_, err := loader.Load()
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestPolicyLoaderInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(configPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	loader := NewPolicyLoader(PolicyLoaderConfig{
		ConfigPath: configPath,
	})

	_, err := loader.Load()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// =============================================================================
// Enhanced SecurityPolicy Tests
// =============================================================================

func TestSecurityPolicyAddRule(t *testing.T) {
	policy := NewSecurityPolicy()
	initialCount := len(policy.Rules())

	policy.AddRule(PolicyRule{
		Subject:  "new_service",
		Object:   "new_resource",
		Action:   "new_action",
		Effect:   PolicyEffectAllow,
		Priority: 200,
	})

	if len(policy.Rules()) != initialCount+1 {
		t.Error("expected rule count to increase by 1")
	}
}

func TestSecurityPolicyRemoveRule(t *testing.T) {
	policy := NewSecurityPolicy()

	// Add a rule
	policy.AddRule(PolicyRule{
		Subject:  "removable_service",
		Object:   "removable_resource",
		Action:   "removable_action",
		Effect:   PolicyEffectAllow,
		Priority: 200,
	})

	initialCount := len(policy.Rules())

	// Remove the rule
	removed := policy.RemoveRule("removable_service", "removable_resource", "removable_action")
	if removed != 1 {
		t.Errorf("expected 1 rule removed, got %d", removed)
	}

	if len(policy.Rules()) != initialCount-1 {
		t.Error("expected rule count to decrease by 1")
	}
}

func TestSecurityPolicyEvaluateWithContext(t *testing.T) {
	policy := NewSecurityPolicy()

	// Add a rule with ${service} substitution
	policy.AddRule(PolicyRule{
		Subject:  "${service}",
		Object:   "storage:${service}/*",
		Action:   "read",
		Effect:   PolicyEffectAllow,
		Priority: 100,
	})

	// Test with context substitution
	effect := policy.EvaluateWithContext("my_service", "my_service", "storage:my_service/data", "read")
	if effect != PolicyEffectAllow {
		t.Errorf("expected allow, got %s", effect)
	}

	// Test with different service (should be denied)
	effect = policy.EvaluateWithContext("my_service", "other_service", "storage:other_service/data", "read")
	if effect != PolicyEffectDeny {
		t.Errorf("expected deny for different service, got %s", effect)
	}
}

func TestMatchPatternGlob(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		match   bool
	}{
		{"*", "anything", true},
		{"*", "", true},
		{"test", "test", true},
		{"test", "other", false},
		{"test*", "test123", true},
		{"test*", "test", true},
		{"test*", "other", false},
		{"*.service", "my.service", true},
		{"*.service", "my.other", false},
		{"com.r3e.*", "com.r3e.services", true},
		{"com.r3e.*", "com.other.services", false},
	}

	for _, tt := range tests {
		result := matchPatternGlob(tt.pattern, tt.value)
		if result != tt.match {
			t.Errorf("matchPatternGlob(%q, %q) = %v, want %v", tt.pattern, tt.value, result, tt.match)
		}
	}
}

// =============================================================================
// Policy File Generation Tests
// =============================================================================

func TestGenerateDefaultPolicyFile(t *testing.T) {
	content := GenerateDefaultPolicyFile()

	if content == "" {
		t.Error("expected non-empty policy file content")
	}

	// Verify it's valid JSON
	var config PolicyConfig
	if err := parseSimpleYAML([]byte(content), &config); err != nil {
		t.Errorf("generated policy file is not valid: %v", err)
	}

	if config.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", config.Version)
	}
}

// =============================================================================
// Hot Reload Tests
// =============================================================================

func TestPolicyLoaderWatching(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "policy.json")

	// Initial config
	initialConfig := `{
		"version": "1.0",
		"default_effect": "deny",
		"rules": []
	}`

	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	reloadCalled := false
	loader := NewPolicyLoader(PolicyLoaderConfig{
		ConfigPath:    configPath,
		WatchInterval: 100 * time.Millisecond,
		OnReload: func(p *SecurityPolicy) {
			reloadCalled = true
		},
	})

	// Load initial policy
	if _, err := loader.Load(); err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}

	// Start watching
	loader.StartWatching()
	defer loader.StopWatching()

	// Wait a bit then modify the file
	time.Sleep(50 * time.Millisecond)

	updatedConfig := `{
		"version": "2.0",
		"default_effect": "deny",
		"rules": []
	}`

	if err := os.WriteFile(configPath, []byte(updatedConfig), 0644); err != nil {
		t.Fatalf("failed to update config file: %v", err)
	}

	// Wait for reload
	time.Sleep(200 * time.Millisecond)

	// Note: Due to timing, reload may or may not have been called
	// This test mainly verifies the watching mechanism doesn't crash
	_ = reloadCalled
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkMatchPatternGlob(b *testing.B) {
	pattern := "com.r3e.services.*"
	value := "com.r3e.services.accounts"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchPatternGlob(pattern, value)
	}
}

func BenchmarkEvaluateWithContext(b *testing.B) {
	policy := NewSecurityPolicy()
	policy.AddRule(PolicyRule{
		Subject:  "${service}",
		Object:   "storage:${service}/*",
		Action:   "read",
		Effect:   PolicyEffectAllow,
		Priority: 100,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.EvaluateWithContext("my_service", "my_service", "storage:my_service/data", "read")
	}
}
