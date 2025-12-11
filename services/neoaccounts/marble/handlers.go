// Package neoaccounts provides HTTP handlers for the neoaccounts service.
package neoaccountsmarble

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/R3E-Network/service_layer/internal/httputil"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

// handleInfo returns pool statistics with per-token breakdowns.
func (s *Service) handleInfo(w http.ResponseWriter, r *http.Request) {
	info, err := s.GetPoolInfo(r.Context())
	if err != nil {
		httputil.InternalError(w, "failed to get pool info")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, info)
}

// handleListAccounts returns accounts locked by a service with optional token filtering.
func (s *Service) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	serviceID := r.URL.Query().Get("service_id")
	if serviceID == "" {
		httputil.BadRequest(w, "service_id required")
		return
	}

	// Parse optional token filter
	tokenType := r.URL.Query().Get("token")

	// Parse optional min_balance filter
	var minBalance *int64
	if minBalStr := r.URL.Query().Get("min_balance"); minBalStr != "" {
		var mb int64
		if _, err := fmt.Sscanf(minBalStr, "%d", &mb); err == nil {
			minBalance = &mb
		}
	}

	accounts, err := s.ListAccountsByService(r.Context(), serviceID, tokenType, minBalance)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, ListAccountsResponse{
		Accounts: accounts,
	})
}

// handleRequestAccounts locks and returns accounts for a service.
func (s *Service) handleRequestAccounts(w http.ResponseWriter, r *http.Request) {
	var input RequestAccountsInput
	if !httputil.DecodeJSON(w, r, &input) {
		return
	}

	if input.ServiceID == "" {
		httputil.BadRequest(w, "service_id required")
		return
	}
	if input.Count <= 0 {
		input.Count = 1
	}

	accounts, lockID, err := s.RequestAccounts(r.Context(), input.ServiceID, input.Count, input.Purpose)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, RequestAccountsResponse{
		Accounts: accounts,
		LockID:   lockID,
	})
}

// handleReleaseAccounts releases previously locked accounts.
func (s *Service) handleReleaseAccounts(w http.ResponseWriter, r *http.Request) {
	var input ReleaseAccountsInput
	if !httputil.DecodeJSON(w, r, &input) {
		return
	}

	if input.ServiceID == "" {
		httputil.BadRequest(w, "service_id required")
		return
	}

	var released int
	var err error

	if len(input.AccountIDs) > 0 {
		released, err = s.ReleaseAccounts(r.Context(), input.ServiceID, input.AccountIDs)
	} else {
		released, err = s.ReleaseAllByService(r.Context(), input.ServiceID)
	}

	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, ReleaseAccountsResponse{
		ReleasedCount: released,
	})
}

// handleSignTransaction signs a transaction hash with an account's private key.
func (s *Service) handleSignTransaction(w http.ResponseWriter, r *http.Request) {
	var input SignTransactionInput
	if !httputil.DecodeJSON(w, r, &input) {
		return
	}

	if input.ServiceID == "" || input.AccountID == "" || len(input.TxHash) == 0 {
		httputil.BadRequest(w, "service_id, account_id, and tx_hash required")
		return
	}

	resp, err := s.SignTransaction(r.Context(), input.ServiceID, input.AccountID, input.TxHash)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// handleBatchSign signs multiple transactions.
func (s *Service) handleBatchSign(w http.ResponseWriter, r *http.Request) {
	var input BatchSignInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}

	if input.ServiceID == "" {
		httputil.BadRequest(w, "service_id required")
		return
	}

	resp := s.BatchSign(r.Context(), input.ServiceID, input.Requests)
	httputil.WriteJSON(w, http.StatusOK, resp)
}

// handleUpdateBalance updates an account's token balance.
func (s *Service) handleUpdateBalance(w http.ResponseWriter, r *http.Request) {
	var input UpdateBalanceInput
	if !httputil.DecodeJSON(w, r, &input) {
		return
	}

	if input.ServiceID == "" || input.AccountID == "" {
		httputil.BadRequest(w, "service_id and account_id required")
		return
	}

	// Default to GAS if no token specified
	if input.Token == "" {
		input.Token = TokenTypeGAS
	}

	oldBalance, newBalance, txCount, err := s.UpdateBalance(
		r.Context(),
		input.ServiceID,
		input.AccountID,
		input.Token,
		input.Delta,
		input.Absolute,
	)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, UpdateBalanceResponse{
		AccountID:  input.AccountID,
		Token:      input.Token,
		OldBalance: oldBalance,
		NewBalance: newBalance,
		TxCount:    txCount,
	})
}
