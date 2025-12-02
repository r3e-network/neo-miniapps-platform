// Package secrets provides the Secrets Service as a ServicePackage.
// This package is self-contained with its own:
// - Domain types (domain.go)
// - Store interface and implementation (store.go, store_postgres.go)
// - Service logic with HTTP API methods (service.go)
//
// HTTP API endpoints are automatically discovered via HTTP{Method}{Path} naming convention.
package secrets

import (
	"context"

	"github.com/R3E-Network/service_layer/system/os/framework"
	pkg "github.com/R3E-Network/service_layer/system/os/runtime"
)

// Package implements the ServicePackage interface using PackageTemplate.
type Package struct {
	pkg.PackageTemplate
}

func init() {
	pkg.MustRegisterPackage("com.r3e.services.secrets", func() (pkg.ServicePackage, error) {
		return &Package{
			PackageTemplate: pkg.NewPackageTemplate(pkg.PackageTemplateConfig{
				PackageID:   "com.r3e.services.secrets",
				DisplayName: "Secrets Service",
				Description: "Secret management",
				ServiceName: "secrets",
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
	log := pkg.GetLogger(runtime, "secrets")

	svc := New(accounts, store, log)

	// HTTP API endpoints are automatically discovered via HTTP{Method}{Path} methods on the service:
	// - HTTPGetSecrets: GET /secrets
	// - HTTPPostSecrets: POST /secrets
	// - HTTPGetSecretsById: GET /secrets/{id}
	// - HTTPPutSecretsById: PUT /secrets/{id}
	// - HTTPDeleteSecretsById: DELETE /secrets/{id}

	return []framework.ServiceModule{svc}, nil
}
