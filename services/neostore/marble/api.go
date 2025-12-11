// Package neostore provides API routes for the neostore service.
package neostoremarble

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers service-specific HTTP handlers.
// Note: /health and /info are registered by BaseService.RegisterStandardRoutes()
func (s *Service) registerRoutes() {
	r := s.Router()
	r.HandleFunc("/secrets", s.handleListSecrets).Methods("GET")
	r.HandleFunc("/secrets", s.handleCreateSecret).Methods("POST")
	r.HandleFunc("/secrets/{name}", s.handleGetSecret).Methods("GET")
	r.HandleFunc("/secrets/{name}", s.handleDeleteSecret).Methods("DELETE")
	r.HandleFunc("/secrets/{name}/permissions", s.handleGetSecretPermissions).Methods("GET")
	r.HandleFunc("/secrets/{name}/permissions", s.handleSetSecretPermissions).Methods("PUT")
	r.HandleFunc("/audit", s.handleGetAuditLogs).Methods("GET")
	r.HandleFunc("/secrets/{name}/audit", s.handleGetSecretAuditLogs).Methods("GET")
}
