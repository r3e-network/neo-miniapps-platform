// Package engine provides the Service Engine (OS Core) for orchestrating service modules.
package engine

import (
	"context"
	"fmt"
)

// =============================================================================
// ServiceRegistry Interface - For Cross-Service Lookup
// =============================================================================

// ServiceRegistry provides type-safe service lookup for cross-service communication.
// This interface enables loose coupling between services by allowing them to
// discover and interact with other services through interfaces rather than
// direct package imports.
//
// Usage:
//
//	// In Functions service, instead of importing automation package directly:
//	automation, err := registry.GetService("automation")
//	if err != nil {
//	    return err
//	}
//	if sched, ok := automation.(AutomationScheduler); ok {
//	    sched.CreateJob(ctx, ...)
//	}
type ServiceRegistry interface {
	// GetService returns a service module by name.
	// Returns an error if the service is not found.
	GetService(name string) (ServiceModule, error)

	// GetServiceAs returns a service cast to the specified interface type.
	// Returns an error if the service is not found or doesn't implement the interface.
	GetServiceAs(name string, target interface{}) error

	// HasService checks if a service is registered.
	HasService(name string) bool

	// ListServices returns all registered service names.
	ListServices() []string
}

// =============================================================================
// Engine implements ServiceRegistry
// =============================================================================

// GetService returns a service module by name.
func (e *Engine) GetService(name string) (ServiceModule, error) {
	mod := e.Lookup(name)
	if mod == nil {
		return nil, fmt.Errorf("service %q not found", name)
	}
	return mod, nil
}

// GetServiceAs returns a service cast to the specified interface type.
// The target must be a pointer to an interface variable.
//
// Example:
//
//	var automation AutomationScheduler
//	if err := registry.GetServiceAs("automation", &automation); err != nil {
//	    return err
//	}
//	automation.CreateJob(ctx, ...)
func (e *Engine) GetServiceAs(name string, target interface{}) error {
	mod := e.Lookup(name)
	if mod == nil {
		return fmt.Errorf("service %q not found", name)
	}

	// Use type switch to handle common service interfaces
	switch t := target.(type) {
	case *ServiceModule:
		*t = mod
		return nil
	case *AccountEngine:
		if v, ok := mod.(AccountEngine); ok {
			*t = v
			return nil
		}
	case *ComputeEngine:
		if v, ok := mod.(ComputeEngine); ok {
			*t = v
			return nil
		}
	case *DataEngine:
		if v, ok := mod.(DataEngine); ok {
			*t = v
			return nil
		}
	case *EventEngine:
		if v, ok := mod.(EventEngine); ok {
			*t = v
			return nil
		}
	}

	return fmt.Errorf("service %q does not implement the requested interface", name)
}

// HasService checks if a service is registered.
func (e *Engine) HasService(name string) bool {
	return e.Lookup(name) != nil
}

// ListServices returns all registered service names.
func (e *Engine) ListServices() []string {
	return e.Modules()
}

// =============================================================================
// Service Adapter Interfaces - For Cross-Service Communication
// =============================================================================

// These interfaces define the contracts for cross-service communication.
// Services should depend on these interfaces rather than concrete service types.
// This enables loose coupling and easier testing.

// AutomationScheduler is the interface for scheduling automation jobs.
// Used by Functions service to create scheduled jobs.
type AutomationScheduler interface {
	CreateJob(ctx context.Context, accountID, functionID, name, schedule, description string) (interface{}, error)
	UpdateJob(ctx context.Context, accountID, jobID string, updates map[string]interface{}) (interface{}, error)
	DeleteJob(ctx context.Context, accountID, jobID string) error
	GetJob(ctx context.Context, accountID, jobID string) (interface{}, error)
}

// OraclePriceFeed is the interface for getting price data from oracle.
// Used by GasBank service for fee calculations.
type OraclePriceFeed interface {
	GetPrice(ctx context.Context, asset string) (float64, error)
	GetPriceWithTimestamp(ctx context.Context, asset string) (price float64, timestamp int64, err error)
}

// DataFeedProvider is the interface for data feed operations.
// Used by Functions service to interact with data feeds.
type DataFeedProvider interface {
	GetLatestValue(ctx context.Context, feedID string) (interface{}, error)
	Subscribe(ctx context.Context, feedID string, handler func(interface{})) error
}

// DataStreamProvider is the interface for data stream operations.
// Used by Functions service to interact with data streams.
type DataStreamProvider interface {
	Push(ctx context.Context, streamID string, data interface{}) error
	GetLatest(ctx context.Context, streamID string) (interface{}, error)
}

// DataLinkProvider is the interface for data link operations.
// Used by Functions service to interact with data links.
type DataLinkProvider interface {
	Send(ctx context.Context, linkID string, data interface{}) error
	Receive(ctx context.Context, linkID string) (interface{}, error)
}

// VRFProvider is the interface for VRF (Verifiable Random Function) operations.
// Used by Functions service to request random values.
type VRFProvider interface {
	RequestRandomness(ctx context.Context, accountID string, seed []byte) (requestID string, err error)
	GetRandomness(ctx context.Context, requestID string) (value []byte, proof []byte, err error)
}

// GasBankProvider is the interface for gas bank operations.
// Used by Functions service to manage gas fees.
type GasBankProvider interface {
	GetBalance(ctx context.Context, accountID string) (float64, error)
	Deposit(ctx context.Context, accountID string, amount float64) error
	Withdraw(ctx context.Context, accountID string, amount float64) error
}

// FeeCollector is the interface for collecting service fees from gas accounts.
// Used by Oracle service to charge fees for oracle requests.
// Implemented by GasBank service.
type FeeCollector interface {
	// CollectFee deducts a fee from the account's gas bank.
	// Returns error if insufficient funds.
	CollectFee(ctx context.Context, accountID string, amount int64, reference string) error
	// RefundFee returns a previously collected fee (e.g., on request failure).
	RefundFee(ctx context.Context, accountID string, amount int64, reference string) error
}

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ ServiceRegistry = (*Engine)(nil)
