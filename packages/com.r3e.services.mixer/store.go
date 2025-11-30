package mixer

import (
	"context"
	"time"
)

// Store defines the persistence interface for the mixer service.
type Store interface {
	// MixRequest operations
	CreateMixRequest(ctx context.Context, req MixRequest) (MixRequest, error)
	UpdateMixRequest(ctx context.Context, req MixRequest) (MixRequest, error)
	GetMixRequest(ctx context.Context, id string) (MixRequest, error)
	GetMixRequestByProofHash(ctx context.Context, proofHash string) (MixRequest, error)
	ListMixRequests(ctx context.Context, accountID string, limit int) ([]MixRequest, error)
	ListMixRequestsByStatus(ctx context.Context, status RequestStatus, limit int) ([]MixRequest, error)
	ListPendingMixRequests(ctx context.Context) ([]MixRequest, error)
	ListExpiredMixRequests(ctx context.Context, before time.Time) ([]MixRequest, error)

	// PoolAccount operations
	CreatePoolAccount(ctx context.Context, pool PoolAccount) (PoolAccount, error)
	UpdatePoolAccount(ctx context.Context, pool PoolAccount) (PoolAccount, error)
	GetPoolAccount(ctx context.Context, id string) (PoolAccount, error)
	GetPoolAccountByWallet(ctx context.Context, wallet string) (PoolAccount, error)
	ListPoolAccounts(ctx context.Context, status PoolAccountStatus) ([]PoolAccount, error)
	ListActivePoolAccounts(ctx context.Context) ([]PoolAccount, error)
	ListRetirablePoolAccounts(ctx context.Context, before time.Time) ([]PoolAccount, error)

	// MixTransaction operations
	CreateMixTransaction(ctx context.Context, tx MixTransaction) (MixTransaction, error)
	UpdateMixTransaction(ctx context.Context, tx MixTransaction) (MixTransaction, error)
	GetMixTransaction(ctx context.Context, id string) (MixTransaction, error)
	GetMixTransactionByHash(ctx context.Context, txHash string) (MixTransaction, error)
	ListMixTransactions(ctx context.Context, requestID string, limit int) ([]MixTransaction, error)
	ListMixTransactionsByPool(ctx context.Context, poolID string, limit int) ([]MixTransaction, error)
	ListScheduledMixTransactions(ctx context.Context, before time.Time, limit int) ([]MixTransaction, error)
	ListPendingMixTransactions(ctx context.Context) ([]MixTransaction, error)

	// WithdrawalClaim operations
	CreateWithdrawalClaim(ctx context.Context, claim WithdrawalClaim) (WithdrawalClaim, error)
	UpdateWithdrawalClaim(ctx context.Context, claim WithdrawalClaim) (WithdrawalClaim, error)
	GetWithdrawalClaim(ctx context.Context, id string) (WithdrawalClaim, error)
	GetWithdrawalClaimByRequest(ctx context.Context, requestID string) (WithdrawalClaim, error)
	ListWithdrawalClaims(ctx context.Context, accountID string, limit int) ([]WithdrawalClaim, error)
	ListClaimableWithdrawals(ctx context.Context, before time.Time) ([]WithdrawalClaim, error)

	// ServiceDeposit operations
	GetServiceDeposit(ctx context.Context) (ServiceDeposit, error)
	UpdateServiceDeposit(ctx context.Context, deposit ServiceDeposit) (ServiceDeposit, error)

	// Statistics
	GetMixStats(ctx context.Context) (MixStats, error)
}

// AccountChecker validates account existence.
type AccountChecker interface {
	AccountExists(ctx context.Context, accountID string) error
	AccountTenant(ctx context.Context, accountID string) string
}

// TEEManager handles TEE operations for key management and signing.
type TEEManager interface {
	// GeneratePoolKey creates a new key pair in TEE and returns the public address.
	GeneratePoolKey(ctx context.Context) (keyID string, walletAddress string, encryptedPrivKey string, err error)

	// SignTransaction signs a transaction using TEE-managed key.
	SignTransaction(ctx context.Context, keyID string, txData []byte) (signature []byte, err error)

	// GenerateZKProof generates a zero-knowledge proof for the mix request.
	GenerateZKProof(ctx context.Context, req MixRequest) (proofHash string, err error)

	// SignAttestation creates a TEE attestation signature.
	SignAttestation(ctx context.Context, data []byte) (signature string, err error)

	// VerifyAttestation verifies a TEE attestation.
	VerifyAttestation(ctx context.Context, data []byte, signature string) (bool, error)
}

// ChainClient handles blockchain interactions.
type ChainClient interface {
	// GetBalance returns the balance of an address.
	GetBalance(ctx context.Context, address string, tokenAddress string) (string, error)

	// SendTransaction submits a signed transaction to the chain.
	SendTransaction(ctx context.Context, signedTx []byte) (txHash string, err error)

	// GetTransactionStatus checks if a transaction is confirmed.
	GetTransactionStatus(ctx context.Context, txHash string) (confirmed bool, blockNumber int64, err error)

	// BuildTransferTx builds an unsigned transfer transaction.
	BuildTransferTx(ctx context.Context, from, to, amount, tokenAddress string) (txData []byte, err error)

	// SubmitMixProof submits the ZKP and TEE signature to the on-chain contract.
	SubmitMixProof(ctx context.Context, requestID, proofHash, teeSignature string) (txHash string, err error)

	// SubmitCompletionProof submits the completion proof to the on-chain contract.
	SubmitCompletionProof(ctx context.Context, requestID string, deliveredAmount string) (txHash string, err error)

	// GetWithdrawableRequests returns requests that users can force-withdraw.
	GetWithdrawableRequests(ctx context.Context) ([]string, error)
}
