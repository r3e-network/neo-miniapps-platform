package pkg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
)

// PackageConfig holds configuration for creating a service package.
// Use this with PackageTemplate to reduce boilerplate in package.go files.
type PackageTemplateConfig struct {
	// Required fields
	PackageID   string // e.g., "com.r3e.services.datafeeds"
	DisplayName string // e.g., "Data Feeds Service"
	Description string // e.g., "Data feed aggregation"

	// Service configuration
	ServiceName   string   // e.g., "datafeeds"
	ServiceDomain string   // defaults to ServiceName if empty
	Capabilities  []string // defaults to []string{ServiceName} if empty

	// Optional: Version (defaults to "1.0.0")
	Version string

	// Optional: Resource quotas (have sensible defaults)
	MaxStorageBytes       int64
	MaxConcurrentRequests int
	MaxRequestsPerSecond  int
	MaxEventsPerSecond    int
}

// PackageTemplate provides a base implementation for service packages.
// Embed this in your Package struct to reduce boilerplate.
//
// Example usage:
//
//	type Package struct {
//	    pkg.PackageTemplate
//	}
//
//	func init() {
//	    pkg.MustRegisterPackage("com.r3e.services.myservice", func() (pkg.ServicePackage, error) {
//	        return &Package{
//	            PackageTemplate: pkg.NewPackageTemplate(pkg.PackageTemplateConfig{
//	                PackageID:   "com.r3e.services.myservice",
//	                DisplayName: "My Service",
//	                Description: "My service description",
//	                ServiceName: "myservice",
//	            }),
//	        }, nil
//	    })
//	}
type PackageTemplate struct {
	config   PackageTemplateConfig
	manifest PackageManifest
}

// NewPackageTemplate creates a new package template with the given configuration.
func NewPackageTemplate(cfg PackageTemplateConfig) PackageTemplate {
	// Apply defaults
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}
	if cfg.ServiceDomain == "" {
		cfg.ServiceDomain = cfg.ServiceName
	}
	if len(cfg.Capabilities) == 0 {
		cfg.Capabilities = []string{cfg.ServiceName}
	}

	// Apply default quotas
	if cfg.MaxStorageBytes == 0 {
		cfg.MaxStorageBytes = 100 * 1024 * 1024 // 100MB
	}
	if cfg.MaxConcurrentRequests == 0 {
		cfg.MaxConcurrentRequests = 1000
	}
	if cfg.MaxRequestsPerSecond == 0 {
		cfg.MaxRequestsPerSecond = 5000
	}
	if cfg.MaxEventsPerSecond == 0 {
		cfg.MaxEventsPerSecond = 1000
	}

	// Build manifest
	manifest := PackageManifest{
		PackageID:   cfg.PackageID,
		Version:     cfg.Version,
		DisplayName: cfg.DisplayName,
		Description: cfg.Description,
		Author:      "R3E Network",
		License:     "MIT",

		Services: []ServiceDeclaration{
			{
				Name:         cfg.ServiceName,
				Domain:       cfg.ServiceDomain,
				Description:  cfg.Description,
				Capabilities: cfg.Capabilities,
				Layer:        "service",
			},
		},

		Permissions: []Permission{
			{
				Name:        "engine.api.storage",
				Description: "Required for data persistence",
				Required:    true,
			},
		},

		Resources: ResourceQuotas{
			MaxStorageBytes:       cfg.MaxStorageBytes,
			MaxConcurrentRequests: cfg.MaxConcurrentRequests,
			MaxRequestsPerSecond:  cfg.MaxRequestsPerSecond,
			MaxEventsPerSecond:    cfg.MaxEventsPerSecond,
		},

		Dependencies: []Dependency{
			{EngineModule: "store", Required: true},
		},
	}

	return PackageTemplate{
		config:   cfg,
		manifest: manifest,
	}
}

// Manifest returns the package manifest.
func (p *PackageTemplate) Manifest() PackageManifest {
	return p.manifest
}

