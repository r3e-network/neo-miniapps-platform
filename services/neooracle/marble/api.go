// Package neooracle provides API routes for the neooracle service.
package neooracle

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers service-specific HTTP handlers.
// Note: /health and /info are registered by BaseService.RegisterStandardRoutes()
func (s *Service) registerRoutes() {
	s.RegisterStandardRoutes()
	r := s.Router()
	r.HandleFunc("/query", s.handleQuery).Methods("POST")
}
