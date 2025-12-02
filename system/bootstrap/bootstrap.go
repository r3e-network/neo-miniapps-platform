// Package bootstrap provides Service Engine initialization with PackageLoader integration.
// This is the Android-style bootstrapper that loads service packages dynamically.
//
// ARCHITECTURE NOTE: This package is service-agnostic. It provides generic bootstrap
// infrastructure without knowledge of specific services. Service packages should be
// imported in the application layer (e.g., applications/services.go) to trigger
// self-registration via init().
package bootstrap

import (
	"context"
	"fmt"
	"log"

	engine "github.com/R3E-Network/service_layer/system/engine/core"
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
	pkg "github.com/R3E-Network/service_layer/system/os/runtime"
)

// Config holds bootstrap configuration.
type Config struct {
	// Logger is the engine logger. If nil, uses default.
	Logger *log.Logger

	// PackageIDs specifies which packages to load. If empty, loads all registered packages.
	PackageIDs []string

	// SkipStart skips calling Engine.Start() after loading packages.
	SkipStart bool

	// StoreProvider provides typed database access to packages.
	// If nil, packages will receive a nil store provider.
	StoreProvider pkg.StoreProvider

	// Tracer propagates tracing spans into package runtimes.
	Tracer core.Tracer

	// Metrics records service-level metrics.
	Metrics framework.Metrics
}

// DefaultPackageIDs returns the list of all core service package IDs.
func DefaultPackageIDs() []string {
	return []string{
		"com.r3e.services.accounts",
		"com.r3e.services.secrets",
		"com.r3e.services.gasbank",
		"com.r3e.services.automation",
		"com.r3e.services.datafeeds",
		"com.r3e.services.datastreams",
		"com.r3e.services.datalink",
		"com.r3e.services.dta",
		"com.r3e.services.confidential",
		"com.r3e.services.oracle",
		"com.r3e.services.cre",
		"com.r3e.services.ccip",
		"com.r3e.services.vrf",
	}
}

// Bootstrap creates and initializes a Service Engine with all registered packages.
// This is the main entry point for Android-style service loading.
func Bootstrap(ctx context.Context, cfg Config) (*engine.Engine, pkg.PackageLoader, error) {
	// Create engine with optional logger
	var engineOpts []engine.Option
	if cfg.Logger != nil {
		engineOpts = append(engineOpts, engine.WithLogger(cfg.Logger))
	}
	eng := engine.New(engineOpts...)

	// Get the global package loader
	loader := pkg.GlobalLoader()

	// Set the store provider BEFORE installing packages
	if cfg.StoreProvider != nil {
		pkg.SetGlobalStoreProvider(cfg.StoreProvider)
	}
	if cfg.Tracer != nil {
		pkg.SetGlobalTracer(cfg.Tracer)
	}
	if cfg.Metrics != nil {
		pkg.SetGlobalMetrics(cfg.Metrics)
	}

	// Determine which packages to load
	packageIDs := cfg.PackageIDs
	if len(packageIDs) == 0 {
		packageIDs = DefaultPackageIDs()
	}

	// Load and install each package
	for _, id := range packageIDs {
		servicePkg, err := loader.LoadPackage(ctx, id)
		if err != nil {
			return nil, nil, fmt.Errorf("load package %s: %w", id, err)
		}

		if err := loader.InstallPackage(ctx, servicePkg, eng); err != nil {
			return nil, nil, fmt.Errorf("install package %s: %w", id, err)
		}

		if cfg.Logger != nil {
			cfg.Logger.Printf("Installed package: %s", id)
		}
	}

	// Start the engine if not skipped
	if !cfg.SkipStart {
		if err := eng.Start(ctx); err != nil {
			return nil, nil, fmt.Errorf("start engine: %w", err)
		}
	}

	return eng, loader, nil
}

// BootstrapResult contains the result of a bootstrap operation.
type BootstrapResult struct {
	Engine  *engine.Engine
	Loader  pkg.PackageLoader
	Modules []string
}

// BootstrapWithResult is like Bootstrap but returns a structured result.
func BootstrapWithResult(ctx context.Context, cfg Config) (*BootstrapResult, error) {
	eng, loader, err := Bootstrap(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &BootstrapResult{
		Engine:  eng,
		Loader:  loader,
		Modules: eng.Modules(),
	}, nil
}

// Shutdown gracefully stops the engine and uninstalls all packages.
func Shutdown(ctx context.Context, eng *engine.Engine, loader pkg.PackageLoader) error {
	// Stop the engine first
	if err := eng.Stop(ctx); err != nil {
		return fmt.Errorf("stop engine: %w", err)
	}

	// Uninstall all packages
	installed := loader.ListInstalled()
	for _, info := range installed {
		if err := loader.UninstallPackage(ctx, info.Manifest.PackageID, eng); err != nil {
			// Log but continue
			if log := eng.Logger(); log != nil {
				log.Printf("warning: failed to uninstall package %s: %v", info.Manifest.PackageID, err)
			}
		}
	}

	return nil
}
