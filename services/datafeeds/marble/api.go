// Package datafeeds provides API routes for the price feed aggregation service.
package datafeedsmarble

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
	// Use path pattern that matches feed IDs with slashes (e.g., BTC/USD)
	router.PathPrefix("/price/").HandlerFunc(s.handleGetPrice).Methods("GET")
	router.HandleFunc("/prices", s.handleGetPrices).Methods("GET")
	router.HandleFunc("/feeds", s.handleListFeeds).Methods("GET")
	router.HandleFunc("/config", s.handleGetConfig).Methods("GET")
	router.HandleFunc("/sources", s.handleListSources).Methods("GET")
}
