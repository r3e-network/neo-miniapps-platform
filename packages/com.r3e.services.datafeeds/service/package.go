// Package datafeeds provides the Data Feeds Service as a ServicePackage.
package datafeeds

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
	pkg.MustRegisterPackage("com.r3e.services.datafeeds", func() (pkg.ServicePackage, error) {
		return &Package{
			PackageTemplate: pkg.NewPackageTemplate(pkg.PackageTemplateConfig{
				PackageID:   "com.r3e.services.datafeeds",
				DisplayName: "Data Feeds Service",
				Description: "Data feed aggregation",
				ServiceName: "datafeeds",
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
	log := pkg.GetLogger(runtime, "datafeeds")

	svc := New(accounts, store, log)
	return []framework.ServiceModule{svc}, nil
}
