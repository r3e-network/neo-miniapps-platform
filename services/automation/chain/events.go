package automationchain

import (
	"fmt"

	"github.com/R3E-Network/service_layer/internal/chain"
)

// =============================================================================
// Automation Service Events (Trigger-Based Pattern)
// =============================================================================
// Note: Automation uses trigger pattern - users register triggers via Gateway,
// TEE monitors conditions and executes callbacks when conditions are met.

// AutomationTriggerRegisteredEvent represents a TriggerRegistered event.
// Event: TriggerRegistered(triggerId, owner, targetContract, triggerType, condition)
type AutomationTriggerRegisteredEvent struct {
	TriggerID      uint64
	Owner          string
	TargetContract string
	TriggerType    uint8
	Condition      string
}

// ParseAutomationTriggerRegisteredEvent parses a TriggerRegistered event.
func ParseAutomationTriggerRegisteredEvent(event *chain.ContractEvent) (*AutomationTriggerRegisteredEvent, error) {
	if event.EventName != "TriggerRegistered" {
		return nil, fmt.Errorf("not a TriggerRegistered event")
	}
	if len(event.State) < 5 {
		return nil, fmt.Errorf("invalid event state: expected 5 items, got %d", len(event.State))
	}

	triggerID, err := chain.ParseInteger(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse triggerId: %w", err)
	}

	owner, err := chain.ParseHash160(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse owner: %w", err)
	}

	targetContract, err := chain.ParseHash160(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse targetContract: %w", err)
	}

	triggerType, err := chain.ParseInteger(event.State[3])
	if err != nil {
		return nil, fmt.Errorf("parse triggerType: %w", err)
	}

	condition, err := chain.ParseStringFromItem(event.State[4])
	if err != nil {
		return nil, fmt.Errorf("parse condition: %w", err)
	}

	return &AutomationTriggerRegisteredEvent{
		TriggerID:      triggerID.Uint64(),
		Owner:          owner,
		TargetContract: targetContract,
		TriggerType:    uint8(triggerType.Int64()),
		Condition:      condition,
	}, nil
}

// AutomationTriggerExecutedEvent represents a TriggerExecuted event.
// Event: TriggerExecuted(triggerId, targetContract, success, timestamp)
type AutomationTriggerExecutedEvent struct {
	TriggerID      uint64
	TargetContract string
	Success        bool
	Timestamp      uint64
}

// ParseAutomationTriggerExecutedEvent parses a TriggerExecuted event.
func ParseAutomationTriggerExecutedEvent(event *chain.ContractEvent) (*AutomationTriggerExecutedEvent, error) {
	if event.EventName != "TriggerExecuted" {
		return nil, fmt.Errorf("not a TriggerExecuted event")
	}
	if len(event.State) < 4 {
		return nil, fmt.Errorf("invalid event state: expected 4 items, got %d", len(event.State))
	}

	triggerID, err := chain.ParseInteger(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse triggerId: %w", err)
	}

	targetContract, err := chain.ParseHash160(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse targetContract: %w", err)
	}

	success, err := chain.ParseBoolean(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse success: %w", err)
	}

	timestamp, err := chain.ParseInteger(event.State[3])
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}

	return &AutomationTriggerExecutedEvent{
		TriggerID:      triggerID.Uint64(),
		TargetContract: targetContract,
		Success:        success,
		Timestamp:      timestamp.Uint64(),
	}, nil
}

// AutomationTriggerPausedEvent represents a TriggerPaused event.
// Event: TriggerPaused(triggerId)
type AutomationTriggerPausedEvent struct {
	TriggerID uint64
}

// ParseAutomationTriggerPausedEvent parses a TriggerPaused event.
func ParseAutomationTriggerPausedEvent(event *chain.ContractEvent) (*AutomationTriggerPausedEvent, error) {
	if event.EventName != "TriggerPaused" {
		return nil, fmt.Errorf("not a TriggerPaused event")
	}
	if len(event.State) < 1 {
		return nil, fmt.Errorf("invalid event state: expected 1 item, got %d", len(event.State))
	}

	triggerID, err := chain.ParseInteger(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse triggerId: %w", err)
	}

	return &AutomationTriggerPausedEvent{
		TriggerID: triggerID.Uint64(),
	}, nil
}

// AutomationTriggerResumedEvent represents a TriggerResumed event.
// Event: TriggerResumed(triggerId)
type AutomationTriggerResumedEvent struct {
	TriggerID uint64
}

// ParseAutomationTriggerResumedEvent parses a TriggerResumed event.
func ParseAutomationTriggerResumedEvent(event *chain.ContractEvent) (*AutomationTriggerResumedEvent, error) {
	if event.EventName != "TriggerResumed" {
		return nil, fmt.Errorf("not a TriggerResumed event")
	}
	if len(event.State) < 1 {
		return nil, fmt.Errorf("invalid event state: expected 1 item, got %d", len(event.State))
	}

	triggerID, err := chain.ParseInteger(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse triggerId: %w", err)
	}

	return &AutomationTriggerResumedEvent{
		TriggerID: triggerID.Uint64(),
	}, nil
}

// AutomationTriggerCancelledEvent represents a TriggerCancelled event.
// Event: TriggerCancelled(triggerId)
type AutomationTriggerCancelledEvent struct {
	TriggerID uint64
}

// ParseAutomationTriggerCancelledEvent parses a TriggerCancelled event.
func ParseAutomationTriggerCancelledEvent(event *chain.ContractEvent) (*AutomationTriggerCancelledEvent, error) {
	if event.EventName != "TriggerCancelled" {
		return nil, fmt.Errorf("not a TriggerCancelled event")
	}
	if len(event.State) < 1 {
		return nil, fmt.Errorf("invalid event state: expected 1 item, got %d", len(event.State))
	}

	triggerID, err := chain.ParseInteger(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse triggerId: %w", err)
	}

	return &AutomationTriggerCancelledEvent{
		TriggerID: triggerID.Uint64(),
	}, nil
}
