// Package neovault provides API routes for the privacy neovault service.
package neovaultmarble

import (
	"net/http"

	"github.com/R3E-Network/service_layer/internal/httputil"
)

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers service-specific HTTP routes.
// Note: /health is registered by BaseService.RegisterStandardRoutes()
// /info is custom because it requires async network calls to neoaccounts service
func (s *Service) registerRoutes() {
	router := s.Router()

	// Public endpoints (no registration required)
	router.HandleFunc("/info", s.handleInfo).Methods("GET")

	// Registration endpoints
	router.HandleFunc("/registration/apply", s.handleRegistrationApply).Methods("POST")
	router.HandleFunc("/registration/status", s.handleRegistrationStatus).Methods("GET")

	// Admin endpoints (require admin role via gateway)
	router.HandleFunc("/admin/registrations", s.handleAdminListRegistrations).Methods("GET")
	router.HandleFunc("/admin/registrations/review", s.handleAdminReviewRegistration).Methods("POST")

	// Protected endpoints (require approved registration)
	router.HandleFunc("/request", s.withRegistrationCheck(s.handleCreateRequest)).Methods("POST")
	router.HandleFunc("/status/{id}", s.handleGetStatus).Methods("GET")
	router.HandleFunc("/requests", s.handleListRequests).Methods("GET")
	router.HandleFunc("/request/{id}", s.handleGetRequest).Methods("GET")
	router.HandleFunc("/request/{id}/deposit", s.withRegistrationCheck(s.handleConfirmDeposit)).Methods("POST")
	router.HandleFunc("/request/{id}/resume", s.handleResumeRequest).Methods("POST")
	router.HandleFunc("/request/{id}/dispute", s.handleDispute).Methods("POST")
	router.HandleFunc("/request/{id}/proof", s.handleGetCompletionProof).Methods("GET")
}

// withRegistrationCheck wraps a handler to require approved registration.
func (s *Service) withRegistrationCheck(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.RequireUserID(w, r)
		if !ok {
			return
		}

		// Check registration status
		if _, ok := s.requireApprovedRegistration(w, r, userID); !ok {
			return
		}

		// User is approved, proceed to handler
		next(w, r)
	}
}
