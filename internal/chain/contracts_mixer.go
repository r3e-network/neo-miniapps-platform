package chain

import (
	"context"
	"fmt"
	"math/big"
)

// =============================================================================
// Mixer Contract Interface
// =============================================================================

// MixerContract provides interaction with the MixerService contract.
type MixerContract struct {
	client       *Client
	contractHash string
	wallet       *Wallet
}

// NewMixerContract creates a new mixer contract interface.
func NewMixerContract(client *Client, contractHash string, wallet *Wallet) *MixerContract {
	return &MixerContract{
		client:       client,
		contractHash: contractHash,
		wallet:       wallet,
	}
}

// GetPool returns pool information for a denomination.
func (m *MixerContract) GetPool(ctx context.Context, denomination *big.Int) (*MixerPool, error) {
	params := []ContractParam{NewIntegerParam(denomination)}
	result, err := m.client.InvokeFunction(ctx, m.contractHash, "getPool", params)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("no result")
	}
	return parseMixerPool(result.Stack[0])
}

// IsNullifierUsed checks if a nullifier has been used.
func (m *MixerContract) IsNullifierUsed(ctx context.Context, nullifier []byte) (bool, error) {
	params := []ContractParam{NewByteArrayParam(nullifier)}
	result, err := m.client.InvokeFunction(ctx, m.contractHash, "isNullifierUsed", params)
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

// IsCommitmentExists checks if a commitment exists.
func (m *MixerContract) IsCommitmentExists(ctx context.Context, commitment []byte) (bool, error) {
	params := []ContractParam{NewByteArrayParam(commitment)}
	result, err := m.client.InvokeFunction(ctx, m.contractHash, "isCommitmentExists", params)
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

// GetMerkleRoot returns the current Merkle root for a pool.
func (m *MixerContract) GetMerkleRoot(ctx context.Context, denomination *big.Int) ([]byte, error) {
	params := []ContractParam{NewIntegerParam(denomination)}
	result, err := m.client.InvokeFunction(ctx, m.contractHash, "getMerkleRoot", params)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("no result")
	}
	return parseByteArray(result.Stack[0])
}
