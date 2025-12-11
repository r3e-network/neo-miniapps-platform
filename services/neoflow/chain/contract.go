package neoflowchain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/R3E-Network/service_layer/internal/chain"
)

// =============================================================================
// NeoFlow Contract Interface (Trigger-Based Pattern)
// =============================================================================

// NeoFlowContract provides interaction with the NeoFlowService contract.
// This contract implements the Trigger pattern - users register triggers,
// TEE monitors conditions and executes callbacks.
type NeoFlowContract struct {
	client       *chain.Client
	contractHash string
	wallet       *chain.Wallet
}

// NewNeoFlowContract creates a new NeoFlow contract interface.
func NewNeoFlowContract(client *chain.Client, contractHash string, wallet *chain.Wallet) *NeoFlowContract {
	return &NeoFlowContract{
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

// GetTrigger returns a trigger by ID.
func (a *NeoFlowContract) GetTrigger(ctx context.Context, triggerID *big.Int) (*chain.Trigger, error) {
	params := []chain.ContractParam{chain.NewIntegerParam(triggerID)}
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
	return chain.ParseTrigger(result.Stack[0])
}

// CanExecute checks if a trigger can be executed.
func (a *NeoFlowContract) CanExecute(ctx context.Context, triggerID *big.Int) (bool, error) {
	params := []chain.ContractParam{chain.NewIntegerParam(triggerID)}
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
	return chain.ParseBoolean(result.Stack[0])
}

// GetExecution returns an execution record.
func (a *NeoFlowContract) GetExecution(ctx context.Context, triggerID, executionNumber *big.Int) (*chain.ExecutionRecord, error) {
	params := []chain.ContractParam{
		chain.NewIntegerParam(triggerID),
		chain.NewIntegerParam(executionNumber),
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
	return chain.ParseExecutionRecord(result.Stack[0])
}

// IsTEEAccount checks if an account is a registered TEE account.
func (a *NeoFlowContract) IsTEEAccount(ctx context.Context, account string) (bool, error) {
	params := []chain.ContractParam{chain.NewHash160Param(account)}
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
	return chain.ParseBoolean(result.Stack[0])
}
