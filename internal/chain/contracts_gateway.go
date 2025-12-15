package chain

import (
	"context"
	"math/big"
)

// =============================================================================
// Gateway Contract Interface
// =============================================================================

// GatewayContract provides interaction with the ServiceLayerGateway contract.
type GatewayContract struct {
	client       *Client
	contractHash string
	wallet       *Wallet
}

// NewGatewayContract creates a new gateway contract interface.
func NewGatewayContract(client *Client, contractHash string, wallet *Wallet) *GatewayContract {
	return &GatewayContract{
		client:       client,
		contractHash: contractHash,
		wallet:       wallet,
	}
}

// GetAdmin returns the admin address.
func (g *GatewayContract) GetAdmin(ctx context.Context) (string, error) {
	return InvokeStruct(ctx, g.client, g.contractHash, "admin", ParseHash160)
}

// IsTEEAccount checks if an account is a registered TEE account.
func (g *GatewayContract) IsTEEAccount(ctx context.Context, account string) (bool, error) {
	return IsTEEAccount(ctx, g.client, g.contractHash, account)
}

// GetTEEPublicKey returns the TEE public key for a TEE account.
func (g *GatewayContract) GetTEEPublicKey(ctx context.Context, teeAccount string) ([]byte, error) {
	return InvokeStruct(ctx, g.client, g.contractHash, "getTEEPublicKey", ParseByteArray, NewHash160Param(teeAccount))
}

// GetServiceContract returns a registered service contract hash.
func (g *GatewayContract) GetServiceContract(ctx context.Context, serviceType string) (string, error) {
	return InvokeStruct(ctx, g.client, g.contractHash, "getServiceContract", ParseHash160, NewStringParam(serviceType))
}

// GetServiceFee returns the fee for a service type.
//
// Deprecated: On-chain fee handling is deprecated. Use gasbank.GetServiceFee() instead.
// Fee management has moved to off-chain Supabase-based system for better flexibility.
// This method is kept for backward compatibility with existing contracts.
func (g *GatewayContract) GetServiceFee(ctx context.Context, serviceType string) (*big.Int, error) {
	return InvokeInt(ctx, g.client, g.contractHash, "getServiceFee", NewStringParam(serviceType))
}

// BalanceOf returns user balance in the gateway.
//
// Deprecated: On-chain balance is deprecated. Use gasbank.Manager.GetBalance() instead.
// All balance management has moved to off-chain Supabase-based system.
// Users deposit directly to Service Layer account, not to gateway contract.
func (g *GatewayContract) BalanceOf(ctx context.Context, account string) (*big.Int, error) {
	return InvokeInt(ctx, g.client, g.contractHash, "balanceOf", NewHash160Param(account))
}

// GetRequest returns a service request by ID.
func (g *GatewayContract) GetRequest(ctx context.Context, requestID *big.Int) (*ContractServiceRequest, error) {
	return InvokeStruct(ctx, g.client, g.contractHash, "getRequest", ParseServiceRequest, NewIntegerParam(requestID))
}

// IsPaused returns whether the contract is paused.
func (g *GatewayContract) IsPaused(ctx context.Context) (bool, error) {
	return InvokeBool(ctx, g.client, g.contractHash, "paused")
}

// =============================================================================
// TEE Master Key Verification
// =============================================================================

// GetTEEMasterPubKey returns the anchored TEE master public key.
func (g *GatewayContract) GetTEEMasterPubKey(ctx context.Context) ([]byte, error) {
	return InvokeStruct(ctx, g.client, g.contractHash, "getTEEMasterPubKey", ParseByteArray)
}

// GetTEEMasterPubKeyHash returns the SHA-256 hash of the anchored TEE master public key.
func (g *GatewayContract) GetTEEMasterPubKeyHash(ctx context.Context) ([]byte, error) {
	return InvokeStruct(ctx, g.client, g.contractHash, "getTEEMasterPubKeyHash", ParseByteArray)
}

// GetTEEMasterAttestationHash returns the attestation hash/CID for the TEE master key.
func (g *GatewayContract) GetTEEMasterAttestationHash(ctx context.Context) ([]byte, error) {
	return InvokeStruct(ctx, g.client, g.contractHash, "getTEEMasterAttestationHash", ParseByteArray)
}
