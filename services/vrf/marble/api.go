package vrfmarble

import (
	"github.com/R3E-Network/service_layer/internal/marble"
)

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers HTTP routes.
func (s *Service) registerRoutes() {
	router := s.Router()
	router.HandleFunc("/health", marble.HealthHandler(s.Service)).Methods("GET")
	router.HandleFunc("/info", s.handleInfo).Methods("GET")
	router.HandleFunc("/pubkey", s.handlePublicKey).Methods("GET")
	router.HandleFunc("/request", s.handleCreateRequest).Methods("POST")
	router.HandleFunc("/request/{id}", s.handleGetRequest).Methods("GET")
	router.HandleFunc("/requests", s.handleListRequests).Methods("GET")
	// Direct API for off-chain usage
	router.HandleFunc("/random", s.handleDirectRandom).Methods("POST")
	router.HandleFunc("/verify", s.handleVerify).Methods("POST")
}
