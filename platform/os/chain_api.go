// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// chainAPIImpl implements ChainAPI.
// Provides blockchain interaction capabilities through the TEE.
type chainAPIImpl struct {
	ctx       *ServiceContext
	serviceID string

	mu            sync.RWMutex
	subscriptions map[string]*chainSubscription
}

func newChainAPI(ctx *ServiceContext, serviceID string) *chainAPIImpl {
	return &chainAPIImpl{
		ctx:           ctx,
		serviceID:     serviceID,
		subscriptions: make(map[string]*chainSubscription),
	}
}

// chainSubscription represents an active event subscription.
type chainSubscription struct {
	id       string
	chainID  string
	filter   *EventFilter
	handler  func(*ChainEvent)
	ctx      context.Context
	cancel   context.CancelFunc
	active   bool
}

func (s *chainSubscription) Unsubscribe() error {
	s.cancel()
	s.active = false
	return nil
}

func (c *chainAPIImpl) GetBlock(ctx context.Context, chainID string, blockRef string) (*Block, error) {
	if err := c.ctx.RequireCapability(CapChain); err != nil {
		return nil, err
	}

	// Build RPC request based on chain type
	var req HTTPRequest
	var method string

	switch chainID {
	case "neo", "neo-mainnet", "neo-testnet":
		method = "getblock"
		req = c.buildNeoRPCRequest(method, []any{blockRef, 1})
	case "ethereum", "eth-mainnet", "eth-sepolia":
		method = "eth_getBlockByNumber"
		req = c.buildEthRPCRequest(method, []any{blockRef, true})
	default:
		// Generic JSON-RPC
		method = "getblock"
		req = c.buildGenericRPCRequest(chainID, method, []any{blockRef})
	}

	resp, err := c.ctx.Network().Fetch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("RPC error: status %d", resp.StatusCode)
	}

	// Parse response
	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp.Body, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Parse block based on chain type
	block := &Block{
		ChainID: chainID,
	}

	if err := json.Unmarshal(rpcResp.Result, block); err != nil {
		// Try to extract basic fields
		var rawBlock map[string]any
		if err := json.Unmarshal(rpcResp.Result, &rawBlock); err == nil {
			if hash, ok := rawBlock["hash"].(string); ok {
				block.Hash = hash
			}
			if height, ok := rawBlock["index"].(float64); ok {
				block.Height = uint64(height)
			} else if height, ok := rawBlock["number"].(string); ok {
				// Ethereum hex format
				fmt.Sscanf(height, "0x%x", &block.Height)
			}
		}
	}

	return block, nil
}

func (c *chainAPIImpl) GetTransaction(ctx context.Context, chainID string, txHash string) (*Transaction, error) {
	if err := c.ctx.RequireCapability(CapChain); err != nil {
		return nil, err
	}

	var req HTTPRequest
	var method string

	switch chainID {
	case "neo", "neo-mainnet", "neo-testnet":
		method = "getrawtransaction"
		req = c.buildNeoRPCRequest(method, []any{txHash, 1})
	case "ethereum", "eth-mainnet", "eth-sepolia":
		method = "eth_getTransactionByHash"
		req = c.buildEthRPCRequest(method, []any{txHash})
	default:
		method = "gettransaction"
		req = c.buildGenericRPCRequest(chainID, method, []any{txHash})
	}

	resp, err := c.ctx.Network().Fetch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transaction: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("RPC error: status %d", resp.StatusCode)
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp.Body, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	tx := &Transaction{
		ChainID: chainID,
	}

	if err := json.Unmarshal(rpcResp.Result, tx); err != nil {
		var rawTx map[string]any
		if err := json.Unmarshal(rpcResp.Result, &rawTx); err == nil {
			if hash, ok := rawTx["hash"].(string); ok {
				tx.Hash = hash
			}
		}
	}

	return tx, nil
}

