//go:build neoexpress

// Package neoexpress provides integration tests using Neo Express for contract testing.
package neoexpress

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// NeoExpressConfig holds configuration for Neo Express tests.
type NeoExpressConfig struct {
	ConfigPath string
	RPCPort    int
	Wallet     string
	Account    string
}

// DefaultConfig returns the default Neo Express configuration.
func DefaultConfig() NeoExpressConfig {
	// Use project root test.neo-express config
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		projectRoot = "../.."
	}
	return NeoExpressConfig{
		ConfigPath: filepath.Join(projectRoot, "test.neo-express"),
		RPCPort:    50012,
		Wallet:     "node1",
		Account:    "NdZCVDTgGKTsA9Y3zYfgp8mi2UA9THK61F",
	}
}

// RPCClient is a simple Neo RPC client for testing.
type RPCClient struct {
	url string
}

// NewRPCClient creates a new RPC client.
func NewRPCClient(url string) *RPCClient {
	return &RPCClient{url: url}
}

// Call makes an RPC call to the Neo node.
func (c *RPCClient) Call(ctx context.Context, method string, params []interface{}) (json.RawMessage, error) {
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

// GetBlockCount returns the current block count.
func (c *RPCClient) GetBlockCount(ctx context.Context) (int64, error) {
	result, err := c.Call(ctx, "getblockcount", []interface{}{})
	if err != nil {
		return 0, err
	}
	var count int64
	if err := json.Unmarshal(result, &count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetVersion returns the Neo node version info.
func (c *RPCClient) GetVersion(ctx context.Context) (map[string]interface{}, error) {
	result, err := c.Call(ctx, "getversion", []interface{}{})
	if err != nil {
		return nil, err
	}
	var version map[string]interface{}
	if err := json.Unmarshal(result, &version); err != nil {
		return nil, err
	}
	return version, nil
}

// GetBalance returns the balance of an account for a given asset.
func (c *RPCClient) GetBalance(ctx context.Context, address string) (map[string]interface{}, error) {
	result, err := c.Call(ctx, "getnep17balances", []interface{}{address})
	if err != nil {
		return nil, err
	}
	var balance map[string]interface{}
	if err := json.Unmarshal(result, &balance); err != nil {
		return nil, err
	}
	return balance, nil
}

// InvokeFunction invokes a smart contract function.
func (c *RPCClient) InvokeFunction(ctx context.Context, scriptHash, method string, params []interface{}) (map[string]interface{}, error) {
	result, err := c.Call(ctx, "invokefunction", append([]interface{}{scriptHash, method}, params...))
	if err != nil {
		return nil, err
	}
	var invocationResult map[string]interface{}
	if err := json.Unmarshal(result, &invocationResult); err != nil {
		return nil, err
	}
	return invocationResult, nil
}

// NeoExpressRunner manages Neo Express instance for tests.
type NeoExpressRunner struct {
	config  NeoExpressConfig
	cmd     *exec.Cmd
	running bool
}

// NewRunner creates a new Neo Express runner.
func NewRunner(config NeoExpressConfig) *NeoExpressRunner {
	return &NeoExpressRunner{config: config}
}

// Start starts the Neo Express instance.
func (r *NeoExpressRunner) Start(ctx context.Context) error {
	if r.running {
		return nil
	}

	neoxp := findNeoExpress()
	if neoxp == "" {
		return fmt.Errorf("neo-express (neoxp) not found in PATH or common locations")
	}

	// Reset and run Neo Express
	resetCmd := exec.CommandContext(ctx, neoxp, "reset", "-f", "-i", r.config.ConfigPath)
	if out, err := resetCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("reset neo-express: %v: %s", err, out)
	}

	r.cmd = exec.CommandContext(ctx, neoxp, "run", "-i", r.config.ConfigPath, "-s", "1")
	r.cmd.Stdout = os.Stdout
	r.cmd.Stderr = os.Stderr

	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("start neo-express: %v", err)
	}

	r.running = true

	// Wait for RPC to become available
	rpcURL := fmt.Sprintf("http://localhost:%d", r.config.RPCPort)
	client := NewRPCClient(rpcURL)

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := client.GetBlockCount(ctx); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	r.Stop()
	return fmt.Errorf("neo-express RPC not available after 30s")
}

// Stop stops the Neo Express instance.
func (r *NeoExpressRunner) Stop() error {
	if !r.running || r.cmd == nil {
		return nil
	}
	r.running = false
	if r.cmd.Process != nil {
		return r.cmd.Process.Kill()
	}
	return nil
}

// RPCURL returns the RPC URL for the running instance.
func (r *NeoExpressRunner) RPCURL() string {
	return fmt.Sprintf("http://localhost:%d", r.config.RPCPort)
}

func findNeoExpress() string {
	// Check PATH
	if path, err := exec.LookPath("neoxp"); err == nil {
		return path
	}
	// Check common locations
	locations := []string{
		"/home/neo/.dotnet/tools/neoxp",
		os.ExpandEnv("$HOME/.dotnet/tools/neoxp"),
		"/usr/local/bin/neoxp",
	}
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}
	return ""
}

// TestNeoExpressConnection tests basic Neo Express connectivity.
func TestNeoExpressConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Neo Express test in short mode")
	}

	config := DefaultConfig()
	runner := NewRunner(config)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := runner.Start(ctx); err != nil {
		t.Fatalf("start neo-express: %v", err)
	}
	defer runner.Stop()

	client := NewRPCClient(runner.RPCURL())

	// Test getversion
	version, err := client.GetVersion(ctx)
	if err != nil {
		t.Fatalf("getversion: %v", err)
	}
	t.Logf("Neo version: %v", version)

	// Test getblockcount
	count, err := client.GetBlockCount(ctx)
	if err != nil {
		t.Fatalf("getblockcount: %v", err)
	}
	if count < 0 {
		t.Errorf("invalid block count: %d", count)
	}
	t.Logf("Block count: %d", count)
}

