package neorand

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers service-specific HTTP routes.
// Note: /health is registered by BaseService.RegisterStandardRoutes()
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

	s.BaseService.RegisterStandardRoutes()
}
