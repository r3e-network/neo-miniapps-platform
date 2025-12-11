// Package plugin provides a plugin system for service registration and discovery.
// All services are compiled into a single binary but can be enabled/disabled via configuration.
// This maintains MarbleRun compatibility where each service runs as a separate Marble.
package plugin

import (
	"context"
	"net/http"
)

// ServicePlugin defines the contract that all Neo services must implement.
// Each service is a separate Marble in MarbleRun, running in its own TEE enclave.
type ServicePlugin interface {
	// Identity returns service identification information
	ID() string      // Unique service identifier (e.g., "neorand")
	Name() string    // Human-readable name (e.g., "NeoRand Service")
	Version() string // Semantic version (e.g., "2.0.0")

	// Lifecycle manages service startup and shutdown
	Start(ctx context.Context) error
	Stop() error

	// HTTP returns the service's HTTP router for API endpoints
	Router() http.Handler

	// HealthCheck performs a health check and returns nil if healthy
	HealthCheck() error

	// Metadata returns optional service metadata
	Metadata() map[string]interface{}
}

// ServiceInfo contains static information about a registered service.
type ServiceInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}
