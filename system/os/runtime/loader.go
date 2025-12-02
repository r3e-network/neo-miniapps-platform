package pkg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	engine "github.com/R3E-Network/service_layer/system/os/core"
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
	"github.com/R3E-Network/service_layer/system/os/sandbox"
)

// loader is the default implementation of PackageLoader.
type loader struct {
	mu            sync.RWMutex
	installed     map[string]*installedPackageRecord
	factories     map[string]PackageFactory // for dynamic loading
	storeProvider StoreProvider             // shared store provider for all packages
	tracer        core.Tracer
	metrics       framework.Metrics

	// Sandbox integration (optional - for enhanced isolation)
	sandboxManager *sandbox.Manager
	useSandbox     bool // feature flag for sandbox mode
	db             *sql.DB

	// Permission policy for security evaluation
	permissionPolicy PermissionPolicy
}

// PermissionPolicy defines the interface for permission evaluation.
type PermissionPolicy interface {
	// IsAllowed checks if a package is allowed to have a specific permission.
	IsAllowed(packageID, permission string) bool
}

// PackageFactory creates a ServicePackage instance.
type PackageFactory func() (ServicePackage, error)

// installedPackageRecord tracks an installed package.
type installedPackageRecord struct {
	Package     ServicePackage
	Manifest    PackageManifest
	InstalledAt time.Time
	Enabled     bool
	Services    []string // service names registered by this package
	Runtime     PackageRuntime
}

// NewPackageLoader creates a new package loader.
func NewPackageLoader() PackageLoader {
	return &loader{
		installed:     make(map[string]*installedPackageRecord),
		factories:     make(map[string]PackageFactory),
		storeProvider: NilStoreProvider(),
		tracer:        core.NoopTracer,
		metrics:       framework.NoopMetrics(),
	}
}

// NewPackageLoaderWithStores creates a new package loader with store provider.
func NewPackageLoaderWithStores(stores StoreProvider) PackageLoader {
	return &loader{
		installed:     make(map[string]*installedPackageRecord),
		factories:     make(map[string]PackageFactory),
		storeProvider: stores,
		tracer:        core.NoopTracer,
		metrics:       framework.NoopMetrics(),
	}
}

// NewSandboxedPackageLoader creates a package loader with sandbox isolation enabled.
// This provides Android-style service isolation for enhanced security.
func NewSandboxedPackageLoader(db *sql.DB, stores StoreProvider) PackageLoader {
	sandboxMgr := sandbox.NewManager(db, sandbox.DefaultManagerConfig())
	return &loader{
		installed:      make(map[string]*installedPackageRecord),
		factories:      make(map[string]PackageFactory),
		storeProvider:  stores,
		tracer:         core.NoopTracer,
		metrics:        framework.NoopMetrics(),
		sandboxManager: sandboxMgr,
		useSandbox:     true,
		db:             db,
	}
}

// EnableSandbox enables sandbox mode for the loader.
// Must be called before installing packages.
func (l *loader) EnableSandbox(db *sql.DB) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.sandboxManager == nil {
		l.sandboxManager = sandbox.NewManager(db, sandbox.DefaultManagerConfig())
	}
	l.db = db
	l.useSandbox = true
}

// DisableSandbox disables sandbox mode (for backward compatibility).
func (l *loader) DisableSandbox() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.useSandbox = false
}

// SandboxManager returns the sandbox manager (nil if sandbox not enabled).
func (l *loader) SandboxManager() *sandbox.Manager {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.sandboxManager
}

// SetStoreProvider sets the store provider for the loader.
// This should be called before installing packages.
func (l *loader) SetStoreProvider(stores StoreProvider) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.storeProvider = stores
}

// SetTracer configures the tracer available to package runtimes.
func (l *loader) SetTracer(t core.Tracer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if t == nil {
		t = core.NoopTracer
	}
	l.tracer = t
}

// SetMetrics configures the metrics recorder for package runtimes.
func (l *loader) SetMetrics(m framework.Metrics) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if m == nil {
		m = framework.NoopMetrics()
	}
	l.metrics = m
}

// RegisterFactory registers a package factory for dynamic loading.
// This allows packages to self-register during init().
func (l *loader) RegisterFactory(packageID string, factory PackageFactory) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.factories[packageID]; exists {
		return fmt.Errorf("package factory already registered: %s", packageID)
	}

	l.factories[packageID] = factory
	return nil
}

func (l *loader) LoadPackage(ctx context.Context, source string) (ServicePackage, error) {
	_ = ctx

	// Try loading from registered factories first
	l.mu.RLock()
	factory, exists := l.factories[source]
	l.mu.RUnlock()

	if exists {
		return factory()
	}

	// TODO: Support loading from file system, network, etc.
	return nil, fmt.Errorf("package not found: %s", source)
}

