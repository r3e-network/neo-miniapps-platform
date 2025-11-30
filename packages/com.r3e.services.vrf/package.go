// Package vrf provides the VRF Service as a ServicePackage.
// This package is self-contained with its own:
// - Domain types (model.go)
// - Store interface and implementation (store.go, store_postgres.go)
// - Service logic (service.go)
// - HTTP handlers (http.go)
package vrf

import (
	"context"
	"net/http"

	engine "github.com/R3E-Network/service_layer/system/core"
	pkg "github.com/R3E-Network/service_layer/system/runtime"
)

// Package implements the ServicePackage interface using PackageTemplate.
type Package struct {
	pkg.PackageTemplate
	httpHandler *HTTPHandler
}

func init() {
	pkg.MustRegisterPackage("com.r3e.services.vrf", func() (pkg.ServicePackage, error) {
		return &Package{
			PackageTemplate: pkg.NewPackageTemplate(pkg.PackageTemplateConfig{
				PackageID:    "com.r3e.services.vrf",
				DisplayName:  "VRF Service",
				Description:  "Verifiable Random Function service",
				ServiceName:  "vrf",
				Capabilities: []string{"vrf.request", "vrf.verify"},
			}),
		}, nil
	})
}

func (p *Package) CreateServices(ctx context.Context, runtime pkg.PackageRuntime) ([]engine.ServiceModule, error) {
	_ = ctx

	db, err := pkg.GetDatabase(runtime)
	if err != nil {
		return nil, err
	}

	accounts := pkg.NewAccountChecker(runtime)
	store := NewPostgresStore(db, accounts)
	log := pkg.GetLogger(runtime, "vrf")

	svc := New(accounts, store, log)

	// Create HTTP handler for this service
	p.httpHandler = NewHTTPHandler(svc)

	return []engine.ServiceModule{svc}, nil
}

// RegisterRoutes implements the RouteRegistrar interface.
// This allows the service to register its own HTTP routes.
func (p *Package) RegisterRoutes(mux *http.ServeMux, basePath string) {
	if p.httpHandler != nil {
		p.httpHandler.RegisterRoutes(mux, basePath)
	}
}

// HTTPHandler returns the HTTP handler for external registration.
func (p *Package) HTTPHandler() *HTTPHandler {
	return p.httpHandler
}
