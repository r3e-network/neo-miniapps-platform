package neoflowchain

import (
	"fmt"

	"github.com/R3E-Network/service_layer/internal/chain"
)

// =============================================================================
// NeoFlow Service Events (Trigger-Based Pattern)
// =============================================================================
// Note: NeoFlow uses trigger pattern - users register triggers via Gateway,
// TEE monitors conditions and executes callbacks when conditions are met.

// NeoFlowTriggerRegisteredEvent represents a TriggerRegistered event.
// Event: TriggerRegistered(triggerId, owner, targetContract, triggerType, condition)
type NeoFlowTriggerRegisteredEvent struct {
	TriggerID      uint64
	Owner          string
	TargetContract string
	TriggerType    uint8
	Condition      string
}

// ParseNeoFlowTriggerRegisteredEvent parses a TriggerRegistered event.
func ParseNeoFlowTriggerRegisteredEvent(event *chain.ContractEvent) (*NeoFlowTriggerRegisteredEvent, error) {
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

	return &NeoFlowTriggerRegisteredEvent{
		TriggerID:      triggerID.Uint64(),
		Owner:          owner,
		TargetContract: targetContract,
		TriggerType:    uint8(triggerType.Int64()),
		Condition:      condition,
	}, nil
}

// NeoFlowTriggerExecutedEvent represents a TriggerExecuted event.
// Event: TriggerExecuted(triggerId, targetContract, success, timestamp)
type NeoFlowTriggerExecutedEvent struct {
	TriggerID      uint64
	TargetContract string
	Success        bool
	Timestamp      uint64
}

// ParseNeoFlowTriggerExecutedEvent parses a TriggerExecuted event.
func ParseNeoFlowTriggerExecutedEvent(event *chain.ContractEvent) (*NeoFlowTriggerExecutedEvent, error) {
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

	return &NeoFlowTriggerExecutedEvent{
		TriggerID:      triggerID.Uint64(),
		TargetContract: targetContract,
		Success:        success,
		Timestamp:      timestamp.Uint64(),
	}, nil
}

// NeoFlowTriggerPausedEvent represents a TriggerPaused event.
// Event: TriggerPaused(triggerId)
type NeoFlowTriggerPausedEvent struct {
	TriggerID uint64
}

// ParseNeoFlowTriggerPausedEvent parses a TriggerPaused event.
func ParseNeoFlowTriggerPausedEvent(event *chain.ContractEvent) (*NeoFlowTriggerPausedEvent, error) {
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

	return &NeoFlowTriggerPausedEvent{
		TriggerID: triggerID.Uint64(),
	}, nil
}

// NeoFlowTriggerResumedEvent represents a TriggerResumed event.
// Event: TriggerResumed(triggerId)
type NeoFlowTriggerResumedEvent struct {
	TriggerID uint64
}

// ParseNeoFlowTriggerResumedEvent parses a TriggerResumed event.
func ParseNeoFlowTriggerResumedEvent(event *chain.ContractEvent) (*NeoFlowTriggerResumedEvent, error) {
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

	return &NeoFlowTriggerResumedEvent{
		TriggerID: triggerID.Uint64(),
	}, nil
}

// NeoFlowTriggerCancelledEvent represents a TriggerCancelled event.
// Event: TriggerCancelled(triggerId)
type NeoFlowTriggerCancelledEvent struct {
	TriggerID uint64
}

// ParseNeoFlowTriggerCancelledEvent parses a TriggerCancelled event.
func ParseNeoFlowTriggerCancelledEvent(event *chain.ContractEvent) (*NeoFlowTriggerCancelledEvent, error) {
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

	return &NeoFlowTriggerCancelledEvent{
		TriggerID: triggerID.Uint64(),
	}, nil
}
