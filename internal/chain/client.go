// Package chain provides Neo N3 blockchain interaction for the Service Layer.
package chain

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
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
// RPC Types
// =============================================================================

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
	Data    string `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
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

// =============================================================================
// Contract Invocation
// =============================================================================

// InvokeResult represents the result of a contract invocation.
type InvokeResult struct {
	Script      string      `json:"script"`
	State       string      `json:"state"`
	GasConsumed string      `json:"gasconsumed"`
	Stack       []StackItem `json:"stack"`
	Exception   string      `json:"exception,omitempty"`
	Tx          string      `json:"tx,omitempty"`
}

// StackItem represents a stack item from contract execution.
type StackItem struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// InvokeFunction invokes a contract function (read-only).
func (c *Client) InvokeFunction(ctx context.Context, scriptHash string, method string, params []ContractParam) (*InvokeResult, error) {
	args := []interface{}{scriptHash, method, params}
	result, err := c.Call(ctx, "invokefunction", args)
	if err != nil {
		return nil, err
	}

	var invokeResult InvokeResult
	if err := json.Unmarshal(result, &invokeResult); err != nil {
		return nil, err
	}
	return &invokeResult, nil
}

// InvokeScript invokes a script (read-only).
func (c *Client) InvokeScript(ctx context.Context, script string, signers []Signer) (*InvokeResult, error) {
	args := []interface{}{script}
	if len(signers) > 0 {
		args = append(args, signers)
	}

	result, err := c.Call(ctx, "invokescript", args)
	if err != nil {
		return nil, err
	}

	var invokeResult InvokeResult
	if err := json.Unmarshal(result, &invokeResult); err != nil {
		return nil, err
	}
	return &invokeResult, nil
}

// SendRawTransaction sends a signed transaction.
func (c *Client) SendRawTransaction(ctx context.Context, txHex string) (string, error) {
	result, err := c.Call(ctx, "sendrawtransaction", []interface{}{txHex})
	if err != nil {
		return "", err
	}

	var response struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return "", err
	}
	return response.Hash, nil
}

// =============================================================================
// Contract Parameter Types
// =============================================================================

// ContractParam represents a contract parameter.
type ContractParam struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// NewStringParam creates a string parameter.
func NewStringParam(value string) ContractParam {
	return ContractParam{Type: "String", Value: value}
}

// NewIntegerParam creates an integer parameter.
func NewIntegerParam(value *big.Int) ContractParam {
	return ContractParam{Type: "Integer", Value: value.String()}
}

// NewBoolParam creates a boolean parameter.
func NewBoolParam(value bool) ContractParam {
	return ContractParam{Type: "Boolean", Value: value}
}

// NewByteArrayParam creates a byte array parameter.
func NewByteArrayParam(value []byte) ContractParam {
	return ContractParam{Type: "ByteArray", Value: hex.EncodeToString(value)}
}

// NewHash160Param creates a Hash160 (address) parameter.
func NewHash160Param(value string) ContractParam {
	return ContractParam{Type: "Hash160", Value: value}
}

// NewHash256Param creates a Hash256 parameter.
func NewHash256Param(value string) ContractParam {
	return ContractParam{Type: "Hash256", Value: value}
}

// NewPublicKeyParam creates a public key parameter.
func NewPublicKeyParam(value string) ContractParam {
	return ContractParam{Type: "PublicKey", Value: value}
}

// NewArrayParam creates an array parameter.
func NewArrayParam(values []ContractParam) ContractParam {
	return ContractParam{Type: "Array", Value: values}
}

// =============================================================================
// Signer Types
// =============================================================================

// Signer represents a transaction signer.
type Signer struct {
	Account          string   `json:"account"`
	Scopes           string   `json:"scopes"`
	AllowedContracts []string `json:"allowedcontracts,omitempty"`
	AllowedGroups    []string `json:"allowedgroups,omitempty"`
}

// WitnessScope constants.
const (
	ScopeNone            = "None"
	ScopeCalledByEntry   = "CalledByEntry"
	ScopeCustomContracts = "CustomContracts"
	ScopeCustomGroups    = "CustomGroups"
	ScopeGlobal          = "Global"
	ScopeWitnessRules    = "WitnessRules"
)

// =============================================================================
// Block and Transaction Types
// =============================================================================

// Block represents a Neo N3 block.
type Block struct {
	Hash              string        `json:"hash"`
	Size              int           `json:"size"`
	Version           int           `json:"version"`
	PreviousBlockHash string        `json:"previousblockhash"`
	MerkleRoot        string        `json:"merkleroot"`
	Time              uint64        `json:"time"`
	Nonce             string        `json:"nonce"`
	Index             uint64        `json:"index"`
	NextConsensus     string        `json:"nextconsensus"`
	Witnesses         []Witness     `json:"witnesses"`
	Tx                []Transaction `json:"tx"`
}

// Transaction represents a Neo N3 transaction.
type Transaction struct {
	Hash            string        `json:"hash"`
	Size            int           `json:"size"`
	Version         int           `json:"version"`
	Nonce           uint32        `json:"nonce"`
	Sender          string        `json:"sender"`
	SystemFee       string        `json:"sysfee"`
	NetworkFee      string        `json:"netfee"`
	ValidUntilBlock uint64        `json:"validuntilblock"`
	Signers         []Signer      `json:"signers"`
	Attributes      []TxAttribute `json:"attributes"`
	Script          string        `json:"script"`
	Witnesses       []Witness     `json:"witnesses"`
}

// Witness represents a transaction witness.
type Witness struct {
	Invocation   string `json:"invocation"`
	Verification string `json:"verification"`
}

// TxAttribute represents a transaction attribute.
type TxAttribute struct {
	Type string `json:"type"`
}

// ApplicationLog represents the application log of a transaction.
type ApplicationLog struct {
	TxID       string      `json:"txid"`
	Executions []Execution `json:"executions"`
}

// Execution represents a single execution in the application log.
type Execution struct {
	Trigger       string         `json:"trigger"`
	VMState       string         `json:"vmstate"`
	Exception     string         `json:"exception,omitempty"`
	GasConsumed   string         `json:"gasconsumed"`
	Stack         []StackItem    `json:"stack"`
	Notifications []Notification `json:"notifications"`
}

// Notification represents a contract notification.
type Notification struct {
	Contract  string    `json:"contract"`
	EventName string    `json:"eventname"`
	State     StackItem `json:"state"`
}

// =============================================================================
// Wallet and Signing
// =============================================================================

// Wallet represents a Neo N3 wallet for signing transactions.
type Wallet struct {
	privateKey *ecdsa.PrivateKey
	publicKey  []byte
	scriptHash []byte
	address    string
}

// NewWallet creates a new wallet from a private key.
func NewWallet(privateKeyHex string) (*Wallet, error) {
	keyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	// Set the private key D value
	keyPair.PrivateKey.D = new(big.Int).SetBytes(keyBytes)
	keyPair.PrivateKey.PublicKey.X, keyPair.PrivateKey.PublicKey.Y =
		keyPair.PrivateKey.Curve.ScalarBaseMult(keyBytes)

	publicKey := crypto.PublicKeyToBytes(&keyPair.PrivateKey.PublicKey)
	scriptHash := crypto.PublicKeyToScriptHash(publicKey)
	address := crypto.ScriptHashToAddress(scriptHash)

	return &Wallet{
		privateKey: keyPair.PrivateKey,
		publicKey:  publicKey,
		scriptHash: scriptHash,
		address:    address,
	}, nil
}

// Address returns the wallet address.
func (w *Wallet) Address() string {
	return w.address
}

// ScriptHash returns the wallet script hash.
func (w *Wallet) ScriptHash() []byte {
	return w.scriptHash
}

// ScriptHashHex returns the wallet script hash as hex string.
func (w *Wallet) ScriptHashHex() string {
	// Reverse for Neo N3 little-endian format
	reversed := make([]byte, len(w.scriptHash))
	for i, b := range w.scriptHash {
		reversed[len(w.scriptHash)-1-i] = b
	}
	return hex.EncodeToString(reversed)
}

// PublicKey returns the wallet public key.
func (w *Wallet) PublicKey() []byte {
	return w.publicKey
}

// Sign signs data with the wallet's private key.
func (w *Wallet) Sign(data []byte) ([]byte, error) {
	return crypto.Sign(w.privateKey, data)
}
