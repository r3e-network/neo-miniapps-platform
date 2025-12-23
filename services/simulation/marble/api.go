package neosimulation

import (
	"net/http"
)

// registerRoutes registers the service-specific HTTP routes.
// Note: /health, /ready, and /info are registered by BaseService.RegisterStandardRoutes().
func (s *Service) registerRoutes() {
	router := s.Router()

	// Simulation control endpoints
	router.HandleFunc("/start", s.handleStart).Methods(http.MethodPost)
	router.HandleFunc("/stop", s.handleStop).Methods(http.MethodPost)
	router.HandleFunc("/status", s.handleStatus).Methods(http.MethodGet)
}
