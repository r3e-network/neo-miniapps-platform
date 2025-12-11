// Package neovaultmarble provides registration handlers for the NeoVault service.
package neovaultmarble

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/internal/logging"
	neovaultsupabase "github.com/R3E-Network/service_layer/services/neovault/supabase"
)

// Validation patterns
var (
	emailRegex        = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	jurisdictionRegex = regexp.MustCompile(`^[A-Z]{2}$`) // ISO 3166-1 alpha-2
	validVolumes      = map[string]bool{"low": true, "medium": true, "high": true}
)

// =============================================================================
// Registration Handlers
// =============================================================================

// handleRegistrationApply processes a new registration application.
func (s *Service) handleRegistrationApply(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	var input RegistrationApplyInput
	if !httputil.DecodeJSON(w, r, &input) {
		return
	}

	// Validate input
	if !input.AcceptTerms {
		httputil.BadRequest(w, "you must accept the terms of service")
		return
	}
	if input.Email == "" || !emailRegex.MatchString(input.Email) {
		httputil.BadRequest(w, "valid email address is required")
		return
	}
	if input.Jurisdiction == "" || !jurisdictionRegex.MatchString(input.Jurisdiction) {
		httputil.BadRequest(w, "valid jurisdiction (ISO country code) is required")
		return
	}
	if !validVolumes[input.ExpectedVolume] {
		httputil.BadRequest(w, "expected_volume must be: low, medium, or high")
		return
	}

	// Check for existing registration
	existing, err := s.repo.GetRegistrationByUserID(r.Context(), userID)
	if err == nil && existing != nil {
		// User already has a registration
		switch existing.Status {
		case neovaultsupabase.RegStatusApproved:
			httputil.WriteJSON(w, http.StatusOK, RegistrationResponse{
				ID:      existing.ID,
				Status:  string(existing.Status),
				Message: "You are already registered and approved for NeoVault service",
			})
			return
		case neovaultsupabase.RegStatusPending:
			httputil.WriteJSON(w, http.StatusOK, RegistrationResponse{
				ID:      existing.ID,
				Status:  string(existing.Status),
				Message: "Your registration is pending review",
			})
			return
		case neovaultsupabase.RegStatusRejected:
			// Allow reapplication after rejection
			// Update existing registration
			existing.Email = input.Email
			existing.Jurisdiction = input.Jurisdiction
			existing.Purpose = input.Purpose
			existing.ExpectedVolume = input.ExpectedVolume
			existing.TermsVersion = CurrentTermsVersion
			existing.TermsAcceptedAt = time.Now()
			existing.Status = neovaultsupabase.RegStatusPending
			existing.RejectionReason = ""
			if err := s.repo.UpdateRegistration(r.Context(), existing); err != nil {
				httputil.InternalError(w, "failed to update registration")
				return
			}
			// Audit log
			s.logAudit(r, userID, "", neovaultsupabase.AuditActionRegistrationSubmitted, "registration", existing.ID, map[string]interface{}{
				"reapplication": true,
				"jurisdiction":  input.Jurisdiction,
			})
			httputil.WriteJSON(w, http.StatusOK, RegistrationResponse{
				ID:           existing.ID,
				Status:       string(existing.Status),
				Message:      "Registration resubmitted for review",
				TermsVersion: CurrentTermsVersion,
			})
			return
		case neovaultsupabase.RegStatusSuspended, neovaultsupabase.RegStatusRevoked:
			httputil.WriteError(w, http.StatusForbidden, "your access has been suspended or revoked, contact support")
			return
		}
	} else if err != nil && !database.IsNotFound(err) {
		httputil.InternalError(w, "failed to check existing registration")
		return
	}

	// Create new registration
	reg := &neovaultsupabase.Registration{
		UserID:          userID,
		Status:          neovaultsupabase.RegStatusPending,
		Email:           input.Email,
		Jurisdiction:    input.Jurisdiction,
		TermsVersion:    CurrentTermsVersion,
		TermsAcceptedAt: time.Now(),
		Purpose:         input.Purpose,
		ExpectedVolume:  input.ExpectedVolume,
	}

	if err := s.repo.CreateRegistration(r.Context(), reg); err != nil {
		httputil.InternalError(w, "failed to create registration")
		return
	}

	// Audit log
	s.logAudit(r, userID, "", neovaultsupabase.AuditActionRegistrationSubmitted, "registration", reg.ID, map[string]interface{}{
		"jurisdiction": input.Jurisdiction,
		"email_domain": extractEmailDomain(input.Email),
	})

	httputil.WriteJSON(w, http.StatusCreated, RegistrationResponse{
		ID:           reg.ID,
		Status:       string(reg.Status),
		Message:      "Registration submitted successfully. You will be notified once reviewed.",
		TermsVersion: CurrentTermsVersion,
	})
}

