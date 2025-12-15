// Package service provides common service infrastructure for marble services.
package service

import (
	"context"

	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/chain"
)

// =============================================================================
// Core Service Interfaces
// =============================================================================

// MarbleService is the interface all marble services must implement.
// This ensures consistent lifecycle management across all services.
type MarbleService interface {
	// Identity
	ID() string
	Name() string
	Version() string

	// Lifecycle
	Start(ctx context.Context) error
	Stop() error

	// HTTP
	Router() *mux.Router
}

// =============================================================================
// Optional Capability Interfaces
// =============================================================================

// StatisticsProvider provides runtime statistics for the /info endpoint.
// Services implementing this interface will have their statistics included
// in the standard info response.
type StatisticsProvider interface {
	// Statistics returns service-specific runtime statistics.
	// The returned map will be included in the /info response under "statistics".
	Statistics() map[string]any
}

// Hydratable services can reload state from persistence on startup.
// This is called during Start() after the base service is initialized
// but before background workers are started.
type Hydratable interface {
	// Hydrate loads persistent state into memory.
	// Called once during service startup.
	Hydrate(ctx context.Context) error
}

// ChainIntegrated services interact with the blockchain.
// This interface helps identify services that need chain connectivity.
type ChainIntegrated interface {
	// ChainClient returns the chain client for blockchain interactions.
	ChainClient() *chain.Client

	// TEEFulfiller returns the TEE fulfiller for submitting transactions.
	TEEFulfiller() *chain.TEEFulfiller
}

// =============================================================================
// Worker Interfaces
// =============================================================================

// BackgroundWorker represents a service that runs background tasks.
type BackgroundWorker interface {
	// Workers returns the number of active background workers.
	Workers() int
}

// =============================================================================
// Health Check Interface
// =============================================================================

// HealthChecker provides custom health check logic.
// Services implementing this can provide detailed health status.
type HealthChecker interface {
	// HealthStatus returns the current health status.
	// Returns "healthy", "degraded", or "unhealthy".
	HealthStatus() string

	// HealthDetails returns detailed health information.
	HealthDetails() map[string]any
}
