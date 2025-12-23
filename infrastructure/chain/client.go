// Package chain provides Neo N3 blockchain interaction for the Service Layer.
package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/gas"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"

	"github.com/R3E-Network/service_layer/infrastructure/httputil"
)

// Client provides Neo N3 RPC client functionality.
type Client struct {
	rpcURL     string
	httpClient *http.Client
	networkID  uint32

	// Persistent actor for concurrent transaction support
	persistentRPC   *rpcclient.Client
	persistentActor *actor.Actor
	actorAccount    *wallet.Account
	actorMu         sync.Mutex
}

// Config holds client configuration.
type Config struct {
	RPCURL     string
	NetworkID  uint32 // MainNet: 860833102, TestNet: 894710606
	Timeout    time.Duration
	HTTPClient *http.Client // Optional custom HTTP client (e.g. Marble.ExternalHTTPClient()).
}

// NewClient creates a new Neo N3 client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.RPCURL == "" {
		return nil, fmt.Errorf("RPC URL required")
	}

	normalizedURL, _, err := httputil.NormalizeBaseURL(cfg.RPCURL, httputil.BaseURLOptions{RequireHTTPSInStrictMode: true})
	if err != nil {
		return nil, fmt.Errorf("invalid RPC URL: %w", err)
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	forceTimeout := cfg.Timeout != 0

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		transport := httputil.DefaultTransportWithMinTLS12()

		httpClient = &http.Client{
			Timeout:   timeout,
			Transport: transport,
		}
	} else {
		httpClient = httputil.CopyHTTPClientWithTimeout(httpClient, timeout, forceTimeout)
	}

	return &Client{
		rpcURL:     normalizedURL,
		httpClient: httpClient,
		networkID:  cfg.NetworkID,
	}, nil
}

// NetworkID returns the configured Neo N3 network magic for this client.
func (c *Client) NetworkID() uint32 {
	if c == nil {
		return 0
	}
	return c.networkID
}

// CloneWithRPCURL returns a new Client that uses the provided RPC URL while
// retaining the existing client's NetworkID and HTTP client configuration.
func (c *Client) CloneWithRPCURL(rpcURL string) (*Client, error) {
	if c == nil {
		return nil, fmt.Errorf("chain client is nil")
	}

	timeout := time.Duration(0)
	if c.httpClient != nil {
		timeout = c.httpClient.Timeout
	}

	return NewClient(Config{
		RPCURL:     rpcURL,
		NetworkID:  c.networkID,
		Timeout:    timeout,
		HTTPClient: c.httpClient,
	})
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, truncated, readErr := httputil.ReadAllWithLimit(resp.Body, 32<<10)
		if readErr != nil {
			return nil, fmt.Errorf("read error response: %w", readErr)
		}
		msg := strings.TrimSpace(string(respBody))
		if truncated {
			msg += "...(truncated)"
		}
		return nil, fmt.Errorf("rpc http error %d: %s", resp.StatusCode, msg)
	}

	respBody, err := httputil.ReadAllStrict(resp.Body, 8<<20)
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

// TransferGAS transfers GAS from a signer account to a target address using the neo-go actor pattern.
// This uses a persistent actor for the account to support concurrent transactions with proper nonce management.
func (c *Client) TransferGAS(ctx context.Context, account *wallet.Account, to util.Uint160, amount *big.Int) (util.Uint256, error) {
	// Get or create the actor (hold lock only during setup)
	act, err := c.getOrCreateActor(ctx, account)
	if err != nil {
		return util.Uint256{}, err
	}

	// Get GAS contract using the actor
	gasContract := gas.New(act)

	// Transfer GAS - this can run concurrently, actor handles nonce management
	txHash, _, err := gasContract.Transfer(account.ScriptHash(), to, amount, nil)
	if err != nil {
		// If transfer fails, reset the actor so it gets recreated on next call
		c.resetActor()
		return util.Uint256{}, fmt.Errorf("transfer: %w", err)
	}

	return txHash, nil
}

// getOrCreateActor returns the persistent actor, creating it if necessary.
func (c *Client) getOrCreateActor(ctx context.Context, account *wallet.Account) (*actor.Actor, error) {
	c.actorMu.Lock()
	defer c.actorMu.Unlock()

	// Check if we need to create or recreate the persistent actor
	needNewActor := c.persistentActor == nil ||
		c.actorAccount == nil ||
		c.actorAccount.ScriptHash() != account.ScriptHash()

	if needNewActor {
		// Close existing RPC client if any
		if c.persistentRPC != nil {
			c.persistentRPC.Close()
			c.persistentRPC = nil
			c.persistentActor = nil
		}

		// Create a new rpcclient connection
		rpcClient, err := rpcclient.New(ctx, c.rpcURL, rpcclient.Options{})
		if err != nil {
			return nil, fmt.Errorf("create rpc client: %w", err)
		}

		// Create actor for signing transactions
		act, err := actor.NewSimple(rpcClient, account)
		if err != nil {
			rpcClient.Close()
			return nil, fmt.Errorf("create actor: %w", err)
		}

		c.persistentRPC = rpcClient
		c.persistentActor = act
		c.actorAccount = account
	}

	return c.persistentActor, nil
}

// resetActor clears the persistent actor so it gets recreated on next call.
func (c *Client) resetActor() {
	c.actorMu.Lock()
	defer c.actorMu.Unlock()

	if c.persistentRPC != nil {
		c.persistentRPC.Close()
	}
	c.persistentRPC = nil
	c.persistentActor = nil
	c.actorAccount = nil
}
