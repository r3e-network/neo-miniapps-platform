// Package framework provides the service engine framework.
package framework

import (
	"context"
	"time"
)

// ContractClient provides smart contract interaction capabilities for services.
// This interface abstracts blockchain-specific details, allowing services to
// interact with contracts without knowing the underlying chain implementation.
//
// Design follows the Android pattern where system services provide common
// infrastructure that apps (services) can use without reimplementing.
type ContractClient interface {
	// --- Contract Invocation ---

	// InvokeContract calls a contract method and returns the result.
	// This is the primary method for contract interaction.
	InvokeContract(ctx context.Context, req ContractInvokeRequest) (ContractInvokeResult, error)

	// InvokeContractReadOnly calls a contract method without state changes.
	// Use this for queries that don't require gas.
	InvokeContractReadOnly(ctx context.Context, req ContractInvokeRequest) (ContractInvokeResult, error)

	// --- Contract Deployment ---

	// DeployContract deploys a new contract to the blockchain.
	DeployContract(ctx context.Context, req ContractDeployRequest) (ContractDeployResult, error)

	// --- Transaction Status ---

	// GetTransactionStatus checks the status of a submitted transaction.
	GetTransactionStatus(ctx context.Context, network, txHash string) (TransactionStatus, error)

	// WaitForConfirmation waits for a transaction to be confirmed.
	WaitForConfirmation(ctx context.Context, network, txHash string, timeout time.Duration) (TransactionStatus, error)

	// --- Contract Registry ---

	// GetContractAddress returns the address of a registered contract.
	// This is useful for looking up engine contracts by name.
	GetContractAddress(ctx context.Context, network, contractName string) (string, error)

	// GetContractABI returns the ABI of a registered contract.
	GetContractABI(ctx context.Context, network, contractAddress string) (string, error)

	// --- Network Info ---

	// GetNetworkConfig returns configuration for a network.
	GetNetworkConfig(ctx context.Context, network string) (NetworkInfo, error)

	// ListSupportedNetworks returns all supported networks.
	ListSupportedNetworks(ctx context.Context) ([]string, error)
}

