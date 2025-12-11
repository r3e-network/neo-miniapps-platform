// Package vrfchain provides VRF-specific chain interaction.
package vrfchain

import (
	"context"
	"fmt"
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
	params := []chain.ContractParam{chain.NewIntegerParam(requestID)}
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
	return chain.ParseByteArray(result.Stack[0])
}

// GetProof returns the proof for a VRF request.
func (v *VRFContract) GetProof(ctx context.Context, requestID *big.Int) ([]byte, error) {
	params := []chain.ContractParam{chain.NewIntegerParam(requestID)}
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
	return chain.ParseByteArray(result.Stack[0])
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
	return chain.ParseByteArray(result.Stack[0])
}

// VerifyProof verifies a VRF proof on-chain.
func (v *VRFContract) VerifyProof(ctx context.Context, seed, randomWords, proof []byte) (bool, error) {
	params := []chain.ContractParam{
		chain.NewByteArrayParam(seed),
		chain.NewByteArrayParam(randomWords),
		chain.NewByteArrayParam(proof),
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
	return chain.ParseBoolean(result.Stack[0])
}
