// Package pkg defines the Service Package model - analogous to Android APK.
// A ServicePackage is a self-contained unit that can be loaded, installed,
// and executed by the Service Engine.
package pkg

import (
	"context"
	"fmt"

	engine "github.com/R3E-Network/service_layer/system/engine/core"
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// PackageManifest describes a service package - analogous to Android AndroidManifest.xml.
// This declares what the package provides, what it requires, and how it should be treated.
type PackageManifest struct {
	// Package identification
	PackageID   string `json:"package_id" yaml:"package_id"`     // e.g., "com.r3e.services.accounts"
	Version     string `json:"version" yaml:"version"`           // Semantic version
	DisplayName string `json:"display_name" yaml:"display_name"` // Human-readable name

	// Service declaration (a package may contain multiple services)
	Services []ServiceDeclaration `json:"services" yaml:"services"`

	// Permissions requested (Android-style)
	Permissions []Permission `json:"permissions" yaml:"permissions"`

	// Resource quotas
	Resources ResourceQuotas `json:"resources" yaml:"resources"`

	// Dependencies on other packages
	Dependencies []Dependency `json:"dependencies" yaml:"dependencies"`

	// Metadata
	Author      string            `json:"author,omitempty" yaml:"author,omitempty"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	License     string            `json:"license,omitempty" yaml:"license,omitempty"`
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ServiceDeclaration describes a service within the package.
type ServiceDeclaration struct {
	Name         string   `json:"name" yaml:"name"`
	Domain       string   `json:"domain" yaml:"domain"`
	Description  string   `json:"description,omitempty" yaml:"description,omitempty"`
	Capabilities []string `json:"capabilities" yaml:"capabilities"`
	Layer        string   `json:"layer" yaml:"layer"` // service, infrastructure, etc.
}

// Permission represents a requested capability - analogous to Android permissions.
type Permission struct {
	Name        string `json:"name" yaml:"name"`                                   // e.g., "engine.api.store"
	Description string `json:"description,omitempty" yaml:"description,omitempty"` // Why this permission is needed
	Required    bool   `json:"required" yaml:"required"`                           // Is this permission mandatory?
}

// ResourceQuotas defines resource limits for the package.
type ResourceQuotas struct {
	// Storage quotas
	MaxStorageBytes int64 `json:"max_storage_bytes,omitempty" yaml:"max_storage_bytes,omitempty"`

	// Compute quotas
	MaxConcurrentRequests int `json:"max_concurrent_requests,omitempty" yaml:"max_concurrent_requests,omitempty"`
	MaxRequestsPerSecond  int `json:"max_requests_per_second,omitempty" yaml:"max_requests_per_second,omitempty"`

	// Bus quotas
	MaxEventsPerSecond   int `json:"max_events_per_second,omitempty" yaml:"max_events_per_second,omitempty"`
	MaxDataPushPerSecond int `json:"max_data_push_per_second,omitempty" yaml:"max_data_push_per_second,omitempty"`

	// Custom quotas (extensible)
	Custom map[string]string `json:"custom,omitempty" yaml:"custom,omitempty"`
}

// Dependency declares a dependency on another package or engine component.
type Dependency struct {
	PackageID    string `json:"package_id,omitempty" yaml:"package_id,omitempty"`       // e.g., "com.r3e.services.accounts"
	EngineModule string `json:"engine_module,omitempty" yaml:"engine_module,omitempty"` // e.g., "store-postgres"
	MinVersion   string `json:"min_version,omitempty" yaml:"min_version,omitempty"`
	Required     bool   `json:"required" yaml:"required"`
}

// ServicePackage is the complete package that can be loaded into the engine.
// This is analogous to an Android APK.
type ServicePackage interface {
	// Manifest returns the package manifest
	Manifest() PackageManifest

	// CreateServices instantiates the services defined in this package
	// ctx: context for initialization
	// runtime: runtime dependencies provided by the engine
	CreateServices(ctx context.Context, runtime PackageRuntime) ([]engine.ServiceModule, error)

	// OnInstall is called when the package is first installed (optional lifecycle hook)
	OnInstall(ctx context.Context, runtime PackageRuntime) error

	// OnUninstall is called when the package is uninstalled (optional lifecycle hook)
	OnUninstall(ctx context.Context, runtime PackageRuntime) error

	// OnUpgrade is called when upgrading from an older version (optional lifecycle hook)
	OnUpgrade(ctx context.Context, runtime PackageRuntime, oldVersion string) error
}

// PackageRuntime provides access to engine resources - analogous to Android Context.
// This is the ONLY interface services can use to access engine capabilities.
type PackageRuntime interface {
	// Core APIs
	Logger() any           // Returns a logger instance
	Config() PackageConfig // Package-specific configuration

	// Storage access (if permission granted) - generic key-value storage
	Storage() (PackageStorage, error)

	// StoreProvider returns database access - analogous to Android's ContentResolver.
	// This provides generic database access. Services create their own typed stores.
	StoreProvider() StoreProvider

	// Bus access (if permission granted)
	Bus() (framework.BusClient, error)

	// RPC access (if permission granted)
	RPCClient() (any, error)

	// Ledger access (if permission granted)
	LedgerClient() (any, error)

	// Resource enforcement
	EnforceQuota(resource string, amount int64) error

	// Quota returns the quota enforcer for manual checks.
	Quota() framework.QuotaEnforcer

	// Metrics access
	Metrics() framework.Metrics

	// Tracer access
	Tracer() core.Tracer
}

// PackageConfig provides configuration for the package.
type PackageConfig interface {
	Get(key string) (string, bool)
	GetInt(key string) (int, bool)
	GetBool(key string) (bool, bool)
	GetAll() map[string]string
}

// PackageStorage provides isolated storage access for the package.
type PackageStorage interface {
	// Key-value storage (namespaced to this package)
	Set(ctx context.Context, key string, value []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)

	// Quota tracking
	UsedBytes() int64
	AvailableBytes() int64
}

// PackageLoader handles loading and installing packages.
type PackageLoader interface {
	// LoadPackage loads a package from a path or identifier
	LoadPackage(ctx context.Context, source string) (ServicePackage, error)

	// InstallPackage installs a loaded package into the engine
	InstallPackage(ctx context.Context, pkg ServicePackage, eng *engine.Engine) error

	// UninstallPackage removes a package from the engine
	UninstallPackage(ctx context.Context, packageID string, eng *engine.Engine) error

	// ListInstalled returns all installed packages
	ListInstalled() []InstalledPackage
}

// InstalledPackage represents a package that is currently installed.
type InstalledPackage struct {
	Manifest    PackageManifest
	InstalledAt string // RFC3339 timestamp
	Enabled     bool
	Services    []string // Service names registered by this package
}

// Validate checks if the manifest is valid.
func (m *PackageManifest) Validate() error {
	if m.PackageID == "" {
		return fmt.Errorf("package_id is required")
	}
	if m.Version == "" {
		return fmt.Errorf("version is required")
	}
	if len(m.Services) == 0 {
		return fmt.Errorf("at least one service must be declared")
	}

	// Validate services
	for i, svc := range m.Services {
		if svc.Name == "" {
			return fmt.Errorf("service[%d]: name is required", i)
		}
		if svc.Domain == "" {
			return fmt.Errorf("service[%d]: domain is required", i)
		}
	}

	return nil
}

// CheckPermissions verifies if all required permissions are satisfied.
func (m *PackageManifest) CheckPermissions(granted map[string]bool) []string {
	var missing []string
	for _, perm := range m.Permissions {
		if perm.Required && !granted[perm.Name] {
			missing = append(missing, perm.Name)
		}
	}
	return missing
}

// =============================================================================
// Store Provider Interface (Android ContentResolver equivalent)
// =============================================================================

// StoreProvider mirrors framework.StoreProvider for compatibility.
type StoreProvider = framework.StoreProvider
