package neofeedschain

import (
	"context"
	"fmt"
	"math/big"
	"github.com/R3E-Network/service_layer/internal/chain"
)

// =============================================================================
// NeoFeeds Contract Interface (Push/Auto-Update Pattern)
// =============================================================================

// NeoFeedsContract provides interaction with the NeoFeedsService contract.
// This contract implements the Push pattern - TEE periodically updates prices.
type NeoFeedsContract struct {
	client       *chain.Client
	contractHash string
	wallet       *chain.Wallet
}

// NewNeoFeedsContract creates a new NeoFeeds contract interface.
func NewNeoFeedsContract(client *chain.Client, contractHash string, wallet *chain.Wallet) *NeoFeedsContract {
	return &NeoFeedsContract{
		client:       client,
		contractHash: contractHash,
		wallet:       wallet,
	}
}

// GetLatestPrice returns the latest price for a feed.
func (d *NeoFeedsContract) GetLatestPrice(ctx context.Context, feedID string) (*chain.PriceData, error) {
	params := []chain.ContractParam{chain.NewStringParam(feedID)}
	result, err := d.client.InvokeFunction(ctx, d.contractHash, "getLatestPrice", params)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("no result")
	}
	return chain.ParsePriceData(result.Stack[0])
}

// GetPrice returns the raw price value for a feed.
func (d *NeoFeedsContract) GetPrice(ctx context.Context, feedID string) (*big.Int, error) {
	params := []chain.ContractParam{chain.NewStringParam(feedID)}
	result, err := d.client.InvokeFunction(ctx, d.contractHash, "getPrice", params)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("no result")
	}
	return chain.ParseInteger(result.Stack[0])
}

// GetPriceTimestamp returns the timestamp of the latest price update.
func (d *NeoFeedsContract) GetPriceTimestamp(ctx context.Context, feedID string) (uint64, error) {
	params := []chain.ContractParam{chain.NewStringParam(feedID)}
	result, err := d.client.InvokeFunction(ctx, d.contractHash, "getPriceTimestamp", params)
	if err != nil {
		return 0, err
	}
	if result.State != "HALT" {
		return 0, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return 0, fmt.Errorf("no result")
	}
	ts, err := chain.ParseInteger(result.Stack[0])
	if err != nil {
		return 0, err
	}
	return ts.Uint64(), nil
}

// IsPriceFresh checks if the price is within the staleness threshold.
func (d *NeoFeedsContract) IsPriceFresh(ctx context.Context, feedID string) (bool, error) {
	params := []chain.ContractParam{chain.NewStringParam(feedID)}
	result, err := d.client.InvokeFunction(ctx, d.contractHash, "isPriceFresh", params)
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

// GetFeedConfig returns the configuration for a price feed.
func (d *NeoFeedsContract) GetFeedConfig(ctx context.Context, feedID string) (*chain.ContractFeedConfig, error) {
	params := []chain.ContractParam{chain.NewStringParam(feedID)}
	result, err := d.client.InvokeFunction(ctx, d.contractHash, "getFeedConfig", params)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("execution failed: %s", result.Exception)
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("no result")
	}
	return chain.ParseFeedConfig(result.Stack[0])
}

// IsTEEAccount checks if an account is a registered TEE account.
func (d *NeoFeedsContract) IsTEEAccount(ctx context.Context, account string) (bool, error) {
	params := []chain.ContractParam{chain.NewHash160Param(account)}
	result, err := d.client.InvokeFunction(ctx, d.contractHash, "isTEEAccount", params)
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
