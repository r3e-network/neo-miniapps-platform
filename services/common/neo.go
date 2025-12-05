// Package common provides Neo N3 blockchain integration for services.
package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NeoClient is a Neo N3 RPC client.
type NeoClient struct {
	rpcURL     string
	httpClient *http.Client
}

// NewNeoClient creates a new Neo N3 client.
func NewNeoClient(rpcURL string) *NeoClient {
	return &NeoClient{
		rpcURL: rpcURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RPCRequest is a JSON-RPC request.
type RPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

// RPCResponse is a JSON-RPC response.
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Call makes an RPC call.
func (c *NeoClient) Call(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	if params == nil {
		params = []any{}
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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
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

	return rpcResp.Result, nil
}

// GetBlockCount returns the current block height.
func (c *NeoClient) GetBlockCount(ctx context.Context) (uint32, error) {
	result, err := c.Call(ctx, "getblockcount")
	if err != nil {
		return 0, err
	}

	var count uint32
	if err := json.Unmarshal(result, &count); err != nil {
		return 0, fmt.Errorf("unmarshal block count: %w", err)
	}

	return count, nil
}

// GetVersion returns the node version.
func (c *NeoClient) GetVersion(ctx context.Context) (*NodeVersion, error) {
	result, err := c.Call(ctx, "getversion")
	if err != nil {
		return nil, err
	}

	var version NodeVersion
	if err := json.Unmarshal(result, &version); err != nil {
		return nil, fmt.Errorf("unmarshal version: %w", err)
	}

	return &version, nil
}

// NodeVersion represents Neo node version info.
type NodeVersion struct {
	TCPPort   int    `json:"tcpport"`
	WSPort    int    `json:"wsport"`
	Nonce     int64  `json:"nonce"`
	UserAgent string `json:"useragent"`
	Protocol  struct {
		Network       uint32 `json:"network"`
		ValidatorsCount int  `json:"validatorscount"`
		MSPerBlock    int    `json:"msperblock"`
	} `json:"protocol"`
}

// InvokeFunction invokes a contract function (read-only).
func (c *NeoClient) InvokeFunction(ctx context.Context, scriptHash, operation string, params []any) (*InvokeResult, error) {
	if params == nil {
		params = []any{}
	}

	result, err := c.Call(ctx, "invokefunction", scriptHash, operation, params)
	if err != nil {
		return nil, err
	}

	var invokeResult InvokeResult
	if err := json.Unmarshal(result, &invokeResult); err != nil {
		return nil, fmt.Errorf("unmarshal invoke result: %w", err)
	}

	return &invokeResult, nil
}

// InvokeResult is the result of invokefunction.
type InvokeResult struct {
	Script      string      `json:"script"`
	State       string      `json:"state"`
	GasConsumed string      `json:"gasconsumed"`
	Exception   string      `json:"exception,omitempty"`
	Stack       []StackItem `json:"stack"`
}

// StackItem is a Neo VM stack item.
type StackItem struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// SendRawTransaction sends a signed transaction.
func (c *NeoClient) SendRawTransaction(ctx context.Context, txHex string) (string, error) {
	result, err := c.Call(ctx, "sendrawtransaction", txHex)
	if err != nil {
		return "", err
	}

	var response struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	return response.Hash, nil
}

// GetApplicationLog gets the application log for a transaction.
func (c *NeoClient) GetApplicationLog(ctx context.Context, txHash string) (*ApplicationLog, error) {
	result, err := c.Call(ctx, "getapplicationlog", txHash)
	if err != nil {
		return nil, err
	}

	var appLog ApplicationLog
	if err := json.Unmarshal(result, &appLog); err != nil {
		return nil, fmt.Errorf("unmarshal application log: %w", err)
	}

	return &appLog, nil
}

// ApplicationLog is the application log for a transaction.
type ApplicationLog struct {
	TxID       string       `json:"txid"`
	Executions []Execution  `json:"executions"`
}

// Execution is a single execution in the application log.
type Execution struct {
	Trigger     string        `json:"trigger"`
	VMState     string        `json:"vmstate"`
	GasConsumed string        `json:"gasconsumed"`
	Stack       []StackItem   `json:"stack"`
	Notifications []Notification `json:"notifications"`
}

// Notification is a contract notification.
type Notification struct {
	Contract  string    `json:"contract"`
	EventName string    `json:"eventname"`
	State     StackItem `json:"state"`
}

// GetContractState gets contract state.
func (c *NeoClient) GetContractState(ctx context.Context, scriptHash string) (*ContractState, error) {
	result, err := c.Call(ctx, "getcontractstate", scriptHash)
	if err != nil {
		return nil, err
	}

	var state ContractState
	if err := json.Unmarshal(result, &state); err != nil {
		return nil, fmt.Errorf("unmarshal contract state: %w", err)
	}

	return &state, nil
}

// ContractState is the state of a contract.
type ContractState struct {
	ID            int    `json:"id"`
	UpdateCounter int    `json:"updatecounter"`
	Hash          string `json:"hash"`
	NEF           struct {
		Magic    int    `json:"magic"`
		Compiler string `json:"compiler"`
	} `json:"nef"`
	Manifest struct {
		Name string `json:"name"`
		ABI  struct {
			Methods []struct {
				Name       string `json:"name"`
				Parameters []struct {
					Name string `json:"name"`
					Type string `json:"type"`
				} `json:"parameters"`
				ReturnType string `json:"returntype"`
			} `json:"methods"`
		} `json:"abi"`
	} `json:"manifest"`
}
