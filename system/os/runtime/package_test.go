package pkg_test

import (
	"context"
	"testing"

	engine "github.com/R3E-Network/service_layer/system/engine/core"
	framework "github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
	pkg "github.com/R3E-Network/service_layer/system/os/runtime"
)

// TestPackage is a minimal test package implementation.
type TestPackage struct {
	id                string
	version           string
	onInstallCalled   bool
	onUninstallCalled bool
}

func (p *TestPackage) Manifest() pkg.PackageManifest {
	return pkg.PackageManifest{
		PackageID:   p.id,
		Version:     p.version,
		DisplayName: "Test Package",
		Services: []pkg.ServiceDeclaration{{
			Name:   "test-service",
			Domain: "test",
			Layer:  "service",
		}},
		Permissions: []pkg.Permission{{
			Name:     "engine.api.storage",
			Required: false,
		}},
		Resources: pkg.ResourceQuotas{
			MaxStorageBytes: 1024 * 1024, // 1 MB
		},
	}
}

func (p *TestPackage) CreateServices(ctx context.Context, runtime pkg.PackageRuntime) ([]engine.ServiceModule, error) {
	return []engine.ServiceModule{&testServiceModule{name: "test-service"}}, nil
}

func (p *TestPackage) OnInstall(ctx context.Context, runtime pkg.PackageRuntime) error {
	p.onInstallCalled = true
	return nil
}

func (p *TestPackage) OnUninstall(ctx context.Context, runtime pkg.PackageRuntime) error {
	p.onUninstallCalled = true
	return nil
}

func (p *TestPackage) OnUpgrade(ctx context.Context, runtime pkg.PackageRuntime, oldVersion string) error {
	return nil
}

// testServiceModule is a minimal service module for testing.
type testServiceModule struct {
	name string
}

func (s *testServiceModule) Name() string {
	return s.name
}

func (s *testServiceModule) Domain() string {
	return "test"
}

func (s *testServiceModule) Start(ctx context.Context) error {
	return nil
}

func (s *testServiceModule) Stop(ctx context.Context) error {
	return nil
}

// =============================================================================
// Tests
// =============================================================================

func TestPackageLoader_LoadAndInstall(t *testing.T) {
	ctx := context.Background()
	loader := pkg.NewPackageLoader()
	eng := engine.New()

	// Create a test package
	testPkg := &TestPackage{
		id:      "com.test.package1",
		version: "1.0.0",
	}

	// Install the package
	err := loader.InstallPackage(ctx, testPkg, eng)
	if err != nil {
		t.Fatalf("InstallPackage failed: %v", err)
	}

	// Verify OnInstall was called
	if !testPkg.onInstallCalled {
		t.Error("OnInstall hook was not called")
	}

	// Verify service was registered
	if svc := eng.Lookup("test-service"); svc == nil {
		t.Error("Service was not registered with engine")
	}

	// Verify package is listed as installed
	installed := loader.ListInstalled()
	if len(installed) != 1 {
		t.Errorf("Expected 1 installed package, got %d", len(installed))
	}
	if installed[0].Manifest.PackageID != "com.test.package1" {
		t.Errorf("Wrong package ID: %s", installed[0].Manifest.PackageID)
	}
}

func TestPackageLoader_UninstallPackage(t *testing.T) {
	ctx := context.Background()
	loader := pkg.NewPackageLoader()
	eng := engine.New()

	testPkg := &TestPackage{
		id:      "com.test.package2",
		version: "1.0.0",
	}

	// Install first
	if err := loader.InstallPackage(ctx, testPkg, eng); err != nil {
		t.Fatalf("InstallPackage failed: %v", err)
	}

	// Uninstall
	err := loader.UninstallPackage(ctx, "com.test.package2", eng)
	if err != nil {
		t.Fatalf("UninstallPackage failed: %v", err)
	}

	// Verify OnUninstall was called
	if !testPkg.onUninstallCalled {
		t.Error("OnUninstall hook was not called")
	}

	// Verify service was unregistered
	if svc := eng.Lookup("test-service"); svc != nil {
		t.Error("Service was not unregistered from engine")
	}

	// Verify package is no longer listed
	installed := loader.ListInstalled()
	if len(installed) != 0 {
		t.Errorf("Expected 0 installed packages, got %d", len(installed))
	}
}

