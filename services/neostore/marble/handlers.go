// Package neostore provides HTTP handlers for the neostore service.
package neostoremarble

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/httputil"
	neostoresupabase "github.com/R3E-Network/service_layer/services/neostore/supabase"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

// handleListSecrets lists metadata for a user's secrets (no plaintext).
func (s *Service) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if !s.authorizeServiceCaller(w, r) {
		return
	}

	records, err := s.db.GetSecrets(r.Context(), userID)
	if err != nil {
		httputil.InternalError(w, "failed to load secrets")
		return
	}

	result := make([]SecretRecord, 0, len(records))
	for _, rec := range records {
		result = append(result, SecretRecord{
			ID:        rec.ID,
			Name:      rec.Name,
			Version:   rec.Version,
			CreatedAt: rec.CreatedAt,
			UpdatedAt: rec.UpdatedAt,
		})
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}

// handleCreateSecret creates or updates a secret for the user.
func (s *Service) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if !s.authorizeServiceCaller(w, r) {
		return
	}

	var input CreateSecretInput
	if !httputil.DecodeJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || input.Value == "" {
		httputil.BadRequest(w, "name and value required")
		return
	}

	cipher, err := s.encrypt([]byte(input.Value))
	if err != nil {
		s.logAudit(r.Context(), userID, input.Name, "create", "", false, "encryption failed", r)
		httputil.InternalError(w, "failed to encrypt secret")
		return
	}

	// Check if secret already exists
	existing, err := s.db.GetSecretByName(r.Context(), userID, input.Name)
	if err != nil {
		s.logAudit(r.Context(), userID, input.Name, "create", "", false, "database error", r)
		httputil.InternalError(w, "failed to check existing secret")
		return
	}

	now := time.Now()
	var rec *neostoresupabase.Secret
	var statusCode int
	var action string

	if existing != nil {
		// Update existing secret
		action = "update"
		rec = existing
		rec.EncryptedValue = cipher
		rec.Version = existing.Version + 1
		rec.UpdatedAt = now

		if err := s.db.UpdateSecret(r.Context(), rec); err != nil {
			s.logAudit(r.Context(), userID, input.Name, action, "", false, err.Error(), r)
			httputil.InternalError(w, "failed to update secret")
			return
		}
		statusCode = http.StatusOK
	} else {
		// Create new secret
		action = "create"
		rec = &neostoresupabase.Secret{
			ID:             uuid.New().String(),
			UserID:         userID,
			Name:           input.Name,
			EncryptedValue: cipher,
			Version:        1,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		if err := s.db.CreateSecret(r.Context(), rec); err != nil {
			s.logAudit(r.Context(), userID, input.Name, action, "", false, err.Error(), r)
			httputil.InternalError(w, "failed to store secret")
			return
		}
		// Reset permissions to empty on create
		if err := s.db.SetAllowedServices(r.Context(), userID, input.Name, nil); err != nil {
			log.Printf("[neostore] failed to reset permissions for secret: %v", err)
		}
		statusCode = http.StatusCreated
	}

	// Log successful operation
	s.logAudit(r.Context(), userID, input.Name, action, "", true, "", r)

	httputil.WriteJSON(w, statusCode, SecretRecord{
		ID:        rec.ID,
		Name:      rec.Name,
		Version:   rec.Version,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	})
}

// handleGetSecret returns the plaintext secret for the user.
func (s *Service) handleGetSecret(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if !s.authorizeServiceCaller(w, r) {
		return
	}
	name := mux.Vars(r)["name"]
	if name == "" {
		httputil.BadRequest(w, "name required")
		return
	}

	records, err := s.db.GetSecrets(r.Context(), userID)
	if err != nil {
		s.logAudit(r.Context(), userID, name, "read", r.Header.Get(ServiceIDHeader), false, "database error", r)
		httputil.InternalError(w, "failed to load secrets")
		return
	}

	var rec *neostoresupabase.Secret
	for i := range records {
		if records[i].Name == name {
			rec = &records[i]
			break
		}
	}
	if rec == nil {
		s.logAudit(r.Context(), userID, name, "read", r.Header.Get(ServiceIDHeader), false, "secret not found", r)
		httputil.NotFound(w, "secret not found")
		return
	}

	// If a service is calling, enforce per-secret policy.
	serviceID := r.Header.Get(ServiceIDHeader)
	if serviceID != "" {
		allowed, err := s.isServiceAllowedForSecret(r.Context(), userID, name, serviceID)
		if err != nil {
			s.logAudit(r.Context(), userID, name, "read", serviceID, false, "permission check failed", r)
			httputil.InternalError(w, "failed to check permissions")
			return
		}
		if !allowed {
			s.logAudit(r.Context(), userID, name, "read", serviceID, false, "service not allowed", r)
			httputil.Unauthorized(w, "service not allowed for secret")
			return
		}
	}

	plain, err := s.decrypt(rec.EncryptedValue)
	if err != nil {
		s.logAudit(r.Context(), userID, name, "read", serviceID, false, "decryption failed", r)
		httputil.InternalError(w, "failed to decrypt secret")
		return
	}

	// Log successful read
	s.logAudit(r.Context(), userID, name, "read", serviceID, true, "", r)

	httputil.WriteJSON(w, http.StatusOK, GetSecretResponse{
		Name:    rec.Name,
		Value:   string(plain),
		Version: rec.Version,
	})
}

