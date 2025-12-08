// Package fairy provides a Go client for Neo Fairy RPC.
package fairy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	DefaultRPCURL  = "http://127.0.0.1:16868"
	DefaultTimeout = 30 * time.Second
)

// Client is a Neo Fairy RPC client.
type Client struct {
	url    string
	client *http.Client
}

// NewClient creates a new Fairy client.
func NewClient(url string) *Client {
	if url == "" {
		url = DefaultRPCURL
	}
	return &Client{
		url: url,
		client: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// RPCRequest represents a JSON-RPC request.
type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// RPCResponse represents a JSON-RPC response.
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *Client) call(method string, params ...interface{}) (*RPCResponse, error) {
	if params == nil {
		params = []interface{}{}
	}
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

	httpReq, err := http.NewRequest("POST", c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
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
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return &rpcResp, nil
}

// HelloFairy tests connectivity to Fairy.
func (c *Client) HelloFairy() (map[string]interface{}, error) {
	resp, err := c.call("hellofairy") // lowercase required
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// NewSession creates a new testing session.
func (c *Client) NewSession() (string, error) {
	sessionID := fmt.Sprintf("test-%d", time.Now().UnixNano())
	resp, err := c.call("newsnapshotsfromcurrentsystem", sessionID) // creates session
	if err != nil {
		return "", err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", err
	}
	return sessionID, nil
}

// SetupSessionWithGas creates a session and funds the wallet with GAS.
// Reads NEO_TESTNET_WIF from environment.
func (c *Client) SetupSessionWithGas(gasAmount int64) (string, string, error) {
	sessionID := fmt.Sprintf("test-%d", time.Now().UnixNano())

	// Create session
	_, err := c.call("newsnapshotsfromcurrentsystem", sessionID)
	if err != nil {
		return "", "", fmt.Errorf("create session: %w", err)
	}

	// Get WIF from environment
	wif := os.Getenv("NEO_TESTNET_WIF")
	if wif == "" {
		return "", "", fmt.Errorf("NEO_TESTNET_WIF environment variable not set")
	}

	// Set session wallet with testnet account WIF
	resp, err := c.call("setsessionfairywalletwithwif", sessionID, wif)
	if err != nil {
		return "", "", fmt.Errorf("set wallet: %w", err)
	}

	// Parse response to get account address
	var walletInfo map[string]interface{}
	if err := json.Unmarshal(resp.Result, &walletInfo); err != nil {
		return "", "", fmt.Errorf("parse wallet info: %w", err)
	}

	// Get the account script hash from response
	var accountHash string
	for _, v := range walletInfo {
		if addr, ok := v.(string); ok {
			accountHash = addr
			break
		}
	}

	if accountHash == "" {
		return "", "", fmt.Errorf("could not get account hash from wallet")
	}

	// Set GAS balance for the account
	_, err = c.call("setgasbalance", sessionID, accountHash, fmt.Sprintf("%d", gasAmount))
	if err != nil {
		return "", "", fmt.Errorf("set gas balance: %w", err)
	}

	return sessionID, accountHash, nil
}

// FundTEEAccount funds a TEE account with GAS in the session.
// teeAccount is the script hash of the TEE account.
func (c *Client) FundTEEAccount(sessionID, teeAccountHash string, gasAmount int64) error {
	_, err := c.call("setgasbalance", sessionID, teeAccountHash, fmt.Sprintf("%d", gasAmount))
	return err
}

// DeleteSession deletes a session.
func (c *Client) DeleteSession(sessionID string) error {
	_, err := c.call("deletesnapshots", sessionID)
	return err
}

// VirtualDeployResult represents the result of VirtualDeploy.
type VirtualDeployResult struct {
	ContractHash string `json:"contracthash"`
	GasConsumed  string `json:"gasconsumed"`
	State        string `json:"state"`
}

// VirtualDeploy deploys a contract virtually in a session.
func (c *Client) VirtualDeploy(sessionID string, nefPath, manifestPath string) (*VirtualDeployResult, error) {
	nefData, err := os.ReadFile(nefPath)
	if err != nil {
		return nil, fmt.Errorf("read nef: %w", err)
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	nefBase64 := base64.StdEncoding.EncodeToString(nefData)

	// VirtualDeploy params: session, nefBase64, manifestJSON, signers(empty array)
	resp, err := c.call("virtualdeploy", sessionID, nefBase64, string(manifestData), []interface{}{})
	if err != nil {
		return nil, err
	}

	var result VirtualDeployResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	// Contract hash is in the session field
	var rawResult map[string]interface{}
	if err := json.Unmarshal(resp.Result, &rawResult); err != nil {
		return nil, err
	}
	if hash, ok := rawResult[sessionID].(string); ok {
		result.ContractHash = hash
	}
	return &result, nil
}

// InvokeResult represents the result of a contract invocation.
type InvokeResult struct {
	Script      string `json:"script"`
	State       string `json:"state"`
	GasConsumed string `json:"gasconsumed"`
	Exception   string `json:"exception,omitempty"`
	Stack       []StackItem `json:"stack"`
}

// StackItem represents a stack item.
type StackItem struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// InvokeFunctionWithSession invokes a contract method in a session.
func (c *Client) InvokeFunctionWithSession(sessionID string, writeSnapshot bool, contractHash, method string, args []interface{}) (*InvokeResult, error) {
	if args == nil {
		args = []interface{}{}
	}
	params := []interface{}{
		sessionID,
		writeSnapshot,
		contractHash,
		method,
		args,
	}

	resp, err := c.call("invokefunctionwithsession", params...) // lowercase required
	if err != nil {
		return nil, err
	}

	var result InvokeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SetTime sets the virtual time for a session.
func (c *Client) SetTime(sessionID string, timestamp uint64) error {
	_, err := c.call("settime", sessionID, timestamp) // lowercase required
	return err
}

// SetGasBalance sets GAS balance for an account in a session.
func (c *Client) SetGasBalance(sessionID, account string, balance int64) error {
	_, err := c.call("setgasbalance", sessionID, account, balance) // lowercase required
	return err
}

// IsAvailable checks if Fairy is available.
func (c *Client) IsAvailable() bool {
	_, err := c.HelloFairy()
	return err == nil
}
