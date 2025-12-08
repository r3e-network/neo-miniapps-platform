package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/google/uuid"
)

func (s *Service) checkAndExecuteTriggers(ctx context.Context) {
	triggers, err := s.DB().GetPendingTriggers(ctx)
	if err != nil {
		return
	}

	now := time.Now()
	for i := range triggers {
		trigger := &triggers[i]
		if !trigger.Enabled {
			continue
		}
		if trigger.TriggerType == "cron" && !trigger.NextExecution.IsZero() {
			if now.After(trigger.NextExecution) {
				go s.executeTrigger(ctx, trigger)
			}
		}
	}
}

func (s *Service) executeTrigger(ctx context.Context, trigger *database.AutomationTrigger) {
	var actionType string
	if len(trigger.Action) > 0 {
		var act Action
		if err := json.Unmarshal(trigger.Action, &act); err == nil {
			actionType = act.Type
		}
	}

	// Execute the action (best-effort)
	err := s.dispatchAction(ctx, trigger.Action)

	// Update last execution and calculate next
	trigger.LastExecution = time.Now()
	if trigger.TriggerType == "cron" && trigger.Schedule != "" {
		next, _ := s.parseNextCronExecution(trigger.Schedule)
		trigger.NextExecution = next
	}
	_ = s.DB().UpdateAutomationTrigger(ctx, trigger)

	// Persist execution log
	if s.DB() != nil {
		exec := &database.AutomationExecution{
			ID:            uuid.New().String(),
			TriggerID:     trigger.ID,
			ExecutedAt:    trigger.LastExecution,
			Success:       err == nil,
			ActionType:    actionType,
			ActionPayload: trigger.Action,
		}
		if err != nil {
			exec.Error = err.Error()
		}
		_ = s.DB().CreateAutomationExecution(ctx, exec)
	}
}

func (s *Service) dispatchAction(ctx context.Context, actionRaw json.RawMessage) error {
	if len(actionRaw) == 0 {
		return nil
	}
	var action Action
	if err := json.Unmarshal(actionRaw, &action); err != nil {
		return err
	}

	switch strings.ToLower(action.Type) {
	case "webhook":
		method := strings.ToUpper(action.Method)
		if method == "" {
			method = http.MethodPost
		}
		if action.URL == "" {
			return fmt.Errorf("webhook url required")
		}
		req, err := http.NewRequestWithContext(ctx, method, action.URL, bytes.NewReader(action.Body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("webhook status %d", resp.StatusCode)
		}
	default:
		// Unknown action type; ignore
	}
	return nil
}

// parseNextCronExecution parses a cron expression and returns the next execution time.
func (s *Service) parseNextCronExecution(cronExpr string) (time.Time, error) {
	parts := strings.Fields(cronExpr)
	if len(parts) != 5 {
		return time.Time{}, fmt.Errorf("invalid cron expression: expected 5 fields")
	}

	now := time.Now()

	// Simple implementation for common patterns
	// Production would use a full cron parser
	minute, _ := strconv.Atoi(parts[0])
	if parts[0] == "*" {
		minute = now.Minute() + 1
	}

	next := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), minute, 0, 0, now.Location())
	if next.Before(now) {
		next = next.Add(time.Hour)
	}

	return next, nil
}

// RegisterChainTrigger registers an on-chain trigger for monitoring.
func (s *Service) RegisterChainTrigger(trigger *chain.Trigger) {
	s.scheduler.mu.Lock()
	defer s.scheduler.mu.Unlock()
	s.scheduler.chainTriggers[trigger.TriggerID.Uint64()] = trigger
}

// UnregisterChainTrigger removes an on-chain trigger from monitoring.
func (s *Service) UnregisterChainTrigger(triggerID uint64) {
	s.scheduler.mu.Lock()
	defer s.scheduler.mu.Unlock()
	delete(s.scheduler.chainTriggers, triggerID)
}

// checkChainTriggers checks all registered on-chain triggers and executes if conditions are met.
func (s *Service) checkChainTriggers(ctx context.Context) {
	if !s.enableChainExec || s.teeFulfiller == nil || s.automationHash == "" {
		return
	}

	s.scheduler.mu.RLock()
	triggers := make([]*chain.Trigger, 0, len(s.scheduler.chainTriggers))
	for _, t := range s.scheduler.chainTriggers {
		triggers = append(triggers, t)
	}
	s.scheduler.mu.RUnlock()

	for _, trigger := range triggers {
		if trigger.Status != chain.TriggerStatusActive {
			continue
		}

		// Check if trigger condition is met
		shouldExecute, executionData := s.evaluateTriggerCondition(ctx, trigger)
		if !shouldExecute {
			continue
		}

		// Execute trigger on-chain
		go s.executeChainTrigger(ctx, trigger, executionData)
	}
}

// evaluateTriggerCondition evaluates whether a trigger's condition is met.
func (s *Service) evaluateTriggerCondition(ctx context.Context, trigger *chain.Trigger) (bool, []byte) {
	switch trigger.TriggerType {
	case TriggerTypeTime:
		return s.evaluateTimeTrigger(trigger)
	case TriggerTypePrice:
		return s.evaluatePriceTrigger(ctx, trigger)
	case TriggerTypeEvent:
		// Event triggers are handled by the event listener
		return false, nil
	case TriggerTypeThreshold:
		return s.evaluateThresholdTrigger(ctx, trigger)
	default:
		return false, nil
	}
}

