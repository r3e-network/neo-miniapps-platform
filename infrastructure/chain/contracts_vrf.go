package chain

import (
	"context"
	"math/big"
)

// =============================================================================
// VRF Contract Interface (Request-Response Pattern)
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
	return InvokeStruct(ctx, v.client, v.contractHash, "getRandomness", ParseByteArray, NewIntegerParam(requestID))
}

// GetProof returns the proof for a VRF request.
func (v *VRFContract) GetProof(ctx context.Context, requestID *big.Int) ([]byte, error) {
	return InvokeStruct(ctx, v.client, v.contractHash, "getProof", ParseByteArray, NewIntegerParam(requestID))
}

// GetVRFPublicKey returns the VRF public key.
func (v *VRFContract) GetVRFPublicKey(ctx context.Context) ([]byte, error) {
	return InvokeStruct(ctx, v.client, v.contractHash, "getVRFPublicKey", ParseByteArray)
}

// VerifyProof verifies a VRF proof on-chain.
func (v *VRFContract) VerifyProof(ctx context.Context, seed, randomWords, proof []byte) (bool, error) {
	return InvokeBool(ctx, v.client, v.contractHash, "verifyProof", NewByteArrayParam(seed), NewByteArrayParam(randomWords), NewByteArrayParam(proof))
}

