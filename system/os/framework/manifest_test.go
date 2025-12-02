package framework

import (
	"errors"
	"testing"

	"github.com/R3E-Network/service_layer/system/engine/core"
	service "github.com/R3E-Network/service_layer/system/os/framework/core"
)

func TestManifest_Normalize(t *testing.T) {
	m := &Manifest{
		Name:         "  test-service  ",
		Domain:       "  test  ",
		Description:  "  A test service  ",
		Version:      "  1.0.0  ",
		Layer:        "  SERVICE  ",
		RequiresAPIs: []engine.APISurface{"store", "store", "compute"},
		DependsOn:    []string{"dep1", "dep1", "dep2"},
		Capabilities: []string{"cap1", "cap1", "cap2"},
		Quotas:       map[string]string{"  gas  ": "  100  ", "": "empty", "valid": ""},
		Tags:         map[string]string{"  env  ": "  prod  ", "": "empty"},
	}

	m.Normalize()

	if m.Name != "test-service" {
		t.Errorf("Name = %q, want 'test-service'", m.Name)
	}
	if m.Domain != "test" {
		t.Errorf("Domain = %q, want 'test'", m.Domain)
	}
	if m.Description != "A test service" {
		t.Errorf("Description = %q, want 'A test service'", m.Description)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want '1.0.0'", m.Version)
	}
	if m.Layer != "service" {
		t.Errorf("Layer = %q, want 'service'", m.Layer)
	}

	// Check dedupe
	if len(m.RequiresAPIs) != 2 {
		t.Errorf("RequiresAPIs len = %d, want 2", len(m.RequiresAPIs))
	}
	if len(m.DependsOn) != 2 {
		t.Errorf("DependsOn len = %d, want 2", len(m.DependsOn))
	}
	if len(m.Capabilities) != 2 {
		t.Errorf("Capabilities len = %d, want 2", len(m.Capabilities))
	}

	// Check quotas cleanup
	if v, ok := m.Quotas["gas"]; !ok || v != "100" {
		t.Errorf("Quotas[gas] = %q, %v; want '100', true", v, ok)
	}
	if _, ok := m.Quotas[""]; ok {
		t.Error("empty key should be removed from Quotas")
	}
	if _, ok := m.Quotas["valid"]; ok {
		t.Error("key with empty value should be removed from Quotas")
	}

	// Check tags cleanup
	if v, ok := m.Tags["env"]; !ok || v != "prod" {
		t.Errorf("Tags[env] = %q, %v; want 'prod', true", v, ok)
	}
}

