package engine

import "context"

// ServiceModule is the common contract every service must implement to plug into the Engine.
// Each module advertises a name and domain, and exposes lifecycle hooks for Start/Stop.
type ServiceModule interface {
	Name() string
	Domain() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// AccountEngine covers account lifecycle and tenancy.
type AccountEngine interface {
	ServiceModule
	CreateAccount(ctx context.Context, owner string, metadata map[string]string) (string, error)
	ListAccounts(ctx context.Context) ([]any, error)
}

// StoreEngine abstracts persistence (e.g., Postgres, in-memory).
type StoreEngine interface {
	ServiceModule
	Ping(ctx context.Context) error
}

// ComputeEngine abstracts execution of user functions or jobs.
type ComputeEngine interface {
	ServiceModule
	Invoke(ctx context.Context, payload any) (any, error)
}

// DataEngine abstracts data-plane services like feeds/streams/datalink.
type DataEngine interface {
	ServiceModule
	Push(ctx context.Context, topic string, payload any) error
}

// EventEngine abstracts event dispatch/subscribe.
type EventEngine interface {
	ServiceModule
	Publish(ctx context.Context, event string, payload any) error
	Subscribe(ctx context.Context, event string, handler func(context.Context, any) error) error
}

// LedgerEngine abstracts a full node for a specific network (e.g., Neo).
type LedgerEngine interface {
	ServiceModule
	LedgerInfo() string
}

// IndexerEngine abstracts a chain indexer.
type IndexerEngine interface {
	ServiceModule
	IndexerInfo() string
}

// RPCEngine exposes generic chain RPC fan-out (btc/eth/neox/etc.).
type RPCEngine interface {
	ServiceModule
	RPCInfo() string
	RPCEndpoints() map[string]string
}

// DataSourceEngine exposes upstream data sources usable by feeds/triggers.
type DataSourceEngine interface {
	ServiceModule
	DataSourcesInfo() string
}

// ContractsEngine manages deployment/invocation of service-layer contracts.
type ContractsEngine interface {
	ServiceModule
	ContractsNetwork() string
}

// ServiceBankEngine controls GAS usage owned by the service layer.
type ServiceBankEngine interface {
	ServiceModule
	ServiceBankInfo() string
}

// CryptoEngine exposes advanced cryptography helpers (ZKP/FHE/MPC).
type CryptoEngine interface {
	ServiceModule
	CryptoInfo() string
}

// Capability markers allow adapters to avoid advertising interfaces they cannot serve.

// AccountCapable indicates whether a module supports account operations.
type AccountCapable interface {
	HasAccount() bool
}

// ComputeCapable indicates whether a module supports compute operations.
type ComputeCapable interface {
	HasCompute() bool
}

// DataCapable indicates whether a module supports data operations.
type DataCapable interface {
	HasData() bool
}

// EventCapable indicates whether a module supports event operations.
type EventCapable interface {
	HasEvent() bool
}

// ReadyChecker reports whether a module is currently ready to serve traffic.
type ReadyChecker interface {
	Ready(ctx context.Context) error
}

// ReadySetter can be implemented by modules to allow the engine to mark readiness explicitly.
type ReadySetter interface {
	SetReady(status string, errMsg string)
}

// EventHandler is a callback used by SubscribeEvent for in-process consumers.
type EventHandler func(context.Context, any) error

// InvokeResult captures the outcome of a ComputeEngine invocation.
type InvokeResult struct {
	Module string
	Result any
	Err    error
}
