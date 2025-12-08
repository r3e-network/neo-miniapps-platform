// Package accountpool provides API routes for the account pool service.
package accountpoolmarble

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
	router.HandleFunc("/info", s.handleInfo).Methods("GET")
	router.HandleFunc("/accounts", s.handleListAccounts).Methods("GET")
	router.HandleFunc("/request", s.handleRequestAccounts).Methods("POST")
	router.HandleFunc("/release", s.handleReleaseAccounts).Methods("POST")
	router.HandleFunc("/sign", s.handleSignTransaction).Methods("POST")
	router.HandleFunc("/batch-sign", s.handleBatchSign).Methods("POST")
	router.HandleFunc("/balance", s.handleUpdateBalance).Methods("POST")
}
