package httpapi

import (
	"fmt"
	"net/http"

	"github.com/R3E-Network/service_layer/packages/com.r3e.services.cre"
)

func (h *handler) accountCRE(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.services.CREService() == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("cre service not configured"))
		return
	}
	if len(rest) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch rest[0] {
	case "playbooks":
		h.accountCREPlaybooks(w, r, accountID, rest[1:])
	case "runs":
		h.accountCRERuns(w, r, accountID, rest[1:])
	case "executors":
		h.accountCREExecutors(w, r, accountID, rest[1:])
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *handler) accountCREPlaybooks(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	switch len(rest) {
	case 0:
		switch r.Method {
		case http.MethodPost:
			var payload struct {
				Name        string            `json:"name"`
				Description string            `json:"description"`
				Tags        []string          `json:"tags"`
				Metadata    map[string]string `json:"metadata"`
				Steps       []cre.Step  `json:"steps"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			pb := cre.Playbook{
				AccountID:   accountID,
				Name:        payload.Name,
				Description: payload.Description,
				Tags:        payload.Tags,
				Metadata:    payload.Metadata,
				Steps:       payload.Steps,
			}
			created, err := h.services.CREService().CreatePlaybook(r.Context(), pb)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, created)
		case http.MethodGet:
			playbooks, err := h.services.CREService().ListPlaybooks(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, playbooks)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	default:
		playbookID := rest[0]
		if len(rest) > 1 && rest[1] == "runs" {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var payload struct {
				ExecutorID string         `json:"executor_id"`
				Params     map[string]any `json:"params"`
				Tags       []string       `json:"tags"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			run, err := h.services.CREService().CreateRun(r.Context(), accountID, playbookID, payload.Params, payload.Tags, payload.ExecutorID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, run)
			return
		}
		switch r.Method {
		case http.MethodGet:
			pb, err := h.services.CREService().GetPlaybook(r.Context(), accountID, playbookID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, pb)
		case http.MethodPut:
			var payload struct {
				Name        string            `json:"name"`
				Description string            `json:"description"`
				Tags        []string          `json:"tags"`
				Metadata    map[string]string `json:"metadata"`
				Steps       []cre.Step  `json:"steps"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			pb := cre.Playbook{
				ID:          playbookID,
				AccountID:   accountID,
				Name:        payload.Name,
				Description: payload.Description,
				Tags:        payload.Tags,
				Metadata:    payload.Metadata,
				Steps:       payload.Steps,
			}
			updated, err := h.services.CREService().UpdatePlaybook(r.Context(), pb)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func (h *handler) accountCRERuns(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	switch len(rest) {
	case 0:
		switch r.Method {
		case http.MethodGet:
			limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			runs, err := h.services.CREService().ListRuns(r.Context(), accountID, limit)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, runs)
		case http.MethodPost:
			var payload struct {
				PlaybookID string         `json:"playbook_id"`
				ExecutorID string         `json:"executor_id"`
				Params     map[string]any `json:"params"`
				Tags       []string       `json:"tags"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			run, err := h.services.CREService().CreateRun(r.Context(), accountID, payload.PlaybookID, payload.Params, payload.Tags, payload.ExecutorID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, run)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	default:
		runID := rest[0]
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		run, err := h.services.CREService().GetRun(r.Context(), accountID, runID)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, run)
	}
}

func (h *handler) accountCREExecutors(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	switch len(rest) {
	case 0:
		switch r.Method {
		case http.MethodGet:
			execs, err := h.services.CREService().ListExecutors(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, execs)
		case http.MethodPost:
			var payload struct {
				Name     string            `json:"name"`
				Type     string            `json:"type"`
				Endpoint string            `json:"endpoint"`
				Metadata map[string]string `json:"metadata"`
				Tags     []string          `json:"tags"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			exec := cre.Executor{
				AccountID: accountID,
				Name:      payload.Name,
				Type:      payload.Type,
				Endpoint:  payload.Endpoint,
				Metadata:  payload.Metadata,
				Tags:      payload.Tags,
			}
			created, err := h.services.CREService().CreateExecutor(r.Context(), exec)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, created)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	default:
		execID := rest[0]
		switch r.Method {
		case http.MethodGet:
			exec, err := h.services.CREService().GetExecutor(r.Context(), accountID, execID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, exec)
		case http.MethodPut:
			var payload struct {
				Name     string            `json:"name"`
				Type     string            `json:"type"`
				Endpoint string            `json:"endpoint"`
				Metadata map[string]string `json:"metadata"`
				Tags     []string          `json:"tags"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			exec := cre.Executor{
				ID:        execID,
				AccountID: accountID,
				Name:      payload.Name,
				Type:      payload.Type,
				Endpoint:  payload.Endpoint,
				Metadata:  payload.Metadata,
				Tags:      payload.Tags,
			}
			updated, err := h.services.CREService().UpdateExecutor(r.Context(), exec)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}
