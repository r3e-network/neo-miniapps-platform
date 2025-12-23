package neoaccounts

import (
	neoaccountssupabase "github.com/R3E-Network/service_layer/infrastructure/accountpool/supabase"
	neoaccountstypes "github.com/R3E-Network/service_layer/infrastructure/accountpool/types"
)

// Re-export token constants for convenience
const (
	TokenTypeNEO = neoaccountstypes.TokenTypeNEO
	TokenTypeGAS = neoaccountstypes.TokenTypeGAS
)

// TokenBalance is the API representation of a token balance.
type TokenBalance = neoaccountstypes.TokenBalance

// TokenStats represents aggregated statistics for a token type.
type TokenStats = neoaccountstypes.TokenStats

// AccountInfo represents public account information returned to clients.
// Private keys are never exposed. Balances are tracked per-token.
type AccountInfo = neoaccountstypes.AccountInfo

// RequestAccountsInput for requesting accounts from the pool.
type RequestAccountsInput = neoaccountstypes.RequestAccountsInput

// RequestAccountsResponse returns the requested accounts.
type RequestAccountsResponse = neoaccountstypes.RequestAccountsResponse

// ReleaseAccountsInput for releasing previously requested accounts.
type ReleaseAccountsInput = neoaccountstypes.ReleaseAccountsInput

// ReleaseAccountsResponse confirms release.
type ReleaseAccountsResponse = neoaccountstypes.ReleaseAccountsResponse

// SignTransactionInput for signing a transaction with an account's private key.
type SignTransactionInput = neoaccountstypes.SignTransactionInput

// SignTransactionResponse returns the signature.
type SignTransactionResponse = neoaccountstypes.SignTransactionResponse

// BatchSignInput for signing multiple transactions.
type BatchSignInput = neoaccountstypes.BatchSignInput

// SignRequest represents a single signing request within a batch.
type SignRequest = neoaccountstypes.SignRequest

// BatchSignResponse returns multiple signatures.
type BatchSignResponse = neoaccountstypes.BatchSignResponse

// UpdateBalanceInput for updating an account's token balance.
type UpdateBalanceInput = neoaccountstypes.UpdateBalanceInput

// UpdateBalanceResponse confirms balance update.
type UpdateBalanceResponse = neoaccountstypes.UpdateBalanceResponse

// PoolInfoResponse returns pool statistics with per-token breakdowns.
type PoolInfoResponse = neoaccountstypes.PoolInfoResponse

// ListAccountsInput for listing accounts with filters.
type ListAccountsInput struct {
	ServiceID  string `json:"service_id"`            // Required: only list accounts locked by this service
	Token      string `json:"token,omitempty"`       // Optional: filter by token type
	MinBalance *int64 `json:"min_balance,omitempty"` // Optional: minimum balance for specified token
}

// ListAccountsResponse returns filtered accounts.
type ListAccountsResponse = neoaccountstypes.ListAccountsResponse

// TransferInput for transferring tokens from a pool account.
type TransferInput = neoaccountstypes.TransferInput

// TransferResponse returns the transfer result.
type TransferResponse = neoaccountstypes.TransferResponse

// MasterKeyAttestation is a non-sensitive bundle proving the master key hash
// is bound to enclave report data.
type MasterKeyAttestation = neoaccountstypes.MasterKeyAttestation

// ContractParam represents a parameter for contract invocation.
type ContractParam = neoaccountstypes.ContractParam

// DeployContractInput deploys a new contract using a pool account.
type DeployContractInput = neoaccountstypes.DeployContractInput

// DeployContractResponse returns the deployment result.
type DeployContractResponse = neoaccountstypes.DeployContractResponse

// UpdateContractInput updates an existing contract using a pool account.
type UpdateContractInput = neoaccountstypes.UpdateContractInput

// UpdateContractResponse returns the update result.
type UpdateContractResponse = neoaccountstypes.UpdateContractResponse

// InvokeContractInput invokes a contract method using a pool account.
type InvokeContractInput = neoaccountstypes.InvokeContractInput

// InvokeContractResponse returns the invocation result.
type InvokeContractResponse = neoaccountstypes.InvokeContractResponse

// SimulateContractInput simulates a contract invocation without signing.
type SimulateContractInput = neoaccountstypes.SimulateContractInput

// SimulateContractResponse returns the simulation result.
type SimulateContractResponse = neoaccountstypes.SimulateContractResponse

// FundAccountInput funds a pool account from the master wallet.
type FundAccountInput = neoaccountstypes.FundAccountInput

// FundAccountResponse returns the funding result.
type FundAccountResponse = neoaccountstypes.FundAccountResponse

// InvokeMasterInput invokes a contract using the master wallet (TEE operations).
type InvokeMasterInput = neoaccountstypes.InvokeMasterInput

// DeployMasterInput deploys a contract using the master wallet.
type DeployMasterInput = neoaccountstypes.DeployMasterInput

// DeployMasterResponse returns the deployment result using master wallet.
type DeployMasterResponse = neoaccountstypes.DeployMasterResponse

// AccountInfoFromWithBalances converts AccountWithBalances to AccountInfo.
func AccountInfoFromWithBalances(acc *neoaccountssupabase.AccountWithBalances) AccountInfo {
	return AccountInfo{
		ID:         acc.ID,
		Address:    acc.Address,
		CreatedAt:  acc.CreatedAt,
		LastUsedAt: acc.LastUsedAt,
		TxCount:    acc.TxCount,
		IsRetiring: acc.IsRetiring,
		LockedBy:   acc.LockedBy,
		LockedAt:   acc.LockedAt,
		Balances:   acc.Balances,
	}
}

// AccountInfoFromAccount converts Account to AccountInfo with empty balances.
func AccountInfoFromAccount(acc *neoaccountssupabase.Account) AccountInfo {
	return AccountInfo{
		ID:         acc.ID,
		Address:    acc.Address,
		CreatedAt:  acc.CreatedAt,
		LastUsedAt: acc.LastUsedAt,
		TxCount:    acc.TxCount,
		IsRetiring: acc.IsRetiring,
		LockedBy:   acc.LockedBy,
		LockedAt:   acc.LockedAt,
		Balances:   make(map[string]TokenBalance),
	}
}