// handleRegistrationStatus returns the current registration status.
func (s *Service) handleRegistrationStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	reg, err := s.repo.GetRegistrationByUserID(r.Context(), userID)
	if err != nil {
		if database.IsNotFound(err) {
			httputil.WriteJSON(w, http.StatusOK, RegistrationResponse{
				Status:  "not_registered",
				Message: "You have not registered for NeoVault service. Please apply first.",
			})
			return
		}
		httputil.InternalError(w, "failed to fetch registration")
		return
	}

	response := RegistrationResponse{
		ID:           reg.ID,
		Status:       string(reg.Status),
		TermsVersion: reg.TermsVersion,
	}

	switch reg.Status {
	case neovaultsupabase.RegStatusApproved:
		response.Message = "Your registration is approved. You may use NeoVault service."
	case neovaultsupabase.RegStatusPending:
		response.Message = "Your registration is pending review."
	case neovaultsupabase.RegStatusRejected:
		response.Message = "Your registration was rejected."
		response.RejectionReason = reg.RejectionReason
	case neovaultsupabase.RegStatusSuspended:
		response.Message = "Your access has been suspended."
	case neovaultsupabase.RegStatusRevoked:
		response.Message = "Your access has been revoked."
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// =============================================================================
// Admin Handlers
// =============================================================================

// handleAdminListRegistrations lists all registrations for admin review.
func (s *Service) handleAdminListRegistrations(w http.ResponseWriter, r *http.Request) {
	adminID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	if !httputil.RequireAdminRole(w, r) {
		return
	}

	statusFilter := r.URL.Query().Get("status")
	logging.Default().LogSecurityEvent(r.Context(), "neovault_admin_list_registrations", map[string]interface{}{
		"admin_id":      adminID,
		"status_filter": statusFilter,
	})

	var registrations []neovaultsupabase.Registration
	var err error

	if statusFilter != "" {
		status := neovaultsupabase.RegistrationStatus(statusFilter)
		registrations, err = s.repo.ListRegistrationsByStatus(r.Context(), status)
	} else {
		registrations, err = s.repo.ListPendingRegistrations(r.Context())
	}

	if err != nil {
		httputil.InternalError(w, "failed to list registrations")
		return
	}

	summaries := make([]RegistrationSummary, 0, len(registrations))
	for _, reg := range registrations {
		summaries = append(summaries, RegistrationSummary{
			ID:             reg.ID,
			UserID:         reg.UserID,
			Email:          reg.Email,
			Status:         string(reg.Status),
			Jurisdiction:   reg.Jurisdiction,
			ExpectedVolume: reg.ExpectedVolume,
			CreatedAt:      reg.CreatedAt.Format(time.RFC3339),
		})
	}

	httputil.WriteJSON(w, http.StatusOK, AdminListResponse{
		Registrations: summaries,
		Total:         len(summaries),
	})
}

// handleAdminReviewRegistration processes admin approval/rejection.
func (s *Service) handleAdminReviewRegistration(w http.ResponseWriter, r *http.Request) {
	adminID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	if !httputil.RequireAdminRole(w, r) {
		return
	}

	var input AdminApproveInput
	if !httputil.DecodeJSON(w, r, &input) {
		return
	}

	if input.RegistrationID == "" {
		httputil.BadRequest(w, "registration_id is required")
		return
	}

	reg, err := s.repo.GetRegistrationByID(r.Context(), input.RegistrationID)
	if err != nil {
		if database.IsNotFound(err) {
			httputil.NotFound(w, "registration not found")
			return
		}
		httputil.InternalError(w, "failed to fetch registration")
		return
	}

	var newStatus neovaultsupabase.RegistrationStatus
	var auditAction string
	var message string

	switch strings.ToLower(input.Action) {
	case "approve":
		newStatus = neovaultsupabase.RegStatusApproved
		auditAction = neovaultsupabase.AuditActionRegistrationApproved
		message = "Registration approved"
	case "reject":
		if input.RejectionReason == "" {
			httputil.BadRequest(w, "rejection_reason is required for rejection")
			return
		}
		newStatus = neovaultsupabase.RegStatusRejected
		auditAction = neovaultsupabase.AuditActionRegistrationRejected
		message = "Registration rejected"
		reg.RejectionReason = input.RejectionReason
	case "suspend":
		newStatus = neovaultsupabase.RegStatusSuspended
		auditAction = neovaultsupabase.AuditActionRegistrationSuspended
		message = "Registration suspended"
	case "revoke":
		newStatus = neovaultsupabase.RegStatusRevoked
		auditAction = neovaultsupabase.AuditActionRegistrationRevoked
		message = "Registration revoked"
	default:
		httputil.BadRequest(w, "action must be: approve, reject, suspend, or revoke")
		return
	}

	// Update registration
	reg.Status = newStatus
	reg.ReviewedBy = adminID
	reg.ReviewedAt = time.Now()
	reg.ReviewNotes = input.Notes

	// Apply custom limits if provided
	if input.MaxDailyAmount != nil {
		reg.MaxDailyAmount = input.MaxDailyAmount
	}
	if input.MaxMonthlyAmount != nil {
		reg.MaxMonthlyAmount = input.MaxMonthlyAmount
	}
	if input.MaxSingleAmount != nil {
		reg.MaxSingleAmount = input.MaxSingleAmount
	}

	if err := s.repo.UpdateRegistration(r.Context(), reg); err != nil {
		httputil.InternalError(w, "failed to update registration")
		return
	}

	// Audit log
	s.logAudit(r, reg.UserID, adminID, auditAction, "registration", reg.ID, map[string]interface{}{
		"action": input.Action,
		"notes":  input.Notes,
	})

	httputil.WriteJSON(w, http.StatusOK, RegistrationResponse{
		ID:      reg.ID,
		Status:  string(reg.Status),
		Message: message,
	})
}

// =============================================================================
// Registration Check Helper
// =============================================================================

// requireApprovedRegistration checks if the user has an approved registration.
// Returns the registration if approved, nil otherwise (and writes error response).
func (s *Service) requireApprovedRegistration(w http.ResponseWriter, r *http.Request, userID string) (*neovaultsupabase.Registration, bool) {
	reg, err := s.repo.GetRegistrationByUserID(r.Context(), userID)
	if err != nil {
		if database.IsNotFound(err) {
			httputil.WriteError(w, http.StatusForbidden, "registration required: please apply for NeoVault service access")
			return nil, false
		}
		httputil.InternalError(w, "failed to verify registration")
		return nil, false
	}

	if !reg.Status.IsApproved() {
		var message string
		switch reg.Status {
		case neovaultsupabase.RegStatusPending:
			message = "registration pending: your application is under review"
		case neovaultsupabase.RegStatusRejected:
			message = "registration rejected: " + reg.RejectionReason
		case neovaultsupabase.RegStatusSuspended:
			message = "access suspended: please contact support"
		case neovaultsupabase.RegStatusRevoked:
			message = "access revoked: please contact support"
		default:
			message = "registration not approved"
		}
		httputil.WriteError(w, http.StatusForbidden, message)
		return nil, false
	}

	return reg, true
}

// =============================================================================
// Audit Logging Helper
// =============================================================================

// logAudit creates an audit log entry asynchronously.
func (s *Service) logAudit(r *http.Request, userID, adminID, action, entityType, entityID string, details map[string]interface{}) {
	log := &neovaultsupabase.AuditLog{
		UserID:      userID,
		AdminID:     adminID,
		Action:      action,
		EntityType:  entityType,
		EntityID:    entityID,
		RequestPath: r.URL.Path,
		Details:     details,
	}

	// Extract IP and User-Agent
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		log.IPAddress = strings.Split(ip, ",")[0]
	} else {
		log.IPAddress = r.RemoteAddr
	}
	log.UserAgent = r.Header.Get("User-Agent")

	// Create audit log asynchronously (don't block request)
	go func() {
		_ = s.repo.CreateAuditLog(r.Context(), log)
	}()
}

// =============================================================================
// Utility Functions
// =============================================================================

// extractEmailDomain extracts the domain from an email for logging (privacy).
func extractEmailDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return "unknown"
}
