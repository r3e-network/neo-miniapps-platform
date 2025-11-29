package httpapi

import (
	"fmt"
	"net/http"

	"github.com/R3E-Network/service_layer/domain/function"
	"github.com/R3E-Network/service_layer/domain/secret"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.secrets"
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
			methodNotAllowed(w, http.MethodPost, http.MethodGet)
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
				methodNotAllowed(w, http.MethodPost)
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
			methodNotAllowed(w, http.MethodGet)
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
			methodNotAllowed(w, http.MethodGet)
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
		methodNotAllowed(w, http.MethodGet)
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
			// Enhanced payload with ACL support
			// Aligned with SecretsVault.cs contract ACL model
			var payload struct {
				Name  string `json:"name"`
				Value string `json:"value"`
				ACL   *uint8 `json:"acl,omitempty"` // Optional ACL flags
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			opts := secrets.CreateOptions{}
			if payload.ACL != nil {
				acl := secret.ACL(*payload.ACL)
				opts.ACL = acl
			}
			meta, err := h.app.Secrets.CreateWithOptions(r.Context(), accountID, payload.Name, payload.Value, opts)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, meta)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
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
		// Enhanced payload with ACL support
		// Aligned with SecretsVault.cs contract ACL model
		var payload struct {
			Value *string `json:"value,omitempty"` // Optional - only update if provided
			ACL   *uint8  `json:"acl,omitempty"`   // Optional ACL flags
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		opts := secrets.UpdateOptions{}
		if payload.Value != nil {
			opts.Value = payload.Value
		}
		if payload.ACL != nil {
			acl := secret.ACL(*payload.ACL)
			opts.ACL = &acl
		}
		// Require at least one field to update
		if opts.Value == nil && opts.ACL == nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("at least one of 'value' or 'acl' is required"))
			return
		}
		meta, err := h.app.Secrets.UpdateWithOptions(r.Context(), accountID, name, opts)
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
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
	}
}
