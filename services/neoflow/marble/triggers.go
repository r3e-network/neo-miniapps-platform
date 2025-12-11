package neoflowmarble

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	neoflowchain "github.com/R3E-Network/service_layer/services/neoflow/chain"
	neoflowsupabase "github.com/R3E-Network/service_layer/services/neoflow/supabase"
	"github.com/google/uuid"
)

func (s *Service) checkAndExecuteTriggers(ctx context.Context) {
	if s.repo == nil {
		return
	}
	triggers, err := s.repo.GetPendingTriggers(ctx)
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

func (s *Service) executeTrigger(ctx context.Context, trigger *neoflowsupabase.Trigger) {
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
	if updateErr := s.repo.UpdateTrigger(ctx, trigger); updateErr != nil {
		log.Printf("[neoflow] failed to update trigger %s: %v", trigger.ID, updateErr)
	}

	// Persist execution log
	if s.repo != nil {
		exec := &neoflowsupabase.Execution{
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
		if execErr := s.repo.CreateExecution(ctx, exec); execErr != nil {
			log.Printf("[neoflow] failed to persist execution log for trigger %s: %v", trigger.ID, execErr)
		}
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

		// Use Marble mTLS client for secure inter-service communication
		// Falls back to default client with timeout for external webhooks
		var httpClient *http.Client
		if s.Marble() != nil {
			httpClient = s.Marble().HTTPClient()
		}
		if httpClient == nil {
			httpClient = &http.Client{Timeout: 30 * time.Second}
		}

		resp, err := httpClient.Do(req)
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
// Supports standard 5-field cron: minute hour day-of-month month day-of-week
// Supports: specific values (5), wildcards (*), ranges (1-5), lists (1,3,5), steps (*/15)
func (s *Service) parseNextCronExecution(cronExpr string) (time.Time, error) {
	parts := strings.Fields(cronExpr)
	if len(parts) != 5 {
		return time.Time{}, fmt.Errorf("invalid cron expression: expected 5 fields")
	}

	// Parse each field into allowed values
	minutes, err := parseCronField(parts[0], 0, 59)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid minute field: %w", err)
	}
	hours, err := parseCronField(parts[1], 0, 23)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid hour field: %w", err)
	}
	days, err := parseCronField(parts[2], 1, 31)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day field: %w", err)
	}
	months, err := parseCronField(parts[3], 1, 12)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month field: %w", err)
	}
	weekdays, err := parseCronField(parts[4], 0, 6)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid weekday field: %w", err)
	}

	// Find next matching time (search up to 1 year ahead)
	now := time.Now()
	candidate := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
	candidate = candidate.Add(time.Minute) // Start from next minute

	maxIterations := 525600 // 1 year in minutes
	for i := 0; i < maxIterations; i++ {
		if months[int(candidate.Month())] &&
			days[candidate.Day()] &&
			weekdays[int(candidate.Weekday())] &&
			hours[candidate.Hour()] &&
			minutes[candidate.Minute()] {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("no matching time found within 1 year")
}

// parseCronField parses a single cron field and returns a map of allowed values.
func parseCronField(field string, min, max int) (map[int]bool, error) {
	allowed := make(map[int]bool)

	// Handle wildcard
	if field == "*" {
		for i := min; i <= max; i++ {
			allowed[i] = true
		}
		return allowed, nil
	}

	// Handle step with wildcard (*/n)
	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(field[2:])
		if err != nil || step <= 0 {
			return nil, fmt.Errorf("invalid step: %s", field)
		}
		for i := min; i <= max; i += step {
			allowed[i] = true
		}
		return allowed, nil
	}

	// Handle comma-separated list
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)

		// Handle range (n-m) or range with step (n-m/s)
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "/")
			rangeStr := rangeParts[0]
			step := 1
			if len(rangeParts) == 2 {
				var err error
				step, err = strconv.Atoi(rangeParts[1])
				if err != nil || step <= 0 {
					return nil, fmt.Errorf("invalid step in range: %s", part)
				}
			}

			bounds := strings.Split(rangeStr, "-")
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err1 := strconv.Atoi(bounds[0])
			end, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil || start < min || end > max || start > end {
				return nil, fmt.Errorf("invalid range values: %s", part)
			}
			for i := start; i <= end; i += step {
				allowed[i] = true
			}
		} else {
			// Single value
			val, err := strconv.Atoi(part)
			if err != nil || val < min || val > max {
				return nil, fmt.Errorf("invalid value: %s", part)
			}
			allowed[val] = true
		}
	}

	return allowed, nil
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
	if !s.enableChainExec || s.teeFulfiller == nil || s.neoflowHash == "" {
		return
	}

	s.scheduler.mu.RLock()
	triggers := make([]*chain.Trigger, 0, len(s.scheduler.chainTriggers))
	for _, t := range s.scheduler.chainTriggers {
		triggers = append(triggers, t)
	}
	s.scheduler.mu.RUnlock()

	for _, trigger := range triggers {
		if trigger.Status != neoflowchain.TriggerStatusActive {
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
	if s.neoFeedsContract == nil {
		return false, nil
	}

	// Parse price condition from trigger.Condition
	var condition PriceCondition
	if err := json.Unmarshal([]byte(trigger.Condition), &condition); err != nil {
		return false, nil
	}

	// Get current price from NeoFeeds contract
	price, err := s.neoFeedsContract.GetPrice(ctx, condition.FeedID)
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

	// Validate required fields
	if condition.Address == "" || condition.Asset == "" || condition.Operator == "" {
		return false, nil
	}

	// Query balance via chain client RPC
	if s.chainClient == nil {
		return false, nil
	}

	balance, err := s.queryNep17Balance(ctx, condition.Address, condition.Asset)
	if err != nil {
		return false, nil
	}

	// Compare balance against threshold
	threshold := condition.Threshold
	var shouldExecute bool
	switch condition.Operator {
	case "<":
		shouldExecute = balance < threshold
	case "<=":
		shouldExecute = balance <= threshold
	case ">":
		shouldExecute = balance > threshold
	case ">=":
		shouldExecute = balance >= threshold
	case "==":
		shouldExecute = balance == threshold
	default:
		return false, nil
	}

	if shouldExecute {
		// Return execution data with balance info
		data, _ := json.Marshal(map[string]interface{}{
			"address":   condition.Address,
			"asset":     condition.Asset,
			"balance":   balance,
			"threshold": threshold,
			"operator":  condition.Operator,
		})
		return true, data
	}
	return false, nil
}

// queryNep17Balance queries the NEP-17 token balance for an address.
func (s *Service) queryNep17Balance(ctx context.Context, address, assetHash string) (int64, error) {
	// Call getnep17balances RPC method
	result, err := s.chainClient.Call(ctx, "getnep17balances", []interface{}{address})
	if err != nil {
		return 0, err
	}

	// Parse response
	var response struct {
		Balance []struct {
			AssetHash        string `json:"assethash"`
			Amount           string `json:"amount"`
			LastUpdatedBlock int64  `json:"lastupdatedblock"`
		} `json:"balance"`
		Address string `json:"address"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return 0, err
	}

	// Find balance for the specified asset
	for _, b := range response.Balance {
		if strings.EqualFold(b.AssetHash, assetHash) {
			balance, _ := strconv.ParseInt(b.Amount, 10, 64)
			return balance, nil
		}
	}

	// Asset not found means balance is 0
	return 0, nil
}

// executeChainTrigger executes a trigger on-chain.
func (s *Service) executeChainTrigger(ctx context.Context, trigger *chain.Trigger, executionData []byte) {
	_, err := s.teeFulfiller.ExecuteTrigger(ctx, s.neoflowHash, trigger.TriggerID, executionData)
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
			t.Status = neoflowchain.TriggerStatusExpired
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
		parsed, err := neoflowchain.ParseNeoFlowTriggerRegisteredEvent(event)
		if err != nil {
			return err
		}

		// Fetch full trigger details from contract
		neoflowContract := neoflowchain.NewNeoFlowContract(s.chainClient, s.neoflowHash, nil)
		trigger, err := neoflowContract.GetTrigger(context.Background(), big.NewInt(int64(parsed.TriggerID)))
		if err != nil {
			return err
		}

		s.RegisterChainTrigger(trigger)
		return nil
	})

	// Listen for TriggerCancelled events to remove triggers
	s.eventListener.On("TriggerCancelled", func(event *chain.ContractEvent) error {
		parsed, err := neoflowchain.ParseNeoFlowTriggerCancelledEvent(event)
		if err != nil {
			return err
		}
		s.UnregisterChainTrigger(parsed.TriggerID)
		return nil
	})

	// Listen for TriggerPaused events
	s.eventListener.On("TriggerPaused", func(event *chain.ContractEvent) error {
		parsed, err := neoflowchain.ParseNeoFlowTriggerPausedEvent(event)
		if err != nil {
			return err
		}

		s.scheduler.mu.Lock()
		if t, ok := s.scheduler.chainTriggers[parsed.TriggerID]; ok {
			t.Status = neoflowchain.TriggerStatusPaused
		}
		s.scheduler.mu.Unlock()
		return nil
	})

	// Listen for TriggerResumed events
	s.eventListener.On("TriggerResumed", func(event *chain.ContractEvent) error {
		parsed, err := neoflowchain.ParseNeoFlowTriggerResumedEvent(event)
		if err != nil {
			return err
		}

		s.scheduler.mu.Lock()
		if t, ok := s.scheduler.chainTriggers[parsed.TriggerID]; ok {
			t.Status = neoflowchain.TriggerStatusActive
		}
		s.scheduler.mu.Unlock()
		return nil
	})
}
