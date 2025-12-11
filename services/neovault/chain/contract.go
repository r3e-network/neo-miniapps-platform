package neovaultchain

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/R3E-Network/service_layer/internal/chain"
)

// =============================================================================
// NeoVault Contract Interface (v5.1 - Off-Chain First with On-Chain Dispute)
// =============================================================================
//
// Architecture: Off-Chain Mixing with On-Chain Dispute Resolution Only
// - User requests mix via API → NeoVault service directly (NO on-chain)
// - NeoVault returns RequestProof (requestHash + TEE signature) + deposit address
// - User deposits DIRECTLY to pool account on-chain (NOT gasbank)
// - NeoVault processes off-chain (HD pool accounts, random mixing)
// - NeoVault delivers NetAmount to targets (fee deducted from delivery)
// - Fee collected from random pool account to master fee address
// - Normal path: User happy, ZERO on-chain link to service layer
// - Dispute path: User submits dispute → NeoVault submits CompletionProof on-chain
//
// Contract Role (Minimal):
// - Service registration and bond management
// - Dispute submission by user
// - Dispute resolution by TEE (completion proof)
// - Refund if TEE fails to resolve within deadline

// NeoVaultContract provides interaction with the NeoVaultService contract.
type NeoVaultContract struct {
	client       *chain.Client
	contractHash string
	wallet       *chain.Wallet
}

// NewNeoVaultContract creates a new neovault contract interface.
func NewNeoVaultContract(client *chain.Client, contractHash string, wallet *chain.Wallet) *NeoVaultContract {
	return &NeoVaultContract{
		client:       client,
		contractHash: contractHash,
		wallet:       wallet,
	}
}

// =============================================================================
// Read Methods
// =============================================================================

// GetAdmin returns the contract admin address.
func (m *NeoVaultContract) GetAdmin(ctx context.Context) (string, error) {
	result, err := m.client.InvokeFunction(ctx, m.contractHash, "getAdmin", nil)
	if err != nil {
		return "", err
	}
	if result.State != "HALT" {
		return "", fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return "", fmt.Errorf("no result")
	}
	return chain.ParseHash160(result.Stack[0])
}

// IsPaused returns whether the contract is paused.
func (m *NeoVaultContract) IsPaused(ctx context.Context) (bool, error) {
	result, err := m.client.InvokeFunction(ctx, m.contractHash, "isPaused", nil)
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

// GetService returns service information by service ID.
func (m *NeoVaultContract) GetService(ctx context.Context, serviceID []byte) (*NeoVaultServiceInfo, error) {
	params := []chain.ContractParam{chain.NewByteArrayParam(serviceID)}
	result, err := m.client.InvokeFunction(ctx, m.contractHash, "getService", params)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("no result")
	}
	return parseNeoVaultServiceInfo(result.Stack[0])
}

// GetDispute returns dispute information by request hash.
func (m *NeoVaultContract) GetDispute(ctx context.Context, requestHash []byte) (*NeoVaultDisputeInfo, error) {
	params := []chain.ContractParam{chain.NewByteArrayParam(requestHash)}
	result, err := m.client.InvokeFunction(ctx, m.contractHash, "getDispute", params)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("no result")
	}
	return parseNeoVaultDisputeInfo(result.Stack[0])
}

