package httpapi

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	randomdomain "github.com/R3E-Network/service_layer/internal/app/domain/random"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	"github.com/R3E-Network/service_layer/internal/app/services/random"
)

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

	switch rest[0] {
	case "executions":
		h.accountFunctionExecutionLookup(w, r, accountID, rest[1:])
		return
	default:
		functionID := rest[0]
		if len(rest) < 2 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch rest[1] {
		case "execute":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			def, err := h.app.Functions.Get(r.Context(), functionID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			if def.AccountID != accountID {
				w.WriteHeader(http.StatusForbidden)
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
		case "executions":
			h.accountFunctionExecutions(w, r, accountID, functionID, rest[2:])
			return
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}
}

func (h *handler) accountFunctionExecutions(w http.ResponseWriter, r *http.Request, accountID, functionID string, rest []string) {
	def, err := h.app.Functions.Get(r.Context(), functionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if def.AccountID != accountID {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	switch len(rest) {
	case 0:
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		execs, err := h.app.Functions.ListExecutions(r.Context(), functionID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, execs)
	case 1:
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		exec, err := h.app.Functions.GetExecution(r.Context(), rest[0])
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		if exec.AccountID != accountID {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		writeJSON(w, http.StatusOK, exec)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *handler) accountFunctionExecutionLookup(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) != 1 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	exec, err := h.app.Functions.GetExecution(r.Context(), rest[0])
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if exec.AccountID != accountID {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	writeJSON(w, http.StatusOK, exec)
}

func (h *handler) accountTriggers(w http.ResponseWriter, r *http.Request, accountID string) {
	switch r.Method {
	case http.MethodPost:
		var payload struct {
			FunctionID string            `json:"function_id"`
			Type       string            `json:"type"`
			Rule       string            `json:"rule"`
			Config     map[string]string `json:"config"`
			Enabled    *bool             `json:"enabled"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		enabled := true
		if payload.Enabled != nil {
			enabled = *payload.Enabled
		}

		trg := trigger.Trigger{
			AccountID:  accountID,
			FunctionID: payload.FunctionID,
			Type:       trigger.Type(payload.Type),
			Rule:       payload.Rule,
			Config:     payload.Config,
			Enabled:    enabled,
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

func (h *handler) accountSecrets(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.Secrets == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("secrets service not configured"))
		return
	}

	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			items, err := h.app.Secrets.List(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			var payload struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			meta, err := h.app.Secrets.Create(r.Context(), accountID, payload.Name, payload.Value)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, meta)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	if len(rest) != 1 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	name := rest[0]
	switch r.Method {
	case http.MethodGet:
		sec, err := h.app.Secrets.Get(r.Context(), accountID, name)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, sec)
	case http.MethodPut:
		var payload struct {
			Value string `json:"value"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		meta, err := h.app.Secrets.Update(r.Context(), accountID, name, payload.Value)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, meta)
	case http.MethodDelete:
		if err := h.app.Secrets.Delete(r.Context(), accountID, name); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type randomResponse struct {
	AccountID string    `json:"AccountID"`
	Length    int       `json:"Length"`
	Value     string    `json:"Value"`
	CreatedAt time.Time `json:"CreatedAt"`
	RequestID string    `json:"RequestID"`
	Counter   uint64    `json:"Counter"`
	Signature string    `json:"Signature"`
	PublicKey string    `json:"PublicKey"`
}

func (h *handler) accountRandom(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.Random == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("random service not configured"))
		return
	}
	if len(rest) == 0 {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Length    int    `json:"length"`
			RequestID string `json:"request_id"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if payload.Length == 0 {
			payload.Length = 32
		}

		res, err := h.app.Random.Generate(r.Context(), accountID, payload.Length, payload.RequestID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, newRandomResponse(accountID, res))
		return
	}

	switch rest[0] {
	case "requests":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		results, err := h.app.Random.List(r.Context(), accountID, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		out := make([]randomResponse, len(results))
		for i, res := range results {
			out[i] = newRandomResponse(accountID, res)
		}
		writeJSON(w, http.StatusOK, out)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newRandomResponse(accountID string, res randomdomain.Result) randomResponse {
	return randomResponse{
		AccountID: accountID,
		Length:    res.Length,
		Value:     random.EncodeResult(res),
		CreatedAt: res.CreatedAt,
		RequestID: res.RequestID,
		Counter:   res.Counter,
		Signature: base64.StdEncoding.EncodeToString(res.Signature),
		PublicKey: base64.StdEncoding.EncodeToString(res.PublicKey),
	}
}
