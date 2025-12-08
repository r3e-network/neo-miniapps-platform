// Package secrets provides HTTP handlers for the secrets service.
package secrets

import (
	"net/http"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (s *Service) registerRoutes() {
	r := s.Router()
	r.HandleFunc("/health", marbleHealth(s)).Methods("GET")
	r.HandleFunc("/secrets", s.handleListSecrets).Methods("GET")
	r.HandleFunc("/secrets", s.handleCreateSecret).Methods("POST")
	r.HandleFunc("/secrets/{name}", s.handleGetSecret).Methods("GET")
}

// marbleHealth wraps the base health handler without importing marble in this file.
func marbleHealth(s *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"status":   "healthy",
			"service":  s.Name(),
			"version":  s.Version(),
			"enclave":  s.Marble().IsEnclave(),
			"datetime": time.Now().Format(time.RFC3339),
		})
	}
}

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
		httputil.InternalError(w, "failed to encrypt secret")
		return
	}

	rec := &database.Secret{
		ID:             uuid.New().String(),
		UserID:         userID,
		Name:           input.Name,
		EncryptedValue: cipher,
		Version:        1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.db.CreateSecret(r.Context(), rec); err != nil {
		httputil.InternalError(w, "failed to store secret")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, SecretRecord{
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
		httputil.InternalError(w, "failed to load secrets")
		return
	}

	var rec *database.Secret
	for i := range records {
		if records[i].Name == name {
			rec = &records[i]
			break
		}
	}
	if rec == nil {
		httputil.NotFound(w, "secret not found")
		return
	}

	plain, err := s.decrypt(rec.EncryptedValue)
	if err != nil {
		httputil.InternalError(w, "failed to decrypt secret")
		return
	}

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
