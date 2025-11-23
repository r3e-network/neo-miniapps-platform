package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	domaindf "github.com/R3E-Network/service_layer/internal/app/domain/datafeeds"
)

func (h *handler) accountDataFeeds(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.DataFeeds == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("data feeds service not configured"))
		return
	}
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			feeds, err := h.app.DataFeeds.ListFeeds(r.Context(), accountID)
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
			feed := domaindf.Feed{
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
			created, err := h.app.DataFeeds.CreateFeed(r.Context(), feed)
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

	feedID := rest[0]
	if len(rest) == 1 {
		switch r.Method {
		case http.MethodGet:
			feed, err := h.app.DataFeeds.GetFeed(r.Context(), accountID, feedID)
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
			feed := domaindf.Feed{
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
			updated, err := h.app.DataFeeds.UpdateFeed(r.Context(), feed)
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
				updates, err := h.app.DataFeeds.ListUpdates(r.Context(), accountID, feedID, limit)
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
				created, err := h.app.DataFeeds.SubmitUpdate(r.Context(), accountID, feedID, payload.RoundID, payload.Price, payload.Timestamp, payload.Signer, payload.Signature, payload.Metadata)
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
	case "latest":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		latest, err := h.app.DataFeeds.LatestUpdate(r.Context(), accountID, feedID)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, latest)
		return
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
