// Package oracle provides API routes for the oracle service.
package oraclemarble

import (
	"github.com/R3E-Network/service_layer/internal/marble"
)

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers HTTP handlers.
func (s *Service) registerRoutes() {
	r := s.Router()
	r.HandleFunc("/health", marble.HealthHandler(s.Service)).Methods("GET")
	r.HandleFunc("/query", s.handleQuery).Methods("POST")
}
