// Package api implements the MarbleRun-compatible REST API for the Coordinator.
package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/R3E-Network/service_layer/coordinator/core"
	"github.com/R3E-Network/service_layer/manifest"
)

// Handler implements the Coordinator REST API.
type Handler struct {
	coordinator *core.Coordinator
}

// NewHandler creates a new API handler.
func NewHandler(c *core.Coordinator) *Handler {
	return &Handler{coordinator: c}
}

// RegisterRoutes registers API routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// MarbleRun-compatible API v2
	mux.HandleFunc("/api/v2/manifest", h.handleManifest)
	mux.HandleFunc("/api/v2/quote", h.handleQuote)
	mux.HandleFunc("/api/v2/secrets", h.handleSecrets)
	mux.HandleFunc("/api/v2/status", h.handleStatus)
	mux.HandleFunc("/api/v2/recover", h.handleRecover)
	mux.HandleFunc("/api/v2/update", h.handleUpdate)

	// Marble activation endpoint
	mux.HandleFunc("/api/v2/marble/activate", h.handleActivate)

	// Health check
	mux.HandleFunc("/health", h.handleHealth)
}

// =============================================================================
// Response Types (JSend-compatible)
// =============================================================================

// Response is a JSend-compatible response.
type Response struct {
	Status  string `json:"status"` // "success", "fail", "error"
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

func successResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Status: "success",
		Data:   data,
	})
}

func errorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Status:  "error",
		Message: message,
	})
}

func failResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Status: "fail",
		Data:   data,
	})
}

// =============================================================================
// Manifest Endpoints
// =============================================================================

func (h *Handler) handleManifest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getManifest(w, r)
	case http.MethodPost:
		h.setManifest(w, r)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getManifest(w http.ResponseWriter, r *http.Request) {
	m, fingerprint := h.coordinator.GetManifest()
	if m == nil {
		errorResponse(w, http.StatusNotFound, "manifest not set")
		return
	}

	manifestJSON, err := json.Marshal(m)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "failed to marshal manifest")
		return
	}

	successResponse(w, map[string]any{
		"manifest":    base64.StdEncoding.EncodeToString(manifestJSON),
		"fingerprint": fingerprint,
	})
}

func (h *Handler) setManifest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Manifest string `json:"manifest"` // Base64-encoded manifest
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	manifestData, err := base64.StdEncoding.DecodeString(req.Manifest)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid base64 encoding")
		return
	}

	m, err := manifest.Parse(manifestData, "manifest.json")
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid manifest: "+err.Error())
		return
	}

	resp, err := h.coordinator.SetManifest(r.Context(), m)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Encode recovery secrets
	recoverySecrets := make(map[string]string, len(resp.RecoverySecrets))
	for name, data := range resp.RecoverySecrets {
		recoverySecrets[name] = base64.StdEncoding.EncodeToString(data)
	}

	successResponse(w, map[string]any{
		"recovery_secrets": recoverySecrets,
	})
}

// =============================================================================
// Quote Endpoint
// =============================================================================

func (h *Handler) handleQuote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get nonce from query parameter
	nonce := []byte(r.URL.Query().Get("nonce"))

	quote, err := h.coordinator.GetQuote(r.Context(), nonce)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(w, map[string]any{
		"quote": base64.StdEncoding.EncodeToString(quote),
	})
}

// =============================================================================
// Secrets Endpoints
// =============================================================================

func (h *Handler) handleSecrets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getSecrets(w, r)
	case http.MethodPost:
		h.setSecrets(w, r)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getSecrets(w http.ResponseWriter, r *http.Request) {
	// Get secret names from query parameter
	names := r.URL.Query()["name"]
	if len(names) == 0 {
		errorResponse(w, http.StatusBadRequest, "secret names required")
		return
	}

	secrets, err := h.coordinator.GetSecrets(r.Context(), names)
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	// Encode secrets
	encodedSecrets := make(map[string]string, len(secrets))
	for name, data := range secrets {
		encodedSecrets[name] = base64.StdEncoding.EncodeToString(data)
	}

	successResponse(w, map[string]any{
		"secrets": encodedSecrets,
	})
}

func (h *Handler) setSecrets(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Secrets map[string]string `json:"secrets"` // name -> base64-encoded value
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for name, encodedValue := range req.Secrets {
		value, err := base64.StdEncoding.DecodeString(encodedValue)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid base64 for secret "+name)
			return
		}

		if err := h.coordinator.SetSecret(r.Context(), name, value); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	successResponse(w, nil)
}

// =============================================================================
// Status Endpoint
// =============================================================================

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	status := h.coordinator.GetStatus()
	successResponse(w, status)
}

// =============================================================================
// Recovery Endpoint
// =============================================================================

func (h *Handler) handleRecover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RecoverySecrets map[string]string `json:"recovery_secrets"` // name -> base64-encoded decrypted key
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Decode recovery secrets
	shares := make(map[string][]byte, len(req.RecoverySecrets))
	for name, encoded := range req.RecoverySecrets {
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid base64 for "+name)
			return
		}
		shares[name] = data
	}

	if err := h.coordinator.Recover(r.Context(), shares); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	successResponse(w, nil)
}

// =============================================================================
// Update Endpoint
// =============================================================================

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// TODO: Implement manifest update
	errorResponse(w, http.StatusNotImplemented, "not implemented")
}

// =============================================================================
// Marble Activation Endpoint
// =============================================================================

func (h *Handler) handleActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req core.ActivationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.coordinator.Activate(r.Context(), &req)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Encode secrets for response
	encodedSecrets := make(map[string]string, len(resp.Secrets))
	for name, data := range resp.Secrets {
		encodedSecrets[name] = base64.StdEncoding.EncodeToString(data)
	}

	successResponse(w, map[string]any{
		"secrets":     encodedSecrets,
		"env":         resp.Env,
		"files":       encodeFiles(resp.Files),
		"certificate": base64.StdEncoding.EncodeToString(resp.Certificate),
		"private_key": base64.StdEncoding.EncodeToString(resp.PrivateKey),
		"root_ca":     base64.StdEncoding.EncodeToString(resp.RootCA),
	})
}

func encodeFiles(files map[string][]byte) map[string]string {
	result := make(map[string]string, len(files))
	for path, content := range files {
		result[path] = base64.StdEncoding.EncodeToString(content)
	}
	return result
}

// =============================================================================
// Health Endpoint
// =============================================================================

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}
