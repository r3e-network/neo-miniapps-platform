// Package neocompute provides API routes for the neocompute service.
package neocomputemarble

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers service-specific HTTP handlers.
// Note: /health and /info are registered by BaseService.RegisterStandardRoutes()
func (s *Service) registerRoutes() {
	router := s.Router()
	router.HandleFunc("/execute", s.handleExecute).Methods("POST")
	router.HandleFunc("/jobs/{id}", s.handleGetJob).Methods("GET")
	router.HandleFunc("/jobs", s.handleListJobs).Methods("GET")
}
