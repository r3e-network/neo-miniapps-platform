package neoflowmarble

import (
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/internal/httputil"
	neoflowsupabase "github.com/R3E-Network/service_layer/services/neoflow/supabase"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (s *Service) handleListTriggers(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	triggers, err := s.repo.GetTriggers(r.Context(), userID)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}

	responses := make([]TriggerResponse, len(triggers))
	for i, t := range triggers {
		responses[i] = TriggerResponse{
			ID:          t.ID,
			Name:        t.Name,
			TriggerType: t.TriggerType,
			Schedule:    t.Schedule,
			Condition:   t.Condition,
			Action:      t.Action,
			Enabled:     t.Enabled,
			CreatedAt:   t.CreatedAt,
		}
	}

	httputil.WriteJSON(w, http.StatusOK, responses)
}

func (s *Service) handleCreateTrigger(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	var req TriggerRequest
	if !httputil.DecodeJSON(w, r, &req) {
		return
	}

	if req.Name == "" || req.TriggerType == "" {
		httputil.BadRequest(w, "name and trigger_type required")
		return
	}

	// Calculate next execution for cron triggers
	var nextExec time.Time
	if req.TriggerType == "cron" && req.Schedule != "" {
		next, err := s.parseNextCronExecution(req.Schedule)
		if err != nil {
			httputil.BadRequest(w, "invalid cron schedule: "+err.Error())
			return
		}
		nextExec = next
	}

	trigger := &neoflowsupabase.Trigger{
		ID:            uuid.New().String(),
		UserID:        userID,
		Name:          req.Name,
		TriggerType:   req.TriggerType,
		Schedule:      req.Schedule,
		Condition:     req.Condition,
		Action:        req.Action,
		Enabled:       true,
		NextExecution: nextExec,
		CreatedAt:     time.Now(),
	}

	if err := s.repo.CreateTrigger(r.Context(), trigger); err != nil {
		httputil.InternalError(w, "failed to persist trigger")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, TriggerResponse{
		ID:          trigger.ID,
		Name:        trigger.Name,
		TriggerType: trigger.TriggerType,
		Schedule:    trigger.Schedule,
		Action:      trigger.Action,
		Enabled:     trigger.Enabled,
		CreatedAt:   trigger.CreatedAt,
	})
}

func (s *Service) handleGetTrigger(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]
	trigger, err := s.repo.GetTrigger(r.Context(), id, userID)
	if err != nil {
		httputil.NotFound(w, "trigger not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, trigger)
}

func (s *Service) handleUpdateTrigger(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]

	var req TriggerRequest
	if !httputil.DecodeJSON(w, r, &req) {
		return
	}

	trigger, err := s.repo.GetTrigger(r.Context(), id, userID)
	if err != nil {
		httputil.NotFound(w, "trigger not found")
		return
	}

	trigger.Name = req.Name
	trigger.TriggerType = req.TriggerType
	trigger.Schedule = req.Schedule
	trigger.Condition = req.Condition
	trigger.Action = req.Action

	if trigger.TriggerType == "cron" && trigger.Schedule != "" {
		if next, err := s.parseNextCronExecution(trigger.Schedule); err == nil {
			trigger.NextExecution = next
		}
	}

	if err := s.repo.UpdateTrigger(r.Context(), trigger); err != nil {
		httputil.InternalError(w, "failed to update trigger")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, trigger)
}

func (s *Service) handleDeleteTrigger(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]
	if err := s.repo.DeleteTrigger(r.Context(), id, userID); err != nil {
		httputil.NotFound(w, "trigger not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleEnableTrigger(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]
	if err := s.repo.SetTriggerEnabled(r.Context(), id, userID, true); err != nil {
		httputil.NotFound(w, "trigger not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

func (s *Service) handleDisableTrigger(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]
	if err := s.repo.SetTriggerEnabled(r.Context(), id, userID, false); err != nil {
		httputil.NotFound(w, "trigger not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (s *Service) handleListExecutions(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]
	// Ensure trigger belongs to user
	if _, err := s.repo.GetTrigger(r.Context(), id, userID); err != nil {
		httputil.NotFound(w, "trigger not found")
		return
	}
	limit := httputil.QueryInt(r, "limit", 50)
	if limit > 500 {
		limit = 500
	}
	execs, err := s.repo.GetExecutions(r.Context(), id, limit)
	if err != nil {
		httputil.InternalError(w, "failed to load executions")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, execs)
}

// handleResumeTrigger re-enqueues a trigger by id (e.g., after restart).
func (s *Service) handleResumeTrigger(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]
	trigger, err := s.repo.GetTrigger(r.Context(), id, userID)
	if err != nil {
		httputil.NotFound(w, "trigger not found")
		return
	}
	// Add to scheduler cache for in-memory checks
	s.scheduler.mu.Lock()
	s.scheduler.triggers[trigger.ID] = trigger
	s.scheduler.mu.Unlock()

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}
