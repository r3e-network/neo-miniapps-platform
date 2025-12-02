// Package oracle provides the Oracle Service as a ServicePackage.
package oracle

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
	pkg.MustRegisterPackage("com.r3e.services.oracle", func() (pkg.ServicePackage, error) {
		return &Package{
			PackageTemplate: pkg.NewPackageTemplate(pkg.PackageTemplateConfig{
				PackageID:    "com.r3e.services.oracle",
				DisplayName:  "Oracle Service",
				Description:  "Decentralized oracle data feeds",
				ServiceName:  "oracle",
				Capabilities: []string{"oracle.request", "oracle.fulfill"},
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
	log := pkg.GetLogger(runtime, "oracle")

	svc := New(accounts, store, log)
	return []framework.ServiceModule{svc}, nil
}
