package vrf

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// HTTPHandler handles HTTP requests for the VRF service.
// This is self-contained within the service package.
type HTTPHandler struct {
	svc *Service
}

// NewHTTPHandler creates a new HTTP handler for the VRF service.
func NewHTTPHandler(svc *Service) *HTTPHandler {
	return &HTTPHandler{svc: svc}
}

// RegisterRoutes registers the VRF service routes on the given mux.
// Pattern: /accounts/{accountID}/vrf/...
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux, basePath string) {
	mux.HandleFunc(basePath+"/keys", h.handleKeys)
	mux.HandleFunc(basePath+"/keys/", h.handleKeyWithID)
	mux.HandleFunc(basePath+"/requests", h.handleRequests)
	mux.HandleFunc(basePath+"/requests/", h.handleRequestWithID)
}

// Handle handles VRF requests with path parsing.
// This is the main entry point called from the application layer.
func (h *HTTPHandler) Handle(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch rest[0] {
	case "keys":
		h.handleKeysWithRest(w, r, accountID, rest[1:])
	case "requests":
		h.handleRequestsWithRest(w, r, accountID, rest[1:])
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *HTTPHandler) handleKeys(w http.ResponseWriter, r *http.Request) {
	accountID := extractAccountID(r.URL.Path)
	if accountID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("account_id required"))
		return
	}
	h.handleKeysWithRest(w, r, accountID, nil)
}

func (h *HTTPHandler) handleKeyWithID(w http.ResponseWriter, r *http.Request) {
	accountID := extractAccountID(r.URL.Path)
	if accountID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("account_id required"))
		return
	}
	rest := parseVRFPath(r.URL.Path, "keys")
	h.handleKeysWithRest(w, r, accountID, rest)
}

func (h *HTTPHandler) handleRequests(w http.ResponseWriter, r *http.Request) {
	accountID := extractAccountID(r.URL.Path)
	if accountID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("account_id required"))
		return
	}
	h.handleRequestsWithRest(w, r, accountID, nil)
}

func (h *HTTPHandler) handleRequestWithID(w http.ResponseWriter, r *http.Request) {
	accountID := extractAccountID(r.URL.Path)
	if accountID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("account_id required"))
		return
	}
	rest := parseVRFPath(r.URL.Path, "requests")
	h.handleRequestsWithRest(w, r, accountID, rest)
}

func (h *HTTPHandler) handleKeysWithRest(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			keys, err := h.svc.ListKeys(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, keys)
		case http.MethodPost:
			var payload struct {
				PublicKey     string            `json:"public_key"`
				Label         string            `json:"label"`
				Status        string            `json:"status"`
				WalletAddress string            `json:"wallet_address"`
				Attestation   string            `json:"attestation"`
				Metadata      map[string]string `json:"metadata"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			key := Key{
				AccountID:     accountID,
				PublicKey:     payload.PublicKey,
				Label:         payload.Label,
				Status:        KeyStatus(payload.Status),
				WalletAddress: payload.WalletAddress,
				Attestation:   payload.Attestation,
				Metadata:      payload.Metadata,
			}
			created, err := h.svc.CreateKey(r.Context(), key)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, created)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
		return
	}

	keyID := rest[0]
	if len(rest) == 1 {
		switch r.Method {
		case http.MethodGet:
			key, err := h.svc.GetKey(r.Context(), accountID, keyID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, key)
		case http.MethodPut:
			var payload struct {
				PublicKey     string            `json:"public_key"`
				Label         string            `json:"label"`
				Status        string            `json:"status"`
				WalletAddress string            `json:"wallet_address"`
				Attestation   string            `json:"attestation"`
				Metadata      map[string]string `json:"metadata"`
			}
			existing, err := h.svc.GetKey(r.Context(), accountID, keyID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			key := Key{
				ID:            keyID,
				AccountID:     existing.AccountID,
				PublicKey:     payload.PublicKey,
				Label:         payload.Label,
				Status:        KeyStatus(payload.Status),
				WalletAddress: payload.WalletAddress,
				Attestation:   payload.Attestation,
				Metadata:      payload.Metadata,
			}
			updated, err := h.svc.UpdateKey(r.Context(), accountID, key)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPut)
		}
		return
	}

	// /keys/{keyID}/requests - create request for specific key
	if rest[1] == "requests" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		var payload struct {
			Consumer string            `json:"consumer"`
			Seed     string            `json:"seed"`
			Metadata map[string]string `json:"metadata"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if _, err := h.svc.GetKey(r.Context(), accountID, keyID); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		req, err := h.svc.CreateRequest(r.Context(), accountID, keyID, payload.Consumer, payload.Seed, payload.Metadata)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, req)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (h *HTTPHandler) handleRequestsWithRest(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	switch len(rest) {
	case 0:
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		reqs, err := h.svc.ListRequests(r.Context(), accountID, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, reqs)
	default:
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		requestID := rest[0]
		req, err := h.svc.GetRequest(r.Context(), accountID, requestID)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, req)
	}
}

// Helper functions

func extractAccountID(path string) string {
	// Path: /accounts/{accountID}/vrf/...
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "accounts" {
		return parts[1]
	}
	return ""
}

func parseVRFPath(path, resource string) []string {
	// Path: /accounts/{accountID}/vrf/{resource}/{rest...}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Find the resource index and return everything after it
	for i, p := range parts {
		if p == resource && i+1 < len(parts) {
			return parts[i+1:]
		}
	}
	return nil
}

func parseLimitParam(value string, defaultLimit int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultLimit, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return 0, fmt.Errorf("limit must be a positive integer")
	}
	if limit > 1000 {
		limit = 1000
	}
	return limit, nil
}

func decodeJSON(body io.ReadCloser, dst interface{}) error {
	defer body.Close()
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	w.WriteHeader(http.StatusMethodNotAllowed)
}
