package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/domain/gasbank"
	gasbanksvc "github.com/R3E-Network/service_layer/internal/services/gasbank"
)

func (h *handler) accountGasBank(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.GasBank == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("gas bank service not configured"))
		return
	}

	switch len(rest) {
	case 0:
		switch r.Method {
		case http.MethodGet:
			gasID := r.URL.Query().Get("gas_account_id")
			if strings.TrimSpace(gasID) != "" {
				acct, err := h.resolveGasAccount(r.Context(), accountID, gasID)
				if err != nil {
					writeError(w, http.StatusNotFound, err)
					return
				}
				writeJSON(w, http.StatusOK, acct)
				return
			}

			accts, err := h.app.GasBank.ListAccounts(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, accts)
		case http.MethodPost:
			var payload struct {
				WalletAddress         string   `json:"wallet_address"`
				MinBalance            *float64 `json:"min_balance"`
				DailyLimit            *float64 `json:"daily_limit"`
				NotificationThreshold *float64 `json:"notification_threshold"`
				RequiredApprovals     *int     `json:"required_approvals"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			acct, err := h.app.GasBank.EnsureAccountWithOptions(r.Context(), accountID, gasbanksvc.EnsureAccountOptions{
				WalletAddress:         payload.WalletAddress,
				MinBalance:            payload.MinBalance,
				DailyLimit:            payload.DailyLimit,
				NotificationThreshold: payload.NotificationThreshold,
				RequiredApprovals:     payload.RequiredApprovals,
			})
			if err != nil {
				status := http.StatusBadRequest
				if errors.Is(err, gasbanksvc.ErrWalletInUse) {
					status = http.StatusConflict
				}
				writeError(w, status, err)
				return
			}
			writeJSON(w, http.StatusOK, acct)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
	default:
		action := rest[0]
		if action == "summary" {
			if r.Method != http.MethodGet {
				methodNotAllowed(w, http.MethodGet)
				return
			}
			summary, err := h.app.GasBank.Summary(r.Context(), accountID)
			if err != nil {
				status := http.StatusInternalServerError
				if strings.Contains(err.Error(), "account_id") {
					status = http.StatusBadRequest
				}
				writeError(w, status, err)
				return
			}
			writeJSON(w, http.StatusOK, summary)
			return
		}
		if action == "approvals" {
			if len(rest) < 2 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			txID := rest[1]
			switch r.Method {
			case http.MethodGet:
				approvals, err := h.app.GasBank.ListApprovals(r.Context(), txID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err)
					return
				}
				writeJSON(w, http.StatusOK, approvals)
			case http.MethodPost:
				var payload struct {
					Approver  string `json:"approver"`
					Approve   bool   `json:"approve"`
					Signature string `json:"signature"`
					Note      string `json:"note"`
				}
				if err := decodeJSON(r.Body, &payload); err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				if strings.TrimSpace(payload.Approver) == "" {
					writeError(w, http.StatusBadRequest, fmt.Errorf("approver is required"))
					return
				}
				recorded, tx, err := h.app.GasBank.SubmitApproval(r.Context(), txID, payload.Approver, payload.Signature, payload.Note, payload.Approve)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				writeJSON(w, http.StatusOK, struct {
					Approval    gasbank.WithdrawalApproval `json:"approval"`
					Transaction gasbank.Transaction        `json:"transaction"`
				}{
					Approval:    recorded,
					Transaction: tx,
				})
			default:
				methodNotAllowed(w, http.MethodGet, http.MethodPost)
			}
			return
		}

		switch action {
		case "deposit":
			if r.Method != http.MethodPost {
				methodNotAllowed(w, http.MethodPost)
				return
			}
			var payload struct {
				GasAccountID string  `json:"gas_account_id"`
				Amount       float64 `json:"amount"`
				TxID         string  `json:"tx_id"`
				FromAddress  string  `json:"from_address"`
				ToAddress    string  `json:"to_address"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if strings.TrimSpace(payload.GasAccountID) == "" {
				writeError(w, http.StatusBadRequest, fmt.Errorf("gas_account_id is required"))
				return
			}
			if payload.Amount <= 0 {
				writeError(w, http.StatusBadRequest, fmt.Errorf("amount must be positive"))
				return
			}
			acct, err := h.resolveGasAccount(r.Context(), accountID, payload.GasAccountID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			updated, tx, err := h.app.GasBank.Deposit(r.Context(), acct.ID, payload.Amount, payload.TxID, payload.FromAddress, payload.ToAddress)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, struct {
				Account     gasbank.Account     `json:"account"`
				Transaction gasbank.Transaction `json:"transaction"`
			}{
				Account:     updated,
				Transaction: tx,
			})
		case "withdraw":
			if r.Method != http.MethodPost {
				methodNotAllowed(w, http.MethodPost)
				return
			}
			var payload struct {
				GasAccountID string  `json:"gas_account_id"`
				Amount       float64 `json:"amount"`
				ToAddress    string  `json:"to_address"`
				ScheduleAt   string  `json:"schedule_at"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if strings.TrimSpace(payload.GasAccountID) == "" {
				writeError(w, http.StatusBadRequest, fmt.Errorf("gas_account_id is required"))
				return
			}
			if payload.Amount <= 0 {
				writeError(w, http.StatusBadRequest, fmt.Errorf("amount must be positive"))
				return
			}
			acct, err := h.resolveGasAccount(r.Context(), accountID, payload.GasAccountID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			var schedulePtr *time.Time
			if strings.TrimSpace(payload.ScheduleAt) != "" {
				t, err := time.Parse(time.RFC3339, payload.ScheduleAt)
				if err != nil {
					writeError(w, http.StatusBadRequest, fmt.Errorf("invalid schedule_at: %w", err))
					return
				}
				schedulePtr = &t
			}
			updated, tx, err := h.app.GasBank.WithdrawWithOptions(r.Context(), accountID, acct.ID, gasbanksvc.WithdrawOptions{
				Amount:     payload.Amount,
				ToAddress:  payload.ToAddress,
				ScheduleAt: schedulePtr,
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, struct {
				Account     gasbank.Account     `json:"account"`
				Transaction gasbank.Transaction `json:"transaction"`
			}{
				Account:     updated,
				Transaction: tx,
			})
		case "transactions":
			if r.Method != http.MethodGet {
				methodNotAllowed(w, http.MethodGet)
				return
			}
			gasAcct, err := h.resolveGasAccount(r.Context(), accountID, r.URL.Query().Get("gas_account_id"))
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			txType := strings.TrimSpace(r.URL.Query().Get("type"))
			status := strings.TrimSpace(r.URL.Query().Get("status"))
			txs, err := h.app.GasBank.ListTransactionsFiltered(r.Context(), gasAcct.ID, txType, status, limit)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, txs)
		case "deposits":
			if r.Method != http.MethodGet {
				methodNotAllowed(w, http.MethodGet)
				return
			}
			gasAcct, err := h.resolveGasAccount(r.Context(), accountID, r.URL.Query().Get("gas_account_id"))
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			txs, err := h.app.GasBank.ListTransactionsFiltered(r.Context(), gasAcct.ID, gasbank.TransactionDeposit, "", limit)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, txs)
		case "withdrawals":
			if len(rest) == 1 {
				if r.Method != http.MethodGet {
					methodNotAllowed(w, http.MethodGet)
					return
				}
				gasAcctID := strings.TrimSpace(r.URL.Query().Get("gas_account_id"))
				if gasAcctID == "" {
					writeError(w, http.StatusBadRequest, fmt.Errorf("gas_account_id is required"))
					return
				}
				gasAcct, err := h.resolveGasAccount(r.Context(), accountID, gasAcctID)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				status := strings.TrimSpace(r.URL.Query().Get("status"))
				txs, err := h.app.GasBank.ListTransactionsFiltered(r.Context(), gasAcct.ID, gasbank.TransactionWithdrawal, status, limit)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err)
					return
				}
				writeJSON(w, http.StatusOK, txs)
				return
			}
			txID := rest[1]
			if len(rest) >= 3 && rest[2] == "attempts" {
				if r.Method != http.MethodGet {
					methodNotAllowed(w, http.MethodGet)
					return
				}
				limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				attempts, err := h.app.GasBank.ListSettlementAttempts(r.Context(), accountID, txID, limit)
				if err != nil {
					status := http.StatusBadRequest
					if strings.Contains(strings.ToLower(err.Error()), "not owned") || strings.Contains(strings.ToLower(err.Error()), "not found") {
						status = http.StatusNotFound
					}
					writeError(w, status, err)
					return
				}
				writeJSON(w, http.StatusOK, attempts)
				return
			}
			switch r.Method {
			case http.MethodGet:
				tx, err := h.app.GasBank.GetWithdrawal(r.Context(), accountID, txID)
				if err != nil {
					status := http.StatusBadRequest
					if strings.Contains(strings.ToLower(err.Error()), "not owned") || strings.Contains(strings.ToLower(err.Error()), "not a withdrawal") {
						status = http.StatusNotFound
					}
					writeError(w, status, err)
					return
				}
				writeJSON(w, http.StatusOK, tx)
			case http.MethodPatch:
				var payload struct {
					Action string `json:"action"`
					Reason string `json:"reason"`
				}
				if err := decodeJSON(r.Body, &payload); err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				switch strings.ToLower(strings.TrimSpace(payload.Action)) {
				case "cancel":
					tx, err := h.app.GasBank.CancelWithdrawal(r.Context(), accountID, txID, strings.TrimSpace(payload.Reason))
					if err != nil {
						writeError(w, http.StatusBadRequest, err)
						return
					}
					writeJSON(w, http.StatusOK, tx)
				default:
					writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported action %q", payload.Action))
				}
			default:
				methodNotAllowed(w, http.MethodGet, http.MethodPatch)
			}
		case "withdrawals_attempts":
			w.WriteHeader(http.StatusNotFound)
		case "deadletters":
			if len(rest) == 1 {
				if r.Method != http.MethodGet {
					methodNotAllowed(w, http.MethodGet)
					return
				}
				limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				items, err := h.app.GasBank.ListDeadLetters(r.Context(), accountID, limit)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err)
					return
				}
				writeJSON(w, http.StatusOK, items)
				return
			}
			txID := rest[1]
			if len(rest) >= 3 && rest[2] == "retry" {
				if r.Method != http.MethodPost {
					methodNotAllowed(w, http.MethodPost)
					return
				}
				tx, err := h.app.GasBank.RetryDeadLetter(r.Context(), accountID, txID)
				if err != nil {
					status := http.StatusBadRequest
					if strings.Contains(strings.ToLower(err.Error()), "not found") {
						status = http.StatusNotFound
					}
					writeError(w, status, err)
					return
				}
				writeJSON(w, http.StatusOK, tx)
				return
			}
			if r.Method != http.MethodDelete {
				methodNotAllowed(w, http.MethodDelete)
				return
			}
			if err := h.app.GasBank.DeleteDeadLetter(r.Context(), accountID, txID); err != nil {
				status := http.StatusBadRequest
				if strings.Contains(strings.ToLower(err.Error()), "not found") {
					status = http.StatusNotFound
				}
				writeError(w, status, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func (h *handler) resolveGasAccount(ctx context.Context, accountID string, gasAccountID string) (gasbank.Account, error) {
	if strings.TrimSpace(gasAccountID) == "" {
		return gasbank.Account{}, fmt.Errorf("gas_account_id is required")
	}

	acct, err := h.app.GasBank.GetAccount(ctx, gasAccountID)
	if err != nil {
		return gasbank.Account{}, err
	}
	if acct.AccountID != accountID {
		return gasbank.Account{}, fmt.Errorf("gas account %s not owned by %s", gasAccountID, accountID)
	}
	return acct, nil
}
