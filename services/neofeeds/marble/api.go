// Package neofeeds provides API routes for the price feed aggregation service.
package neofeeds

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers service-specific HTTP routes.
// Note: /health and /info are registered by BaseService.RegisterStandardRoutes()
func (s *Service) registerRoutes() {
	router := s.Router()
	// Use path pattern that matches feed IDs with slashes (e.g., BTC/USD)
	router.PathPrefix("/price/").HandlerFunc(s.handleGetPrice).Methods("GET")
	router.HandleFunc("/prices", s.handleGetPrices).Methods("GET")
	router.HandleFunc("/feeds", s.handleListFeeds).Methods("GET")
	router.HandleFunc("/config", s.handleGetConfig).Methods("GET")
	router.HandleFunc("/sources", s.handleListSources).Methods("GET")
}
