// Package types defines the shared API types for the neoaccounts service.
//
// This package is intentionally free of HTTP/DB dependencies so it can be used by:
// - the account pool server implementation (`infrastructure/accountpool/marble`)
// - service-to-service clients (`infrastructure/accountpool/client`)
// - other services consuming the API (e.g. `neocompute`)
package types

import "time"

// Well-known token types.
const (
	TokenTypeNEO = "NEO"
	TokenTypeGAS = "GAS"
)

// TokenBalance represents a balance for a specific token type.
type TokenBalance struct {
	TokenType  string    `json:"token_type"`
	ScriptHash string    `json:"script_hash"`
	Amount     int64     `json:"amount"`
	Decimals   int       `json:"decimals"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

// TokenStats represents aggregated statistics for a token type.
type TokenStats struct {
	TokenType        string `json:"token_type"`
	ScriptHash       string `json:"script_hash"`
	TotalBalance     int64  `json:"total_balance"`
	LockedBalance    int64  `json:"locked_balance"`
	AvailableBalance int64  `json:"available_balance"`
}

// AccountInfo represents public account information returned to clients.
// Private keys are never exposed. Balances are tracked per-token.
type AccountInfo struct {
	ID         string                  `json:"id"`
	Address    string                  `json:"address"`
	CreatedAt  time.Time               `json:"created_at"`
	LastUsedAt time.Time               `json:"last_used_at"`
	TxCount    int64                   `json:"tx_count"`
	IsRetiring bool                    `json:"is_retiring"`
	LockedBy   string                  `json:"locked_by,omitempty"`
	LockedAt   time.Time               `json:"locked_at,omitempty"`
	Balances   map[string]TokenBalance `json:"balances"` // key: token_type (e.g., "NEO", "GAS")
}

// RequestAccountsInput requests accounts from the pool.
type RequestAccountsInput struct {
	ServiceID string `json:"service_id"` // ID of requesting service (e.g., "neocompute")
	Count     int    `json:"count"`      // Number of accounts needed
	Purpose   string `json:"purpose"`    // Description of purpose (for audit)
}

// RequestAccountsResponse returns the requested accounts.
type RequestAccountsResponse struct {
	Accounts []AccountInfo `json:"accounts"`
	LockID   string        `json:"lock_id"` // ID to reference this lock for release/signing
}

// ReleaseAccountsInput releases previously requested accounts.
type ReleaseAccountsInput struct {
	ServiceID  string   `json:"service_id"`
	LockID     string   `json:"lock_id,omitempty"`     // Release by lock ID
	AccountIDs []string `json:"account_ids,omitempty"` // Or release specific accounts
}

// ReleaseAccountsResponse confirms release.
type ReleaseAccountsResponse struct {
	ReleasedCount int `json:"released_count"`
}

// SignTransactionInput signs a transaction with an account's private key.
type SignTransactionInput struct {
	ServiceID string `json:"service_id"`
	AccountID string `json:"account_id"`
	TxHash    []byte `json:"tx_hash"` // Transaction hash to sign (base64 in JSON)
}

// SignTransactionResponse returns the signature.
type SignTransactionResponse struct {
	AccountID string `json:"account_id"`
	Signature []byte `json:"signature"`  // base64 in JSON
	PublicKey []byte `json:"public_key"` // base64 in JSON (compressed)
}

// BatchSignInput signs multiple transaction hashes.
type BatchSignInput struct {
	ServiceID string        `json:"service_id"`
	Requests  []SignRequest `json:"requests"`
}

// SignRequest represents a single signing request within a batch.
type SignRequest struct {
	AccountID string `json:"account_id"`
	TxHash    []byte `json:"tx_hash"` // base64 in JSON
}

// BatchSignResponse returns multiple signatures.
type BatchSignResponse struct {
	Signatures []SignTransactionResponse `json:"signatures"`
	Errors     []string                  `json:"errors,omitempty"`
}

// UpdateBalanceInput updates an account's token balance.
type UpdateBalanceInput struct {
	ServiceID string `json:"service_id"`
	AccountID string `json:"account_id"`
	Token     string `json:"token"`              // Token type: "NEO", "GAS", or custom NEP-17
	Delta     int64  `json:"delta"`              // Positive to add, negative to subtract
	Absolute  *int64 `json:"absolute,omitempty"` // Or set absolute value
}

// UpdateBalanceResponse confirms balance update.
type UpdateBalanceResponse struct {
	AccountID  string `json:"account_id"`
	Token      string `json:"token"`
	OldBalance int64  `json:"old_balance"`
	NewBalance int64  `json:"new_balance"`
	TxCount    int64  `json:"tx_count"` // Updated transaction count
}

// PoolInfoResponse returns pool statistics with per-token breakdowns.
type PoolInfoResponse struct {
	TotalAccounts    int                   `json:"total_accounts"`
	ActiveAccounts   int                   `json:"active_accounts"`
	LockedAccounts   int                   `json:"locked_accounts"`
	RetiringAccounts int                   `json:"retiring_accounts"`
	TokenStats       map[string]TokenStats `json:"token_stats"` // key: token_type
}

// ListAccountsResponse returns filtered accounts.
type ListAccountsResponse struct {
	Accounts []AccountInfo `json:"accounts"`
}

// TransferInput transfers tokens from a pool account.
type TransferInput struct {
	ServiceID string `json:"service_id"`
	AccountID string `json:"account_id"`
	ToAddress string `json:"to_address"`
	Amount    int64  `json:"amount"`
	TokenHash string `json:"token_hash,omitempty"` // NEP-17 script hash (defaults to GAS)
}

// TransferResponse returns the transfer result.
type TransferResponse struct {
	TxHash    string `json:"tx_hash"`
	AccountID string `json:"account_id"`
	ToAddress string `json:"to_address"`
	Amount    int64  `json:"amount"`
}

// MasterKeyAttestation is a non-sensitive bundle proving the master key hash
// is bound to enclave report data. The quote is intended for off-chain
// verification; the account pool does not parse or validate it here.
type MasterKeyAttestation struct {
	Hash      string `json:"hash"`
	PubKey    string `json:"pubkey,omitempty"`
	Quote     string `json:"quote,omitempty"`
	MRENCLAVE string `json:"mrenclave,omitempty"`
	MRSIGNER  string `json:"mrsigner,omitempty"`
	ProdID    uint16 `json:"prod_id,omitempty"`
	ISVSVN    uint16 `json:"isvsvn,omitempty"`
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
	Simulated bool   `json:"simulated"`
}

// ContractParam represents a parameter for contract invocation.
type ContractParam struct {
	Type  string `json:"type"`  // Hash160, Integer, String, ByteArray, Array, etc.
	Value any    `json:"value"` // Parameter value
}

// DeployContractInput deploys a new contract using a pool account.
// All signing happens inside TEE - private keys never leave the enclave.
type DeployContractInput struct {
	ServiceID    string `json:"service_id"`
	AccountID    string `json:"account_id"`
	NEFBase64    string `json:"nef_base64"`      // Base64-encoded NEF file
	ManifestJSON string `json:"manifest_json"`   // JSON manifest string
	Data         any    `json:"data,omitempty"`  // Optional deployment data
}

// DeployContractResponse returns the deployment result.
type DeployContractResponse struct {
	TxHash       string `json:"tx_hash"`
	ContractHash string `json:"contract_hash"`
	GasConsumed  string `json:"gas_consumed"`
	AccountID    string `json:"account_id"`
}

// UpdateContractInput updates an existing contract using a pool account.
type UpdateContractInput struct {
	ServiceID    string `json:"service_id"`
	AccountID    string `json:"account_id"`
	ContractHash string `json:"contract_hash"`   // Existing contract hash to update
	NEFBase64    string `json:"nef_base64"`      // Base64-encoded NEF file
	ManifestJSON string `json:"manifest_json"`   // JSON manifest string
	Data         any    `json:"data,omitempty"`  // Optional update data
}

// UpdateContractResponse returns the update result.
type UpdateContractResponse struct {
	TxHash       string `json:"tx_hash"`
	ContractHash string `json:"contract_hash"`
	GasConsumed  string `json:"gas_consumed"`
	AccountID    string `json:"account_id"`
}

// InvokeContractInput invokes a contract method using a pool account.
// All signing happens inside TEE - private keys never leave the enclave.
type InvokeContractInput struct {
	ServiceID    string          `json:"service_id"`
	AccountID    string          `json:"account_id"`
	ContractHash string          `json:"contract_hash"` // Contract to invoke
	Method       string          `json:"method"`        // Method name
	Params       []ContractParam `json:"params"`        // Method parameters
}

// InvokeContractResponse returns the invocation result.
type InvokeContractResponse struct {
	TxHash      string `json:"tx_hash"`
	State       string `json:"state"`       // HALT or FAULT
	GasConsumed string `json:"gas_consumed"`
	Exception   string `json:"exception,omitempty"`
	AccountID   string `json:"account_id"`
}

// SimulateContractInput simulates a contract invocation without signing.
type SimulateContractInput struct {
	ServiceID    string          `json:"service_id"`
	AccountID    string          `json:"account_id"`    // Account to use as signer
	ContractHash string          `json:"contract_hash"` // Contract to invoke
	Method       string          `json:"method"`        // Method name
	Params       []ContractParam `json:"params"`        // Method parameters
}

// SimulateContractResponse returns the simulation result.
type SimulateContractResponse struct {
	State       string `json:"state"`       // HALT or FAULT
	GasConsumed string `json:"gas_consumed"`
	Exception   string `json:"exception,omitempty"`
	Stack       []any  `json:"stack,omitempty"` // Return values
}
