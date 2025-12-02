// Package datalink provides the Data Link Service as a ServicePackage.
package datalink

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
	pkg.MustRegisterPackage("com.r3e.services.datalink", func() (pkg.ServicePackage, error) {
		return &Package{
			PackageTemplate: pkg.NewPackageTemplate(pkg.PackageTemplateConfig{
				PackageID:    "com.r3e.services.datalink",
				DisplayName:  "Data Link Service",
				Description:  "Cross-chain data linking",
				ServiceName:  "datalink",
				Capabilities: []string{"link.create", "link.query"},
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
	log := pkg.GetLogger(runtime, "datalink")

	svc := New(accounts, store, log)
	return []framework.ServiceModule{svc}, nil
}
