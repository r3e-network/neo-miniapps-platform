// Package api provides HTTP handlers for the user service API.
//
// DEPRECATED: This file contains legacy service-specific HTTP handlers.
// New services should use the declarative HTTP API pattern with
// HTTP{Method}{Path} methods in service packages. See ServiceRouter
// in system/framework/core for the new pattern.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/engine/events"
)

// HTTPHandler provides HTTP endpoints for the user service.
type HTTPHandler struct {
	svc *UserService
	log *logger.Logger
}

// NewHTTPHandler creates a new HTTP handler.
func NewHTTPHandler(svc *UserService, log *logger.Logger) *HTTPHandler {
	if log == nil {
		log = logger.NewDefault("api-handler")
	}
	return &HTTPHandler{svc: svc, log: log}
}

// RegisterRoutes registers all API routes on the given mux.
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	// Account endpoints
	mux.HandleFunc("/api/v1/accounts", h.handleAccounts)
	mux.HandleFunc("/api/v1/accounts/", h.handleAccount)

	// Wallet endpoints
	mux.HandleFunc("/api/v1/wallets", h.handleWallets)

	// Secret endpoints
	mux.HandleFunc("/api/v1/secrets", h.handleSecrets)
	mux.HandleFunc("/api/v1/secrets/", h.handleSecret)

	// Contract endpoints
	mux.HandleFunc("/api/v1/contracts", h.handleContracts)
	mux.HandleFunc("/api/v1/contracts/", h.handleContract)

	// Function endpoints
	mux.HandleFunc("/api/v1/functions", h.handleFunctions)
	mux.HandleFunc("/api/v1/functions/", h.handleFunction)

	// Trigger endpoints
	mux.HandleFunc("/api/v1/triggers", h.handleTriggers)

	// Balance endpoints
	mux.HandleFunc("/api/v1/balance", h.handleBalance)
	mux.HandleFunc("/api/v1/balance/transactions", h.handleTransactions)
	mux.HandleFunc("/api/v1/balance/estimate", h.handleEstimateFee)

	// Request endpoints
	mux.HandleFunc("/api/v1/requests", h.handleRequests)
	mux.HandleFunc("/api/v1/requests/", h.handleRequest)
}

// Response helpers

type apiResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (h *HTTPHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiResponse{Success: true, Data: data})
}

func (h *HTTPHandler) writeError(w http.ResponseWriter, status int, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiResponse{Success: false, Error: err})
}

func (h *HTTPHandler) getAccountID(r *http.Request) string {
	return r.Header.Get("X-Account-ID")
}

// Account handlers

