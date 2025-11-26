package httpapi

import (
	"fmt"
	"net/http"

	domaincre "github.com/R3E-Network/service_layer/internal/domain/cre"
)

func (h *handler) accountCRE(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.CRE == nil {
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
				Steps       []domaincre.Step  `json:"steps"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			pb := domaincre.Playbook{
				AccountID:   accountID,
				Name:        payload.Name,
				Description: payload.Description,
				Tags:        payload.Tags,
				Metadata:    payload.Metadata,
				Steps:       payload.Steps,
			}
			created, err := h.app.CRE.CreatePlaybook(r.Context(), pb)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, created)
		case http.MethodGet:
			playbooks, err := h.app.CRE.ListPlaybooks(r.Context(), accountID)
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
			run, err := h.app.CRE.CreateRun(r.Context(), accountID, playbookID, payload.Params, payload.Tags, payload.ExecutorID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, run)
			return
		}
		switch r.Method {
		case http.MethodGet:
			pb, err := h.app.CRE.GetPlaybook(r.Context(), accountID, playbookID)
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
				Steps       []domaincre.Step  `json:"steps"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			pb := domaincre.Playbook{
				ID:          playbookID,
				AccountID:   accountID,
				Name:        payload.Name,
				Description: payload.Description,
				Tags:        payload.Tags,
				Metadata:    payload.Metadata,
				Steps:       payload.Steps,
			}
			updated, err := h.app.CRE.UpdatePlaybook(r.Context(), pb)
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
			runs, err := h.app.CRE.ListRuns(r.Context(), accountID, limit)
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
			run, err := h.app.CRE.CreateRun(r.Context(), accountID, payload.PlaybookID, payload.Params, payload.Tags, payload.ExecutorID)
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
		run, err := h.app.CRE.GetRun(r.Context(), accountID, runID)
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
			execs, err := h.app.CRE.ListExecutors(r.Context(), accountID)
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
			exec := domaincre.Executor{
				AccountID: accountID,
				Name:      payload.Name,
				Type:      payload.Type,
				Endpoint:  payload.Endpoint,
				Metadata:  payload.Metadata,
				Tags:      payload.Tags,
			}
			created, err := h.app.CRE.CreateExecutor(r.Context(), exec)
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
			exec, err := h.app.CRE.GetExecutor(r.Context(), accountID, execID)
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
			exec := domaincre.Executor{
				ID:        execID,
				AccountID: accountID,
				Name:      payload.Name,
				Type:      payload.Type,
				Endpoint:  payload.Endpoint,
				Metadata:  payload.Metadata,
				Tags:      payload.Tags,
			}
			updated, err := h.app.CRE.UpdateExecutor(r.Context(), exec)
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
