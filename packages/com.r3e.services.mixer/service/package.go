// Package mixer provides the Privacy Mixer Service as a ServicePackage.
// This package is self-contained with its own:
// - Domain types (domain.go)
// - Store interface and implementation (store.go, store_postgres.go)
// - Service logic with HTTP API methods (service.go)
// - Documentation (doc.go)
//
// HTTP API endpoints are automatically discovered via HTTP{Method}{Path} naming convention.
package mixer

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

func (p *Package) CreateServices(ctx context.Context, runtime pkg.PackageRuntime) ([]framework.ServiceModule, error) {
	_ = ctx

	db, err := pkg.GetDatabase(runtime)
	if err != nil {
		return nil, err
	}

	accounts := pkg.NewAccountChecker(runtime)
	store := NewPostgresStore(db)
	log := pkg.GetLogger(runtime, "mixer")

	// TEE, Master key provider, and Chain clients would be injected from runtime config
	// For now, pass nil - they will be configured separately via dependency injection
	var tee TEEManager
	var master MasterKeyProvider
	var chain ChainClient

	svc := New(accounts, store, tee, master, chain, log)

	// HTTP API endpoints are automatically discovered via HTTP{Method}{Path} methods on the service:
	// - HTTPGetRequests: GET /requests
	// - HTTPPostRequests: POST /requests
	// - HTTPGetRequestsById: GET /requests/{id}
	// - HTTPPostRequestsIdDeposit: POST /requests/{id}/deposit
	// - HTTPPostRequestsIdClaim: POST /requests/{id}/claim
	// - HTTPGetStats: GET /stats

	return []framework.ServiceModule{svc}, nil
}
