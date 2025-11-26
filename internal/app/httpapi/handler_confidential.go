package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	domainconf "github.com/R3E-Network/service_layer/internal/domain/confidential"
)

func (h *handler) accountConfCompute(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.Confidential == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("confidential service not configured"))
		return
	}
	if len(rest) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch rest[0] {
	case "enclaves":
		h.accountConfEnclaves(w, r, accountID, rest[1:])
	case "sealed_keys":
		h.accountConfSealedKeys(w, r, accountID, rest[1:])
	case "attestations":
		h.accountConfAttestations(w, r, accountID, rest[1:])
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *handler) accountConfEnclaves(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			enclaves, err := h.app.Confidential.ListEnclaves(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, enclaves)
		case http.MethodPost:
			var payload struct {
				Name        string            `json:"name"`
				Endpoint    string            `json:"endpoint"`
				Provider    string            `json:"provider"`
				Attestation string            `json:"attestation"`
				Measurement string            `json:"measurement"`
				Status      string            `json:"status"`
				Metadata    map[string]string `json:"metadata"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			meta := cloneStringMap(payload.Metadata)
			if payload.Provider != "" {
				if meta == nil {
					meta = map[string]string{}
				}
				meta["provider"] = payload.Provider
			}
			if payload.Measurement != "" {
				if meta == nil {
					meta = map[string]string{}
				}
				meta["measurement"] = payload.Measurement
			}
			enclave := domainconf.Enclave{
				AccountID:   accountID,
				Name:        payload.Name,
				Endpoint:    payload.Endpoint,
				Attestation: payload.Attestation,
				Status:      domainconf.EnclaveStatus(payload.Status),
				Metadata:    meta,
			}
			created, err := h.app.Confidential.CreateEnclave(r.Context(), enclave)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, created)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	enclaveID := rest[0]
	if len(rest) == 1 {
		switch r.Method {
		case http.MethodGet:
			enclave, err := h.app.Confidential.GetEnclave(r.Context(), accountID, enclaveID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, enclave)
		case http.MethodPut:
			var payload struct {
				Name        string            `json:"name"`
				Endpoint    string            `json:"endpoint"`
				Provider    string            `json:"provider"`
				Attestation string            `json:"attestation"`
				Measurement string            `json:"measurement"`
				Status      string            `json:"status"`
				Metadata    map[string]string `json:"metadata"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			meta := cloneStringMap(payload.Metadata)
			if payload.Provider != "" {
				if meta == nil {
					meta = map[string]string{}
				}
				meta["provider"] = payload.Provider
			}
			if payload.Measurement != "" {
				if meta == nil {
					meta = map[string]string{}
				}
				meta["measurement"] = payload.Measurement
			}
			enclave := domainconf.Enclave{
				ID:          enclaveID,
				AccountID:   accountID,
				Name:        payload.Name,
				Endpoint:    payload.Endpoint,
				Attestation: payload.Attestation,
				Status:      domainconf.EnclaveStatus(payload.Status),
				Metadata:    meta,
			}
			updated, err := h.app.Confidential.UpdateEnclave(r.Context(), enclave)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	if rest[1] == "sealed_keys" {
		switch r.Method {
		case http.MethodGet:
			limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			keys, err := h.app.Confidential.ListSealedKeys(r.Context(), accountID, enclaveID, limit)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, keys)
		case http.MethodPost:
			var payload struct {
				Name     string            `json:"name"`
				Blob     []byte            `json:"blob"`
				Metadata map[string]string `json:"metadata"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			key := domainconf.SealedKey{
				AccountID: accountID,
				EnclaveID: enclaveID,
				Name:      payload.Name,
				Blob:      payload.Blob,
				Metadata:  payload.Metadata,
			}
			created, err := h.app.Confidential.CreateSealedKey(r.Context(), key)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, created)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (h *handler) accountConfSealedKeys(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) != 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodPost:
		var payload struct {
			EnclaveID string            `json:"enclave_id"`
			Name      string            `json:"name"`
			Blob      []byte            `json:"blob"`
			Metadata  map[string]string `json:"metadata"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		key := domainconf.SealedKey{
			AccountID: accountID,
			EnclaveID: payload.EnclaveID,
			Name:      payload.Name,
			Blob:      payload.Blob,
			Metadata:  payload.Metadata,
		}
		created, err := h.app.Confidential.CreateSealedKey(r.Context(), key)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	case http.MethodGet:
		enclaveID := strings.TrimSpace(r.URL.Query().Get("enclave_id"))
		if enclaveID == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("enclave_id is required"))
			return
		}
		limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		keys, err := h.app.Confidential.ListSealedKeys(r.Context(), accountID, enclaveID, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, keys)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *handler) accountConfAttestations(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) != 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodPost:
		var payload struct {
			EnclaveID string            `json:"enclave_id"`
			Report    string            `json:"report"`
			Status    string            `json:"status"`
			Metadata  map[string]string `json:"metadata"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		att := domainconf.Attestation{
			AccountID: accountID,
			EnclaveID: payload.EnclaveID,
			Report:    payload.Report,
			Status:    payload.Status,
			Metadata:  payload.Metadata,
		}
		created, err := h.app.Confidential.CreateAttestation(r.Context(), att)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	case http.MethodGet:
		enclaveID := strings.TrimSpace(r.URL.Query().Get("enclave_id"))
		limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		var result []domainconf.Attestation
		if enclaveID == "" {
			result, err = h.app.Confidential.ListAccountAttestations(r.Context(), accountID, limit)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		} else {
			result, err = h.app.Confidential.ListAttestations(r.Context(), accountID, enclaveID, limit)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
		writeJSON(w, http.StatusOK, result)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