func (c *chainAPIImpl) GetBalance(ctx context.Context, chainID string, address string, asset string) (*Balance, error) {
	if err := c.ctx.RequireCapability(CapChain); err != nil {
		return nil, err
	}

	var req HTTPRequest
	var method string

	switch chainID {
	case "neo", "neo-mainnet", "neo-testnet":
		method = "getnep17balances"
		req = c.buildNeoRPCRequest(method, []any{address})
	case "ethereum", "eth-mainnet", "eth-sepolia":
		if asset == "" || asset == "ETH" {
			method = "eth_getBalance"
			req = c.buildEthRPCRequest(method, []any{address, "latest"})
		} else {
			// ERC20 balance - would need contract call
			return nil, fmt.Errorf("ERC20 balance queries not yet implemented")
		}
	default:
		method = "getbalance"
		req = c.buildGenericRPCRequest(chainID, method, []any{address, asset})
	}

	resp, err := c.ctx.Network().Fetch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch balance: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("RPC error: status %d", resp.StatusCode)
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp.Body, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	balance := &Balance{
		ChainID: chainID,
		Address: address,
		Asset:   asset,
	}

	// Parse balance based on chain type
	switch chainID {
	case "ethereum", "eth-mainnet", "eth-sepolia":
		var hexBalance string
		if err := json.Unmarshal(rpcResp.Result, &hexBalance); err == nil {
			balance.Amount = hexBalance
		}
	default:
		if err := json.Unmarshal(rpcResp.Result, balance); err != nil {
			balance.Amount = string(rpcResp.Result)
		}
	}

	return balance, nil
}

func (c *chainAPIImpl) SendTransaction(ctx context.Context, chainID string, signedTx []byte) (string, error) {
	if err := c.ctx.RequireCapability(CapChain); err != nil {
		return "", err
	}

	var req HTTPRequest
	var method string
	txHex := fmt.Sprintf("%x", signedTx)

	switch chainID {
	case "neo", "neo-mainnet", "neo-testnet":
		method = "sendrawtransaction"
		req = c.buildNeoRPCRequest(method, []any{txHex})
	case "ethereum", "eth-mainnet", "eth-sepolia":
		method = "eth_sendRawTransaction"
		req = c.buildEthRPCRequest(method, []any{"0x" + txHex})
	default:
		method = "sendrawtransaction"
		req = c.buildGenericRPCRequest(chainID, method, []any{txHex})
	}

	resp, err := c.ctx.Network().Fetch(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("RPC error: status %d", resp.StatusCode)
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp.Body, &rpcResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if rpcResp.Error != nil {
		return "", fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	var txHash string
	if err := json.Unmarshal(rpcResp.Result, &txHash); err != nil {
		// Try to extract from object
		var result map[string]any
		if err := json.Unmarshal(rpcResp.Result, &result); err == nil {
			if hash, ok := result["hash"].(string); ok {
				txHash = hash
			}
		}
	}

	return txHash, nil
}

func (c *chainAPIImpl) CallContract(ctx context.Context, chainID string, call *ContractCall) ([]byte, error) {
	if err := c.ctx.RequireCapability(CapChain); err != nil {
		return nil, err
	}

	var req HTTPRequest
	var method string

	switch chainID {
	case "neo", "neo-mainnet", "neo-testnet":
		method = "invokefunction"
		params := []any{call.Contract, call.Method}
		if len(call.Args) > 0 {
			params = append(params, call.Args)
		}
		req = c.buildNeoRPCRequest(method, params)
	case "ethereum", "eth-mainnet", "eth-sepolia":
		method = "eth_call"
		callObj := map[string]any{
			"to": call.Contract,
		}
		if call.Caller != "" {
			callObj["from"] = call.Caller
		}
		// Encode method and args as data if needed
		if len(call.Args) > 0 {
			// For Ethereum, args would need ABI encoding
			// This is a simplified version
			callObj["data"] = call.Method
		}
		req = c.buildEthRPCRequest(method, []any{callObj, "latest"})
	default:
		method = "invokefunction"
		req = c.buildGenericRPCRequest(chainID, method, []any{call.Contract, call.Method, call.Args})
	}

	resp, err := c.ctx.Network().Fetch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to call contract: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("RPC error: status %d", resp.StatusCode)
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp.Body, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

func (c *chainAPIImpl) InvokeContract(ctx context.Context, chainID string, invoke *ContractInvoke) (string, error) {
	if err := c.ctx.RequireCapability(CapChain); err != nil {
		return "", err
	}

	// For invoke, we need to sign and send the transaction
	// This requires a signer address
	if invoke.Signer == "" {
		return "", fmt.Errorf("signer is required for contract invocation")
	}

	// Build the transaction based on chain type
	var req HTTPRequest

	switch chainID {
	case "neo", "neo-mainnet", "neo-testnet":
		// Neo requires building a transaction script
		params := []any{invoke.Contract, invoke.Method}
		if len(invoke.Args) > 0 {
			params = append(params, invoke.Args)
		}
		params = append(params, []map[string]string{{"account": invoke.Signer}})
		req = c.buildNeoRPCRequest("invokefunction", params)
	default:
		return "", fmt.Errorf("contract invocation not implemented for chain: %s", chainID)
	}

	resp, err := c.ctx.Network().Fetch(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to invoke contract: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("RPC error: status %d", resp.StatusCode)
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp.Body, &rpcResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if rpcResp.Error != nil {
		return "", fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Extract transaction hash from result
	var result map[string]any
	if err := json.Unmarshal(rpcResp.Result, &result); err == nil {
		if hash, ok := result["hash"].(string); ok {
			return hash, nil
		}
	}

	return "", fmt.Errorf("no transaction hash in response")
}

func (c *chainAPIImpl) SubscribeEvents(ctx context.Context, chainID string, filter *EventFilter, handler func(*ChainEvent)) (Subscription, error) {
	if err := c.ctx.RequireCapability(CapChain); err != nil {
		return nil, err
	}

	// Create subscription
	subCtx, cancel := context.WithCancel(ctx)
	sub := &chainSubscription{
		id:      generateSubscriptionID(),
		chainID: chainID,
		filter:  filter,
		handler: handler,
		ctx:     subCtx,
		cancel:  cancel,
		active:  true,
	}

	c.mu.Lock()
	c.subscriptions[sub.id] = sub
	c.mu.Unlock()

	// Start polling for events (WebSocket would be better but requires more infrastructure)
	go c.pollEvents(sub)

	return sub, nil
}

func (c *chainAPIImpl) GetChainInfo(ctx context.Context, chainID string) (*ChainInfo, error) {
	if err := c.ctx.RequireCapability(CapChain); err != nil {
		return nil, err
	}

	var req HTTPRequest

	switch chainID {
	case "neo", "neo-mainnet", "neo-testnet":
		req = c.buildNeoRPCRequest("getversion", []any{})
	case "ethereum", "eth-mainnet", "eth-sepolia":
		req = c.buildEthRPCRequest("eth_chainId", []any{})
	default:
		req = c.buildGenericRPCRequest(chainID, "getinfo", []any{})
	}

	resp, err := c.ctx.Network().Fetch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain info: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("RPC error: status %d", resp.StatusCode)
	}

	info := &ChainInfo{
		ChainID: chainID,
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
	}

	if err := json.Unmarshal(resp.Body, &rpcResp); err == nil {
		var result map[string]any
		if err := json.Unmarshal(rpcResp.Result, &result); err == nil {
			if name, ok := result["useragent"].(string); ok {
				info.Name = name
			}
			if network, ok := result["network"].(float64); ok {
				info.NetworkID = fmt.Sprintf("%d", int(network))
			}
		}
	}

	return info, nil
}

// Helper methods for building RPC requests

func (c *chainAPIImpl) buildNeoRPCRequest(method string, params []any) HTTPRequest {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})

	return HTTPRequest{
		Method: "POST",
		URL:    c.getChainRPCEndpoint("neo"),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: body,
	}
}

func (c *chainAPIImpl) buildEthRPCRequest(method string, params []any) HTTPRequest {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})

	return HTTPRequest{
		Method: "POST",
		URL:    c.getChainRPCEndpoint("ethereum"),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: body,
	}
}

