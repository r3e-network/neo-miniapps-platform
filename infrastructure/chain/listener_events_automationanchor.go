package chain

import (
	"fmt"
	"math/big"
)

// =============================================================================
// AutomationAnchor Events (platform AutomationAnchor contract)
// =============================================================================

// AutomationAnchorTaskRegisteredEvent represents a TaskRegistered event.
// Event: TaskRegistered(taskId, target, method)
type AutomationAnchorTaskRegisteredEvent struct {
	TaskID []byte
	Target string
	Method string
}

func ParseAutomationAnchorTaskRegisteredEvent(event *ContractEvent) (*AutomationAnchorTaskRegisteredEvent, error) {
	if event.EventName != "TaskRegistered" {
		return nil, fmt.Errorf("not a TaskRegistered event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state: expected 3 items, got %d", len(event.State))
	}

	taskID, err := ParseByteArray(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse taskId: %w", err)
	}

	target, err := ParseHash160(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse target: %w", err)
	}

	method, err := ParseStringFromItem(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse method: %w", err)
	}

	return &AutomationAnchorTaskRegisteredEvent{
		TaskID: taskID,
		Target: target,
		Method: method,
	}, nil
}

// AutomationAnchorExecutedEvent represents an Executed event.
// Event: Executed(taskId, nonce, txHash)
type AutomationAnchorExecutedEvent struct {
	TaskID []byte
	Nonce  *big.Int
	TxHash []byte
}

func ParseAutomationAnchorExecutedEvent(event *ContractEvent) (*AutomationAnchorExecutedEvent, error) {
	if event.EventName != "Executed" {
		return nil, fmt.Errorf("not an Executed event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state: expected 3 items, got %d", len(event.State))
	}

	taskID, err := ParseByteArray(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse taskId: %w", err)
	}

	nonce, err := ParseInteger(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse nonce: %w", err)
	}

	txHash, err := ParseByteArray(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse txHash: %w", err)
	}

	return &AutomationAnchorExecutedEvent{
		TaskID: taskID,
		Nonce:  nonce,
		TxHash: txHash,
	}, nil
}
