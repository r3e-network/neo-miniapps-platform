// Package client provides a client SDK for the NeoAccounts service.
//
// API types are re-exported from `infrastructure/accountpool/types` to keep a single
// canonical definition shared between server and clients.
package client

import neoaccountstypes "github.com/R3E-Network/service_layer/infrastructure/accountpool/types"

// Re-export token constants for convenience.
const (
	TokenTypeNEO = neoaccountstypes.TokenTypeNEO
	TokenTypeGAS = neoaccountstypes.TokenTypeGAS
)

type (
	TokenBalance            = neoaccountstypes.TokenBalance
	TokenStats              = neoaccountstypes.TokenStats
	AccountInfo             = neoaccountstypes.AccountInfo
	RequestAccountsInput    = neoaccountstypes.RequestAccountsInput
	RequestAccountsResponse = neoaccountstypes.RequestAccountsResponse
	ReleaseAccountsInput    = neoaccountstypes.ReleaseAccountsInput
	ReleaseAccountsResponse = neoaccountstypes.ReleaseAccountsResponse
	SignTransactionInput    = neoaccountstypes.SignTransactionInput
	SignTransactionResponse = neoaccountstypes.SignTransactionResponse
	BatchSignInput          = neoaccountstypes.BatchSignInput
	SignRequest             = neoaccountstypes.SignRequest
	BatchSignResponse       = neoaccountstypes.BatchSignResponse
	UpdateBalanceInput      = neoaccountstypes.UpdateBalanceInput
	UpdateBalanceResponse   = neoaccountstypes.UpdateBalanceResponse
	PoolInfoResponse        = neoaccountstypes.PoolInfoResponse
	ListAccountsResponse    = neoaccountstypes.ListAccountsResponse
	TransferInput           = neoaccountstypes.TransferInput
	TransferResponse        = neoaccountstypes.TransferResponse
	MasterKeyAttestation    = neoaccountstypes.MasterKeyAttestation

	// Contract operation types - all signing happens inside TEE
	ContractParam            = neoaccountstypes.ContractParam
	DeployContractInput      = neoaccountstypes.DeployContractInput
	DeployContractResponse   = neoaccountstypes.DeployContractResponse
	UpdateContractInput      = neoaccountstypes.UpdateContractInput
	UpdateContractResponse   = neoaccountstypes.UpdateContractResponse
	InvokeContractInput      = neoaccountstypes.InvokeContractInput
	InvokeContractResponse   = neoaccountstypes.InvokeContractResponse
	SimulateContractInput    = neoaccountstypes.SimulateContractInput
	SimulateContractResponse = neoaccountstypes.SimulateContractResponse
)
