package framework

import (
	"context"
	"strconv"
	"strings"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// StoreProvider exposes datastore access to services without binding them to specific implementations.
// Runtime packages adapt their native store providers to this interface.
type StoreProvider interface {
	Database() any
	AccountExists(ctx context.Context, accountID string) error
	AccountTenant(ctx context.Context, accountID string) string
}

// QuotaEnforcer wraps resource quota enforcement.
type QuotaEnforcer interface {
	Enforce(resource string, amount int64) error
}

// Metrics records counters, gauges, and histograms without binding services to Prometheus implementation.
type Metrics interface {
	Counter(name string, labels map[string]string, delta float64)
	Gauge(name string, labels map[string]string, value float64)
	Histogram(name string, labels map[string]string, value float64)
}

// Config provides read access to service/package configuration.
type Config interface {
	Get(key string) (string, bool)
	GetInt(key string) (int, bool)
	GetBool(key string) (bool, bool)
	All() map[string]string
}

// ConfigMap is a basic Config backed by a map.
type ConfigMap map[string]string

func (c ConfigMap) Get(key string) (string, bool) {
	if c == nil {
		return "", false
	}
	val, ok := c[key]
	return val, ok
}

func (c ConfigMap) GetInt(key string) (int, bool) {
	val, ok := c.Get(key)
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		return 0, false
	}
	return i, true
}

func (c ConfigMap) GetBool(key string) (bool, bool) {
	val, ok := c.Get(key)
	if !ok {
		return false, false
	}
	b, err := strconv.ParseBool(strings.TrimSpace(val))
	if err != nil {
		return false, false
	}
	return b, true
}

func (c ConfigMap) All() map[string]string {
	if c == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(c))
	for k, v := range c {
		out[k] = v
	}
	return out
}

// Environment describes the execution environment provided to services.
// Runtimes populate this structure so ServiceEngine-backed services can access shared infrastructure.
type Environment struct {
	StoreProvider  StoreProvider
	Bus            BusClient
	RPCClient      any
	LedgerClient   any
	ContractClient ContractClient // Smart contract interaction client
	Config         Config
	Tracer         core.Tracer
	Metrics        Metrics
	Quota          QuotaEnforcer
}

// EnvironmentAware modules accept an Environment after construction.
type EnvironmentAware interface {
	SetEnvironment(Environment)
}

func normalizeEnvironment(env Environment) Environment {
	if env.Bus == nil {
		env.Bus = noopBus{}
	}
	if env.Config == nil {
		env.Config = ConfigMap(nil)
	}
	if env.Tracer == nil {
		env.Tracer = core.NoopTracer
	}
	if env.Metrics == nil {
		env.Metrics = noopMetrics{}
	}
	if env.Quota == nil {
		env.Quota = noopQuota{}
	}
	if env.ContractClient == nil {
		env.ContractClient = NoopContractClient()
	}
	return env
}

type noopBus struct{}

func (noopBus) PublishEvent(ctx context.Context, event string, payload any) error {
	return core.ErrBusUnavailable
}

func (noopBus) PushData(ctx context.Context, topic string, payload any) error {
	return core.ErrBusUnavailable
}

func (noopBus) InvokeCompute(ctx context.Context, payload any) ([]ComputeResult, error) {
	return nil, core.ErrBusUnavailable
}

type storeAccountChecker struct {
	store StoreProvider
}

func newStoreAccountChecker(sp StoreProvider) AccountChecker {
	if sp == nil {
		return nil
	}
	return &storeAccountChecker{store: sp}
}

func (s *storeAccountChecker) AccountExists(ctx context.Context, accountID string) error {
	if s.store == nil {
		return core.ErrServiceUnavailable
	}
	return s.store.AccountExists(ctx, accountID)
}

func (s *storeAccountChecker) AccountTenant(ctx context.Context, accountID string) string {
	if s.store == nil {
		return ""
	}
	return s.store.AccountTenant(ctx, accountID)
}

type noopMetrics struct{}

func (noopMetrics) Counter(string, map[string]string, float64)   {}
func (noopMetrics) Gauge(string, map[string]string, float64)     {}
func (noopMetrics) Histogram(string, map[string]string, float64) {}

type noopQuota struct{}

func (noopQuota) Enforce(string, int64) error { return nil }

// NoopMetrics returns a Metrics implementation that does nothing.
func NoopMetrics() Metrics { return noopMetrics{} }

// NoopQuota returns a QuotaEnforcer that never limits.
func NoopQuota() QuotaEnforcer { return noopQuota{} }
