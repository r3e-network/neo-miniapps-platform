// Package confidential provides the Confidential Service as a ServicePackage.
package confidential

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
	pkg.MustRegisterPackage("com.r3e.services.confidential", func() (pkg.ServicePackage, error) {
		return &Package{
			PackageTemplate: pkg.NewPackageTemplate(pkg.PackageTemplateConfig{
				PackageID:   "com.r3e.services.confidential",
				DisplayName: "Confidential Service",
				Description: "Confidential computing with enclaves",
				ServiceName: "confidential",
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
	log := pkg.GetLogger(runtime, "confidential")

	svc := New(accounts, store, log)
	return []framework.ServiceModule{svc}, nil
}
