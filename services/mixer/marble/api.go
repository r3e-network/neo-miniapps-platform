// Package mixer provides API routes for the privacy mixer service.
package mixermarble

import (
	"github.com/R3E-Network/service_layer/internal/marble"
)

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers all HTTP routes for the mixer service.
func (s *Service) registerRoutes() {
	router := s.Router()
	router.HandleFunc("/health", marble.HealthHandler(s.Service)).Methods("GET")
	router.HandleFunc("/info", s.handleInfo).Methods("GET")
	router.HandleFunc("/request", s.handleCreateRequest).Methods("POST")
	router.HandleFunc("/status/{id}", s.handleGetStatus).Methods("GET")
	router.HandleFunc("/requests", s.handleListRequests).Methods("GET")
	router.HandleFunc("/request/{id}", s.handleGetRequest).Methods("GET")
	router.HandleFunc("/request/{id}/deposit", s.handleConfirmDeposit).Methods("POST")
	router.HandleFunc("/request/{id}/resume", s.handleResumeRequest).Methods("POST")
	router.HandleFunc("/request/{id}/dispute", s.handleDispute).Methods("POST")
	router.HandleFunc("/request/{id}/proof", s.handleGetCompletionProof).Methods("GET")
}
