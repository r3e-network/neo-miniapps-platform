package httpapi

import (
	"fmt"
	"net/http"

	vrfsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.vrf"
)

func (h *handler) accountVRF(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.services.VRFService() == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("vrf service not configured"))
		return
	}
	if len(rest) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch rest[0] {
	case "keys":
		h.accountVRFKeys(w, r, accountID, rest[1:])
	case "requests":
		h.accountVRFRequests(w, r, accountID, rest[1:])
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *handler) accountVRFKeys(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			keys, err := h.services.VRFService().ListKeys(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, keys)
		case http.MethodPost:
			var payload struct {
				PublicKey     string            `json:"public_key"`
				Label         string            `json:"label"`
				Status        string            `json:"status"`
				WalletAddress string            `json:"wallet_address"`
				Attestation   string            `json:"attestation"`
				Metadata      map[string]string `json:"metadata"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			key := vrfsvc.Key{
				AccountID:     accountID,
				PublicKey:     payload.PublicKey,
				Label:         payload.Label,
				Status:        vrfsvc.KeyStatus(payload.Status),
				WalletAddress: payload.WalletAddress,
				Attestation:   payload.Attestation,
				Metadata:      payload.Metadata,
			}
			created, err := h.services.VRFService().CreateKey(r.Context(), key)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, created)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
		return
	}

	keyID := rest[0]
	if len(rest) == 1 {
		switch r.Method {
		case http.MethodGet:
			key, err := h.services.VRFService().GetKey(r.Context(), accountID, keyID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, key)
		case http.MethodPut:
			var payload struct {
				PublicKey     string            `json:"public_key"`
				Label         string            `json:"label"`
				Status        string            `json:"status"`
				WalletAddress string            `json:"wallet_address"`
				Attestation   string            `json:"attestation"`
				Metadata      map[string]string `json:"metadata"`
			}
			existing, err := h.services.VRFService().GetKey(r.Context(), accountID, keyID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			key := vrfsvc.Key{
				ID:            keyID,
				AccountID:     existing.AccountID,
				PublicKey:     payload.PublicKey,
				Label:         payload.Label,
				Status:        vrfsvc.KeyStatus(payload.Status),
				WalletAddress: payload.WalletAddress,
				Attestation:   payload.Attestation,
				Metadata:      payload.Metadata,
			}
			updated, err := h.services.VRFService().UpdateKey(r.Context(), accountID, key)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPut)
		}
		return
	}

	if rest[1] == "requests" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		var payload struct {
			Consumer string            `json:"consumer"`
			Seed     string            `json:"seed"`
			Metadata map[string]string `json:"metadata"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if _, err := h.services.VRFService().GetKey(r.Context(), accountID, keyID); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		req, err := h.services.VRFService().CreateRequest(r.Context(), accountID, keyID, payload.Consumer, payload.Seed, payload.Metadata)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, req)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (h *handler) accountVRFRequests(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
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
		reqs, err := h.services.VRFService().ListRequests(r.Context(), accountID, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, reqs)
	default:
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		requestID := rest[0]
		req, err := h.services.VRFService().GetRequest(r.Context(), accountID, requestID)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, req)
	}
}