func (l *loader) InstallPackage(ctx context.Context, pkg ServicePackage, eng *engine.Engine) error {
	manifest := pkg.Manifest()

	// Validate manifest
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if already installed
	if existing, exists := l.installed[manifest.PackageID]; exists {
		// Handle upgrade
		if existing.Manifest.Version != manifest.Version {
			if err := pkg.OnUpgrade(ctx, existing.Runtime, existing.Manifest.Version); err != nil {
				return fmt.Errorf("upgrade failed: %w", err)
			}
		} else {
			return fmt.Errorf("package already installed: %s@%s", manifest.PackageID, manifest.Version)
		}
	}

	// Grant permissions
	permissions := l.evaluatePermissions(manifest)

	// Check for missing required permissions
	missing := manifest.CheckPermissions(permissions)
	if len(missing) > 0 {
		return fmt.Errorf("missing required permissions: %v", missing)
	}

	// Create package runtime (sandbox or legacy mode)
	config := NewPackageConfig(nil) // TODO: Load from config file
	var runtime PackageRuntime

	if l.useSandbox && l.sandboxManager != nil {
		// Create sandboxed runtime with Android-style isolation
		sandboxedRuntime, err := NewSandboxedRuntime(ctx, l.sandboxManager, SandboxedRuntimeConfig{
			PackageID:     manifest.PackageID,
			Manifest:      manifest,
			Engine:        eng,
			Config:        config,
			StoreProvider: l.storeProvider,
			Tracer:        l.tracer,
			Metrics:       l.metrics,
			DB:            l.db,
		})
		if err != nil {
			return fmt.Errorf("create sandboxed runtime: %w", err)
		}
		runtime = sandboxedRuntime
	} else {
		// Legacy runtime (backward compatibility)
		runtime = NewPackageRuntime(manifest.PackageID, manifest, eng, config, permissions, l.storeProvider, l.tracer, l.metrics)
	}

	// Call OnInstall hook
	if err := pkg.OnInstall(ctx, runtime); err != nil {
		// Cleanup sandbox on failure
		if l.useSandbox && l.sandboxManager != nil {
			_ = l.sandboxManager.DestroySandbox(ctx, manifest.PackageID)
		}
		return fmt.Errorf("install hook failed: %w", err)
	}

	// Create services
	services, err := pkg.CreateServices(ctx, runtime)
	if err != nil {
		// Cleanup sandbox on failure
		if l.useSandbox && l.sandboxManager != nil {
			_ = l.sandboxManager.DestroySandbox(ctx, manifest.PackageID)
		}
		return fmt.Errorf("create services failed: %w", err)
	}
	env := NewServiceEnvironment(runtime)
	for _, svc := range services {
		if aware, ok := svc.(framework.EnvironmentAware); ok {
			aware.SetEnvironment(env)
		}
	}

	// Register services with engine
	var serviceNames []string
	for _, svc := range services {
		if err := eng.Register(svc); err != nil {
			// Rollback: unregister already registered services
			for _, name := range serviceNames {
				_ = eng.Unregister(name)
			}
			// Cleanup sandbox on failure
			if l.useSandbox && l.sandboxManager != nil {
				_ = l.sandboxManager.DestroySandbox(ctx, manifest.PackageID)
			}
			return fmt.Errorf("register service %s: %w", svc.Name(), err)
		}
		serviceNames = append(serviceNames, svc.Name())
	}

	// Record installation
	l.installed[manifest.PackageID] = &installedPackageRecord{
		Package:     pkg,
		Manifest:    manifest,
		InstalledAt: time.Now(),
		Enabled:     true,
		Services:    serviceNames,
		Runtime:     runtime,
	}

	return nil
}

func (l *loader) UninstallPackage(ctx context.Context, packageID string, eng *engine.Engine) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	record, exists := l.installed[packageID]
	if !exists {
		return fmt.Errorf("package not installed: %s", packageID)
	}

	// Call OnUninstall hook
	if err := record.Package.OnUninstall(ctx, record.Runtime); err != nil {
		return fmt.Errorf("uninstall hook failed: %w", err)
	}

	// Unregister services from engine
	for _, svcName := range record.Services {
		if err := eng.Unregister(svcName); err != nil {
			// Log but continue
			if log := eng.Logger(); log != nil {
				log.Printf("warning: failed to unregister service %s: %v", svcName, err)
			}
		}
	}

	// Destroy sandbox if enabled
	if l.useSandbox && l.sandboxManager != nil {
		if err := l.sandboxManager.DestroySandbox(ctx, packageID); err != nil {
			// Log but continue - sandbox may not exist for legacy packages
			if log := eng.Logger(); log != nil {
				log.Printf("warning: failed to destroy sandbox for %s: %v", packageID, err)
			}
		}
	}

	// Remove from installed packages
	delete(l.installed, packageID)

	return nil
}