func (c *chainAPIImpl) buildGenericRPCRequest(chainID, method string, params []any) HTTPRequest {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})

	return HTTPRequest{
		Method: "POST",
		URL:    c.getChainRPCEndpoint(chainID),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: body,
	}
}

func (c *chainAPIImpl) getChainRPCEndpoint(chainID string) string {
	// Get endpoint from config
	ctx := context.Background()
	endpoint, err := c.ctx.Config().GetString(ctx, "chain."+chainID+".rpc_endpoint")
	if err == nil && endpoint != "" {
		return endpoint
	}

	// Default endpoints
	switch chainID {
	case "neo", "neo-mainnet":
		return "https://mainnet1.neo.coz.io:443"
	case "neo-testnet":
		return "https://testnet1.neo.coz.io:443"
	case "ethereum", "eth-mainnet":
		return "https://eth.llamarpc.com"
	case "eth-sepolia":
		return "https://sepolia.drpc.org"
	default:
		return ""
	}
}

func (c *chainAPIImpl) pollEvents(sub *chainSubscription) {
	// Simple polling implementation
	// In production, this would use WebSocket subscriptions
	for sub.active {
		select {
		case <-sub.ctx.Done():
			return
		default:
			// Poll for new events
			// This is a placeholder - real implementation would query for events
		}
	}
}

func generateSubscriptionID() string {
	return fmt.Sprintf("sub-%d", generateUniqueID())
}

var subscriptionCounter uint64
var subscriptionMu sync.Mutex

func generateUniqueID() uint64 {
	subscriptionMu.Lock()
	defer subscriptionMu.Unlock()
	subscriptionCounter++
	return subscriptionCounter
}
