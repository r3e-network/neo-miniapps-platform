package neorand

import (
	commonservice "github.com/R3E-Network/service_layer/services/common/service"
)

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers service-specific HTTP routes.
// Note: /health and /ready are registered by BaseService.RegisterStandardRoutesWithOptions()
// /info is custom because it requires async database calls for request stats
func (s *Service) registerRoutes() {
	router := s.Router()
	router.HandleFunc("/info", s.handleInfo).Methods("GET")
	router.HandleFunc("/pubkey", s.handlePublicKey).Methods("GET")
	router.HandleFunc("/request", s.handleCreateRequest).Methods("POST")
	router.HandleFunc("/request/{id}", s.handleGetRequest).Methods("GET")
	router.HandleFunc("/requests", s.handleListRequests).Methods("GET")
	// Direct API for off-chain usage
	router.HandleFunc("/random", s.handleDirectRandom).Methods("POST")
	router.HandleFunc("/verify", s.handleVerify).Methods("POST")

	// Register standard routes but skip /info (we have custom implementation above)
	s.BaseService.RegisterStandardRoutesWithOptions(commonservice.RouteOptions{SkipInfo: true})
}