// evaluateTimeTrigger checks if a time-based trigger should execute.
func (s *Service) evaluateTimeTrigger(trigger *chain.Trigger) (bool, []byte) {
	// Parse cron expression from condition
	cronExpr := trigger.Condition
	if cronExpr == "" {
		return false, nil
	}

	nextExec, err := s.parseNextCronExecution(cronExpr)
	if err != nil {
		return false, nil
	}

	now := time.Now()

	// Check if we're within the execution window (1 minute tolerance)
	if now.After(nextExec) && now.Sub(nextExec) < time.Minute {
		// Check if we haven't executed recently
		if trigger.LastExecutedAt == 0 || now.Unix()-int64(trigger.LastExecutedAt/1000) > 60 {
			return true, []byte(fmt.Sprintf(`{"executed_at":%d}`, now.Unix()))
		}
	}

	return false, nil
}

// evaluatePriceTrigger checks if a price-based trigger should execute.
func (s *Service) evaluatePriceTrigger(ctx context.Context, trigger *chain.Trigger) (bool, []byte) {
	if s.dataFeedsContract == nil {
		return false, nil
	}

	// Parse price condition from trigger.Condition
	var condition PriceCondition
	if err := json.Unmarshal([]byte(trigger.Condition), &condition); err != nil {
		return false, nil
	}

	// Get current price from DataFeeds contract
	price, err := s.dataFeedsContract.GetPrice(ctx, condition.FeedID)
	if err != nil {
		return false, nil
	}

	currentPrice := price.Int64()
	shouldExecute := false

	switch condition.Operator {
	case ">":
		shouldExecute = currentPrice > condition.Threshold
	case "<":
		shouldExecute = currentPrice < condition.Threshold
	case ">=":
		shouldExecute = currentPrice >= condition.Threshold
	case "<=":
		shouldExecute = currentPrice <= condition.Threshold
	case "==":
		shouldExecute = currentPrice == condition.Threshold
	}

	if shouldExecute {
		executionData, _ := json.Marshal(map[string]interface{}{
			"feed_id":       condition.FeedID,
			"current_price": currentPrice,
			"threshold":     condition.Threshold,
			"operator":      condition.Operator,
			"timestamp":     time.Now().Unix(),
		})
		return true, executionData
	}

	return false, nil
}

// evaluateThresholdTrigger checks if a threshold-based trigger should execute.
func (s *Service) evaluateThresholdTrigger(ctx context.Context, trigger *chain.Trigger) (bool, []byte) {
	// Parse threshold condition
	var condition ThresholdCondition
	if err := json.Unmarshal([]byte(trigger.Condition), &condition); err != nil {
		return false, nil
	}

	// Query balance via automation contract helper if available
	// Note: no direct GetBalance helper exists; threshold evaluation requires integration with a balance oracle or off-chain query.
	// Leaving as no-op until balance source is available.
	return false, nil
}

// executeChainTrigger executes a trigger on-chain.
func (s *Service) executeChainTrigger(ctx context.Context, trigger *chain.Trigger, executionData []byte) {
	_, err := s.teeFulfiller.ExecuteTrigger(ctx, s.automationHash, trigger.TriggerID, executionData)
	if err != nil {
		// Log error but continue - trigger will be retried on next check
		return
	}

	// Update local trigger state
	s.scheduler.mu.Lock()
	if t, ok := s.scheduler.chainTriggers[trigger.TriggerID.Uint64()]; ok {
		t.LastExecutedAt = uint64(time.Now().UnixMilli())
		t.ExecutionCount = new(big.Int).Add(t.ExecutionCount, big.NewInt(1))

		// Check if max executions reached
		if t.MaxExecutions.Cmp(big.NewInt(0)) > 0 && t.ExecutionCount.Cmp(t.MaxExecutions) >= 0 {
			t.Status = chain.TriggerStatusExpired
		}
	}
	s.scheduler.mu.Unlock()
}

// SetupEventTriggerListener sets up the event listener for event-based triggers.
func (s *Service) SetupEventTriggerListener() {
	if s.eventListener == nil {
		return
	}

	// Listen for TriggerRegistered events to add new triggers
	s.eventListener.On("TriggerRegistered", func(event *chain.ContractEvent) error {
		parsed, err := chain.ParseAutomationTriggerRegisteredEvent(event)
		if err != nil {
			return err
		}

		// Fetch full trigger details from contract
		automationContract := chain.NewAutomationContract(s.chainClient, s.automationHash, nil)
		trigger, err := automationContract.GetTrigger(context.Background(), big.NewInt(int64(parsed.TriggerID)))
		if err != nil {
			return err
		}

		s.RegisterChainTrigger(trigger)
		return nil
	})

	// Listen for TriggerCancelled events to remove triggers
	s.eventListener.On("TriggerCancelled", func(event *chain.ContractEvent) error {
		parsed, err := chain.ParseAutomationTriggerCancelledEvent(event)
		if err != nil {
			return err
		}
		s.UnregisterChainTrigger(parsed.TriggerID)
		return nil
	})

	// Listen for TriggerPaused events
	s.eventListener.On("TriggerPaused", func(event *chain.ContractEvent) error {
		parsed, err := chain.ParseAutomationTriggerPausedEvent(event)
		if err != nil {
			return err
		}

		s.scheduler.mu.Lock()
		if t, ok := s.scheduler.chainTriggers[parsed.TriggerID]; ok {
			t.Status = chain.TriggerStatusPaused
		}
		s.scheduler.mu.Unlock()
		return nil
	})

	// Listen for TriggerResumed events
	s.eventListener.On("TriggerResumed", func(event *chain.ContractEvent) error {
		parsed, err := chain.ParseAutomationTriggerResumedEvent(event)
		if err != nil {
			return err
		}

		s.scheduler.mu.Lock()
		if t, ok := s.scheduler.chainTriggers[parsed.TriggerID]; ok {
			t.Status = chain.TriggerStatusActive
		}
		s.scheduler.mu.Unlock()
		return nil
	})
}
