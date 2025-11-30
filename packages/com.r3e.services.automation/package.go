// Package automation provides the Automation Service as a ServicePackage.
package automation

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
	pkg.MustRegisterPackage("com.r3e.services.automation", func() (pkg.ServicePackage, error) {
		return &Package{
			PackageTemplate: pkg.NewPackageTemplate(pkg.PackageTemplateConfig{
				PackageID:    "com.r3e.services.automation",
				DisplayName:  "Automation Service",
				Description:  "Automated task scheduling and execution",
				ServiceName:  "automation",
				Capabilities: []string{"automation.schedule", "automation.execute"},
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

	accountChecker := pkg.NewAccountChecker(runtime)
	store := NewPostgresStore(db, accountChecker)
	log := pkg.GetLogger(runtime, "automation")

	svc := New(accountChecker, store, log)
	return []engine.ServiceModule{svc}, nil
}