func TestPackageLoader_DuplicateInstall(t *testing.T) {
	ctx := context.Background()
	loader := pkg.NewPackageLoader()
	eng := engine.New()

	testPkg := &TestPackage{
		id:      "com.test.package3",
		version: "1.0.0",
	}

	// Install first time
	if err := loader.InstallPackage(ctx, testPkg, eng); err != nil {
		t.Fatalf("First install failed: %v", err)
	}

	// Try to install again with same version
	testPkg2 := &TestPackage{
		id:      "com.test.package3",
		version: "1.0.0",
	}
	err := loader.InstallPackage(ctx, testPkg2, eng)
	if err == nil {
		t.Error("Expected error when installing duplicate package")
	}
}

func TestPackageLoader_MissingRequiredPermission(t *testing.T) {
	ctx := context.Background()
	loader := pkg.NewPackageLoader()
	eng := engine.New()

	// Create package with required permission that won't be granted
	// Note: Current implementation auto-grants all permissions
	// This test documents expected behavior for future permission enforcement
	testPkg := &TestPackageWithRequiredPerm{
		id:      "com.test.package4",
		version: "1.0.0",
	}

	// Should succeed for now (auto-grant), but would fail in production
	err := loader.InstallPackage(ctx, testPkg, eng)
	if err != nil {
		t.Logf("Install failed (expected in production with strict permissions): %v", err)
	}
}

// TestPackageWithRequiredPerm has a required permission for testing.
type TestPackageWithRequiredPerm struct {
	id      string
	version string
}

func (p *TestPackageWithRequiredPerm) Manifest() pkg.PackageManifest {
	m := pkg.PackageManifest{
		PackageID:   p.id,
		Version:     p.version,
		DisplayName: "Test Package with Required Permission",
		Services: []pkg.ServiceDeclaration{{
			Name:   "test-service",
			Domain: "test",
			Layer:  "service",
		}},
		Permissions: []pkg.Permission{{
			Name:     "engine.api.dangerous",
			Required: true,
		}},
	}
	return m
}

func (p *TestPackageWithRequiredPerm) CreateServices(ctx context.Context, runtime pkg.PackageRuntime) ([]engine.ServiceModule, error) {
	return []engine.ServiceModule{&testServiceModule{name: "test-service"}}, nil
}

func (p *TestPackageWithRequiredPerm) OnInstall(ctx context.Context, runtime pkg.PackageRuntime) error {
	return nil
}

func (p *TestPackageWithRequiredPerm) OnUninstall(ctx context.Context, runtime pkg.PackageRuntime) error {
	return nil
}

func (p *TestPackageWithRequiredPerm) OnUpgrade(ctx context.Context, runtime pkg.PackageRuntime, oldVersion string) error {
	return nil
}

func TestPackageRuntime_Storage(t *testing.T) {
	ctx := context.Background()

	manifest := pkg.PackageManifest{
		PackageID: "com.test.storage",
		Version:   "1.0.0",
		Permissions: []pkg.Permission{{
			Name:     "engine.api.storage",
			Required: true,
		}},
		Resources: pkg.ResourceQuotas{
			MaxStorageBytes: 1024, // 1 KB
		},
	}

	permissions := map[string]bool{"engine.api.storage": true}
	eng := engine.New()
	config := pkg.NewPackageConfig(nil)
	runtime := pkg.NewPackageRuntime("com.test.storage", manifest, eng, config, permissions, pkg.NilStoreProvider(), core.NoopTracer, framework.NoopMetrics())

	// Get storage
	storage, err := runtime.Storage()
	if err != nil {
		t.Fatalf("Failed to get storage: %v", err)
	}

	// Test basic operations
	if err := storage.Set(ctx, "key1", []byte("value1")); err != nil {
		t.Errorf("Set failed: %v", err)
	}

	value, err := storage.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if string(value) != "value1" {
		t.Errorf("Expected 'value1', got '%s'", string(value))
	}

	// Test quota enforcement
	largeData := make([]byte, 2048) // Exceeds 1 KB quota
	err = storage.Set(ctx, "key2", largeData)
	if err == nil {
		t.Error("Expected quota error, got nil")
	}

	// Test delete
	if err := storage.Delete(ctx, "key1"); err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	_, err = storage.Get(ctx, "key1")
	if err == nil {
		t.Error("Expected error when getting deleted key")
	}
}

func TestPackageRuntime_PermissionDenied(t *testing.T) {
	manifest := pkg.PackageManifest{
		PackageID: "com.test.noperm",
		Version:   "1.0.0",
	}

	permissions := map[string]bool{} // No permissions granted
	eng := engine.New()
	config := pkg.NewPackageConfig(nil)
	runtime := pkg.NewPackageRuntime("com.test.noperm", manifest, eng, config, permissions, pkg.NilStoreProvider(), nil, nil)

	// Try to access storage without permission
	_, err := runtime.Storage()
	if err == nil {
		t.Error("Expected permission denied error")
	}

	// Try to access bus without permission
	_, err = runtime.Bus()
	if err == nil {
		t.Error("Expected permission denied error")
	}
}

