package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	gasbanksvc "github.com/R3E-Network/service_layer/internal/app/services/gasbank"
)

// handler bundles HTTP endpoints for the application services.
type handler struct {
	app *app.Application
}

// NewHandler returns a mux exposing the core REST API.
func NewHandler(application *app.Application) http.Handler {
	h := &handler{app: application}
	mux := http.NewServeMux()
	mux.HandleFunc("/accounts", h.accounts)
	mux.HandleFunc("/accounts/", h.accountResources)
	return mux
}

func (h *handler) accounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var payload struct {
			Owner    string            `json:"owner"`
			Metadata map[string]string `json:"metadata"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		acct, err := h.app.Accounts.Create(r.Context(), payload.Owner, payload.Metadata)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, acct)

	case http.MethodGet:
		accts, err := h.app.Accounts.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, accts)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *handler) accountResources(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.Trim(strings.TrimPrefix(r.URL.Path, "/accounts"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	accountID := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			acct, err := h.app.Accounts.Get(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, acct)
		case http.MethodDelete:
			if err := h.app.Accounts.Delete(r.Context(), accountID); err != nil {
				status := http.StatusBadRequest
				if errors.Is(err, sql.ErrNoRows) {
					status = http.StatusNotFound
				}
				writeError(w, status, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	resource := parts[1]
	switch resource {
	case "functions":
		h.accountFunctions(w, r, accountID, parts[2:])
	case "triggers":
		h.accountTriggers(w, r, accountID)
	case "gasbank":
		h.accountGasBank(w, r, accountID, parts[2:])
	case "automation":
		h.accountAutomation(w, r, accountID, parts[2:])
	case "pricefeeds":
		h.accountPriceFeeds(w, r, accountID, parts[2:])
	case "oracle":
		h.accountOracle(w, r, accountID, parts[2:])
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *handler) accountFunctions(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodPost:
			var payload struct {
				Name        string   `json:"name"`
				Description string   `json:"description"`
				Source      string   `json:"source"`
				Secrets     []string `json:"secrets"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			def := function.Definition{
				AccountID:   accountID,
				Name:        payload.Name,
				Description: payload.Description,
				Source:      payload.Source,
				Secrets:     payload.Secrets,
			}
			created, err := h.app.Functions.Create(r.Context(), def)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, created)

		case http.MethodGet:
			funcs, err := h.app.Functions.List(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, funcs)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	functionID := rest[0]
	if len(rest) > 1 && rest[1] == "execute" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload map[string]any
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		result, err := h.app.Functions.Execute(r.Context(), functionID, payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (h *handler) accountTriggers(w http.ResponseWriter, r *http.Request, accountID string) {
	switch r.Method {
	case http.MethodPost:
		var payload struct {
			FunctionID string            `json:"function_id"`
			Type       string            `json:"type"`
			Rule       string            `json:"rule"`
			Config     map[string]string `json:"config"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		trg := trigger.Trigger{
			AccountID:  accountID,
			FunctionID: payload.FunctionID,
			Type:       trigger.Type(payload.Type),
			Rule:       payload.Rule,
			Config:     payload.Config,
		}
		created, err := h.app.Triggers.Register(r.Context(), trg)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)

	case http.MethodGet:
		triggers, err := h.app.Triggers.List(r.Context(), accountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, triggers)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

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
				WalletAddress string `json:"wallet_address"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			acct, err := h.app.GasBank.EnsureAccount(r.Context(), accountID, payload.WalletAddress)
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
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	default:
		action := rest[0]
		switch action {
		case "deposit":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
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
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var payload struct {
				GasAccountID string  `json:"gas_account_id"`
				Amount       float64 `json:"amount"`
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
			updated, tx, err := h.app.GasBank.Withdraw(r.Context(), acct.ID, payload.Amount, payload.ToAddress)
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
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			gasAcct, err := h.resolveGasAccount(r.Context(), accountID, r.URL.Query().Get("gas_account_id"))
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			txs, err := h.app.GasBank.ListTransactions(r.Context(), gasAcct.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, txs)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func (h *handler) accountAutomation(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.Automation == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("automation service not configured"))
		return
	}

	if len(rest) == 0 || rest[0] != "jobs" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch len(rest) {
	case 1:
		switch r.Method {
		case http.MethodGet:
			jobs, err := h.app.Automation.ListJobs(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, jobs)
		case http.MethodPost:
			var payload struct {
				FunctionID  string `json:"function_id"`
				Name        string `json:"name"`
				Schedule    string `json:"schedule"`
				Description string `json:"description"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			job, err := h.app.Automation.CreateJob(r.Context(), accountID, payload.FunctionID, payload.Name, payload.Schedule, payload.Description)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, job)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	case 2:
		jobID := rest[1]
		switch r.Method {
		case http.MethodGet:
			job, err := h.app.Automation.GetJob(r.Context(), jobID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			if job.AccountID != accountID {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			writeJSON(w, http.StatusOK, job)
		case http.MethodPatch:
			job, err := h.app.Automation.GetJob(r.Context(), jobID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			if job.AccountID != accountID {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			var payload struct {
				Name        *string `json:"name"`
				Schedule    *string `json:"schedule"`
				Description *string `json:"description"`
				Enabled     *bool   `json:"enabled"`
				NextRun     *string `json:"next_run"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var nextRun *time.Time
			if payload.NextRun != nil {
				trimmed := strings.TrimSpace(*payload.NextRun)
				if trimmed == "" {
					zero := time.Time{}
					nextRun = &zero
				} else {
					parsed, err := time.Parse(time.RFC3339, trimmed)
					if err != nil {
						writeError(w, http.StatusBadRequest, fmt.Errorf("next_run must be RFC3339 timestamp"))
						return
					}
					nextRun = &parsed
				}
			}

			updated := job
			if payload.Name != nil || payload.Schedule != nil || payload.Description != nil || payload.NextRun != nil {
				updated, err = h.app.Automation.UpdateJob(r.Context(), jobID, payload.Name, payload.Schedule, payload.Description, nextRun)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
			}

			if payload.Enabled != nil {
				updated, err = h.app.Automation.SetEnabled(r.Context(), updated.ID, *payload.Enabled)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
			}
			writeJSON(w, http.StatusOK, updated)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *handler) accountPriceFeeds(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.PriceFeeds == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("price feed service not configured"))
		return
	}

	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			feeds, err := h.app.PriceFeeds.ListFeeds(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, feeds)
		case http.MethodPost:
			var payload struct {
				BaseAsset         string  `json:"base_asset"`
				QuoteAsset        string  `json:"quote_asset"`
				UpdateInterval    string  `json:"update_interval"`
				HeartbeatInterval string  `json:"heartbeat_interval"`
				DeviationPercent  float64 `json:"deviation_percent"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			feed, err := h.app.PriceFeeds.CreateFeed(r.Context(), accountID, payload.BaseAsset, payload.QuoteAsset, payload.UpdateInterval, payload.HeartbeatInterval, payload.DeviationPercent)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, feed)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	feedID := rest[0]
	feed, err := h.app.PriceFeeds.GetFeed(r.Context(), feedID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if feed.AccountID != accountID {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if len(rest) == 1 {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, feed)
		case http.MethodPatch:
			var payload struct {
				UpdateInterval    *string  `json:"update_interval"`
				HeartbeatInterval *string  `json:"heartbeat_interval"`
				DeviationPercent  *float64 `json:"deviation_percent"`
				Active            *bool    `json:"active"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			updated := feed
			if payload.UpdateInterval != nil || payload.HeartbeatInterval != nil || payload.DeviationPercent != nil {
				updated, err = h.app.PriceFeeds.UpdateFeed(r.Context(), feedID, payload.UpdateInterval, payload.HeartbeatInterval, payload.DeviationPercent)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
			}
			if payload.Active != nil {
				updated, err = h.app.PriceFeeds.SetActive(r.Context(), feedID, *payload.Active)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
			}
			writeJSON(w, http.StatusOK, updated)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	if len(rest) == 2 && rest[1] == "snapshots" {
		switch r.Method {
		case http.MethodGet:
			snaps, err := h.app.PriceFeeds.ListSnapshots(r.Context(), feedID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, snaps)
		case http.MethodPost:
			var payload struct {
				Price       float64 `json:"price"`
				Source      string  `json:"source"`
				CollectedAt string  `json:"collected_at"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			var collected time.Time
			if strings.TrimSpace(payload.CollectedAt) != "" {
				parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.CollectedAt))
				if err != nil {
					writeError(w, http.StatusBadRequest, fmt.Errorf("collected_at must be RFC3339 timestamp"))
					return
				}
				collected = parsed
			}
			snap, err := h.app.PriceFeeds.RecordSnapshot(r.Context(), feedID, payload.Price, payload.Source, collected)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, snap)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (h *handler) accountOracle(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.Oracle == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("oracle service not configured"))
		return
	}

	if len(rest) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch rest[0] {
	case "sources":
		h.accountOracleSources(w, r, accountID, rest[1:])
	case "requests":
		h.accountOracleRequests(w, r, accountID, rest[1:])
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *handler) accountOracleSources(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			sources, err := h.app.Oracle.ListSources(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, sources)
		case http.MethodPost:
			var payload struct {
				Name        string            `json:"name"`
				URL         string            `json:"url"`
				Method      string            `json:"method"`
				Description string            `json:"description"`
				Headers     map[string]string `json:"headers"`
				Body        string            `json:"body"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			src, err := h.app.Oracle.CreateSource(r.Context(), accountID, payload.Name, payload.URL, payload.Method, payload.Description, payload.Headers, payload.Body)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, src)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	sourceID := rest[0]
	src, err := h.app.Oracle.GetSource(r.Context(), sourceID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if src.AccountID != accountID {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if len(rest) != 1 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, src)
	case http.MethodPatch:
		var payload struct {
			Name        *string           `json:"name"`
			URL         *string           `json:"url"`
			Method      *string           `json:"method"`
			Description *string           `json:"description"`
			Headers     map[string]string `json:"headers"`
			Body        *string           `json:"body"`
			Enabled     *bool             `json:"enabled"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		updated := src
		if payload.Name != nil || payload.URL != nil || payload.Method != nil || payload.Description != nil || payload.Headers != nil || payload.Body != nil {
			updated, err = h.app.Oracle.UpdateSource(r.Context(), sourceID, payload.Name, payload.URL, payload.Method, payload.Description, payload.Headers, payload.Body)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
		if payload.Enabled != nil {
			updated, err = h.app.Oracle.SetSourceEnabled(r.Context(), sourceID, *payload.Enabled)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *handler) accountOracleRequests(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			reqs, err := h.app.Oracle.ListRequests(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, reqs)
		case http.MethodPost:
			var payload struct {
				DataSourceID string `json:"data_source_id"`
				Payload      string `json:"payload"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			req, err := h.app.Oracle.CreateRequest(r.Context(), accountID, payload.DataSourceID, payload.Payload)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, req)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	requestID := rest[0]
	req, err := h.app.Oracle.GetRequest(r.Context(), requestID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if req.AccountID != accountID {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if len(rest) != 1 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, req)
	case http.MethodPatch:
		var payload struct {
			Status *string `json:"status"`
			Result *string `json:"result"`
			Error  *string `json:"error"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if payload.Status == nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("status is required"))
			return
		}
		status := strings.ToLower(strings.TrimSpace(*payload.Status))
		var updated oracle.Request
		switch status {
		case "running":
			updated, err = h.app.Oracle.MarkRunning(r.Context(), requestID)
		case "succeeded", "completed":
			if payload.Result == nil {
				writeError(w, http.StatusBadRequest, fmt.Errorf("result is required for succeeded status"))
				return
			}
			updated, err = h.app.Oracle.CompleteRequest(r.Context(), requestID, *payload.Result)
		case "failed":
			errMsg := ""
			if payload.Error != nil {
				errMsg = *payload.Error
			}
			updated, err = h.app.Oracle.FailRequest(r.Context(), requestID, errMsg)
		default:
			writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported status %s", status))
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
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
