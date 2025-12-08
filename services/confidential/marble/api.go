// Package confidential provides API routes for the confidential compute service.
package confidentialmarble

import (
	"github.com/R3E-Network/service_layer/internal/marble"
)

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers HTTP handlers.
func (s *Service) registerRoutes() {
	router := s.Router()
	router.HandleFunc("/health", marble.HealthHandler(s.Service)).Methods("GET")
	router.HandleFunc("/execute", s.handleExecute).Methods("POST")
	router.HandleFunc("/jobs/{id}", s.handleGetJob).Methods("GET")
	router.HandleFunc("/jobs", s.handleListJobs).Methods("GET")
}
