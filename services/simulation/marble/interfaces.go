// Package neosimulation provides simulation service for automated transaction testing.
package neosimulation

import (
	"context"

	neoaccountsclient "github.com/R3E-Network/service_layer/infrastructure/accountpool/client"
)

// PoolClientInterface defines the interface for pool client operations used by ContractInvoker.
// This interface allows for dependency injection and easier testing.
type PoolClientInterface interface {
	RequestAccounts(ctx context.Context, count int, purpose string) (*neoaccountsclient.RequestAccountsResponse, error)
	ReleaseAccounts(ctx context.Context, accountIDs []string) (*neoaccountsclient.ReleaseAccountsResponse, error)
	InvokeContract(ctx context.Context, accountID, contractHash, method string, params []neoaccountsclient.ContractParam, scope string) (*neoaccountsclient.InvokeContractResponse, error)
	InvokeMaster(ctx context.Context, contractHash, method string, params []neoaccountsclient.ContractParam, scope string) (*neoaccountsclient.InvokeContractResponse, error)
	FundAccount(ctx context.Context, toAddress string, amount int64) (*neoaccountsclient.FundAccountResponse, error)
}

// ContractInvokerInterface defines the interface for contract invocation operations.
// This interface allows for dependency injection and easier testing.
type ContractInvokerInterface interface {
	UpdatePriceFeed(ctx context.Context, symbol string) (string, error)
	RecordRandomness(ctx context.Context) (string, error)
	PayToApp(ctx context.Context, appID string, amount int64, memo string) (string, error)
	GetStats() map[string]interface{}
	GetPriceSymbols() []string
	GetLockedAccountCount() int
	ReleaseAllAccounts(ctx context.Context)
	Close()
}

// Verify interface compliance at compile time
var _ PoolClientInterface = (*neoaccountsclient.Client)(nil)
var _ ContractInvokerInterface = (*ContractInvoker)(nil)
