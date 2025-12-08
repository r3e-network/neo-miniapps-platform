package chain

import (
	"context"
	"fmt"
	"math/big"
)

// =============================================================================
// Automation Contract Interface (Trigger-Based Pattern)
// =============================================================================

// AutomationContract provides interaction with the AutomationService contract.
// This contract implements the Trigger pattern - users register triggers,
// TEE monitors conditions and executes callbacks.
type AutomationContract struct {
	client       *Client
	contractHash string
	wallet       *Wallet
}

// NewAutomationContract creates a new Automation contract interface.
func NewAutomationContract(client *Client, contractHash string, wallet *Wallet) *AutomationContract {
	return &AutomationContract{
		client:       client,
		contractHash: contractHash,
		wallet:       wallet,
	}
}

// Trigger types
const (
	TriggerTypeTime      uint8 = 1 // Cron-based time trigger
	TriggerTypePrice     uint8 = 2 // Price threshold trigger
	TriggerTypeEvent     uint8 = 3 // On-chain event trigger
	TriggerTypeThreshold uint8 = 4 // Balance/value threshold
)

// Trigger status
const (
	TriggerStatusActive    uint8 = 1
	TriggerStatusPaused    uint8 = 2
	TriggerStatusCancelled uint8 = 3
	TriggerStatusExpired   uint8 = 4
)

// Trigger represents a trigger from the contract.
type Trigger struct {
	TriggerID      *big.Int
	RequestID      *big.Int
	Owner          string
	TargetContract string
	CallbackMethod string
	TriggerType    uint8
	Condition      string
	CallbackData   []byte
	MaxExecutions  *big.Int
	ExecutionCount *big.Int
	Status         uint8
	CreatedAt      uint64
	LastExecutedAt uint64
	ExpiresAt      uint64
}

// ExecutionRecord represents an execution record from the contract.
type ExecutionRecord struct {
	TriggerID       *big.Int
	ExecutionNumber *big.Int
	Timestamp       uint64
	Success         bool
	ExecutedBy      string
}

// GetTrigger returns a trigger by ID.
func (a *AutomationContract) GetTrigger(ctx context.Context, triggerID *big.Int) (*Trigger, error) {
	params := []ContractParam{NewIntegerParam(triggerID)}
	result, err := a.client.InvokeFunction(ctx, a.contractHash, "getTrigger", params)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("no result")
	}
	return parseTrigger(result.Stack[0])
}

// CanExecute checks if a trigger can be executed.
func (a *AutomationContract) CanExecute(ctx context.Context, triggerID *big.Int) (bool, error) {
	params := []ContractParam{NewIntegerParam(triggerID)}
	result, err := a.client.InvokeFunction(ctx, a.contractHash, "canExecute", params)
	if err != nil {
		return false, err
	}
	if result.State != "HALT" {
		return false, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return false, fmt.Errorf("no result")
	}
	return parseBoolean(result.Stack[0])
}

// GetExecution returns an execution record.
func (a *AutomationContract) GetExecution(ctx context.Context, triggerID, executionNumber *big.Int) (*ExecutionRecord, error) {
	params := []ContractParam{
		NewIntegerParam(triggerID),
		NewIntegerParam(executionNumber),
	}
	result, err := a.client.InvokeFunction(ctx, a.contractHash, "getExecution", params)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("no result")
	}
	return parseExecutionRecord(result.Stack[0])
}

// IsTEEAccount checks if an account is a registered TEE account.
func (a *AutomationContract) IsTEEAccount(ctx context.Context, account string) (bool, error) {
	params := []ContractParam{NewHash160Param(account)}
	result, err := a.client.InvokeFunction(ctx, a.contractHash, "isTEEAccount", params)
	if err != nil {
		return false, err
	}
	if result.State != "HALT" {
		return false, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return false, fmt.Errorf("no result")
	}
	return parseBoolean(result.Stack[0])
}
