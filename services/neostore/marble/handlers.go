// Package neostore provides HTTP handlers for the neostore service.
package neostoremarble

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/httputil"
	neostoresupabase "github.com/R3E-Network/service_layer/services/neostore/supabase"
)

const (
	maxSecretNameLen        = 128
	maxSecretValueBytes     = 64 * 1024
	maxAllowedServicesCount = 16
)

// =============================================================================
// HTTP Handlers
// =============================================================================

// handleListSecrets lists metadata for a user's secrets (no plaintext).
func (s *Service) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		httputil.ServiceUnavailable(w, "database not configured")
		return
	}
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if callerServiceID(r) != "" {
		httputil.Unauthorized(w, "only user may list secrets")
		return
	}

	records, err := s.db.GetSecrets(r.Context(), userID)
	if err != nil {
		httputil.InternalError(w, "failed to load secrets")
		return
	}

	result := make([]SecretRecord, 0, len(records))
	for i := range records {
		rec := &records[i]
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
	if s.db == nil {
		httputil.ServiceUnavailable(w, "database not configured")
		return
	}
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if callerServiceID(r) != "" {
		httputil.Unauthorized(w, "only user may create or update secrets")
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
	if len(input.Name) > maxSecretNameLen {
		httputil.BadRequest(w, "name too long")
		return
	}
	if len(input.Value) > maxSecretValueBytes {
		httputil.BadRequest(w, "value too large")
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
			s.Logger().WithContext(r.Context()).WithError(err).WithField("secret_name", input.Name).Warn("failed to reset permissions for secret")
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
	if s.db == nil {
		httputil.ServiceUnavailable(w, "database not configured")
		return
	}
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

	serviceID := callerServiceID(r)
	rec, err := s.db.GetSecretByName(r.Context(), userID, name)
	if err != nil {
		s.logAudit(r.Context(), userID, name, "read", serviceID, false, "database error", r)
		httputil.InternalError(w, "failed to load secret")
		return
	}
	if rec == nil {
		s.logAudit(r.Context(), userID, name, "read", serviceID, false, "secret not found", r)
		httputil.NotFound(w, "secret not found")
		return
	}

	// If a service is calling, enforce per-secret policy.
	if serviceID != "" {
		allowed, allowedErr := s.isServiceAllowedForSecret(r.Context(), userID, name, serviceID)
		if allowedErr != nil {
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
	if len(plain) > maxSecretValueBytes {
		s.logAudit(r.Context(), userID, name, "read", serviceID, false, "secret too large", r)
		httputil.InternalError(w, "secret value too large")
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

func callerServiceID(r *http.Request) string {
	serviceID := httputil.GetServiceID(r)
	// Treat the gateway as the user-facing edge, not an internal caller service.
	if strings.EqualFold(serviceID, "gateway") {
		return ""
	}
	return serviceID
}

// authorizeServiceCaller enforces that service-to-service calls present an allowed service ID.
// Users calling directly may omit the header, but if the header is present it must be allowed.
func (s *Service) authorizeServiceCaller(w http.ResponseWriter, r *http.Request) bool {
	svc := callerServiceID(r)
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
	if s.db == nil {
		return false, fmt.Errorf("database not configured")
	}
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
	if s.db == nil {
		httputil.ServiceUnavailable(w, "database not configured")
		return
	}
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if callerServiceID(r) != "" {
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
	if s.db == nil {
		httputil.ServiceUnavailable(w, "database not configured")
		return
	}
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if callerServiceID(r) != "" {
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
	if len(body.Services) > maxAllowedServicesCount {
		httputil.BadRequest(w, "too many services")
		return
	}

	canonicalize := func(raw string) (string, bool) {
		svc := httputil.CanonicalizeServiceID(raw)
		if svc == "" {
			return "", false
		}
		if _, ok := allowedServiceCallers[svc]; !ok {
			return "", false
		}
		return svc, true
	}

	normalized := make([]string, 0, len(body.Services))
	seen := make(map[string]struct{})
	for _, raw := range body.Services {
		svc, ok := canonicalize(raw)
		if !ok {
			httputil.BadRequest(w, "invalid service id")
			return
		}
		if _, ok := seen[svc]; ok {
			continue
		}
		seen[svc] = struct{}{}
		normalized = append(normalized, svc)
	}

	if err := s.db.SetAllowedServices(r.Context(), userID, name, normalized); err != nil {
		httputil.InternalError(w, "failed to set permissions")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"services": normalized})
}

// handleDeleteSecret deletes a secret (user only).
func (s *Service) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		httputil.ServiceUnavailable(w, "database not configured")
		return
	}
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if callerServiceID(r) != "" {
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
		s.Logger().WithContext(r.Context()).WithError(err).WithField("secret_name", name).Warn("failed to delete permissions for secret")
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
	if s.db == nil {
		httputil.ServiceUnavailable(w, "database not configured")
		return
	}
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if callerServiceID(r) != "" {
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
	if s.db == nil {
		httputil.ServiceUnavailable(w, "database not configured")
		return
	}
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	if callerServiceID(r) != "" {
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
	if s.db == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	auditLog := &neostoresupabase.AuditLog{
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
		auditLog.IPAddress = getClientIP(r)
		auditLog.UserAgent = r.UserAgent()
	}

	// Log asynchronously to avoid blocking the main operation
	go func(ctx context.Context, auditLog *neostoresupabase.AuditLog) {
		auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := s.db.CreateAuditLog(auditCtx, auditLog); err != nil {
			s.Logger().WithContext(auditCtx).WithError(err).WithFields(map[string]any{
				"user_id":     auditLog.UserID,
				"secret_name": auditLog.SecretName,
				"action":      auditLog.Action,
				"service_id":  auditLog.ServiceID,
				"success":     auditLog.Success,
			}).Warn("failed to create audit log")
		}
	}(ctx, auditLog)
}

// getClientIP extracts the client IP address from the request.
func getClientIP(r *http.Request) string {
	return httputil.ClientIP(r)
}
