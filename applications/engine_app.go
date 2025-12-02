// Package app provides the application layer for the Service Layer.
// This file implements EngineApplication which uses the Android-style
// Engine + PackageLoader architecture.
package app

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/R3E-Network/service_layer/applications/system"
	"github.com/R3E-Network/service_layer/pkg/logger"
	appmetrics "github.com/R3E-Network/service_layer/pkg/metrics"
	"github.com/R3E-Network/service_layer/pkg/tracing"
	"github.com/R3E-Network/service_layer/system/bootstrap"
	engine "github.com/R3E-Network/service_layer/system/os/core"
	framework "github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
	pkg "github.com/R3E-Network/service_layer/system/os/runtime"

	otel "go.opentelemetry.io/otel"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// EngineApplication wraps the Service Engine and PackageLoader, providing
// access to services loaded via the Android-style package architecture.
// It implements a similar interface to Application for backward compatibility.
type EngineApplication struct {
	ServiceBundle // Embedded for ServiceProvider implementation
	engine        *engine.Engine
	loader        pkg.PackageLoader
	log           *logger.Logger
	traceShutdown func(context.Context) error
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

	// Tracer enables cross-service observability.
	Tracer core.Tracer

	// TracerProvider allows callers to supply an OpenTelemetry provider.
	TracerProvider oteltrace.TracerProvider

	// Tracing configures OTLP tracing exporter when no tracer provider is supplied.
	Tracing TracingConfig

	// Metrics recorder for service-level metrics.
	Metrics framework.Metrics

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

	appTracer := cfg.Tracer
	var tracerProvider oteltrace.TracerProvider
	var traceShutdown func(context.Context) error
	if cfg.TracerProvider != nil {
		tracerProvider = cfg.TracerProvider
	}
	if tracerProvider == nil && strings.TrimSpace(cfg.Tracing.Endpoint) != "" {
		otlpProvider, shutdown, err := tracing.NewOTLPTracerProvider(ctx, tracing.OTLPConfig{
			Endpoint:           cfg.Tracing.Endpoint,
			Insecure:           cfg.Tracing.Insecure,
			ServiceName:        cfg.Tracing.ServiceName,
			ResourceAttributes: cfg.Tracing.ResourceAttributes,
		})
		if err != nil {
			return nil, fmt.Errorf("configure tracing: %w", err)
		}
		tracerProvider = otlpProvider
		traceShutdown = shutdown
	}
	if appTracer == nil {
		if tracerProvider == nil {
			tracerProvider = otel.GetTracerProvider()
		}
		if tracerProvider != nil {
			appTracer = tracing.ConfigureGlobalTracer(tracerProvider, "service-layer")
		}
	}
	if appTracer == nil {
		appTracer = core.NoopTracer
	}

	appMetrics := cfg.Metrics
	if appMetrics == nil {
		appMetrics = appmetrics.NewRecorder(appmetrics.Registry)
	}

	// Bootstrap the engine with packages
	bootCfg := bootstrap.Config{
		Logger:        stdLog,
		PackageIDs:    cfg.PackageIDs,
		SkipStart:     true, // We'll start manually after wiring
		StoreProvider: storeProvider,
		Tracer:        appTracer,
		Metrics:       appMetrics,
	}

	result, err := bootstrap.BootstrapWithResult(ctx, bootCfg)
	if err != nil {
		return nil, fmt.Errorf("bootstrap engine: %w", err)
	}

	app := &EngineApplication{
		engine:        result.Engine,
		loader:        result.Loader,
		log:           appLog,
		traceShutdown: traceShutdown,
	}

	// Extract typed service references from the engine
	if err := app.wireServices(result.Engine); err != nil {
		return nil, fmt.Errorf("wire services: %w", err)
	}

	return app, nil
}

// serviceMapping defines the relationship between engine module names,
// struct field names, and router registration names.
// Format: engineName -> fieldName -> routerName (empty means same as engineName)
var serviceMapping = []struct {
	engineName string
	fieldName  string
	routerName string // empty = use engineName
}{
	{"accounts", "Accounts", "accounts"},
	{"gasbank", "GasBank", "gasbank"},
	{"automation", "Automation", "automation"},
	{"datafeeds", "DataFeeds", "datafeeds"},
	{"datastreams", "DataStreams", "datastreams"},
	{"datalink", "DataLink", "datalink"},
	{"dta", "DTA", "dta"},
	{"confidential", "Confidential", "confcompute"},
	{"oracle", "Oracle", "oracle"},
	{"secrets", "Secrets", "secrets"},
	{"cre", "CRE", "cre"},
	{"ccip", "CCIP", "ccip"},
	{"vrf", "VRF", "vrf"},
	{"mixer", "Mixer", ""},
}

// wireServices extracts typed service references from the engine using reflection.
// This enables backward-compatible access for HTTP handlers.
func (a *EngineApplication) wireServices(eng *engine.Engine) error {
	// Access the embedded ServiceBundle fields
	bundleVal := reflect.ValueOf(&a.ServiceBundle).Elem()

	// Initialize ServiceRouter for automatic HTTP endpoint discovery
	a.ServiceRouter = core.NewServiceRouter("/accounts/{accountID}")

	for _, m := range serviceMapping {
		mod := eng.Lookup(m.engineName)
		if mod == nil {
			continue
		}

		// Set the field in the embedded ServiceBundle using reflection
		field := bundleVal.FieldByName(m.fieldName)
		if !field.IsValid() || !field.CanSet() {
			continue
		}

		modVal := reflect.ValueOf(mod)
		if modVal.Type().AssignableTo(field.Type()) {
			field.Set(modVal)

			// Register with ServiceRouter if routerName is specified
			if m.routerName != "" {
				a.ServiceRouter.Register(m.routerName, mod)
			}
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
	err := bootstrap.Shutdown(ctx, a.engine, a.loader)
	if a.traceShutdown != nil {
		if shutdownErr := a.traceShutdown(ctx); shutdownErr != nil && err == nil {
			err = shutdownErr
		}
	}
	return err
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
		Database: stores.Database,
	})
}

// TracingConfig controls OTLP tracing exporter wiring.
type TracingConfig struct {
	Endpoint           string
	Insecure           bool
	ServiceName        string
	ResourceAttributes map[string]string
}