// authorizeServiceCaller enforces that service-to-service calls present an allowed service ID.
// Users calling directly may omit the header, but if the header is present it must be allowed.
func (s *Service) authorizeServiceCaller(w http.ResponseWriter, r *http.Request) bool {
	svc := r.Header.Get(ServiceIDHeader)
	if svc == "" {
		// User-originated calls (through gateway) may not set a service ID.
		return true
	}
	if _, ok := allowedServiceCallers[strings.ToLower(svc)]; !ok {
		httputil.Unauthorized(w, "service not allowed")
		return false
	}
	return true
}

func (s *Service) isServiceAllowedForSecret(ctx context.Context, userID, secretName, serviceID string) (bool, error) {
	allowedServices, err := s.db.GetAllowedServices(ctx, userID, secretName)
	if err != nil {
		return false, err
	}
	for _, svc := range allowedServices {
		if strings.EqualFold(svc, serviceID) {
			return true, nil
		}
	}
	return false, nil
}

// handleGetSecretPermissions lists allowed services for a secret (user only).
func (s *Service) handleGetSecretPermissions(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if r.Header.Get(ServiceIDHeader) != "" {
		httputil.Unauthorized(w, "only user may manage permissions")
		return
	}
	name := mux.Vars(r)["name"]
	if name == "" {
		httputil.BadRequest(w, "name required")
		return
	}
	allowedServices, err := s.db.GetAllowedServices(r.Context(), userID, name)
	if err != nil {
		httputil.InternalError(w, "failed to load permissions")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"services": allowedServices})
}

// handleSetSecretPermissions replaces the allowed service list (user only).
func (s *Service) handleSetSecretPermissions(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if r.Header.Get(ServiceIDHeader) != "" {
		httputil.Unauthorized(w, "only user may manage permissions")
		return
	}
	name := mux.Vars(r)["name"]
	if name == "" {
		httputil.BadRequest(w, "name required")
		return
	}
	var body struct {
		Services []string `json:"services"`
	}
	if !httputil.DecodeJSON(w, r, &body) {
		return
	}
	if err := s.db.SetAllowedServices(r.Context(), userID, name, body.Services); err != nil {
		httputil.InternalError(w, "failed to set permissions")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"services": body.Services})
}

// handleDeleteSecret deletes a secret (user only).
func (s *Service) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if r.Header.Get(ServiceIDHeader) != "" {
		httputil.Unauthorized(w, "only user may delete secrets")
		return
	}
	name := mux.Vars(r)["name"]
	if name == "" {
		httputil.BadRequest(w, "name required")
		return
	}

	// Check if secret exists
	existing, err := s.db.GetSecretByName(r.Context(), userID, name)
	if err != nil {
		s.logAudit(r.Context(), userID, name, "delete", "", false, "database error", r)
		httputil.InternalError(w, "failed to check secret")
		return
	}
	if existing == nil {
		s.logAudit(r.Context(), userID, name, "delete", "", false, "secret not found", r)
		httputil.NotFound(w, "secret not found")
		return
	}

	// Delete associated permissions first
	if err := s.db.SetAllowedServices(r.Context(), userID, name, nil); err != nil {
		log.Printf("[neostore] failed to delete permissions for secret: %v", err)
	}

	// Delete the secret
	if err := s.db.DeleteSecret(r.Context(), userID, name); err != nil {
		s.logAudit(r.Context(), userID, name, "delete", "", false, err.Error(), r)
		httputil.InternalError(w, "failed to delete secret")
		return
	}

	// Log successful deletion
	s.logAudit(r.Context(), userID, name, "delete", "", true, "", r)

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// handleGetAuditLogs retrieves audit logs for the user.
func (s *Service) handleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if r.Header.Get(ServiceIDHeader) != "" {
		httputil.Unauthorized(w, "only user may view audit logs")
		return
	}

	// Parse limit parameter
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	logs, err := s.db.GetAuditLogs(r.Context(), userID, limit)
	if err != nil {
		httputil.InternalError(w, "failed to load audit logs")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, logs)
}

// handleGetSecretAuditLogs retrieves audit logs for a specific secret.
func (s *Service) handleGetSecretAuditLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if r.Header.Get(ServiceIDHeader) != "" {
		httputil.Unauthorized(w, "only user may view audit logs")
		return
	}

	name := mux.Vars(r)["name"]
	if name == "" {
		httputil.BadRequest(w, "name required")
		return
	}

	// Parse limit parameter
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	logs, err := s.db.GetAuditLogsForSecret(r.Context(), userID, name, limit)
	if err != nil {
		httputil.InternalError(w, "failed to load audit logs")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, logs)
}

// logAudit creates an audit log entry for a secret operation.
func (s *Service) logAudit(ctx context.Context, userID, secretName, action, serviceID string, success bool, errorMsg string, r *http.Request) {
	log := &neostoresupabase.AuditLog{
		ID:           uuid.New().String(),
		UserID:       userID,
		SecretName:   secretName,
		Action:       action,
		ServiceID:    serviceID,
		Success:      success,
		ErrorMessage: errorMsg,
		CreatedAt:    time.Now(),
	}

	if r != nil {
		log.IPAddress = getClientIP(r)
		log.UserAgent = r.UserAgent()
	}

	// Log asynchronously to avoid blocking the main operation
	go func() {
		if err := s.db.CreateAuditLog(context.Background(), log); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("[neostore] failed to create audit log: %v\n", err)
		}
	}()
}

// getClientIP extracts the client IP address from the request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx > 0 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}
