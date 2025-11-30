package httpapi

import (
	"fmt"
	"net/http"

	"github.com/R3E-Network/service_layer/packages/com.r3e.services.mixer"
)

func (h *handler) accountMixer(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.services.MixerService() == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("mixer service not configured"))
		return
	}

	if len(rest) == 0 {
		// /accounts/{id}/mixer - list requests or create new
		switch r.Method {
		case http.MethodGet:
			limit, err := parseLimitParam(r.URL.Query().Get("limit"), 50)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			requests, err := h.services.MixerService().ListMixRequests(r.Context(), accountID, limit)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, requests)

		case http.MethodPost:
			var payload struct {
				SourceWallet string              `json:"source_wallet"`
				Amount       string              `json:"amount"`
				TokenAddress string              `json:"token_address"`
				MixDuration  string              `json:"mix_duration"`
				SplitCount   int                 `json:"split_count"`
				Targets      []mixer.MixTarget   `json:"targets"`
				Metadata     map[string]string   `json:"metadata"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			req := mixer.MixRequest{
				AccountID:    accountID,
				SourceWallet: payload.SourceWallet,
				Amount:       payload.Amount,
				TokenAddress: payload.TokenAddress,
				MixDuration:  mixer.ParseMixDuration(payload.MixDuration),
				SplitCount:   payload.SplitCount,
				Targets:      payload.Targets,
				Metadata:     payload.Metadata,
			}

			created, err := h.services.MixerService().CreateMixRequest(r.Context(), req)
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

	requestID := rest[0]

	if len(rest) == 1 {
		// /accounts/{id}/mixer/{requestID}
		switch r.Method {
		case http.MethodGet:
			req, err := h.services.MixerService().GetMixRequest(r.Context(), accountID, requestID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, req)
		default:
			methodNotAllowed(w, http.MethodGet)
		}
		return
	}

	// /accounts/{id}/mixer/{requestID}/{action}
	action := rest[1]
	switch action {
	case "deposit":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		var payload struct {
			TxHashes []string `json:"tx_hashes"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		req, err := h.services.MixerService().ConfirmDeposit(r.Context(), requestID, payload.TxHashes)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, req)

	case "claim":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		var payload struct {
			ClaimAddress string `json:"claim_address"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		claim, err := h.services.MixerService().CreateWithdrawalClaim(r.Context(), requestID, payload.ClaimAddress)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, claim)

	default:
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown action: %s", action))
	}
}

// mixerStats handles GET /mixer/stats
func (h *handler) mixerStats(w http.ResponseWriter, r *http.Request) {
	if h.services.MixerService() == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("mixer service not configured"))
		return
	}

	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	stats, err := h.services.MixerService().GetMixStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