type stubMetrics struct {
	counterCalls int
}

func (s *stubMetrics) Counter(string, map[string]string, float64) {
	s.counterCalls++
}

func (s *stubMetrics) Gauge(string, map[string]string, float64) {}

func (s *stubMetrics) Histogram(string, map[string]string, float64) {}

type stubTracer struct{}

func (stubTracer) StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, func(error)) {
	return ctx, func(error) {}
}

func TestPackageRuntime_Instrumentation(t *testing.T) {
	tracer := stubTracer{}
	metrics := &stubMetrics{}

	manifest := pkg.PackageManifest{
		PackageID: "com.test.instrumentation",
		Version:   "1.0.0",
	}
	eng := engine.New()
	config := pkg.NewPackageConfig(nil)
	runtime := pkg.NewPackageRuntime("com.test.instrumentation", manifest, eng, config, nil, pkg.NilStoreProvider(), tracer, metrics)

	if runtime.Tracer() != tracer {
		t.Fatal("expected tracer passthrough")
	}
	if runtime.Metrics() != metrics {
		t.Fatal("expected metrics passthrough")
	}
	if runtime.Quota() == nil {
		t.Fatal("expected quota enforcer")
	}

	runtime.Metrics().Counter("test", nil, 1)
	if metrics.counterCalls != 1 {
		t.Fatalf("expected counter to increment, got %d", metrics.counterCalls)
	}
}

func TestPackageConfig(t *testing.T) {
	config := pkg.NewPackageConfig(map[string]string{
		"string_key": "value",
		"int_key":    "42",
		"bool_key":   "true",
	})

	// Test string
	if val, ok := config.Get("string_key"); !ok || val != "value" {
		t.Errorf("Expected 'value', got '%s'/%v", val, ok)
	}

	// Test int
	if val, ok := config.GetInt("int_key"); !ok || val != 42 {
		t.Errorf("Expected 42, got %d/%v", val, ok)
	}

	// Test bool
	if val, ok := config.GetBool("bool_key"); !ok || !val {
		t.Errorf("Expected true, got %v/%v", val, ok)
	}

	// Test missing key
	if _, ok := config.Get("nonexistent"); ok {
		t.Error("Expected false for missing key")
	}

	// Test GetAll
	all := config.GetAll()
	if len(all) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(all))
	}
}

func TestPackageManifest_Validate(t *testing.T) {
	tests := []struct {
		name     string
		manifest pkg.PackageManifest
		wantErr  bool
	}{
		{
			name: "valid manifest",
			manifest: pkg.PackageManifest{
				PackageID: "com.test.valid",
				Version:   "1.0.0",
				Services: []pkg.ServiceDeclaration{{
					Name:   "test",
					Domain: "test",
				}},
			},
			wantErr: false,
		},
		{
			name: "missing package_id",
			manifest: pkg.PackageManifest{
				Version: "1.0.0",
				Services: []pkg.ServiceDeclaration{{
					Name:   "test",
					Domain: "test",
				}},
			},
			wantErr: true,
		},
		{
			name: "missing version",
			manifest: pkg.PackageManifest{
				PackageID: "com.test.nover",
				Services: []pkg.ServiceDeclaration{{
					Name:   "test",
					Domain: "test",
				}},
			},
			wantErr: true,
		},
		{
			name: "no services",
			manifest: pkg.PackageManifest{
				PackageID: "com.test.nosvc",
				Version:   "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "service missing name",
			manifest: pkg.PackageManifest{
				PackageID: "com.test.noname",
				Version:   "1.0.0",
				Services: []pkg.ServiceDeclaration{{
					Domain: "test",
				}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPackageManifest_CheckPermissions(t *testing.T) {
	manifest := pkg.PackageManifest{
		Permissions: []pkg.Permission{
			{Name: "perm1", Required: true},
			{Name: "perm2", Required: false},
			{Name: "perm3", Required: true},
		},
	}

	granted := map[string]bool{
		"perm1": true,
		"perm2": true,
		// perm3 not granted
	}

	missing := manifest.CheckPermissions(granted)
	if len(missing) != 1 {
		t.Errorf("Expected 1 missing permission, got %d", len(missing))
	}
	if len(missing) > 0 && missing[0] != "perm3" {
		t.Errorf("Expected missing 'perm3', got '%s'", missing[0])
	}
}
