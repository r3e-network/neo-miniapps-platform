package mixer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// HTTPHandler handles HTTP requests for the mixer service.
// This is self-contained within the service package.
type HTTPHandler struct {
	svc *Service
}

// NewHTTPHandler creates a new HTTP handler for the mixer service.
func NewHTTPHandler(svc *Service) *HTTPHandler {
	return &HTTPHandler{svc: svc}
}

// RegisterRoutes registers the mixer service routes on the given mux.
// Pattern: /accounts/{accountID}/mixer/...
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux, basePath string) {
	// basePath is typically "/accounts/{accountID}/mixer"
	mux.HandleFunc(basePath, h.handleRoot)
	mux.HandleFunc(basePath+"/", h.handleWithID)
}

// handleRoot handles /accounts/{id}/mixer - list or create requests
func (h *HTTPHandler) handleRoot(w http.ResponseWriter, r *http.Request) {
	accountID := extractAccountID(r.URL.Path)
	if accountID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("account_id required"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.listRequests(w, r, accountID)
	case http.MethodPost:
		h.createRequest(w, r, accountID)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// handleWithID handles /accounts/{id}/mixer/{requestID}[/{action}]
func (h *HTTPHandler) handleWithID(w http.ResponseWriter, r *http.Request) {
	accountID := extractAccountID(r.URL.Path)
	if accountID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("account_id required"))
		return
	}

	// Parse path: /accounts/{accountID}/mixer/{requestID}[/{action}]
	parts := parseMixerPath(r.URL.Path)
	if len(parts) == 0 {
		h.handleRoot(w, r)
		return
	}

	requestID := parts[0]

	if len(parts) == 1 {
		// /accounts/{id}/mixer/{requestID}
		h.getRequest(w, r, accountID, requestID)
		return
	}

	// /accounts/{id}/mixer/{requestID}/{action}
	action := parts[1]
	switch action {
	case "deposit":
		h.confirmDeposit(w, r, requestID)
	case "claim":
		h.createClaim(w, r, requestID)
	default:
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown action: %s", action))
	}
}

func (h *HTTPHandler) listRequests(w http.ResponseWriter, r *http.Request, accountID string) {
	limit, err := parseLimitParam(r.URL.Query().Get("limit"), 50)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	requests, err := h.svc.ListMixRequests(r.Context(), accountID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, requests)
}

func (h *HTTPHandler) createRequest(w http.ResponseWriter, r *http.Request, accountID string) {
	var payload struct {
		SourceWallet string            `json:"source_wallet"`
		Amount       string            `json:"amount"`
		TokenAddress string            `json:"token_address"`
		MixDuration  string            `json:"mix_duration"`
		SplitCount   int               `json:"split_count"`
		Targets      []MixTarget       `json:"targets"`
		Metadata     map[string]string `json:"metadata"`
	}
	if err := decodeJSON(r.Body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	req := MixRequest{
		AccountID:    accountID,
		SourceWallet: payload.SourceWallet,
		Amount:       payload.Amount,
		TokenAddress: payload.TokenAddress,
		MixDuration:  ParseMixDuration(payload.MixDuration),
		SplitCount:   payload.SplitCount,
		Targets:      payload.Targets,
		Metadata:     payload.Metadata,
	}

	created, err := h.svc.CreateMixRequest(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *HTTPHandler) getRequest(w http.ResponseWriter, r *http.Request, accountID, requestID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	req, err := h.svc.GetMixRequest(r.Context(), accountID, requestID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (h *HTTPHandler) confirmDeposit(w http.ResponseWriter, r *http.Request, requestID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var payload struct {
		TxHashes []string `json:"tx_hashes"`
	}
	if err := decodeJSON(r.Body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	req, err := h.svc.ConfirmDeposit(r.Context(), requestID, payload.TxHashes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (h *HTTPHandler) createClaim(w http.ResponseWriter, r *http.Request, requestID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var payload struct {
		ClaimAddress string `json:"claim_address"`
	}
	if err := decodeJSON(r.Body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	claim, err := h.svc.CreateWithdrawalClaim(r.Context(), requestID, payload.ClaimAddress)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, claim)
}

// StatsHandler handles GET /mixer/stats (global endpoint)
func (h *HTTPHandler) StatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	stats, err := h.svc.GetMixStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// Helper functions

func extractAccountID(path string) string {
	// Path: /accounts/{accountID}/mixer/...
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "accounts" {
		return parts[1]
	}
	return ""
}

func parseMixerPath(path string) []string {
	// Path: /accounts/{accountID}/mixer/{rest...}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts[0] = "accounts", parts[1] = accountID, parts[2] = "mixer", parts[3:] = rest
	if len(parts) > 3 {
		return parts[3:]
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
