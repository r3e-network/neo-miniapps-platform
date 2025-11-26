package httpapi

import (
	"fmt"
	"net/http"

	domainds "github.com/R3E-Network/service_layer/internal/domain/datastreams"
)

func (h *handler) accountDataStreams(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.DataStreams == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("data streams service not configured"))
		return
	}
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			streams, err := h.app.DataStreams.ListStreams(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, streams)
		case http.MethodPost:
			var payload struct {
				Name        string            `json:"name"`
				Symbol      string            `json:"symbol"`
				Description string            `json:"description"`
				Frequency   string            `json:"frequency"`
				SLAMs       int               `json:"sla_ms"`
				Status      string            `json:"status"`
				Metadata    map[string]string `json:"metadata"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			stream := domainds.Stream{
				AccountID:   accountID,
				Name:        payload.Name,
				Symbol:      payload.Symbol,
				Description: payload.Description,
				Frequency:   payload.Frequency,
				SLAms:       payload.SLAMs,
				Status:      domainds.StreamStatus(payload.Status),
				Metadata:    payload.Metadata,
			}
			created, err := h.app.DataStreams.CreateStream(r.Context(), stream)
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

	streamID := rest[0]
	if len(rest) == 1 {
		switch r.Method {
		case http.MethodGet:
			stream, err := h.app.DataStreams.GetStream(r.Context(), accountID, streamID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, stream)
		case http.MethodPut:
			var payload struct {
				Name        string            `json:"name"`
				Symbol      string            `json:"symbol"`
				Description string            `json:"description"`
				Frequency   string            `json:"frequency"`
				SLAMs       int               `json:"sla_ms"`
				Status      string            `json:"status"`
				Metadata    map[string]string `json:"metadata"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			stream := domainds.Stream{
				ID:          streamID,
				AccountID:   accountID,
				Name:        payload.Name,
				Symbol:      payload.Symbol,
				Description: payload.Description,
				Frequency:   payload.Frequency,
				SLAms:       payload.SLAMs,
				Status:      domainds.StreamStatus(payload.Status),
				Metadata:    payload.Metadata,
			}
			updated, err := h.app.DataStreams.UpdateStream(r.Context(), stream)
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

	if rest[1] == "frames" {
		switch r.Method {
		case http.MethodGet:
			limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			frames, err := h.app.DataStreams.ListFrames(r.Context(), accountID, streamID, limit)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, frames)
		case http.MethodPost:
			var payload struct {
				Sequence  int64             `json:"sequence"`
				Payload   map[string]any    `json:"payload"`
				LatencyMS int               `json:"latency_ms"`
				Status    string            `json:"status"`
				Metadata  map[string]string `json:"metadata"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			frame, err := h.app.DataStreams.CreateFrame(r.Context(), accountID, streamID, payload.Sequence, payload.Payload, payload.LatencyMS, domainds.FrameStatus(payload.Status), payload.Metadata)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, frame)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	w.WriteHeader(http.StatusNotFound)
}
