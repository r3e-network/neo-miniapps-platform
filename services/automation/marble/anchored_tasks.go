package neoflow

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/infrastructure/chain"
	txproxytypes "github.com/R3E-Network/service_layer/infrastructure/txproxy/types"
)

type anchoredTaskTriggerSpec struct {
	Type      string `json:"type"`
	Schedule  string `json:"schedule,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
	Operator  string `json:"operator,omitempty"`
	Threshold int64  `json:"threshold,omitempty"`
}

type anchoredTaskState struct {
	mu sync.Mutex

	task           *chain.AutomationTask
	trigger        anchoredTaskTriggerSpec
	nextExecution  time.Time
	lastRoundID    *big.Int
	executionCount int64
	lastExecutedAt time.Time
}

func anchoredTaskKey(taskID []byte) string {
	return hex.EncodeToString(taskID)
}

func (s *Service) hydrateAnchoredTasks(ctx context.Context) error {
	if s == nil || !s.enableChainExec {
		return nil
	}
	if s.automationAnchor == nil {
		return nil
	}

	taskIDs := strings.TrimSpace(os.Getenv("NEOFLOW_TASK_IDS"))
	if m := s.Marble(); m != nil {
		if v, ok := m.Secret("NEOFLOW_TASK_IDS"); ok && len(v) > 0 {
			taskIDs = strings.TrimSpace(string(v))
		}
	}
	if taskIDs == "" {
		return nil
	}

	for _, raw := range strings.Split(taskIDs, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		decoded, err := decodeTaskID(raw)
		if err != nil {
			s.Logger().WithContext(ctx).WithError(err).WithField("task_id", raw).Warn("invalid NEOFLOW_TASK_IDS entry")
			continue
		}

		if err := s.loadAndRegisterTask(ctx, decoded); err != nil {
			s.Logger().WithContext(ctx).WithError(err).WithField("task_id", raw).Warn("failed to load automation task")
		}
	}

	return nil
}

func decodeTaskID(raw string) ([]byte, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, fmt.Errorf("empty task id")
	}

	trimmed := strings.TrimPrefix(value, "0x")
	trimmed = strings.TrimPrefix(trimmed, "0X")
	if len(trimmed)%2 == 0 && isHex(trimmed) {
		decoded, err := hex.DecodeString(trimmed)
		if err == nil && len(decoded) > 0 {
			return decoded, nil
		}
	}

	return []byte(value), nil
}

func isHex(value string) bool {
	for _, c := range value {
		if (c >= '0' && c <= '9') ||
			(c >= 'a' && c <= 'f') ||
			(c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}

func (s *Service) setupAutomationAnchorListener() {
	if s == nil || s.eventListener == nil || s.automationAnchor == nil {
		return
	}

	s.eventListener.On("TaskRegistered", func(event *chain.ContractEvent) error {
		parsed, err := chain.ParseAutomationAnchorTaskRegisteredEvent(event)
		if err != nil {
			return err
		}
		return s.loadAndRegisterTask(context.Background(), parsed.TaskID)
	})
}

func (s *Service) loadAndRegisterTask(ctx context.Context, taskID []byte) error {
	if s == nil || s.automationAnchor == nil {
		return fmt.Errorf("automation anchor not configured")
	}

	task, err := s.automationAnchor.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil || len(task.TaskID) == 0 {
		return fmt.Errorf("task not found")
	}
	if !task.Enabled {
		s.scheduler.mu.Lock()
		delete(s.scheduler.anchoredTasks, anchoredTaskKey(task.TaskID))
		s.scheduler.mu.Unlock()
		return nil
	}

	var spec anchoredTaskTriggerSpec
	if len(task.Trigger) > 0 {
		if err := json.Unmarshal(task.Trigger, &spec); err != nil {
			return fmt.Errorf("parse trigger json: %w", err)
		}
	}
	if spec.Type == "" {
		return fmt.Errorf("trigger.type required")
	}

	state := &anchoredTaskState{
		task:    task,
		trigger: spec,
	}

	if strings.EqualFold(spec.Type, "cron") {
		next, err := s.parseNextCronExecution(spec.Schedule)
		if err != nil {
			return fmt.Errorf("parse cron schedule: %w", err)
		}
		state.nextExecution = next
	}

	s.scheduler.mu.Lock()
	s.scheduler.anchoredTasks[anchoredTaskKey(task.TaskID)] = state
	s.scheduler.mu.Unlock()

	return nil
}

func (s *Service) checkAndExecuteAnchoredTasks(ctx context.Context) {
	if s == nil || !s.enableChainExec {
		return
	}
	if s.chainClient == nil || s.txProxy == nil || s.automationAnchor == nil {
		return
	}

	s.scheduler.mu.RLock()
	tasks := make([]*anchoredTaskState, 0, len(s.scheduler.anchoredTasks))
	for _, t := range s.scheduler.anchoredTasks {
		tasks = append(tasks, t)
	}
	s.scheduler.mu.RUnlock()

	now := time.Now()
	for _, state := range tasks {
		if state == nil || state.task == nil || !state.task.Enabled {
			continue
		}

		switch strings.ToLower(state.trigger.Type) {
		case "cron":
			s.checkAndExecuteAnchoredCronTask(ctx, now, state)
		case "price":
			s.checkAndExecuteAnchoredPriceTask(ctx, state)
		default:
			continue
		}
	}
}

func (s *Service) checkAndExecuteAnchoredCronTask(ctx context.Context, now time.Time, task *anchoredTaskState) {
	if task == nil {
		return
	}

	task.mu.Lock()
	nextExecution := task.nextExecution
	task.mu.Unlock()

	if nextExecution.IsZero() {
		next, err := s.parseNextCronExecution(task.trigger.Schedule)
		if err != nil {
			return
		}
		task.mu.Lock()
		task.nextExecution = next
		nextExecution = next
		task.mu.Unlock()
	}

	if !now.After(nextExecution) {
		return
	}
	if now.Sub(nextExecution) > time.Minute {
		// Too old; resync schedule.
		if next, err := s.parseNextCronExecution(task.trigger.Schedule); err == nil {
			task.mu.Lock()
			task.nextExecution = next
			task.mu.Unlock()
		}
		return
	}

	executionData, _ := json.Marshal(map[string]any{
		"type":        "cron",
		"executed_at": now.Unix(),
	})

	go s.executeAnchoredTask(ctx, task, executionData)

	next, err := s.parseNextCronExecution(task.trigger.Schedule)
	if err == nil {
		task.mu.Lock()
		task.nextExecution = next
		task.mu.Unlock()
	}
}

func (s *Service) checkAndExecuteAnchoredPriceTask(ctx context.Context, task *anchoredTaskState) {
	if task == nil || task.task == nil {
		return
	}
	if s.priceFeed == nil {
		return
	}
	symbol := strings.TrimSpace(task.trigger.Symbol)
	if symbol == "" {
		return
	}

	record, err := s.priceFeed.GetLatest(ctx, symbol)
	if err != nil || record == nil || record.RoundID == nil || record.Price == nil {
		return
	}

	// Only evaluate each round once.
	task.mu.Lock()
	lastRoundID := task.lastRoundID
	task.mu.Unlock()

	if lastRoundID != nil && record.RoundID.Cmp(lastRoundID) <= 0 {
		return
	}
	task.mu.Lock()
	task.lastRoundID = new(big.Int).Set(record.RoundID)
	task.mu.Unlock()

	threshold := big.NewInt(task.trigger.Threshold)
	cmp := record.Price.Cmp(threshold)
	shouldExecute := false
	switch task.trigger.Operator {
	case ">":
		shouldExecute = cmp > 0
	case "<":
		shouldExecute = cmp < 0
	case ">=":
		shouldExecute = cmp >= 0
	case "<=":
		shouldExecute = cmp <= 0
	case "==":
		shouldExecute = cmp == 0
	default:
		return
	}

	if !shouldExecute {
		return
	}

	executionData, _ := json.Marshal(map[string]any{
		"type":          "price",
		"symbol":        symbol,
		"round_id":      record.RoundID.String(),
		"price":         record.Price.String(),
		"timestamp":     record.Timestamp,
		"threshold":     task.trigger.Threshold,
		"operator":      task.trigger.Operator,
		"attestation":   hex.EncodeToString(record.AttestationHash),
		"source_set_id": record.SourceSetID.String(),
	})

	go s.executeAnchoredTask(ctx, task, executionData)
}

func (s *Service) executeAnchoredTask(ctx context.Context, task *anchoredTaskState, executionData []byte) {
	if s == nil || task == nil || task.task == nil {
		return
	}
	if s.txProxy == nil || s.automationAnchor == nil {
		return
	}
	if task.task.Target == "" || task.task.Method == "" {
		return
	}

	nonce := big.NewInt(time.Now().UnixNano())
	if nonce.Sign() < 0 {
		nonce.Abs(nonce)
	}

	payload, err := buildAnchoredTaskPayload(task.task.Trigger, executionData)
	if err != nil {
		s.Logger().WithContext(ctx).WithError(err).WithField("task_id", anchoredTaskKey(task.task.TaskID)).Warn("failed to build task payload")
		return
	}

	params := []chain.ContractParam{
		chain.NewByteArrayParam(task.task.TaskID),
		chain.NewByteArrayParam(payload),
	}

	txResult, err := s.txProxy.Invoke(ctx, &txproxytypes.InvokeRequest{
		RequestID:    "neoflow:" + uuid.NewString(),
		ContractHash: task.task.Target,
		Method:       task.task.Method,
		Params:       params,
		Wait:         true,
	})
	if err != nil {
		s.Logger().WithContext(ctx).WithError(err).WithField("task_id", anchoredTaskKey(task.task.TaskID)).Warn("automation task invocation failed")
		return
	}
	if txResult == nil || strings.TrimSpace(txResult.TxHash) == "" {
		s.Logger().WithContext(ctx).WithField("task_id", anchoredTaskKey(task.task.TaskID)).Warn("automation task invocation returned empty tx hash")
		return
	}
	if state := strings.TrimSpace(txResult.VMState); state != "" && !strings.HasPrefix(state, "HALT") {
		entry := s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
			"task_id": anchoredTaskKey(task.task.TaskID),
			"vmstate": state,
		})
		if msg := strings.TrimSpace(txResult.Exception); msg != "" {
			entry = entry.WithField("exception", msg)
		}
		entry.Warn("automation task invocation faulted")
		return
	}

	txHashBytes, decodeErr := hex.DecodeString(strings.TrimPrefix(txResult.TxHash, "0x"))
	if decodeErr != nil || len(txHashBytes) == 0 {
		txHashBytes = []byte(txResult.TxHash)
	}

	markResult, err := s.txProxy.Invoke(ctx, &txproxytypes.InvokeRequest{
		RequestID:    "neoflow:mark:" + uuid.NewString(),
		ContractHash: s.automationAnchorHash,
		Method:       "markExecuted",
		Params: []chain.ContractParam{
			chain.NewByteArrayParam(task.task.TaskID),
			chain.NewIntegerParam(nonce),
			chain.NewByteArrayParam(txHashBytes),
		},
		Wait: true,
	})
	if err != nil {
		s.Logger().WithContext(ctx).WithError(err).WithField("task_id", anchoredTaskKey(task.task.TaskID)).Warn("failed to mark task executed")
	} else if markResult != nil {
		if state := strings.TrimSpace(markResult.VMState); state != "" && !strings.HasPrefix(state, "HALT") {
			entry := s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
				"task_id": anchoredTaskKey(task.task.TaskID),
				"vmstate": state,
			})
			if msg := strings.TrimSpace(markResult.Exception); msg != "" {
				entry = entry.WithField("exception", msg)
			}
			entry.Warn("markExecuted faulted")
		}
	}

	task.mu.Lock()
	task.executionCount++
	task.lastExecutedAt = time.Now()
	task.mu.Unlock()
}

func buildAnchoredTaskPayload(trigger, executionData []byte) ([]byte, error) {
	payload := map[string]any{}
	if len(trigger) > 0 && json.Valid(trigger) {
		payload["trigger"] = json.RawMessage(trigger)
	} else if len(trigger) > 0 {
		payload["trigger_hex"] = hex.EncodeToString(trigger)
	}
	if len(executionData) > 0 && json.Valid(executionData) {
		payload["data"] = json.RawMessage(executionData)
	} else if len(executionData) > 0 {
		payload["data_hex"] = hex.EncodeToString(executionData)
	}

	if len(payload) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(payload)
}
