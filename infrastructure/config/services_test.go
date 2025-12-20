package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultServicesConfig(t *testing.T) {
	cfg := DefaultServicesConfig()
	if cfg == nil {
		t.Fatal("DefaultServicesConfig() returned nil")
	}

	expectedServices := []string{
		"globalsigner",
		"txproxy",
		"neofeeds",
		"neoflow",
		"neoaccounts",
		"neocompute",
		"neovrf",
		"neooracle",
	}

	for _, svc := range expectedServices {
		settings, ok := cfg.Services[svc]
		if !ok {
			t.Errorf("missing service %q in default config", svc)
			continue
		}
		if !settings.Enabled {
			t.Errorf("service %q should be enabled by default", svc)
		}
		if settings.Port == 0 {
			t.Errorf("service %q has no port configured", svc)
		}
		if settings.Description == "" {
			t.Errorf("service %q has no description", svc)
		}
	}
}

func TestGetNeoServiceName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"oracle", "neooracle"},
		{"neofeeds", "neofeeds"},
		{"neoaccounts", "neoaccounts"},
		{"neocompute", "neocompute"},
		{"neoflow", "neoflow"},
		{"vrf", "neovrf"},
		{"tx-proxy", "txproxy"},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := GetNeoServiceName(tt.input)
			if got != tt.expected {
				t.Errorf("GetNeoServiceName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLoadServicesConfigFromPath(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "services.yaml")

		configContent := `
services:
  testservice:
    enabled: true
    port: 8080
    description: "Test service"
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}

		cfg, err := LoadServicesConfigFromPath(configPath)
		if err != nil {
			t.Fatalf("LoadServicesConfigFromPath() error = %v", err)
		}

		if cfg == nil {
			t.Fatal("LoadServicesConfigFromPath() returned nil")
		}

		svc, ok := cfg.Services["testservice"]
		if !ok {
			t.Fatal("testservice not found in config")
		}
		if svc.Port != 8080 {
			t.Errorf("port = %d, want 8080", svc.Port)
		}
		if !svc.Enabled {
			t.Error("service should be enabled")
		}
	})

	t.Run("missing port", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "services.yaml")

		configContent := `
services:
  testservice:
    enabled: true
    description: "Test service"
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}

		_, err := LoadServicesConfigFromPath(configPath)
		if err == nil {
			t.Error("expected error for missing port")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := LoadServicesConfigFromPath("/nonexistent/path/services.yaml")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "services.yaml")

		if err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}

		_, err := LoadServicesConfigFromPath(configPath)
		if err == nil {
			t.Error("expected error for invalid yaml")
		}
	})
}

func TestLoadServicesConfigOrDefault(t *testing.T) {
	// This should return default config since config/services.yaml likely doesn't exist in test
	cfg := LoadServicesConfigOrDefault()
	if cfg == nil {
		t.Fatal("LoadServicesConfigOrDefault() returned nil")
	}

	// Should have default services
	if len(cfg.Services) == 0 {
		t.Error("expected non-empty services map")
	}
}

func TestServiceNameMapping(t *testing.T) {
	if len(ServiceNameMapping) == 0 {
		t.Error("ServiceNameMapping should not be empty")
	}

	// Verify all mappings are valid
	for old, new := range ServiceNameMapping {
		if old == "" {
			t.Error("empty key in ServiceNameMapping")
		}
		if new == "" {
			t.Errorf("empty value for key %q in ServiceNameMapping", old)
		}
	}
}
