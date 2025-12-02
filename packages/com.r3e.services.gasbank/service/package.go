// Package gasbank provides the Gas Bank Service as a ServicePackage.
// This package is self-contained with its own:
// - Domain types (domain.go)
// - Store interface and implementation (store.go, store_postgres.go)
// - Service logic with HTTP API methods (service.go)
//
// HTTP API endpoints are automatically discovered via HTTP{Method}{Path} naming convention.
package gasbank

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

func (p *Package) CreateServices(ctx context.Context, runtime pkg.PackageRuntime) ([]framework.ServiceModule, error) {
	_ = ctx

	db, err := pkg.GetDatabase(runtime)
	if err != nil {
		return nil, err
	}

	accounts := pkg.NewAccountChecker(runtime)
	store := NewPostgresStore(db, accounts)
	log := pkg.GetLogger(runtime, "gasbank")

	svc := New(accounts, store, log)

	// HTTP API endpoints are automatically discovered via HTTP{Method}{Path} methods on the service:
	// - HTTPGetAccounts: GET /accounts
	// - HTTPPostAccounts: POST /accounts
	// - HTTPGetAccountsById: GET /accounts/{id}
	// - HTTPGetAccountsIdTransactions: GET /accounts/{id}/transactions
	// - HTTPPostDeposit: POST /deposit
	// - HTTPPostWithdraw: POST /withdraw

	return []framework.ServiceModule{svc}, nil
}
