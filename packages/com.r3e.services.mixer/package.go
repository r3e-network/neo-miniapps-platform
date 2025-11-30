// Package mixer provides the Privacy Mixer Service as a ServicePackage.
// This package is self-contained with its own:
// - Domain types (domain.go)
// - Store interface and implementation (store.go, store_postgres.go)
// - Service logic (service.go)
// - HTTP handlers (http.go)
// - Documentation (doc.go)
package mixer

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
	pkg.MustRegisterPackage("com.r3e.services.mixer", func() (pkg.ServicePackage, error) {
		return &Package{
			PackageTemplate: pkg.NewPackageTemplate(pkg.PackageTemplateConfig{
				PackageID:    "com.r3e.services.mixer",
				DisplayName:  "Privacy Mixer Service",
				Description:  "Privacy-preserving transaction mixing with TEE and ZKP",
				ServiceName:  "mixer",
				Capabilities: []string{"mixer.request", "mixer.withdraw", "mixer.claim"},
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
	store := NewPostgresStore(db)
	log := pkg.GetLogger(runtime, "mixer")

	// TEE and Chain clients would be injected from runtime config
	// For now, pass nil - they will be configured separately
	var tee TEEManager
	var chain ChainClient

	svc := New(accounts, store, tee, chain, log)

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
// This is used when the application layer needs to integrate routes.
func (p *Package) HTTPHandler() *HTTPHandler {
	return p.httpHandler
}
