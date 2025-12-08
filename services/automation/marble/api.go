// Package automation provides API routes for the task automation service.
package automationmarble

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
