package chain

import (
	"context"
	"fmt"
	"math/big"
)

// =============================================================================
// DataFeeds Contract Interface (Push/Auto-Update Pattern)
// =============================================================================

// DataFeedsContract provides interaction with the DataFeedsService contract.
// This contract implements the Push pattern - TEE periodically updates prices.
type DataFeedsContract struct {
	client       *Client
	contractHash string
	wallet       *Wallet
}

// NewDataFeedsContract creates a new DataFeeds contract interface.
func NewDataFeedsContract(client *Client, contractHash string, wallet *Wallet) *DataFeedsContract {
	return &DataFeedsContract{
		client:       client,
		contractHash: contractHash,
		wallet:       wallet,
	}
}

// PriceData represents price data from the contract.
type PriceData struct {
	FeedID    string
	Price     *big.Int
	Decimals  *big.Int
	Timestamp uint64
	UpdatedBy string
}

// FeedConfig represents a price feed configuration.
type FeedConfig struct {
	FeedID      string
	Description string
	Decimals    *big.Int
	Active      bool
	CreatedAt   uint64
}

// GetLatestPrice returns the latest price for a feed.
func (d *DataFeedsContract) GetLatestPrice(ctx context.Context, feedID string) (*PriceData, error) {
	params := []ContractParam{NewStringParam(feedID)}
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
	return parsePriceData(result.Stack[0])
}

// GetPrice returns the raw price value for a feed.
func (d *DataFeedsContract) GetPrice(ctx context.Context, feedID string) (*big.Int, error) {
	params := []ContractParam{NewStringParam(feedID)}
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
	return parseInteger(result.Stack[0])
}

// GetPriceTimestamp returns the timestamp of the latest price update.
func (d *DataFeedsContract) GetPriceTimestamp(ctx context.Context, feedID string) (uint64, error) {
	params := []ContractParam{NewStringParam(feedID)}
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
	ts, err := parseInteger(result.Stack[0])
	if err != nil {
		return 0, err
	}
	return ts.Uint64(), nil
}

// IsPriceFresh checks if the price is within the staleness threshold.
func (d *DataFeedsContract) IsPriceFresh(ctx context.Context, feedID string) (bool, error) {
	params := []ContractParam{NewStringParam(feedID)}
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
	return parseBoolean(result.Stack[0])
}

// GetFeedConfig returns the configuration for a price feed.
func (d *DataFeedsContract) GetFeedConfig(ctx context.Context, feedID string) (*FeedConfig, error) {
	params := []ContractParam{NewStringParam(feedID)}
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
	return parseFeedConfig(result.Stack[0])
}

// IsTEEAccount checks if an account is a registered TEE account.
func (d *DataFeedsContract) IsTEEAccount(ctx context.Context, account string) (bool, error) {
	params := []ContractParam{NewHash160Param(account)}
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
	return parseBoolean(result.Stack[0])
}