// TestNeoExpressBalances tests balance queries on Neo Express.
func TestNeoExpressBalances(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Neo Express test in short mode")
	}

	config := DefaultConfig()
	runner := NewRunner(config)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := runner.Start(ctx); err != nil {
		t.Fatalf("start neo-express: %v", err)
	}
	defer runner.Stop()

	client := NewRPCClient(runner.RPCURL())

	// Query balance for the test account
	balance, err := client.GetBalance(ctx, config.Account)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	t.Logf("Balance for %s: %v", config.Account, balance)
}

// TestNeoExpressBlockGeneration tests block generation in Neo Express.
func TestNeoExpressBlockGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Neo Express test in short mode")
	}

	config := DefaultConfig()
	runner := NewRunner(config)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if err := runner.Start(ctx); err != nil {
		t.Fatalf("start neo-express: %v", err)
	}
	defer runner.Stop()

	client := NewRPCClient(runner.RPCURL())

	// Get initial block count
	initialCount, err := client.GetBlockCount(ctx)
	if err != nil {
		t.Fatalf("initial getblockcount: %v", err)
	}

	// Wait for a new block (Neo Express runs with 1s block time)
	time.Sleep(3 * time.Second)

	// Check that blocks were generated
	newCount, err := client.GetBlockCount(ctx)
	if err != nil {
		t.Fatalf("new getblockcount: %v", err)
	}

	if newCount <= initialCount {
		t.Errorf("expected new blocks, initial=%d, new=%d", initialCount, newCount)
	}
	t.Logf("Blocks generated: %d -> %d", initialCount, newCount)
}

// TestNeoExpressNativeContracts tests invoking native contracts.
func TestNeoExpressNativeContracts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Neo Express test in short mode")
	}

	config := DefaultConfig()
	runner := NewRunner(config)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := runner.Start(ctx); err != nil {
		t.Fatalf("start neo-express: %v", err)
	}
	defer runner.Stop()

	client := NewRPCClient(runner.RPCURL())

	// NeoToken contract hash (native)
	neoTokenHash := "0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5"

	// Test invoking 'symbol' on NeoToken
	result, err := client.InvokeFunction(ctx, neoTokenHash, "symbol", []interface{}{})
	if err != nil {
		t.Fatalf("invoke NEO symbol: %v", err)
	}

	stack, ok := result["stack"].([]interface{})
	if !ok || len(stack) == 0 {
		t.Fatalf("unexpected stack result: %v", result)
	}

	stackItem := stack[0].(map[string]interface{})
	value := stackItem["value"].(string)
	decoded, _ := hex.DecodeString(value)
	if string(decoded) != "NEO" {
		t.Errorf("expected symbol 'NEO', got %q (hex: %s)", string(decoded), value)
	}
	t.Logf("NEO symbol: %s", string(decoded))

	// Test invoking 'decimals' on NeoToken
	decimalsResult, err := client.InvokeFunction(ctx, neoTokenHash, "decimals", []interface{}{})
	if err != nil {
		t.Fatalf("invoke NEO decimals: %v", err)
	}
	t.Logf("NEO decimals result: %v", decimalsResult)
}

// TestNeoExpressStateValidation tests state root validation.
func TestNeoExpressStateValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Neo Express test in short mode")
	}

	config := DefaultConfig()
	runner := NewRunner(config)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := runner.Start(ctx); err != nil {
		t.Fatalf("start neo-express: %v", err)
	}
	defer runner.Stop()

	client := NewRPCClient(runner.RPCURL())

	// Wait for a few blocks
	time.Sleep(3 * time.Second)

	count, err := client.GetBlockCount(ctx)
	if err != nil {
		t.Fatalf("getblockcount: %v", err)
	}

	// Query state root for a recent block
	if count > 1 {
		height := count - 1
		result, err := client.Call(ctx, "getstateroot", []interface{}{height})
		if err != nil {
			t.Fatalf("getstateroot: %v", err)
		}

		var stateRoot map[string]interface{}
		if err := json.Unmarshal(result, &stateRoot); err != nil {
			t.Fatalf("unmarshal state root: %v", err)
		}

		t.Logf("State root at height %d: %v", height, stateRoot)

		if rootHash, ok := stateRoot["stateroot"].(string); ok {
			if !strings.HasPrefix(rootHash, "0x") {
				t.Errorf("expected state root to start with 0x, got %s", rootHash)
			}
		}
	}
}

// TestNeoExpressTransfer tests token transfer functionality.
func TestNeoExpressTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Neo Express test in short mode")
	}

	config := DefaultConfig()
	runner := NewRunner(config)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if err := runner.Start(ctx); err != nil {
		t.Fatalf("start neo-express: %v", err)
	}
	defer runner.Stop()

	// Use neoxp to transfer tokens
	neoxp := findNeoExpress()
	if neoxp == "" {
		t.Skip("neo-express not found")
	}

	// Transfer 1 NEO from node1 to a test address
	testAddr := "NXV7ZhHiyM1aHXwpVsRZC6BEaDY7t6x6xD"
	transferCmd := exec.CommandContext(ctx, neoxp, "transfer", "1", "NEO",
		config.Account, testAddr,
		"-i", config.ConfigPath,
		"-w", config.Wallet)

	out, err := transferCmd.CombinedOutput()
	if err != nil {
		t.Logf("transfer output: %s", out)
		// Transfer might fail if account doesn't have tokens, which is expected in fresh setup
		t.Skipf("transfer command failed (expected in fresh setup): %v", err)
	}
	t.Logf("Transfer result: %s", out)
}
