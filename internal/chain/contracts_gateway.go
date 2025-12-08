package chain

import (
	"context"
	"fmt"
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
	result, err := g.client.InvokeFunction(ctx, g.contractHash, "admin", nil)
	if err != nil {
		return "", err
	}
	if result.State != "HALT" {
		return "", fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return "", fmt.Errorf("no result")
	}
	return ParseHash160(result.Stack[0])
}

// IsTEEAccount checks if an account is a registered TEE account.
func (g *GatewayContract) IsTEEAccount(ctx context.Context, account string) (bool, error) {
	params := []ContractParam{NewHash160Param(account)}
	result, err := g.client.InvokeFunction(ctx, g.contractHash, "isTEEAccount", params)
	if err != nil {
		return false, err
	}
	if result.State != "HALT" {
		return false, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return false, fmt.Errorf("no result")
	}
	return ParseBoolean(result.Stack[0])
}

// GetTEEPublicKey returns the TEE public key for a TEE account.
func (g *GatewayContract) GetTEEPublicKey(ctx context.Context, teeAccount string) ([]byte, error) {
	params := []ContractParam{NewHash160Param(teeAccount)}
	result, err := g.client.InvokeFunction(ctx, g.contractHash, "getTEEPublicKey", params)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("no result")
	}
	return ParseByteArray(result.Stack[0])
}

// GetServiceContract returns a registered service contract hash.
func (g *GatewayContract) GetServiceContract(ctx context.Context, serviceType string) (string, error) {
	params := []ContractParam{NewStringParam(serviceType)}
	result, err := g.client.InvokeFunction(ctx, g.contractHash, "getServiceContract", params)
	if err != nil {
		return "", err
	}
	if result.State != "HALT" {
		return "", fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return "", fmt.Errorf("no result")
	}
	return ParseHash160(result.Stack[0])
}

// GetServiceFee returns the fee for a service type.
//
// DEPRECATED: On-chain fee handling is deprecated. Use gasbank.GetServiceFee() instead.
// Fee management has moved to off-chain Supabase-based system for better flexibility.
// This method is kept for backward compatibility with existing contracts.
func (g *GatewayContract) GetServiceFee(ctx context.Context, serviceType string) (*big.Int, error) {
	params := []ContractParam{NewStringParam(serviceType)}
	result, err := g.client.InvokeFunction(ctx, g.contractHash, "getServiceFee", params)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("no result")
	}
	return ParseInteger(result.Stack[0])
}

// BalanceOf returns user balance in the gateway.
//
// DEPRECATED: On-chain balance is deprecated. Use gasbank.Manager.GetBalance() instead.
// All balance management has moved to off-chain Supabase-based system.
// Users deposit directly to Service Layer account, not to gateway contract.
func (g *GatewayContract) BalanceOf(ctx context.Context, account string) (*big.Int, error) {
	params := []ContractParam{NewHash160Param(account)}
	result, err := g.client.InvokeFunction(ctx, g.contractHash, "balanceOf", params)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("no result")
	}
	return ParseInteger(result.Stack[0])
}

// GetRequest returns a service request by ID.
func (g *GatewayContract) GetRequest(ctx context.Context, requestID *big.Int) (*ContractServiceRequest, error) {
	params := []ContractParam{NewIntegerParam(requestID)}
	result, err := g.client.InvokeFunction(ctx, g.contractHash, "getRequest", params)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("no result")
	}
	return ParseServiceRequest(result.Stack[0])
}

// IsPaused returns whether the contract is paused.
func (g *GatewayContract) IsPaused(ctx context.Context) (bool, error) {
	result, err := g.client.InvokeFunction(ctx, g.contractHash, "paused", nil)
	if err != nil {
		return false, err
	}
	if result.State != "HALT" {
		return false, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return false, fmt.Errorf("no result")
	}
	return ParseBoolean(result.Stack[0])
}
