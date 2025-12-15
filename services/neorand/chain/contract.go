// Package vrfchain provides VRF-specific chain interaction.
package vrfchain

import (
	"context"
	"math/big"

	"github.com/R3E-Network/service_layer/internal/chain"
)

// =============================================================================
// VRF Chain Module Registration
// =============================================================================

func init() {
	chain.RegisterServiceChain(&Module{})
}

// Module implements chain.ServiceChainModule for VRF service.
type Module struct {
	contract *VRFContract
}

// ServiceType returns the service type identifier.
func (m *Module) ServiceType() string {
	return "neorand"
}

// Initialize initializes the VRF chain module.
func (m *Module) Initialize(client *chain.Client, wallet *chain.Wallet, contractHash string) error {
	m.contract = NewVRFContract(client, contractHash, wallet)
	return nil
}

// Contract returns the VRF contract instance.
func (m *Module) Contract() *VRFContract {
	return m.contract
}

// =============================================================================
// VRF Contract Interface
// =============================================================================

// VRFContract provides interaction with the VRFService contract.
type VRFContract struct {
	client       *chain.Client
	contractHash string
	wallet       *chain.Wallet
}

// NewVRFContract creates a new VRF contract interface.
func NewVRFContract(client *chain.Client, contractHash string, wallet *chain.Wallet) *VRFContract {
	return &VRFContract{
		client:       client,
		contractHash: contractHash,
		wallet:       wallet,
	}
}

// GetRandomness returns the randomness for a VRF request.
func (v *VRFContract) GetRandomness(ctx context.Context, requestID *big.Int) ([]byte, error) {
	return chain.InvokeStruct(ctx, v.client, v.contractHash, "getRandomness", chain.ParseByteArray, chain.NewIntegerParam(requestID))
}

// GetProof returns the proof for a VRF request.
func (v *VRFContract) GetProof(ctx context.Context, requestID *big.Int) ([]byte, error) {
	return chain.InvokeStruct(ctx, v.client, v.contractHash, "getProof", chain.ParseByteArray, chain.NewIntegerParam(requestID))
}

// GetVRFPublicKey returns the VRF public key.
func (v *VRFContract) GetVRFPublicKey(ctx context.Context) ([]byte, error) {
	return chain.InvokeStruct(ctx, v.client, v.contractHash, "getVRFPublicKey", chain.ParseByteArray)
}

// VerifyProof verifies a VRF proof on-chain.
func (v *VRFContract) VerifyProof(ctx context.Context, seed, randomWords, proof []byte) (bool, error) {
	return chain.InvokeBool(ctx, v.client, v.contractHash, "verifyProof", chain.NewByteArrayParam(seed), chain.NewByteArrayParam(randomWords), chain.NewByteArrayParam(proof))
}
