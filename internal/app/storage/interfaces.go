package storage

import (
	"context"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/automation"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
)

// AccountStore persists account records.
type AccountStore interface {
	CreateAccount(ctx context.Context, acct account.Account) (account.Account, error)
	UpdateAccount(ctx context.Context, acct account.Account) (account.Account, error)
	GetAccount(ctx context.Context, id string) (account.Account, error)
	ListAccounts(ctx context.Context) ([]account.Account, error)
	DeleteAccount(ctx context.Context, id string) error
}

// FunctionStore persists function definitions.
type FunctionStore interface {
	CreateFunction(ctx context.Context, def function.Definition) (function.Definition, error)
	UpdateFunction(ctx context.Context, def function.Definition) (function.Definition, error)
	GetFunction(ctx context.Context, id string) (function.Definition, error)
	ListFunctions(ctx context.Context, accountID string) ([]function.Definition, error)
}

// TriggerStore persists trigger records.
type TriggerStore interface {
	CreateTrigger(ctx context.Context, trg trigger.Trigger) (trigger.Trigger, error)
	UpdateTrigger(ctx context.Context, trg trigger.Trigger) (trigger.Trigger, error)
	GetTrigger(ctx context.Context, id string) (trigger.Trigger, error)
	ListTriggers(ctx context.Context, accountID string) ([]trigger.Trigger, error)
}

// GasBankStore persists gas bank accounts and transactions.
type GasBankStore interface {
	CreateGasAccount(ctx context.Context, acct gasbank.Account) (gasbank.Account, error)
	UpdateGasAccount(ctx context.Context, acct gasbank.Account) (gasbank.Account, error)
	GetGasAccount(ctx context.Context, id string) (gasbank.Account, error)
	GetGasAccountByWallet(ctx context.Context, wallet string) (gasbank.Account, error)
	ListGasAccounts(ctx context.Context, accountID string) ([]gasbank.Account, error)

	CreateGasTransaction(ctx context.Context, tx gasbank.Transaction) (gasbank.Transaction, error)
	UpdateGasTransaction(ctx context.Context, tx gasbank.Transaction) (gasbank.Transaction, error)
	GetGasTransaction(ctx context.Context, id string) (gasbank.Transaction, error)
	ListGasTransactions(ctx context.Context, gasAccountID string) ([]gasbank.Transaction, error)
	ListPendingWithdrawals(ctx context.Context) ([]gasbank.Transaction, error)
}

// AutomationStore persists automation jobs.
type AutomationStore interface {
	CreateAutomationJob(ctx context.Context, job automation.Job) (automation.Job, error)
	UpdateAutomationJob(ctx context.Context, job automation.Job) (automation.Job, error)
	GetAutomationJob(ctx context.Context, id string) (automation.Job, error)
	ListAutomationJobs(ctx context.Context, accountID string) ([]automation.Job, error)
}

// PriceFeedStore persists price feed definitions and snapshots.
type PriceFeedStore interface {
	CreatePriceFeed(ctx context.Context, feed pricefeed.Feed) (pricefeed.Feed, error)
	UpdatePriceFeed(ctx context.Context, feed pricefeed.Feed) (pricefeed.Feed, error)
	GetPriceFeed(ctx context.Context, id string) (pricefeed.Feed, error)
	ListPriceFeeds(ctx context.Context, accountID string) ([]pricefeed.Feed, error)

	CreatePriceSnapshot(ctx context.Context, snap pricefeed.Snapshot) (pricefeed.Snapshot, error)
	ListPriceSnapshots(ctx context.Context, feedID string) ([]pricefeed.Snapshot, error)
}

// OracleStore persists oracle data sources and requests.
type OracleStore interface {
	CreateDataSource(ctx context.Context, src oracle.DataSource) (oracle.DataSource, error)
	UpdateDataSource(ctx context.Context, src oracle.DataSource) (oracle.DataSource, error)
	GetDataSource(ctx context.Context, id string) (oracle.DataSource, error)
	ListDataSources(ctx context.Context, accountID string) ([]oracle.DataSource, error)

	CreateRequest(ctx context.Context, req oracle.Request) (oracle.Request, error)
	UpdateRequest(ctx context.Context, req oracle.Request) (oracle.Request, error)
	GetRequest(ctx context.Context, id string) (oracle.Request, error)
	ListRequests(ctx context.Context, accountID string) ([]oracle.Request, error)
	ListPendingRequests(ctx context.Context) ([]oracle.Request, error)
}
