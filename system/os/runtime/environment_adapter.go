package pkg

import (
	"github.com/R3E-Network/service_layer/system/os/framework"
)

// NewServiceEnvironment builds a framework.Environment from the PackageRuntime.
// It gracefully degrades when specific capabilities are not available.
func NewServiceEnvironment(runtime PackageRuntime) framework.Environment {
	if runtime == nil {
		return framework.Environment{}
	}

	env := framework.Environment{
		StoreProvider: runtime.StoreProvider(),
		Config:        configAdapter{cfg: runtime.Config()},
		Tracer:        runtime.Tracer(),
		Metrics:       runtime.Metrics(),
		Quota:         runtime.Quota(),
	}

	if bus, err := runtime.Bus(); err == nil {
		env.Bus = bus
	}
	if rpc, err := runtime.RPCClient(); err == nil {
		env.RPCClient = rpc
	}
	if ledger, err := runtime.LedgerClient(); err == nil {
		env.LedgerClient = ledger
	}

	return env
}

type configAdapter struct {
	cfg PackageConfig
}

func (c configAdapter) Get(key string) (string, bool) {
	if c.cfg == nil {
		return "", false
	}
	return c.cfg.Get(key)
}

func (c configAdapter) GetInt(key string) (int, bool) {
	if c.cfg == nil {
		return 0, false
	}
	return c.cfg.GetInt(key)
}

func (c configAdapter) GetBool(key string) (bool, bool) {
	if c.cfg == nil {
		return false, false
	}
	return c.cfg.GetBool(key)
}

func (c configAdapter) All() map[string]string {
	if c.cfg == nil {
		return map[string]string{}
	}
	return c.cfg.GetAll()
}
