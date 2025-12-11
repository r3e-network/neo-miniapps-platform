// Package neoflow provides API routes for the task neoflow service.
package neoflowmarble

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers service-specific HTTP routes.
// Note: /health and /info are registered by BaseService.RegisterStandardRoutes()
func (s *Service) registerRoutes() {
	router := s.Router()
	router.HandleFunc("/triggers", s.handleListTriggers).Methods("GET")
	router.HandleFunc("/triggers", s.handleCreateTrigger).Methods("POST")
	router.HandleFunc("/triggers/{id}", s.handleGetTrigger).Methods("GET")
	router.HandleFunc("/triggers/{id}", s.handleUpdateTrigger).Methods("PUT")
	router.HandleFunc("/triggers/{id}", s.handleDeleteTrigger).Methods("DELETE")
	router.HandleFunc("/triggers/{id}/enable", s.handleEnableTrigger).Methods("POST")
	router.HandleFunc("/triggers/{id}/disable", s.handleDisableTrigger).Methods("POST")
	router.HandleFunc("/triggers/{id}/executions", s.handleListExecutions).Methods("GET")
	router.HandleFunc("/triggers/{id}/resume", s.handleResumeTrigger).Methods("POST")
}
