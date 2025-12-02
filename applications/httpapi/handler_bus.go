package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

const defaultBusMaxBytes = int64(1 << 20) // 1 MiB

// systemEvents publishes an event to all EventEngines via the core engine fan-out.
func (h *handler) systemEvents(w http.ResponseWriter, r *http.Request) {
	if h.busPub == nil {
		http.Error(w, "event bus not available", http.StatusNotImplemented)
		return
	}
	if !requireAdminRole(w, r) {
		return
	}
	var req struct {
		Event   string `json:"event"`
		Payload any    `json:"payload"`
	}
	limited := h.limitedBody(w, r)
	defer limited.Close()
	dec := json.NewDecoder(limited)
	if err := dec.Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid payload: %v", err), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Event) == "" {
		http.Error(w, "event is required", http.StatusBadRequest)
		return
	}
	if err := h.busPub(r.Context(), req.Event, req.Payload); err != nil {
		http.Error(w, fmt.Sprintf("publish failed: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// systemData pushes a payload to all DataEngines.
func (h *handler) systemData(w http.ResponseWriter, r *http.Request) {
	if h.busPush == nil {
		http.Error(w, "data bus not available", http.StatusNotImplemented)
		return
	}
	if !requireAdminRole(w, r) {
		return
	}
	var req struct {
		Topic   string `json:"topic"`
		Payload any    `json:"payload"`
	}
	limited := h.limitedBody(w, r)
	defer limited.Close()
	dec := json.NewDecoder(limited)
	if err := dec.Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid payload: %v", err), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Topic) == "" {
		http.Error(w, "topic is required", http.StatusBadRequest)
		return
	}
	if err := h.busPush(r.Context(), req.Topic, req.Payload); err != nil {
		http.Error(w, fmt.Sprintf("push failed: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// systemCompute invokes all ComputeEngines with the provided payload.
func (h *handler) systemCompute(w http.ResponseWriter, r *http.Request) {
	if h.invoke == nil {
		http.Error(w, "compute fan-out not available", http.StatusNotImplemented)
		return
	}
	if !requireAdminRole(w, r) {
		return
	}
	var req struct {
		Payload any `json:"payload"`
	}
	limited := h.limitedBody(w, r)
	defer limited.Close()
	dec := json.NewDecoder(limited)
	if err := dec.Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid payload: %v", err), http.StatusBadRequest)
		return
	}
	if req.Payload == nil {
		http.Error(w, "payload is required", http.StatusBadRequest)
		return
	}
	results, err := h.invoke(r.Context(), req.Payload)
	status := http.StatusOK
	if err != nil {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]any{
		"results": results,
		"error":   errString(err),
	})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (h *handler) limitedBody(w http.ResponseWriter, r *http.Request) io.ReadCloser {
	limit := h.busMaxBytes
	if limit <= 0 {
		limit = defaultBusMaxBytes
	}
	return http.MaxBytesReader(w, r.Body, limit)
}

func requireAdminRole(w http.ResponseWriter, r *http.Request) bool {
	role, _ := r.Context().Value(ctxRoleKey).(string)
	if role != "admin" {
		core.WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden: admin only"))
		return false
	}
	return true
}
