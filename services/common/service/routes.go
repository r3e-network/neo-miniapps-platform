// Package service provides common service infrastructure for marble services.
package service

import (
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/internal/httputil"
)

// =============================================================================
// Standard Response Types
// =============================================================================

// HealthResponse is the standard response for /health endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Enclave   bool   `json:"enclave"`
	Timestamp string `json:"timestamp"`
}

// InfoResponse is the standard response for /info endpoint.
type InfoResponse struct {
	Status     string         `json:"status"`
	Service    string         `json:"service"`
	Version    string         `json:"version"`
	Enclave    bool           `json:"enclave"`
	Timestamp  string         `json:"timestamp"`
	Statistics map[string]any `json:"statistics,omitempty"`
}

// =============================================================================
// Standard Handlers
// =============================================================================

// HealthHandler returns a standardized /health handler for BaseService.
func HealthHandler(s *BaseService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "healthy"

		// Check if service implements HealthChecker for custom status
		if checker, ok := interface{}(s).(HealthChecker); ok {
			status = checker.HealthStatus()
		}

		resp := HealthResponse{
			Status:    status,
			Service:   s.Name(),
			Version:   s.Version(),
			Enclave:   s.Marble().IsEnclave(),
			Timestamp: time.Now().Format(time.RFC3339),
		}
		httputil.WriteJSON(w, http.StatusOK, resp)
	}
}

// InfoHandler returns a standardized /info handler for BaseService.
// It includes statistics from the registered stats function if available.
func InfoHandler(s *BaseService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := InfoResponse{
			Status:    "active",
			Service:   s.Name(),
			Version:   s.Version(),
			Enclave:   s.Marble().IsEnclave(),
			Timestamp: time.Now().Format(time.RFC3339),
		}

		// Include statistics if provider is registered
		if s.statsFn != nil {
			resp.Statistics = s.statsFn()
		}

		httputil.WriteJSON(w, http.StatusOK, resp)
	}
}

// =============================================================================
// Route Registration
// =============================================================================

// RegisterStandardRoutes registers the standard /health and /info endpoints.
// This should be called by services that want consistent endpoint behavior.
func (b *BaseService) RegisterStandardRoutes() {
	router := b.Router()
	router.HandleFunc("/health", HealthHandler(b)).Methods("GET")
	router.HandleFunc("/info", InfoHandler(b)).Methods("GET")
}
