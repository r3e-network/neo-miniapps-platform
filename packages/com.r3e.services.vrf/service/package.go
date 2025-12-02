// Package vrf provides the VRF Service as a ServicePackage.
// This package is self-contained with its own:
// - Domain types (model.go)
// - Store interface and implementation (store.go, store_postgres.go)
// - Service logic with HTTP API methods (service.go)
//
// HTTP API endpoints are automatically discovered via HTTP{Method}{Path} naming convention.
package vrf

import (
	"context"

	"github.com/R3E-Network/service_layer/system/framework"
	pkg "github.com/R3E-Network/service_layer/system/runtime"
)

// Package implements the ServicePackage interface using PackageTemplate.
type Package struct {
	pkg.PackageTemplate
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

func (p *Package) CreateServices(ctx context.Context, runtime pkg.PackageRuntime) ([]framework.ServiceModule, error) {
	_ = ctx

	db, err := pkg.GetDatabase(runtime)
	if err != nil {
		return nil, err
	}

	accounts := pkg.NewAccountChecker(runtime)
	store := NewPostgresStore(db, accounts)
	log := pkg.GetLogger(runtime, "vrf")

	svc := New(accounts, store, log)

	// HTTP API endpoints are automatically discovered via HTTP{Method}{Path} methods on the service:
	// - HTTPGetRequests: GET /requests
	// - HTTPPostRequests: POST /requests
	// - HTTPGetRequestsById: GET /requests/{id}

	return []framework.ServiceModule{svc}, nil
}
