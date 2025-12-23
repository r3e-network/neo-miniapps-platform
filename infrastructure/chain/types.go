// Package chain provides Neo N3 blockchain interaction for the Service Layer.
package chain

import (
	"encoding/json"
	"fmt"
	"strings"
)

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

func isNotFoundError(err error) bool {
	if rpcErr, ok := err.(*RPCError); ok {
		msg := strings.ToLower(rpcErr.Message)
		// -100: Unknown transaction
		// -105: Unknown script container (transaction not yet confirmed)
		if strings.Contains(msg, "unknown transaction") ||
			strings.Contains(msg, "unknown script container") ||
			rpcErr.Code == -100 || rpcErr.Code == -105 {
			return true
		}
	}
	return false
}

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
// Invocation Types
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

// TxResult represents the result of a transaction execution.
type TxResult struct {
	TxHash  string          // Transaction hash
	AppLog  *ApplicationLog // Application log (nil if wait=false)
	VMState string          // VM state from execution (HALT = success)
}

// =============================================================================
// Deployment Types
// =============================================================================

// DeployedContract represents a deployed smart contract.
type DeployedContract struct {
	Name        string `json:"name,omitempty"`
	Hash        string `json:"hash"`
	Address     string `json:"address,omitempty"`
	TxHash      string `json:"tx_hash,omitempty"`
	GasConsumed string `json:"gas_consumed,omitempty"`
	State       string `json:"state,omitempty"`
	DeployedAt  string `json:"deployed_at,omitempty"`
}

// DeploymentResult represents the result of a contract deployment operation.
type DeploymentResult struct {
	Contracts []DeployedContract `json:"contracts"`
	Network   string             `json:"network,omitempty"`
	Deployer  string             `json:"deployer,omitempty"`
	SessionID string             `json:"session_id,omitempty"`
	Account   string             `json:"account,omitempty"`
	Timestamp string             `json:"timestamp,omitempty"`
}