// Config returns the template configuration.
func (p *PackageTemplate) Config() PackageTemplateConfig {
	return p.config
}

// OnInstall provides default install logging.
func (p *PackageTemplate) OnInstall(ctx context.Context, runtime PackageRuntime) error {
	_ = ctx
	if log := GetLogger(runtime, p.config.ServiceName); log != nil {
		log.Info(p.config.ServiceName + " package installed")
	}
	return nil
}

// OnUninstall provides default uninstall logging.
func (p *PackageTemplate) OnUninstall(ctx context.Context, runtime PackageRuntime) error {
	_ = ctx
	if log := GetLogger(runtime, p.config.ServiceName); log != nil {
		log.Info(p.config.ServiceName + " package uninstalled")
	}
	return nil
}

// OnUpgrade provides default upgrade logging.
func (p *PackageTemplate) OnUpgrade(ctx context.Context, runtime PackageRuntime, oldVersion string) error {
	_ = ctx
	if log := GetLogger(runtime, p.config.ServiceName); log != nil {
		log.WithField("old_version", oldVersion).
			WithField("new_version", p.config.Version).
			Info(p.config.ServiceName + " package upgraded")
	}
	return nil
}

// =============================================================================
// Helper Functions for Package Implementations
// =============================================================================

// GetDatabase extracts the database connection from runtime.
// Returns an error if the database is not available.
func GetDatabase(runtime PackageRuntime) (*sql.DB, error) {
	sp := runtime.StoreProvider()
	if sp == nil {
		return nil, fmt.Errorf("store provider not available")
	}
	dbAny := sp.Database()
	db, ok := dbAny.(*sql.DB)
	if !ok || db == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	return db, nil
}

// GetLogger extracts or creates a logger from runtime.
// If the runtime logger is not available, creates a default logger.
func GetLogger(runtime PackageRuntime, name string) *logger.Logger {
	if runtime == nil {
		return logger.NewDefault(name)
	}
	if loggerFromRuntime := runtime.Logger(); loggerFromRuntime != nil {
		if l, ok := loggerFromRuntime.(*logger.Logger); ok {
			return l
		}
	}
	return logger.NewDefault(name)
}

// StoreProviderAccountChecker adapts StoreProvider to the AccountChecker interface.
// Use this when creating services that need account validation.
type StoreProviderAccountChecker struct {
	SP StoreProvider
}

// AccountExists checks if an account exists.
func (a *StoreProviderAccountChecker) AccountExists(ctx context.Context, accountID string) error {
	if a.SP == nil {
		return fmt.Errorf("store provider not configured")
	}
	return a.SP.AccountExists(ctx, accountID)
}

// AccountTenant returns the tenant for an account.
func (a *StoreProviderAccountChecker) AccountTenant(ctx context.Context, accountID string) string {
	if a.SP == nil {
		return ""
	}
	return a.SP.AccountTenant(ctx, accountID)
}

// NewAccountChecker creates an AccountChecker from a PackageRuntime.
func NewAccountChecker(runtime PackageRuntime) *StoreProviderAccountChecker {
	return &StoreProviderAccountChecker{SP: runtime.StoreProvider()}
}

// =============================================================================
// Package ID Helpers
// =============================================================================

// ServicePackageID generates a standard package ID for a service.
// Example: ServicePackageID("datafeeds") returns "com.r3e.services.datafeeds"
func ServicePackageID(serviceName string) string {
	return "com.r3e.services." + strings.ToLower(strings.TrimSpace(serviceName))
}

// ExtractServiceName extracts the service name from a package ID.
// Example: ExtractServiceName("com.r3e.services.datafeeds") returns "datafeeds"
func ExtractServiceName(packageID string) string {
	const prefix = "com.r3e.services."
	if strings.HasPrefix(packageID, prefix) {
		return packageID[len(prefix):]
	}
	return packageID
}
