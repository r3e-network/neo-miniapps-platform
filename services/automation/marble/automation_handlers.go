package automationmarble

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	automationsupabase "github.com/R3E-Network/service_layer/services/automation/supabase"
	"github.com/google/uuid"
)

func (s *Service) handleInfo(w http.ResponseWriter, r *http.Request) {
	s.scheduler.mu.RLock()
	activeTriggers := 0
	totalExecutions := int64(0)
	for _, t := range s.scheduler.triggers {
		if t.Enabled {
			activeTriggers++
		}
	}
	chainTriggers := len(s.scheduler.chainTriggers)
	for _, t := range s.scheduler.chainTriggers {
		if t.ExecutionCount != nil {
			totalExecutions += t.ExecutionCount.Int64()
		}
	}
	s.scheduler.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "active",
		"active_triggers":  activeTriggers,
		"chain_triggers":   chainTriggers,
		"total_executions": totalExecutions,
		"service_fee":      ServiceFeePerExecution,
		"trigger_types": map[string]string{
			"time":      "Cron-based time triggers",
			"price":     "Price threshold triggers",
			"event":     "On-chain event triggers",
			"threshold": "Balance/value threshold triggers",
		},
	})
}

func (s *Service) handleListTriggers(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	triggers, err := s.repo.GetTriggers(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

func (s *Service) handleCreateTrigger(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req TriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.TriggerType == "" {
		http.Error(w, "name and trigger_type required", http.StatusBadRequest)
		return
	}

	// Calculate next execution for cron triggers
	var nextExec time.Time
	if req.TriggerType == "cron" && req.Schedule != "" {
		next, err := s.parseNextCronExecution(req.Schedule)
		if err != nil {
			http.Error(w, "invalid cron schedule: "+err.Error(), http.StatusBadRequest)
			return
		}
		nextExec = next
	}

	trigger := &automationsupabase.Trigger{
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
		http.Error(w, "failed to persist trigger", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(TriggerResponse{
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
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/triggers/")
	trigger, err := s.repo.GetTrigger(r.Context(), id, userID)
	if err != nil {
		http.Error(w, "trigger not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trigger)
}

func (s *Service) handleUpdateTrigger(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/triggers/")

	var req TriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	trigger, err := s.repo.GetTrigger(r.Context(), id, userID)
	if err != nil {
		http.Error(w, "trigger not found", http.StatusNotFound)
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
		http.Error(w, "failed to update trigger", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trigger)
}

func (s *Service) handleDeleteTrigger(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/triggers/")
	if err := s.repo.DeleteTrigger(r.Context(), id, userID); err != nil {
		http.Error(w, "trigger not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleEnableTrigger(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/triggers/")
	if err := s.repo.SetTriggerEnabled(r.Context(), id, userID, true); err != nil {
		http.Error(w, "trigger not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "enabled"})
}

func (s *Service) handleDisableTrigger(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/triggers/")
	if err := s.repo.SetTriggerEnabled(r.Context(), id, userID, false); err != nil {
		http.Error(w, "trigger not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "disabled"})
}

func (s *Service) handleListExecutions(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/triggers/")
	// Ensure trigger belongs to user
	if _, err := s.repo.GetTrigger(r.Context(), id, userID); err != nil {
		http.Error(w, "trigger not found", http.StatusNotFound)
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	execs, err := s.repo.GetExecutions(r.Context(), id, limit)
	if err != nil {
		http.Error(w, "failed to load executions", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(execs)
}

// handleResumeTrigger re-enqueues a trigger by id (e.g., after restart).
func (s *Service) handleResumeTrigger(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/triggers/")
	trigger, err := s.repo.GetTrigger(r.Context(), id, userID)
	if err != nil {
		http.Error(w, "trigger not found", http.StatusNotFound)
		return
	}
	// Add to scheduler cache for in-memory checks
	s.scheduler.mu.Lock()
	s.scheduler.triggers[trigger.ID] = trigger
	s.scheduler.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
}
