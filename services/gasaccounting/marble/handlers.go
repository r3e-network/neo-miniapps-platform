// Package gasaccounting provides GAS ledger and accounting service.
package gasaccounting

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/R3E-Network/service_layer/internal/httputil"
)

// =============================================================================
// Route Registration
// =============================================================================

// RegisterRoutes registers GasAccounting HTTP routes.
func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	// Balance operations
	mux.HandleFunc("/balance", s.handleBalance)
	mux.HandleFunc("/deposit", s.handleDeposit)
	mux.HandleFunc("/consume", s.handleConsume)

	// Reservation operations
	mux.HandleFunc("/reserve", s.handleReserve)
	mux.HandleFunc("/release", s.handleRelease)

	// History
	mux.HandleFunc("/history", s.handleHistory)
}

// =============================================================================
// Balance Handlers
// =============================================================================

// handleBalance returns a user's balance.
func (s *Service) handleBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		httputil.WriteError(w, http.StatusBadRequest, "user_id required")
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	resp, err := s.GetBalance(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// handleDeposit records a GAS deposit.
func (s *Service) handleDeposit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.Deposit(r.Context(), &req)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// handleConsume deducts GAS for a service operation.
func (s *Service) handleConsume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ConsumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.Consume(r.Context(), &req)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// =============================================================================
// Reservation Handlers
// =============================================================================

// handleReserve reserves GAS for a pending operation.
func (s *Service) handleReserve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ReserveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.Reserve(r.Context(), &req)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// handleRelease releases or consumes a reservation.
func (s *Service) handleRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ReleaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.Release(r.Context(), &req)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// =============================================================================
// History Handler
// =============================================================================

// handleHistory returns ledger history for a user.
func (s *Service) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		httputil.WriteError(w, http.StatusBadRequest, "user_id required")
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	// Parse optional filters
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	var entryType *EntryType
	if t := r.URL.Query().Get("type"); t != "" {
		et := EntryType(t)
		entryType = &et
	}

	resp, err := s.GetHistory(r.Context(), &LedgerHistoryRequest{
		UserID:    userID,
		EntryType: entryType,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}
