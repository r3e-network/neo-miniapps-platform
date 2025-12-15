package globalsigner

import (
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/internal/middleware"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

// RegisterRoutes registers the GlobalSigner HTTP routes.
func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	// Standard endpoints (from BaseService)
	s.BaseService.RegisterStandardRoutes()

	// SECURITY: Sensitive endpoints require service authentication
	// These endpoints can sign data, rotate keys, or derive keys - must be protected
	mux.Handle("/rotate", middleware.RequireServiceAuth(http.HandlerFunc(s.handleRotate)))
	mux.Handle("/sign", middleware.RequireServiceAuth(http.HandlerFunc(s.handleSign)))
	mux.Handle("/derive", middleware.RequireServiceAuth(http.HandlerFunc(s.handleDerive)))

	// Public endpoints (read-only, safe to expose)
	mux.HandleFunc("/attestation", s.handleAttestation)
	mux.HandleFunc("/keys", s.handleListKeys)
	mux.HandleFunc("/status", s.handleStatus)
}

// handleRotate handles POST /rotate - trigger key rotation.
func (s *Service) handleRotate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req RotateRequest
	if r.Body != nil && r.ContentLength > 0 {
		if !httputil.DecodeJSON(w, r, &req) {
			return
		}
	}

	resp, err := s.Rotate(r.Context(), req.Force)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// handleSign handles POST /sign - domain-separated signing.
func (s *Service) handleSign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req SignRequest
	if !httputil.DecodeJSON(w, r, &req) {
		return
	}

	resp, err := s.Sign(r.Context(), &req)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// handleDerive handles POST /derive - deterministic key derivation.
func (s *Service) handleDerive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req DeriveRequest
	if !httputil.DecodeJSON(w, r, &req) {
		return
	}

	resp, err := s.Derive(r.Context(), &req)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// handleAttestation handles GET /attestation - get current key attestation.
func (s *Service) handleAttestation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	att, err := s.GetAttestation(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, att)
}

// handleListKeys handles GET /keys - list all key versions.
func (s *Service) handleListKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	versions := s.ListKeyVersions()
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"active_version": s.ActiveVersion(),
		"key_versions":   versions,
	})
}

// handleStatus handles GET /status - detailed service status.
func (s *Service) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	versions := s.ListKeyVersions()

	// Calculate next rotation time
	var nextRotation *time.Time
	if activeVersion := s.ActiveVersion(); activeVersion != "" {
		if v, err := s.GetKeyVersion(activeVersion); err == nil && v.ActivatedAt != nil {
			next := v.ActivatedAt.Add(s.rotationConfig.RotationPeriod)
			nextRotation = &next
		}
	}

	resp := StatusResponse{
		Service:          ServiceName,
		Version:          Version,
		Healthy:          s.ActiveVersion() != "",
		ActiveKeyVersion: s.ActiveVersion(),
		KeyVersions:      versions,
		NextRotation:     nextRotation,
		Uptime:           time.Since(s.startTime).String(),
		IsEnclave:        s.Marble().IsEnclave(),
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}
