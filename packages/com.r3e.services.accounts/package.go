// Package accounts provides the Accounts service as a ServicePackage.
package accounts

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
	pkg.MustRegisterPackage("com.r3e.services.accounts", func() (pkg.ServicePackage, error) {
		return &Package{
			PackageTemplate: pkg.NewPackageTemplate(pkg.PackageTemplateConfig{
				PackageID:    "com.r3e.services.accounts",
				DisplayName:  "Accounts Service",
				Description:  "Account registry and metadata management",
				ServiceName:  "accounts",
				Capabilities: []string{"accounts.create", "accounts.list", "accounts.get"},
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

	store := NewPostgresStore(db)
	log := pkg.GetLogger(runtime, "accounts")

	svc := New(store, log)
	return []engine.ServiceModule{svc}, nil
}
