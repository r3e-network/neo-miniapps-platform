// Package app provides the application layer for the Service Layer.
// This file implements EngineApplication which uses the Android-style
// Engine + PackageLoader architecture.
package app

import (
	"context"
	"fmt"
	"log"

	"github.com/R3E-Network/service_layer/applications/storage"
	"github.com/R3E-Network/service_layer/applications/system"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.accounts"
	automationsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.automation"
	ccipsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.ccip"
	confsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.confidential"
	cresvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.cre"
	datafeedsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datafeeds"
	datalinksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datalink"
	datastreamsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datastreams"
	dtasvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.dta"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.functions"
	gasbanksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.gasbank"
	oraclesvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.oracle"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.secrets"
	vrfsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.vrf"
	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/bootstrap"
	engine "github.com/R3E-Network/service_layer/system/core"
	core "github.com/R3E-Network/service_layer/system/framework/core"
	pkg "github.com/R3E-Network/service_layer/system/runtime"
)

// EngineApplication wraps the Service Engine and PackageLoader, providing
// access to services loaded via the Android-style package architecture.
// It implements a similar interface to Application for backward compatibility.
type EngineApplication struct {
	engine *engine.Engine
	loader pkg.PackageLoader
	log    *logger.Logger

	// Service references (populated after bootstrap)
	// These provide typed access to services for the HTTP handlers.
	Accounts     *accounts.Service
	Functions    *functions.Service
	GasBank      *gasbanksvc.Service
	Automation   *automationsvc.Service
	DataFeeds    *datafeedsvc.Service
	DataStreams  *datastreamsvc.Service
	DataLink     *datalinksvc.Service
	DTA          *dtasvc.Service
	Confidential *confsvc.Service
	Oracle       *oraclesvc.Service
	Secrets      *secrets.Service
	CRE          *cresvc.Service
	CCIP         *ccipsvc.Service
	VRF          *vrfsvc.Service

	// Additional components
	WorkspaceWallets   storage.WorkspaceWalletStore
	OracleRunnerTokens []string

	// Background runners
	AutomationRunner  *automationsvc.Scheduler
	OracleRunner      *oraclesvc.Dispatcher
	GasBankSettlement system.Service
}

// EngineAppConfig configures the EngineApplication.
type EngineAppConfig struct {
	// Stores provides persistence layer
	Stores Stores

	// RuntimeConfig for service-specific settings
	Runtime RuntimeConfig

	// Logger for the application
	Logger *logger.Logger

	// PackageIDs to load. If empty, loads all default packages.
	PackageIDs []string

	// Supabase integration components (optional)
	// SupabaseClient provides unified access to Supabase services
	SupabaseClient interface{}
	// BlobStorage provides Supabase Storage-based file storage
	BlobStorage interface{}
	// RealtimeClient provides PostgreSQL LISTEN/NOTIFY subscriptions
	RealtimeClient interface{}
}

// NewEngineApplication creates an EngineApplication using the Android-style
// Engine + PackageLoader architecture.
func NewEngineApplication(ctx context.Context, cfg EngineAppConfig) (*EngineApplication, error) {
	if err := validateStores(cfg.Stores); err != nil {
		return nil, err
	}

	appLog := cfg.Logger
	if appLog == nil {
		appLog = logger.NewDefault("engine-app")
	}

	// Create standard logger for bootstrap
	stdLog := log.New(appLog.Writer(), "[bootstrap] ", log.LstdFlags)

	// Create StoreProvider from Stores (Android ContentResolver pattern)
	storeProvider := storesToStoreProvider(cfg.Stores)

	// Bootstrap the engine with packages
	bootCfg := bootstrap.Config{
		Logger:        stdLog,
		PackageIDs:    cfg.PackageIDs,
		SkipStart:     true, // We'll start manually after wiring
		StoreProvider: storeProvider,
	}

	result, err := bootstrap.BootstrapWithResult(ctx, bootCfg)
	if err != nil {
		return nil, fmt.Errorf("bootstrap engine: %w", err)
	}

	app := &EngineApplication{
		engine:           result.Engine,
		loader:           result.Loader,
		log:              appLog,
		WorkspaceWallets: cfg.Stores.WorkspaceWallets,
	}

	// Extract typed service references from the engine
	if err := app.wireServices(result.Engine); err != nil {
		return nil, fmt.Errorf("wire services: %w", err)
	}

	return app, nil
}

