package chain

import (
	"context"
	"fmt"
	"math/big"
)

// =============================================================================
// VRF Contract Interface
// =============================================================================

// VRFContract provides interaction with the VRFService contract.
type VRFContract struct {
	client       *Client
	contractHash string
	wallet       *Wallet
}

// NewVRFContract creates a new VRF contract interface.
func NewVRFContract(client *Client, contractHash string, wallet *Wallet) *VRFContract {
	return &VRFContract{
		client:       client,
		contractHash: contractHash,
		wallet:       wallet,
	}
}

// GetRandomness returns the randomness for a VRF request.
func (v *VRFContract) GetRandomness(ctx context.Context, requestID *big.Int) ([]byte, error) {
	params := []ContractParam{NewIntegerParam(requestID)}
	result, err := v.client.InvokeFunction(ctx, v.contractHash, "getRandomness", params)
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

// GetProof returns the proof for a VRF request.
func (v *VRFContract) GetProof(ctx context.Context, requestID *big.Int) ([]byte, error) {
	params := []ContractParam{NewIntegerParam(requestID)}
	result, err := v.client.InvokeFunction(ctx, v.contractHash, "getProof", params)
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

// GetVRFPublicKey returns the VRF public key.
func (v *VRFContract) GetVRFPublicKey(ctx context.Context) ([]byte, error) {
	result, err := v.client.InvokeFunction(ctx, v.contractHash, "getVRFPublicKey", nil)
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

// VerifyProof verifies a VRF proof on-chain.
func (v *VRFContract) VerifyProof(ctx context.Context, seed, randomWords, proof []byte) (bool, error) {
	params := []ContractParam{
		NewByteArrayParam(seed),
		NewByteArrayParam(randomWords),
		NewByteArrayParam(proof),
	}
	result, err := v.client.InvokeFunction(ctx, v.contractHash, "verifyProof", params)
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
