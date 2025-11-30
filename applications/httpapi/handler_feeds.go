package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/packages/com.r3e.services.datafeeds"
)

func (h *handler) accountDataFeeds(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.services.DataFeedsService() == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("data feeds service not configured"))
		return
	}
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			feeds, err := h.services.DataFeedsService().ListFeeds(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, feeds)
		case http.MethodPost:
			var payload struct {
				Pair         string            `json:"pair"`
				Description  string            `json:"description"`
				Decimals     int               `json:"decimals"`
				HeartbeatSec int64             `json:"heartbeat_seconds"`
				ThresholdPPM int               `json:"threshold_ppm"`
				SignerSet    []string          `json:"signer_set"`
				Aggregation  string            `json:"aggregation"`
				Metadata     map[string]string `json:"metadata"`
				Tags         []string          `json:"tags"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			feed := datafeeds.Feed{
				AccountID:    accountID,
				Pair:         payload.Pair,
				Description:  payload.Description,
				Decimals:     payload.Decimals,
				Heartbeat:    time.Duration(payload.HeartbeatSec) * time.Second,
				ThresholdPPM: payload.ThresholdPPM,
				SignerSet:    payload.SignerSet,
				Aggregation:  payload.Aggregation,
				Metadata:     payload.Metadata,
				Tags:         payload.Tags,
			}
			created, err := h.services.DataFeedsService().CreateFeed(r.Context(), feed)
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

	feedID := rest[0]
	if len(rest) == 1 {
		switch r.Method {
		case http.MethodGet:
			feed, err := h.services.DataFeedsService().GetFeed(r.Context(), accountID, feedID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, feed)
		case http.MethodPut:
			var payload struct {
				Pair         string            `json:"pair"`
				Description  string            `json:"description"`
				Decimals     int               `json:"decimals"`
				HeartbeatSec int64             `json:"heartbeat_seconds"`
				ThresholdPPM int               `json:"threshold_ppm"`
				SignerSet    []string          `json:"signer_set"`
				Aggregation  string            `json:"aggregation"`
				Metadata     map[string]string `json:"metadata"`
				Tags         []string          `json:"tags"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			feed := datafeeds.Feed{
				ID:           feedID,
				AccountID:    accountID,
				Pair:         payload.Pair,
				Description:  payload.Description,
				Decimals:     payload.Decimals,
				Heartbeat:    time.Duration(payload.HeartbeatSec) * time.Second,
				ThresholdPPM: payload.ThresholdPPM,
				SignerSet:    payload.SignerSet,
				Aggregation:  payload.Aggregation,
				Metadata:     payload.Metadata,
				Tags:         payload.Tags,
			}
			updated, err := h.services.DataFeedsService().UpdateFeed(r.Context(), feed)
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

	switch rest[1] {
	case "updates":
		if len(rest) == 2 {
			switch r.Method {
			case http.MethodGet:
				limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				updates, err := h.services.DataFeedsService().ListUpdates(r.Context(), accountID, feedID, limit)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				writeJSON(w, http.StatusOK, updates)
			case http.MethodPost:
				var payload struct {
					RoundID   int64             `json:"round_id"`
					Price     string            `json:"price"`
					Signer    string            `json:"signer"`
					Timestamp time.Time         `json:"timestamp"`
					Signature string            `json:"signature"`
					Metadata  map[string]string `json:"metadata"`
				}
				if err := decodeJSON(r.Body, &payload); err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				created, err := h.services.DataFeedsService().SubmitUpdate(r.Context(), accountID, feedID, payload.RoundID, payload.Price, payload.Timestamp, payload.Signer, payload.Signature, payload.Metadata)
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
	case "latest":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		latest, err := h.services.DataFeedsService().LatestUpdate(r.Context(), accountID, feedID)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, latest)
		return
	}
}
