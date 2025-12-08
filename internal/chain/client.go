// Package chain provides Neo N3 blockchain interaction for the Service Layer.
package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client provides Neo N3 RPC client functionality.
type Client struct {
	mu         sync.RWMutex
	rpcURL     string
	httpClient *http.Client
	networkID  uint32
}

// Config holds client configuration.
type Config struct {
	RPCURL    string
	NetworkID uint32 // MainNet: 860833102, TestNet: 894710606
	Timeout   time.Duration
}

// NewClient creates a new Neo N3 client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.RPCURL == "" {
		return nil, fmt.Errorf("RPC URL required")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		rpcURL: cfg.RPCURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		networkID: cfg.NetworkID,
	}, nil
}

// =============================================================================
// Core RPC Methods
// =============================================================================

// Call makes an RPC call to the Neo N3 node.
func (c *Client) Call(ctx context.Context, method string, params []interface{}) (json.RawMessage, error) {
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var rpcResp RPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}

	return rpcResp.Result, nil
}

// GetBlockCount returns the current block height.
func (c *Client) GetBlockCount(ctx context.Context) (uint64, error) {
	result, err := c.Call(ctx, "getblockcount", nil)
	if err != nil {
		return 0, err
	}

	var count uint64
	if err := json.Unmarshal(result, &count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetBlock returns a block by index or hash.
func (c *Client) GetBlock(ctx context.Context, indexOrHash interface{}) (*Block, error) {
	result, err := c.Call(ctx, "getblock", []interface{}{indexOrHash, true})
	if err != nil {
		return nil, err
	}

	var block Block
	if err := json.Unmarshal(result, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

// GetTransaction returns a transaction by hash.
func (c *Client) GetTransaction(ctx context.Context, txHash string) (*Transaction, error) {
	result, err := c.Call(ctx, "getrawtransaction", []interface{}{txHash, true})
	if err != nil {
		return nil, err
	}

	var tx Transaction
	if err := json.Unmarshal(result, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// GetApplicationLog returns the application log for a transaction.
func (c *Client) GetApplicationLog(ctx context.Context, txHash string) (*ApplicationLog, error) {
	result, err := c.Call(ctx, "getapplicationlog", []interface{}{txHash})
	if err != nil {
		return nil, err
	}

	var log ApplicationLog
	if err := json.Unmarshal(result, &log); err != nil {
		return nil, err
	}
	return &log, nil
}