func TestManifest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		m       *Manifest
		wantErr bool
	}{
		{"nil manifest", nil, false},
		{"valid", &Manifest{Name: "test"}, false},
		{"missing name", &Manifest{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.m.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestManifest_IsEnabled(t *testing.T) {
	t.Run("nil manifest", func(t *testing.T) {
		var m *Manifest
		if !m.IsEnabled() {
			t.Error("nil manifest should be enabled by default")
		}
	})

	t.Run("nil enabled", func(t *testing.T) {
		m := &Manifest{}
		if !m.IsEnabled() {
			t.Error("nil Enabled should default to true")
		}
	})

	t.Run("enabled true", func(t *testing.T) {
		m := &Manifest{}
		m.SetEnabled(true)
		if !m.IsEnabled() {
			t.Error("should be enabled")
		}
	})

	t.Run("enabled false", func(t *testing.T) {
		m := &Manifest{}
		m.SetEnabled(false)
		if m.IsEnabled() {
			t.Error("should not be enabled")
		}
	})
}

func TestManifest_HasCapability(t *testing.T) {
	m := &Manifest{Capabilities: []string{"compute", "Store"}}

	if !m.HasCapability("compute") {
		t.Error("should have compute capability")
	}
	if !m.HasCapability("COMPUTE") {
		t.Error("capability check should be case-insensitive")
	}
	if !m.HasCapability("store") {
		t.Error("should have store capability")
	}
	if m.HasCapability("data") {
		t.Error("should not have data capability")
	}

	// Nil manifest
	var nilM *Manifest
	if nilM.HasCapability("compute") {
		t.Error("nil manifest should not have any capability")
	}
}

func TestManifest_Tags(t *testing.T) {
	m := &Manifest{}

	// Initial state
	if m.HasTag("env") {
		t.Error("should not have tag initially")
	}

	// Set and get
	m.SetTag("env", "prod")
	if !m.HasTag("env") {
		t.Error("should have tag after setting")
	}
	v, ok := m.GetTag("env")
	if !ok || v != "prod" {
		t.Errorf("GetTag = %q, %v; want 'prod', true", v, ok)
	}

	// Not found
	_, ok = m.GetTag("nonexistent")
	if ok {
		t.Error("should not find nonexistent tag")
	}
}

func TestManifest_RequiresAPI(t *testing.T) {
	m := &Manifest{RequiresAPIs: []engine.APISurface{"store", "compute"}}

	if !m.RequiresAPI("store") {
		t.Error("should require store API")
	}
	if !m.RequiresAPI("STORE") {
		t.Error("API check should be case-insensitive")
	}
	if m.RequiresAPI("data") {
		t.Error("should not require data API")
	}
}

func TestManifest_DependsOnService(t *testing.T) {
	m := &Manifest{DependsOn: []string{"accounts", "Functions"}}

	if !m.DependsOnService("accounts") {
		t.Error("should depend on accounts")
	}
	if !m.DependsOnService("ACCOUNTS") {
		t.Error("dependency check should be case-insensitive")
	}
	if !m.DependsOnService("functions") {
		t.Error("should depend on functions")
	}
	if m.DependsOnService("datafeeds") {
		t.Error("should not depend on datafeeds")
	}
}

func TestManifest_Quotas(t *testing.T) {
	m := &Manifest{}

	// Initial state
	_, ok := m.GetQuota("gas")
	if ok {
		t.Error("should not have quota initially")
	}

	// Set and get
	m.SetQuota("gas", "1000")
	v, ok := m.GetQuota("gas")
	if !ok || v != "1000" {
		t.Errorf("GetQuota = %q, %v; want '1000', true", v, ok)
	}
}

func TestManifest_Merge(t *testing.T) {
	base := &Manifest{
		Name:         "base",
		Domain:       "domain1",
		Description:  "Base service",
		RequiresAPIs: []engine.APISurface{"store"},
		DependsOn:    []string{"dep1"},
		Capabilities: []string{"cap1"},
		Quotas:       map[string]string{"gas": "100"},
		Tags:         map[string]string{"env": "dev"},
	}

	override := &Manifest{
		Name:         "override",
		Version:      "2.0.0",
		RequiresAPIs: []engine.APISurface{"compute"},
		DependsOn:    []string{"dep2"},
		Capabilities: []string{"cap2"},
		Quotas:       map[string]string{"rpc": "50"},
		Tags:         map[string]string{"tier": "premium"},
	}

	base.Merge(override)

	// Override non-empty values
	if base.Name != "override" {
		t.Errorf("Name = %q, want 'override'", base.Name)
	}
	if base.Version != "2.0.0" {
		t.Errorf("Version = %q, want '2.0.0'", base.Version)
	}
	// Original stays if override is empty
	if base.Domain != "domain1" {
		t.Errorf("Domain = %q, want 'domain1'", base.Domain)
	}
	if base.Description != "Base service" {
		t.Errorf("Description = %q, want 'Base service'", base.Description)
	}

	// Lists merged
	if len(base.RequiresAPIs) != 2 {
		t.Errorf("RequiresAPIs len = %d, want 2", len(base.RequiresAPIs))
	}
	if len(base.DependsOn) != 2 {
		t.Errorf("DependsOn len = %d, want 2", len(base.DependsOn))
	}
	if len(base.Capabilities) != 2 {
		t.Errorf("Capabilities len = %d, want 2", len(base.Capabilities))
	}

	// Maps merged
	if base.Quotas["gas"] != "100" {
		t.Error("original quota should be preserved")
	}
	if base.Quotas["rpc"] != "50" {
		t.Error("override quota should be added")
	}
	if base.Tags["env"] != "dev" {
		t.Error("original tag should be preserved")
	}
	if base.Tags["tier"] != "premium" {
		t.Error("override tag should be added")
	}
}

func TestManifest_Clone(t *testing.T) {
	original := &Manifest{
		Name:         "test",
		Domain:       "domain",
		Description:  "Test service",
		Version:      "1.0.0",
		Layer:        "service",
		RequiresAPIs: []engine.APISurface{"store"},
		DependsOn:    []string{"dep1"},
		Capabilities: []string{"cap1"},
		Quotas:       map[string]string{"gas": "100"},
		Tags:         map[string]string{"env": "prod"},
	}
	original.SetEnabled(true)

	clone := original.Clone()

	// Values should match
	if clone.Name != original.Name {
		t.Errorf("Name = %q, want %q", clone.Name, original.Name)
	}
	if clone.Version != original.Version {
		t.Errorf("Version = %q, want %q", clone.Version, original.Version)
	}
	if clone.IsEnabled() != original.IsEnabled() {
		t.Error("Enabled should match")
	}

	// Modifications should not affect original
	clone.Name = "modified"
	clone.RequiresAPIs[0] = "compute"
	clone.Quotas["gas"] = "200"

	if original.Name == "modified" {
		t.Error("original Name should not change")
	}
	if original.RequiresAPIs[0] == "compute" {
		t.Error("original RequiresAPIs should not change")
	}
	if original.Quotas["gas"] == "200" {
		t.Error("original Quotas should not change")
	}
}

func TestManifest_ToDescriptor(t *testing.T) {
	m := &Manifest{
		Name:         "test-service",
		Domain:       "test",
		Layer:        "runner",
		Capabilities: []string{"cap1", "cap2"},
		DependsOn:    []string{"dep1"},
		RequiresAPIs: []engine.APISurface{"store", "compute"},
	}

	d := m.ToDescriptor()

	if d.Name != "test-service" {
		t.Errorf("Name = %q, want 'test-service'", d.Name)
	}
	if d.Domain != "test" {
		t.Errorf("Domain = %q, want 'test'", d.Domain)
	}
	if d.Layer != service.LayerRunner {
		t.Errorf("Layer = %v, want LayerRunner", d.Layer)
	}
	if len(d.Capabilities) != 2 {
		t.Errorf("Capabilities len = %d, want 2", len(d.Capabilities))
	}
	if len(d.RequiresAPIs) != 2 {
		t.Errorf("RequiresAPIs len = %d, want 2", len(d.RequiresAPIs))
	}
}

func TestManifest_ToDescriptor_Layers(t *testing.T) {
	tests := []struct {
		layer    string
		expected service.Layer
	}{
		{"service", service.LayerService},
		{"runner", service.LayerRunner},
		{"infra", service.LayerInfra},
		{"platform", service.LayerPlatform},
		{"", service.LayerService},        // default
		{"unknown", service.LayerService}, // default
	}

	for _, tc := range tests {
		t.Run(tc.layer, func(t *testing.T) {
			m := &Manifest{Name: "test", Layer: tc.layer}
			d := m.ToDescriptor()
			if d.Layer != tc.expected {
				t.Errorf("Layer = %v, want %v", d.Layer, tc.expected)
			}
		})
	}
}

func TestManifestFromDescriptor(t *testing.T) {
	d := service.Descriptor{
		Name:         "test-service",
		Domain:       "test",
		Layer:        service.LayerInfra,
		Capabilities: []string{"cap1"},
		DependsOn:    []string{"dep1"},
		RequiresAPIs: []string{"store"},
	}

	m := ManifestFromDescriptor(d)

	if m.Name != "test-service" {
		t.Errorf("Name = %q, want 'test-service'", m.Name)
	}
	if m.Domain != "test" {
		t.Errorf("Domain = %q, want 'test'", m.Domain)
	}
	if m.Layer != "infra" {
		t.Errorf("Layer = %q, want 'infra'", m.Layer)
	}
	if len(m.Capabilities) != 1 {
		t.Errorf("Capabilities len = %d, want 1", len(m.Capabilities))
	}
	if len(m.RequiresAPIs) != 1 {
		t.Errorf("RequiresAPIs len = %d, want 1", len(m.RequiresAPIs))
	}
}

func TestManifestValidator(t *testing.T) {
	customErr := errors.New("custom validation failed")

	validator := ManifestValidatorFunc(func(m *Manifest) error {
		if m.Version == "" {
			return customErr
		}
		return nil
	})

	t.Run("validation fails", func(t *testing.T) {
		m := &Manifest{Name: "test"}
		err := m.ValidateWith(validator)
		if err != customErr {
			t.Errorf("ValidateWith() = %v, want %v", err, customErr)
		}
	})

	t.Run("validation passes", func(t *testing.T) {
		m := &Manifest{Name: "test", Version: "1.0.0"}
		err := m.ValidateWith(validator)
		if err != nil {
			t.Errorf("ValidateWith() = %v, want nil", err)
		}
	})

	t.Run("nil validator", func(t *testing.T) {
		m := &Manifest{Name: "test"}
		err := m.ValidateWith(nil)
		if err != nil {
			t.Errorf("ValidateWith(nil) = %v, want nil", err)
		}
	})
}

func TestManifest_NilReceiver(t *testing.T) {
	var m *Manifest

	// All these should not panic
	m.Normalize()

	if m.HasCapability("test") {
		t.Error("nil manifest should not have capability")
	}
	if m.HasTag("test") {
		t.Error("nil manifest should not have tag")
	}
	if m.RequiresAPI("test") {
		t.Error("nil manifest should not require API")
	}
	if m.DependsOnService("test") {
		t.Error("nil manifest should not depend on service")
	}

	_, ok := m.GetTag("test")
	if ok {
		t.Error("nil manifest GetTag should return false")
	}
	_, ok = m.GetQuota("test")
	if ok {
		t.Error("nil manifest GetQuota should return false")
	}

	if m.Clone() != nil {
		t.Error("nil manifest Clone should return nil")
	}

	d := m.ToDescriptor()
	if d.Name != "" {
		t.Error("nil manifest ToDescriptor should return empty descriptor")
	}
}
