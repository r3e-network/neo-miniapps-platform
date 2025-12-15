package globalsigner

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/internal/httputil"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

// RegisterRoutes registers the GlobalSigner HTTP routes.
func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	// Standard endpoints (from BaseService)
	s.BaseService.RegisterStandardRoutes()

	// GlobalSigner-specific endpoints
	mux.HandleFunc("/rotate", s.handleRotate)
	mux.HandleFunc("/sign", s.handleSign)
	mux.HandleFunc("/derive", s.handleDerive)
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
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
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
