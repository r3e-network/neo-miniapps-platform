package chain

import (
	"context"
	"math/big"
)

// =============================================================================
// NeoFlow Contract Interface (Trigger-Based Pattern)
// =============================================================================

// NeoFlowContract provides interaction with the NeoFlowService contract.
// This contract implements the Trigger pattern - users register triggers, TEE monitors conditions,
// and executes callbacks when conditions are met.
type NeoFlowContract struct {
	client       *Client
	contractHash string
	wallet       *Wallet
}

// Trigger types (matching NeoFlowService contract).
const (
	NeoFlowTriggerTypeTime      uint8 = 1 // Cron-based time trigger
	NeoFlowTriggerTypePrice     uint8 = 2 // Price threshold trigger
	NeoFlowTriggerTypeEvent     uint8 = 3 // On-chain event trigger
	NeoFlowTriggerTypeThreshold uint8 = 4 // Balance/value threshold trigger
)

// Trigger status (matching NeoFlowService contract).
const (
	NeoFlowTriggerStatusActive    uint8 = 1
	NeoFlowTriggerStatusPaused    uint8 = 2
	NeoFlowTriggerStatusCancelled uint8 = 3
	NeoFlowTriggerStatusExpired   uint8 = 4
)

// NewNeoFlowContract creates a new NeoFlow contract interface.
func NewNeoFlowContract(client *Client, contractHash string, wallet *Wallet) *NeoFlowContract {
	return &NeoFlowContract{
		client:       client,
		contractHash: contractHash,
		wallet:       wallet,
	}
}

// GetTrigger returns a trigger by ID.
func (a *NeoFlowContract) GetTrigger(ctx context.Context, triggerID *big.Int) (*Trigger, error) {
	return InvokeStruct(ctx, a.client, a.contractHash, "getTrigger", ParseTrigger, NewIntegerParam(triggerID))
}

// CanExecute checks if a trigger can be executed.
func (a *NeoFlowContract) CanExecute(ctx context.Context, triggerID *big.Int) (bool, error) {
	return InvokeBool(ctx, a.client, a.contractHash, "canExecute", NewIntegerParam(triggerID))
}

// GetExecution returns an execution record.
func (a *NeoFlowContract) GetExecution(ctx context.Context, triggerID, executionNumber *big.Int) (*ExecutionRecord, error) {
	return InvokeStruct(ctx, a.client, a.contractHash, "getExecution", ParseExecutionRecord, NewIntegerParam(triggerID), NewIntegerParam(executionNumber))
}

// IsTEEAccount checks if an account is a registered TEE account.
func (a *NeoFlowContract) IsTEEAccount(ctx context.Context, account string) (bool, error) {
	return IsTEEAccount(ctx, a.client, a.contractHash, account)
}