// ContractInvokeRequest represents a contract method invocation request.
type ContractInvokeRequest struct {
	// Network identifies the blockchain network
	Network string `json:"network"`

	// ContractAddress is the target contract address
	ContractAddress string `json:"contract_address"`

	// ContractName is an alternative to address for engine contracts
	ContractName string `json:"contract_name,omitempty"`

	// Method is the contract method to call
	Method string `json:"method"`

	// Args are the method arguments
	Args []any `json:"args,omitempty"`

	// ArgsMap provides named arguments (alternative to Args)
	ArgsMap map[string]any `json:"args_map,omitempty"`

	// Signer is the account signing the transaction (for state-changing calls)
	Signer string `json:"signer,omitempty"`

	// GasLimit is the maximum gas to use
	GasLimit int64 `json:"gas_limit,omitempty"`

	// Value is the amount of native token to send
	Value string `json:"value,omitempty"`

	// Metadata for tracking and correlation
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ContractInvokeResult represents the result of a contract invocation.
type ContractInvokeResult struct {
	// TxHash is the transaction hash (for state-changing calls)
	TxHash string `json:"tx_hash,omitempty"`

	// Result is the return value from the contract
	Result any `json:"result,omitempty"`

	// GasUsed is the actual gas consumed
	GasUsed int64 `json:"gas_used,omitempty"`

	// BlockNumber where the transaction was included
	BlockNumber int64 `json:"block_number,omitempty"`

	// Logs are events emitted during execution
	Logs []ContractEventLog `json:"logs,omitempty"`

	// Status indicates success or failure
	Status string `json:"status"`

	// Error message if the call failed
	Error string `json:"error,omitempty"`
}

// ContractDeployRequest represents a contract deployment request.
type ContractDeployRequest struct {
	// Network identifies the blockchain network
	Network string `json:"network"`

	// Bytecode is the compiled contract code
	Bytecode string `json:"bytecode"`

	// ABI is the contract interface definition
	ABI string `json:"abi,omitempty"`

	// ConstructorArgs are arguments for the constructor
	ConstructorArgs []any `json:"constructor_args,omitempty"`

	// Signer is the deploying account
	Signer string `json:"signer"`

	// GasLimit is the maximum gas to use
	GasLimit int64 `json:"gas_limit,omitempty"`

	// Metadata for tracking
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ContractDeployResult represents the result of a contract deployment.
type ContractDeployResult struct {
	// TxHash is the deployment transaction hash
	TxHash string `json:"tx_hash"`

	// ContractAddress is the deployed contract address
	ContractAddress string `json:"contract_address,omitempty"`

	// GasUsed is the actual gas consumed
	GasUsed int64 `json:"gas_used,omitempty"`

	// BlockNumber where the deployment was confirmed
	BlockNumber int64 `json:"block_number,omitempty"`

	// Status indicates success or failure
	Status string `json:"status"`

	// Error message if deployment failed
	Error string `json:"error,omitempty"`
}

// ContractEventLog represents an event emitted by a contract.
type ContractEventLog struct {
	// ContractAddress that emitted the event
	ContractAddress string `json:"contract_address"`

	// EventName is the event identifier
	EventName string `json:"event_name"`

	// Topics are indexed event parameters
	Topics []string `json:"topics,omitempty"`

	// Data contains non-indexed event parameters
	Data map[string]any `json:"data,omitempty"`

	// LogIndex within the transaction
	LogIndex int `json:"log_index"`

	// BlockNumber where the event occurred
	BlockNumber int64 `json:"block_number"`

	// TxHash of the transaction that emitted the event
	TxHash string `json:"tx_hash"`
}

// TransactionStatus represents the status of a blockchain transaction.
type TransactionStatus struct {
	// TxHash is the transaction hash
	TxHash string `json:"tx_hash"`

	// Status: pending, submitted, confirmed, failed
	Status string `json:"status"`

	// BlockNumber where confirmed (0 if pending)
	BlockNumber int64 `json:"block_number,omitempty"`

	// BlockHash of the containing block
	BlockHash string `json:"block_hash,omitempty"`

	// Confirmations is the number of blocks since confirmation
	Confirmations int `json:"confirmations,omitempty"`

	// GasUsed by the transaction
	GasUsed int64 `json:"gas_used,omitempty"`

	// Success indicates if execution succeeded
	Success bool `json:"success"`

	// Error message if failed
	Error string `json:"error,omitempty"`

	// Timestamp of confirmation
	ConfirmedAt time.Time `json:"confirmed_at,omitempty"`
}

// NetworkInfo provides information about a blockchain network.
type NetworkInfo struct {
	// Network identifier
	Network string `json:"network"`

	// ChainID for the network
	ChainID int64 `json:"chain_id"`

	// RPCEndpoint for JSON-RPC calls
	RPCEndpoint string `json:"rpc_endpoint"`

	// WSEndpoint for WebSocket subscriptions
	WSEndpoint string `json:"ws_endpoint,omitempty"`

	// ExplorerURL for transaction viewing
	ExplorerURL string `json:"explorer_url,omitempty"`

	// NativeToken symbol (e.g., "GAS", "ETH")
	NativeToken string `json:"native_token"`

	// NativeDecimals for the native token
	NativeDecimals int `json:"native_decimals"`

	// BlockTime in seconds
	BlockTime int `json:"block_time_seconds"`

	// RequiredConfirmations for finality
	RequiredConfirmations int `json:"required_confirmations"`

	// EngineContracts maps contract names to addresses
	EngineContracts map[string]string `json:"engine_contracts,omitempty"`

	// Enabled indicates if the network is active
	Enabled bool `json:"enabled"`
}

// Transaction status constants
const (
	TxStatusPending   = "pending"
	TxStatusSubmitted = "submitted"
	TxStatusConfirmed = "confirmed"
	TxStatusFailed    = "failed"
)

// NoopContractClient returns a no-op implementation of ContractClient.
func NoopContractClient() ContractClient {
	return &noopContractClient{}
}

type noopContractClient struct{}

func (n *noopContractClient) InvokeContract(ctx context.Context, req ContractInvokeRequest) (ContractInvokeResult, error) {
	return ContractInvokeResult{Status: "noop"}, nil
}

func (n *noopContractClient) InvokeContractReadOnly(ctx context.Context, req ContractInvokeRequest) (ContractInvokeResult, error) {
	return ContractInvokeResult{Status: "noop"}, nil
}

func (n *noopContractClient) DeployContract(ctx context.Context, req ContractDeployRequest) (ContractDeployResult, error) {
	return ContractDeployResult{Status: "noop"}, nil
}

func (n *noopContractClient) GetTransactionStatus(ctx context.Context, network, txHash string) (TransactionStatus, error) {
	return TransactionStatus{TxHash: txHash, Status: TxStatusPending}, nil
}

func (n *noopContractClient) WaitForConfirmation(ctx context.Context, network, txHash string, timeout time.Duration) (TransactionStatus, error) {
	return TransactionStatus{TxHash: txHash, Status: TxStatusConfirmed, Success: true}, nil
}

func (n *noopContractClient) GetContractAddress(ctx context.Context, network, contractName string) (string, error) {
	return "", nil
}

func (n *noopContractClient) GetContractABI(ctx context.Context, network, contractAddress string) (string, error) {
	return "", nil
}

func (n *noopContractClient) GetNetworkConfig(ctx context.Context, network string) (NetworkInfo, error) {
	return NetworkInfo{Network: network}, nil
}

func (n *noopContractClient) ListSupportedNetworks(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

// Ensure noopContractClient implements ContractClient
var _ ContractClient = (*noopContractClient)(nil)