func (h *HTTPHandler) handleAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createAccount(w, r)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPHandler) handleAccount(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimPrefix(r.URL.Path, "/api/v1/accounts/")
	if accountID == "" {
		h.writeError(w, http.StatusBadRequest, "account_id required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getAccount(w, r, accountID)
	case http.MethodPut:
		h.updateAccount(w, r, accountID)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPHandler) createAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Owner    string            `json:"owner"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	id, err := h.svc.CreateAccount(r.Context(), req.Owner, req.Metadata)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *HTTPHandler) getAccount(w http.ResponseWriter, r *http.Request, accountID string) {
	account, err := h.svc.GetAccount(r.Context(), accountID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, account)
}

func (h *HTTPHandler) updateAccount(w http.ResponseWriter, r *http.Request, accountID string) {
	var req struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.UpdateAccount(r.Context(), accountID, req.Metadata); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// Wallet handlers

func (h *HTTPHandler) handleWallets(w http.ResponseWriter, r *http.Request) {
	accountID := h.getAccountID(r)
	if accountID == "" {
		h.writeError(w, http.StatusBadRequest, "X-Account-ID header required")
		return
	}

	switch r.Method {
	case http.MethodPost:
		var req struct {
			Address string `json:"address"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := h.svc.LinkWallet(r.Context(), accountID, req.Address); err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusCreated, map[string]string{"status": "linked"})
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// Secret handlers

func (h *HTTPHandler) handleSecrets(w http.ResponseWriter, r *http.Request) {
	accountID := h.getAccountID(r)
	if accountID == "" {
		h.writeError(w, http.StatusBadRequest, "X-Account-ID header required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		secrets, err := h.svc.ListSecrets(r.Context(), accountID)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, secrets)
	case http.MethodPost:
		var req struct {
			Name      string `json:"name"`
			Value     string `json:"value"`
			Encrypted bool   `json:"encrypted"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := h.svc.SetSecret(r.Context(), accountID, req.Name, []byte(req.Value), req.Encrypted); err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPHandler) handleSecret(w http.ResponseWriter, r *http.Request) {
	accountID := h.getAccountID(r)
	if accountID == "" {
		h.writeError(w, http.StatusBadRequest, "X-Account-ID header required")
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/v1/secrets/")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "secret name required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		value, err := h.svc.GetSecret(r.Context(), accountID, name)
		if err != nil {
			h.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]string{"value": string(value)})
	case http.MethodDelete:
		if err := h.svc.DeleteSecret(r.Context(), accountID, name); err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// Contract handlers

func (h *HTTPHandler) handleContracts(w http.ResponseWriter, r *http.Request) {
	accountID := h.getAccountID(r)
	if accountID == "" {
		h.writeError(w, http.StatusBadRequest, "X-Account-ID header required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		contracts, err := h.svc.ListContracts(r.Context(), accountID)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, contracts)
	case http.MethodPost:
		var spec ContractSpec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		id, err := h.svc.RegisterContract(r.Context(), accountID, &spec)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPHandler) handleContract(w http.ResponseWriter, r *http.Request) {
	contractID := strings.TrimPrefix(r.URL.Path, "/api/v1/contracts/")
	if contractID == "" {
		h.writeError(w, http.StatusBadRequest, "contract_id required")
		return
	}

	// Handle pause/unpause
	if strings.HasSuffix(contractID, "/pause") {
		contractID = strings.TrimSuffix(contractID, "/pause")
		if r.Method == http.MethodPost {
			var req struct {
				Paused bool `json:"paused"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if err := h.svc.PauseContract(r.Context(), contractID, req.Paused); err != nil {
				h.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			h.writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		contract, err := h.svc.GetContract(r.Context(), contractID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, contract)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// Function handlers

func (h *HTTPHandler) handleFunctions(w http.ResponseWriter, r *http.Request) {
	accountID := h.getAccountID(r)
	if accountID == "" {
		h.writeError(w, http.StatusBadRequest, "X-Account-ID header required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		functions, err := h.svc.ListFunctions(r.Context(), accountID)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, functions)
	case http.MethodPost:
		var spec FunctionSpec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		id, err := h.svc.DeployFunction(r.Context(), accountID, &spec)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPHandler) handleFunction(w http.ResponseWriter, r *http.Request) {
	functionID := strings.TrimPrefix(r.URL.Path, "/api/v1/functions/")
	if functionID == "" {
		h.writeError(w, http.StatusBadRequest, "function_id required")
		return
	}

	// Handle enable/disable
	if strings.HasSuffix(functionID, "/enable") {
		functionID = strings.TrimSuffix(functionID, "/enable")
		if r.Method == http.MethodPost {
			var req struct {
				Enabled bool `json:"enabled"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if err := h.svc.EnableFunction(r.Context(), functionID, req.Enabled); err != nil {
				h.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			h.writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		function, err := h.svc.GetFunction(r.Context(), functionID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, function)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// Trigger handlers

func (h *HTTPHandler) handleTriggers(w http.ResponseWriter, r *http.Request) {
	functionID := r.URL.Query().Get("function_id")
	if functionID == "" {
		h.writeError(w, http.StatusBadRequest, "function_id query param required")
		return
	}

	switch r.Method {
	case http.MethodPost:
		var spec TriggerSpec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		id, err := h.svc.CreateTrigger(r.Context(), functionID, &spec)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// Balance handlers

func (h *HTTPHandler) handleBalance(w http.ResponseWriter, r *http.Request) {
	accountID := h.getAccountID(r)
	if accountID == "" {
		h.writeError(w, http.StatusBadRequest, "X-Account-ID header required")
		return
	}

	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	balance, err := h.svc.GetBalance(r.Context(), accountID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, balance)
}

func (h *HTTPHandler) handleTransactions(w http.ResponseWriter, r *http.Request) {
	accountID := h.getAccountID(r)
	if accountID == "" {
		h.writeError(w, http.StatusBadRequest, "X-Account-ID header required")
		return
	}

	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	transactions, err := h.svc.GetTransactionHistory(r.Context(), accountID, limit)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, transactions)
}

func (h *HTTPHandler) handleEstimateFee(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		ServiceType string         `json:"service_type"`
		Params      map[string]any `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fee, err := h.svc.EstimateFee(r.Context(), req.ServiceType, req.Params)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]int64{"fee": fee})
}

// Request handlers

func (h *HTTPHandler) handleRequests(w http.ResponseWriter, r *http.Request) {
	accountID := h.getAccountID(r)
	if accountID == "" {
		h.writeError(w, http.StatusBadRequest, "X-Account-ID header required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		serviceType := events.ServiceType(r.URL.Query().Get("service_type"))
		status := events.RequestStatus(r.URL.Query().Get("status"))
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		requests, err := h.svc.ListRequests(r.Context(), accountID, serviceType, status, limit)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, requests)

	case http.MethodPost:
		var req struct {
			ServiceType string         `json:"service_type"`
			Payload     map[string]any `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		request, err := h.svc.SubmitRequest(r.Context(), accountID, events.ServiceType(req.ServiceType), req.Payload)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusCreated, request)

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPHandler) handleRequest(w http.ResponseWriter, r *http.Request) {
	requestID := strings.TrimPrefix(r.URL.Path, "/api/v1/requests/")
	if requestID == "" {
		h.writeError(w, http.StatusBadRequest, "request_id required")
		return
	}

	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	request, err := h.svc.GetRequest(r.Context(), requestID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, request)
}
