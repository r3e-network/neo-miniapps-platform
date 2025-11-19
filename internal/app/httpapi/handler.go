package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/metrics"
)

// handler bundles HTTP endpoints for the application services.
type handler struct {
	app *app.Application
}

// NewHandler returns a mux exposing the core REST API.
func NewHandler(application *app.Application) http.Handler {
	h := &handler{app: application}
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/healthz", h.health)
	mux.HandleFunc("/system/descriptors", h.systemDescriptors)
	mux.HandleFunc("/system/descriptors.html", h.systemDescriptorsHTML)
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
	case "datafeeds":
		h.accountDataFeeds(w, r, accountID, parts[2:])
	case "oracle":
		h.accountOracle(w, r, accountID, parts[2:])
	case "secrets":
		h.accountSecrets(w, r, accountID, parts[2:])
	case "random":
		h.accountRandom(w, r, accountID, parts[2:])
	case "cre":
		h.accountCRE(w, r, accountID, parts[2:])
	case "ccip":
		h.accountCCIP(w, r, accountID, parts[2:])
	case "vrf":
		h.accountVRF(w, r, accountID, parts[2:])
	case "datastreams":
		h.accountDataStreams(w, r, accountID, parts[2:])
	case "datalink":
		h.accountDataLink(w, r, accountID, parts[2:])
	case "dta":
		h.accountDTA(w, r, accountID, parts[2:])
	case "confcompute":
		h.accountConfCompute(w, r, accountID, parts[2:])
	case "workspace-wallets":
		h.accountWorkspaceWallets(w, r, accountID, parts[2:])
	default:
		w.WriteHeader(http.StatusNotFound)
	}
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

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