// wireServices extracts typed service references from the engine.
// This enables backward-compatible access for HTTP handlers.
func (a *EngineApplication) wireServices(eng *engine.Engine) error {
	// Look up each service module and cast to the appropriate type
	if mod := eng.Lookup("accounts"); mod != nil {
		if svc, ok := mod.(*accounts.Service); ok {
			a.Accounts = svc
		}
	}
	if mod := eng.Lookup("functions"); mod != nil {
		if svc, ok := mod.(*functions.Service); ok {
			a.Functions = svc
		}
	}
	if mod := eng.Lookup("gasbank"); mod != nil {
		if svc, ok := mod.(*gasbanksvc.Service); ok {
			a.GasBank = svc
		}
	}
	if mod := eng.Lookup("automation"); mod != nil {
		if svc, ok := mod.(*automationsvc.Service); ok {
			a.Automation = svc
		}
	}
	if mod := eng.Lookup("datafeeds"); mod != nil {
		if svc, ok := mod.(*datafeedsvc.Service); ok {
			a.DataFeeds = svc
		}
	}
	if mod := eng.Lookup("datastreams"); mod != nil {
		if svc, ok := mod.(*datastreamsvc.Service); ok {
			a.DataStreams = svc
		}
	}
	if mod := eng.Lookup("datalink"); mod != nil {
		if svc, ok := mod.(*datalinksvc.Service); ok {
			a.DataLink = svc
		}
	}
	if mod := eng.Lookup("dta"); mod != nil {
		if svc, ok := mod.(*dtasvc.Service); ok {
			a.DTA = svc
		}
	}
	if mod := eng.Lookup("confidential"); mod != nil {
		if svc, ok := mod.(*confsvc.Service); ok {
			a.Confidential = svc
		}
	}
	if mod := eng.Lookup("oracle"); mod != nil {
		if svc, ok := mod.(*oraclesvc.Service); ok {
			a.Oracle = svc
		}
	}
	if mod := eng.Lookup("secrets"); mod != nil {
		if svc, ok := mod.(*secrets.Service); ok {
			a.Secrets = svc
		}
	}
	if mod := eng.Lookup("cre"); mod != nil {
		if svc, ok := mod.(*cresvc.Service); ok {
			a.CRE = svc
		}
	}
	if mod := eng.Lookup("ccip"); mod != nil {
		if svc, ok := mod.(*ccipsvc.Service); ok {
			a.CCIP = svc
		}
	}
	if mod := eng.Lookup("vrf"); mod != nil {
		if svc, ok := mod.(*vrfsvc.Service); ok {
			a.VRF = svc
		}
	}

	return nil
}

// Start starts the engine and all loaded services.
func (a *EngineApplication) Start(ctx context.Context) error {
	return a.engine.Start(ctx)
}

// Stop stops the engine and all services.
func (a *EngineApplication) Stop(ctx context.Context) error {
	return bootstrap.Shutdown(ctx, a.engine, a.loader)
}

// Engine returns the underlying engine.
func (a *EngineApplication) Engine() *engine.Engine {
	return a.engine
}

// Loader returns the package loader.
func (a *EngineApplication) Loader() pkg.PackageLoader {
	return a.loader
}

// Descriptors returns service descriptors for introspection.
func (a *EngineApplication) Descriptors() []core.Descriptor {
	infos := a.engine.ModulesInfo()
	descs := make([]core.Descriptor, 0, len(infos))
	for _, info := range infos {
		descs = append(descs, core.Descriptor{
			Name:         info.Name,
			Domain:       info.Domain,
			Layer:        core.Layer(info.Layer),
			Capabilities: info.Capabilities,
			DependsOn:    info.DependsOn,
		})
	}
	return descs
}

// InstalledPackages returns information about all installed packages.
func (a *EngineApplication) InstalledPackages() []pkg.InstalledPackage {
	return a.loader.ListInstalled()
}

// ModulesHealth returns health status of all modules.
func (a *EngineApplication) ModulesHealth() []engine.ModuleHealth {
	return a.engine.ModulesHealth()
}

// Attach is not supported in EngineApplication - packages should be loaded via bootstrap.
func (a *EngineApplication) Attach(service system.Service) error {
	a.log.Warn("Attach() is deprecated for EngineApplication; use package loading instead")
	return nil
}

// storesToStoreProvider converts application Stores to runtime StoreProvider.
// This bridges the application-layer storage interfaces to the Android-style
// ContentResolver pattern used by service packages.
func storesToStoreProvider(stores Stores) pkg.StoreProvider {
	return pkg.NewStoreProvider(pkg.StoreProviderConfig{
		Accounts:         stores.Accounts,
		Functions:        stores.Functions,
		GasBank:          stores.GasBank,
		Automation:       stores.Automation,
		DataFeeds:        stores.DataFeeds,
		DataStreams:      stores.DataStreams,
		DataLink:         stores.DataLink,
		DTA:              stores.DTA,
		Confidential:     stores.Confidential,
		Oracle:           stores.Oracle,
		Secrets:          stores.Secrets,
		CRE:              stores.CRE,
		CCIP:             stores.CCIP,
		VRF:              stores.VRF,
		WorkspaceWallets: stores.WorkspaceWallets,
	})
}
