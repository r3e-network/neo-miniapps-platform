// Package neofeeds provides API routes for the price feed aggregation service.
package neofeeds

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers service-specific HTTP routes.
// Note: /health and /info are registered by BaseService.RegisterStandardRoutes()
func (s *Service) registerRoutes() {
	router := s.Router()
	// Match both feed IDs with slashes (e.g., BTC/USD) and pairs without (e.g., BTCUSDT).
	router.HandleFunc("/price/{pair:.+}", s.handleGetPrice).Methods("GET")
	router.HandleFunc("/prices", s.handleGetPrices).Methods("GET")
	router.HandleFunc("/feeds", s.handleListFeeds).Methods("GET")
	router.HandleFunc("/config", s.handleGetConfig).Methods("GET")
	router.HandleFunc("/sources", s.handleListSources).Methods("GET")
}
