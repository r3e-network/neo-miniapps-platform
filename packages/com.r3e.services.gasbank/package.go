// Package gasbank provides the Gas Bank Service as a ServicePackage.
package gasbank

import (
	"context"

	engine "github.com/R3E-Network/service_layer/system/core"
	pkg "github.com/R3E-Network/service_layer/system/runtime"
)

// Package implements the ServicePackage interface using PackageTemplate.
type Package struct {
	pkg.PackageTemplate
}

func init() {
	pkg.MustRegisterPackage("com.r3e.services.gasbank", func() (pkg.ServicePackage, error) {
		return &Package{
			PackageTemplate: pkg.NewPackageTemplate(pkg.PackageTemplateConfig{
				PackageID:    "com.r3e.services.gasbank",
				DisplayName:  "Gas Bank Service",
				Description:  "Gas fee management and sponsorship",
				ServiceName:  "gasbank",
				Capabilities: []string{"gasbank.deposit", "gasbank.withdraw"},
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
	log := pkg.GetLogger(runtime, "gasbank")

	svc := New(accounts, store, log)
	return []engine.ServiceModule{svc}, nil
}
