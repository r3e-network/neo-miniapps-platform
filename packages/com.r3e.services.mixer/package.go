// Package mixer provides the Privacy Mixer Service as a ServicePackage.
package mixer

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

func (p *Package) CreateServices(ctx context.Context, runtime pkg.PackageRuntime) ([]engine.ServiceModule, error) {
	_ = ctx

	db, err := pkg.GetDatabase(runtime)
	if err != nil {
		return nil, err
	}

	accounts := pkg.NewAccountChecker(runtime)
	store := NewPostgresStore(db)
	log := pkg.GetLogger(runtime, "mixer")

	// TEE and Chain clients would be injected from runtime config
	// For now, pass nil - they will be configured separately
	var tee TEEManager
	var chain ChainClient

	svc := New(accounts, store, tee, chain, log)
	return []engine.ServiceModule{svc}, nil
}