func (l *loader) ListInstalled() []InstalledPackage {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []InstalledPackage
	for _, record := range l.installed {
		result = append(result, InstalledPackage{
			Manifest:    record.Manifest,
			InstalledAt: record.InstalledAt.Format(time.RFC3339),
			Enabled:     record.Enabled,
			Services:    record.Services,
		})
	}

	return result
}

// evaluatePermissions determines which permissions to grant based on the manifest.
// Implements policy-based permission evaluation with principle of least privilege.
func (l *loader) evaluatePermissions(manifest PackageManifest) map[string]bool {
	granted := make(map[string]bool)

	// Define permission categories for security classification
	safePermissions := map[string]bool{
		"engine.api.logging":     true,
		"engine.api.metrics":     true,
		"engine.api.health":      true,
		"storage.read":           true,
		"bus.subscribe":          true,
	}

	// Permissions requiring elevated trust (package must be signed/verified)
	elevatedPermissions := map[string]bool{
		"storage.write":          true,
		"bus.publish":            true,
		"service.call":           true,
		"database.read":          true,
	}

	// Sensitive permissions (require explicit policy allowance)
	sensitivePermissions := map[string]bool{
		"database.write":         true,
		"network.outbound":       true,
		"crypto.sign":            true,
		"secrets.read":           true,
		"secrets.write":          true,
		"admin.manage":           true,
	}

	for _, perm := range manifest.Permissions {
		permName := perm.Name

		// Safe permissions are always granted
		if safePermissions[permName] {
			granted[permName] = true
			continue
		}

		// Elevated permissions granted if package is from trusted source
		if elevatedPermissions[permName] {
			if l.isTrustedPackage(manifest) {
				granted[permName] = true
			}
			continue
		}

		// Sensitive permissions require explicit policy check
		if sensitivePermissions[permName] {
			if l.isPolicyAllowed(manifest.PackageID, permName) {
				granted[permName] = true
			}
			continue
		}

		// Unknown permissions are denied by default (principle of least privilege)
		// Log for audit purposes
	}

	// Always grant basic permissions
	granted["engine.api.logging"] = true

	return granted
}

// isTrustedPackage checks if a package comes from a trusted source.
func (l *loader) isTrustedPackage(manifest PackageManifest) bool {
	// Check if package ID is in the trusted namespace
	trustedNamespaces := []string{
		"com.r3e.services.",
		"com.r3e.core.",
		"system.",
	}

	for _, ns := range trustedNamespaces {
		if strings.HasPrefix(manifest.PackageID, ns) {
			return true
		}
	}

	// Check if package has valid signature via metadata
	if sig, ok := manifest.Metadata["signature"]; ok && sig != "" {
		// In production, verify signature against trusted keys
		return true
	}

	return false
}

// isPolicyAllowed checks if a specific permission is allowed by security policy.
func (l *loader) isPolicyAllowed(pkgID, permission string) bool {
	// Check against configured security policies
	// Default: deny sensitive permissions unless explicitly allowed

	// Built-in policy: core services get full access
	if strings.HasPrefix(pkgID, "com.r3e.services.") {
		return true
	}

	// Check loader's permission policy if configured
	if l.permissionPolicy != nil {
		return l.permissionPolicy.IsAllowed(pkgID, permission)
	}

	return false
}

// =============================================================================
// Global Package Registry (for self-registration)
// =============================================================================

var (
	globalLoader     PackageLoader
	globalLoaderOnce sync.Once
)

// GlobalLoader returns the global package loader instance.
func GlobalLoader() PackageLoader {
	globalLoaderOnce.Do(func() {
		globalLoader = NewPackageLoader()
	})
	return globalLoader
}

// RegisterPackage registers a package factory with the global loader.
// This is intended to be called from init() functions.
func RegisterPackage(packageID string, factory PackageFactory) error {
	if l, ok := GlobalLoader().(*loader); ok {
		return l.RegisterFactory(packageID, factory)
	}
	return fmt.Errorf("global loader does not support factory registration")
}

// MustRegisterPackage is like RegisterPackage but panics on error.
func MustRegisterPackage(packageID string, factory PackageFactory) {
	if err := RegisterPackage(packageID, factory); err != nil {
		panic(fmt.Sprintf("failed to register package %s: %v", packageID, err))
	}
}

// SetGlobalStoreProvider sets the store provider for the global loader.
// This should be called before installing any packages.
func SetGlobalStoreProvider(stores StoreProvider) {
	if l, ok := GlobalLoader().(*loader); ok {
		l.SetStoreProvider(stores)
	}
}

// SetGlobalTracer sets the tracer shared across package runtimes.
func SetGlobalTracer(tracer core.Tracer) {
	if l, ok := GlobalLoader().(*loader); ok {
		l.SetTracer(tracer)
	}
}

// SetGlobalMetrics sets the metrics recorder shared across package runtimes.
func SetGlobalMetrics(m framework.Metrics) {
	if l, ok := GlobalLoader().(*loader); ok {
		l.SetMetrics(m)
	}
}
