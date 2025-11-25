package runtime

import (
	"context"
	"database/sql"

	app "github.com/R3E-Network/service_layer/internal/app"
	engine "github.com/R3E-Network/service_layer/internal/engine"
)

// appModule adapts app.Application to the core engine contract.
type appModule struct {
	app *app.Application
}

// storeModule adapts a SQL database to a StoreEngine for lifecycle/ping.
type storeModule struct {
	db *sql.DB
}

func newAppModule(a *app.Application) engine.ServiceModule { return appModule{app: a} }
func (m appModule) Name() string                           { return "core-application" }
func (m appModule) Domain() string                         { return "core" }
func (m appModule) Start(ctx context.Context) error        { _ = ctx; return nil }
func (m appModule) Stop(ctx context.Context) error         { _ = ctx; return nil }

func newStoreModule(db *sql.DB) engine.ServiceModule { return storeModule{db: db} }
func (s storeModule) Name() string                   { return "store-postgres" }
func (s storeModule) Domain() string                 { return "store" }
func (s storeModule) Start(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	return s.db.PingContext(ctx)
}
func (s storeModule) Stop(ctx context.Context) error { return nil }
func (s storeModule) Ready(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	return s.db.PingContext(ctx)
}
func (s storeModule) Ping(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	return s.db.PingContext(ctx)
}