// IsDisputeResolved checks if a dispute has been resolved.
func (m *NeoVaultContract) IsDisputeResolved(ctx context.Context, requestHash []byte) (bool, error) {
	params := []chain.ContractParam{chain.NewByteArrayParam(requestHash)}
	result, err := m.client.InvokeFunction(ctx, m.contractHash, "isDisputeResolved", params)
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

// =============================================================================
// Write Methods (TEE Only)
// =============================================================================

// ResolveDispute submits completion proof to resolve a dispute.
// This method can only be called by the registered TEE service.
// Returns the transaction hash after waiting for execution (2 minute timeout).
func (m *NeoVaultContract) ResolveDispute(ctx context.Context, requestHash, outputsHash, proofSignature []byte) (string, error) {
	if m.wallet == nil {
		return "", fmt.Errorf("wallet required for write operations")
	}

	params := []chain.ContractParam{
		chain.NewByteArrayParam(requestHash),
		chain.NewByteArrayParam(outputsHash),
		chain.NewByteArrayParam(proofSignature),
	}

	txResult, err := m.client.InvokeFunctionAndWait(ctx, m.contractHash, "resolveDispute", params, true)
	if err != nil {
		return "", err
	}

	return txResult.TxHash, nil
}

// =============================================================================
// Types
// =============================================================================

// NeoVaultServiceInfo represents registered service information.
type NeoVaultServiceInfo struct {
	ServiceID  []byte
	TeePubKey  []byte
	Bond       *big.Int
	Active     bool
	RegisterAt uint64
}

// NeoVaultDisputeInfo represents dispute information.
type NeoVaultDisputeInfo struct {
	RequestHash    []byte
	User           string
	Amount         *big.Int
	ServiceID      []byte
	Deadline       uint64
	Status         uint8 // 0=Pending, 1=Resolved, 2=Refunded
	SubmittedAt    uint64
	OutputsHash    []byte // Set when resolved
	ProofSignature []byte // Set when resolved
}

// DisputeStatus constants
const (
	DisputeStatusPending  uint8 = 0
	DisputeStatusResolved uint8 = 1
	DisputeStatusRefunded uint8 = 2
)

// =============================================================================
// Parsers
// =============================================================================

// parseNeoVaultServiceInfo parses service info from contract result.
func parseNeoVaultServiceInfo(item chain.StackItem) (*NeoVaultServiceInfo, error) {
	if item.Type != "Array" && item.Type != "Struct" {
		return nil, fmt.Errorf("expected Array or Struct, got %s", item.Type)
	}

	var arr []chain.StackItem
	if err := json.Unmarshal(item.Value, &arr); err != nil {
		return nil, fmt.Errorf("unmarshal array: %w", err)
	}
	if len(arr) < 5 {
		return nil, fmt.Errorf("invalid NeoVaultServiceInfo: expected 5 items, got %d", len(arr))
	}

	serviceID, err := chain.ParseByteArray(arr[0])
	if err != nil {
		return nil, fmt.Errorf("parse serviceId: %w", err)
	}

	teePubKey, err := chain.ParseByteArray(arr[1])
	if err != nil {
		return nil, fmt.Errorf("parse teePubKey: %w", err)
	}

	bond, err := chain.ParseInteger(arr[2])
	if err != nil {
		return nil, fmt.Errorf("parse bond: %w", err)
	}

	active, err := chain.ParseBoolean(arr[3])
	if err != nil {
		return nil, fmt.Errorf("parse active: %w", err)
	}

	registerAt, err := chain.ParseInteger(arr[4])
	if err != nil {
		return nil, fmt.Errorf("parse registerAt: %w", err)
	}

	return &NeoVaultServiceInfo{
		ServiceID:  serviceID,
		TeePubKey:  teePubKey,
		Bond:       bond,
		Active:     active,
		RegisterAt: registerAt.Uint64(),
	}, nil
}

// parseNeoVaultDisputeInfo parses dispute info from contract result.
func parseNeoVaultDisputeInfo(item chain.StackItem) (*NeoVaultDisputeInfo, error) {
	if item.Type != "Array" && item.Type != "Struct" {
		return nil, fmt.Errorf("expected Array or Struct, got %s", item.Type)
	}

	var arr []chain.StackItem
	if err := json.Unmarshal(item.Value, &arr); err != nil {
		return nil, fmt.Errorf("unmarshal array: %w", err)
	}
	if len(arr) < 8 {
		return nil, fmt.Errorf("invalid NeoVaultDisputeInfo: expected 8 items, got %d", len(arr))
	}

	requestHash, err := chain.ParseByteArray(arr[0])
	if err != nil {
		return nil, fmt.Errorf("parse requestHash: %w", err)
	}

	user, err := chain.ParseHash160(arr[1])
	if err != nil {
		return nil, fmt.Errorf("parse user: %w", err)
	}

	amount, err := chain.ParseInteger(arr[2])
	if err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}

	serviceID, err := chain.ParseByteArray(arr[3])
	if err != nil {
		return nil, fmt.Errorf("parse serviceId: %w", err)
	}

	deadline, err := chain.ParseInteger(arr[4])
	if err != nil {
		return nil, fmt.Errorf("parse deadline: %w", err)
	}

	status, err := chain.ParseInteger(arr[5])
	if err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}

	submittedAt, err := chain.ParseInteger(arr[6])
	if err != nil {
		return nil, fmt.Errorf("parse submittedAt: %w", err)
	}

	// Optional fields (may be empty if not resolved)
	var outputsHash, proofSignature []byte
	if len(arr) > 7 {
		outputsHash, _ = chain.ParseByteArray(arr[7])
	}
	if len(arr) > 8 {
		proofSignature, _ = chain.ParseByteArray(arr[8])
	}

	return &NeoVaultDisputeInfo{
		RequestHash:    requestHash,
		User:           user,
		Amount:         amount,
		ServiceID:      serviceID,
		Deadline:       deadline.Uint64(),
		Status:         uint8(status.Uint64()),
		SubmittedAt:    submittedAt.Uint64(),
		OutputsHash:    outputsHash,
		ProofSignature: proofSignature,
	}, nil
}
