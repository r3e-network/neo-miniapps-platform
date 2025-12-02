package framework

import (
	"context"

	"github.com/R3E-Network/service_layer/pkg/logger"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// SystemService enumerates runtime primitives exposed through EngineContext.
type SystemService string

const (
	SystemServiceStore   SystemService = "store"
	SystemServiceBus     SystemService = "bus"
	SystemServiceRPC     SystemService = "rpc"
	SystemServiceLedger  SystemService = "ledger"
	SystemServiceConfig  SystemService = "config"
	SystemServiceTracer  SystemService = "tracer"
	SystemServiceMetrics SystemService = "metrics"
	SystemServiceQuota   SystemService = "quota"
)

// EngineContext mirrors Android's Context API—services request runtime primitives
// (store, bus, RPC, metrics, etc.) from the engine instead of touching global
// state directly. ServiceEngine implements this interface and supplies a
// context via ServiceEngine.Context().
type EngineContext interface {
	Name() string
	Domain() string
	Logger() *logger.Logger
	Hooks() core.ObservationHooks

	Accounts() AccountChecker
	Wallets() WalletChecker

	StoreProvider() StoreProvider
	Database() any
	Config() Config
	RPCClient() any
	LedgerClient() any
	Bus() BusClient

	PublishEvent(ctx context.Context, event string, payload any) error
	PushData(ctx context.Context, topic string, payload any) error
	InvokeCompute(ctx context.Context, payload any) ([]ComputeResult, error)

	Tracer() core.Tracer
	Metrics() Metrics
	Quota() QuotaEnforcer
	EnforceQuota(resource string, amount int64) error

	StartObservation(ctx context.Context, attrs map[string]string) (context.Context, func(error))
	ObserveOperation(ctx context.Context, attrs map[string]string, op func(context.Context) error) error

	SystemService(name SystemService) any
}

// serviceContext backs EngineContext for ServiceEngine.
type serviceContext struct {
	engine   *ServiceEngine
	services map[SystemService]any
}

func newServiceContext(engine *ServiceEngine) *serviceContext {
	ctx := &serviceContext{engine: engine}
	ctx.refresh()
	return ctx
}

func (c *serviceContext) refresh() {
	if c == nil || c.engine == nil {
		return
	}
	env := c.engine.Environment()
	services := map[SystemService]any{
		SystemServiceStore:   env.StoreProvider,
		SystemServiceBus:     env.Bus,
		SystemServiceRPC:     env.RPCClient,
		SystemServiceLedger:  env.LedgerClient,
		SystemServiceConfig:  env.Config,
		SystemServiceTracer:  env.Tracer,
		SystemServiceMetrics: env.Metrics,
		SystemServiceQuota:   env.Quota,
	}
	c.services = services
}

func (c *serviceContext) Name() string {
	if c.engine == nil {
		return ""
	}
	return c.engine.Name()
}

func (c *serviceContext) Domain() string {
	if c.engine == nil {
		return ""
	}
	return c.engine.Domain()
}

func (c *serviceContext) Logger() *logger.Logger {
	if c.engine == nil {
		return nil
	}
	return c.engine.Logger()
}

func (c *serviceContext) Hooks() core.ObservationHooks {
	if c.engine == nil {
		return core.NoopObservationHooks
	}
	return c.engine.Hooks()
}

func (c *serviceContext) Accounts() AccountChecker {
	if c.engine == nil {
		return nil
	}
	return c.engine.accounts
}

func (c *serviceContext) Wallets() WalletChecker {
	if c.engine == nil {
		return nil
	}
	return c.engine.wallets
}

func (c *serviceContext) StoreProvider() StoreProvider {
	if c.engine == nil {
		return nil
	}
	return c.engine.StoreProvider()
}

func (c *serviceContext) Database() any {
	if c.engine == nil {
		return nil
	}
	return c.engine.Database()
}

func (c *serviceContext) Config() Config {
	if c.engine == nil {
		return ConfigMap(nil)
	}
	return c.engine.Config()
}

func (c *serviceContext) RPCClient() any {
	if c.engine == nil {
		return nil
	}
	return c.engine.RPCClient()
}

func (c *serviceContext) LedgerClient() any {
	if c.engine == nil {
		return nil
	}
	return c.engine.LedgerClient()
}

func (c *serviceContext) Bus() BusClient {
	if c.engine == nil {
		return noopBus{}
	}
	return c.engine.Bus()
}

func (c *serviceContext) PublishEvent(ctx context.Context, event string, payload any) error {
	if c.engine == nil {
		return core.ErrServiceUnavailable
	}
	return c.engine.PublishEvent(ctx, event, payload)
}

func (c *serviceContext) PushData(ctx context.Context, topic string, payload any) error {
	if c.engine == nil {
		return core.ErrServiceUnavailable
	}
	return c.engine.PushData(ctx, topic, payload)
}

func (c *serviceContext) InvokeCompute(ctx context.Context, payload any) ([]ComputeResult, error) {
	if c.engine == nil {
		return nil, core.ErrServiceUnavailable
	}
	return c.engine.InvokeCompute(ctx, payload)
}

func (c *serviceContext) Tracer() core.Tracer {
	if c.engine == nil {
		return core.NoopTracer
	}
	return c.engine.Tracer()
}

func (c *serviceContext) Metrics() Metrics {
	if c.engine == nil {
		return NoopMetrics()
	}
	return c.engine.Metrics()
}

func (c *serviceContext) Quota() QuotaEnforcer {
	if c.engine == nil {
		return NoopQuota()
	}
	return c.engine.Quota()
}

func (c *serviceContext) EnforceQuota(resource string, amount int64) error {
	if c.engine == nil {
		return core.ErrServiceUnavailable
	}
	return c.engine.EnforceQuota(resource, amount)
}

func (c *serviceContext) StartObservation(ctx context.Context, attrs map[string]string) (context.Context, func(error)) {
	if c.engine == nil {
		return ctx, func(error) {}
	}
	return c.engine.StartObservation(ctx, attrs)
}

func (c *serviceContext) ObserveOperation(ctx context.Context, attrs map[string]string, op func(context.Context) error) error {
	if c.engine == nil {
		return op(ctx)
	}
	return c.engine.ObserveOperation(ctx, attrs, op)
}

func (c *serviceContext) SystemService(name SystemService) any {
	if c == nil || c.services == nil {
		return nil
	}
	return c.services[name]
}
