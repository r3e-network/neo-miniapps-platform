package storage

import (
	"context"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/automation"
	"github.com/R3E-Network/service_layer/internal/app/domain/ccip"
	"github.com/R3E-Network/service_layer/internal/app/domain/confidential"
	"github.com/R3E-Network/service_layer/internal/app/domain/cre"
	"github.com/R3E-Network/service_layer/internal/app/domain/datafeeds"
	"github.com/R3E-Network/service_layer/internal/app/domain/datalink"
	"github.com/R3E-Network/service_layer/internal/app/domain/datastreams"
	"github.com/R3E-Network/service_layer/internal/app/domain/dta"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	"github.com/R3E-Network/service_layer/internal/app/domain/secret"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	"github.com/R3E-Network/service_layer/internal/app/domain/vrf"
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
	CreateExecution(ctx context.Context, exec function.Execution) (function.Execution, error)
	GetExecution(ctx context.Context, id string) (function.Execution, error)
	ListFunctionExecutions(ctx context.Context, functionID string, limit int) ([]function.Execution, error)
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
	ListGasTransactions(ctx context.Context, gasAccountID string, limit int) ([]gasbank.Transaction, error)
	ListPendingWithdrawals(ctx context.Context) ([]gasbank.Transaction, error)

	UpsertWithdrawalApproval(ctx context.Context, approval gasbank.WithdrawalApproval) (gasbank.WithdrawalApproval, error)
	ListWithdrawalApprovals(ctx context.Context, transactionID string) ([]gasbank.WithdrawalApproval, error)

	SaveWithdrawalSchedule(ctx context.Context, schedule gasbank.WithdrawalSchedule) (gasbank.WithdrawalSchedule, error)
	GetWithdrawalSchedule(ctx context.Context, transactionID string) (gasbank.WithdrawalSchedule, error)
	DeleteWithdrawalSchedule(ctx context.Context, transactionID string) error
	ListDueWithdrawalSchedules(ctx context.Context, before time.Time, limit int) ([]gasbank.WithdrawalSchedule, error)

	RecordSettlementAttempt(ctx context.Context, attempt gasbank.SettlementAttempt) (gasbank.SettlementAttempt, error)
	ListSettlementAttempts(ctx context.Context, transactionID string, limit int) ([]gasbank.SettlementAttempt, error)

	UpsertDeadLetter(ctx context.Context, entry gasbank.DeadLetter) (gasbank.DeadLetter, error)
	GetDeadLetter(ctx context.Context, transactionID string) (gasbank.DeadLetter, error)
	ListDeadLetters(ctx context.Context, accountID string, limit int) ([]gasbank.DeadLetter, error)
	RemoveDeadLetter(ctx context.Context, transactionID string) error
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
	DeletePriceFeed(ctx context.Context, feedID string) error

	CreatePriceSnapshot(ctx context.Context, snap pricefeed.Snapshot) (pricefeed.Snapshot, error)
	ListPriceSnapshots(ctx context.Context, feedID string) ([]pricefeed.Snapshot, error)

	CreatePriceRound(ctx context.Context, round pricefeed.Round) (pricefeed.Round, error)
	GetLatestPriceRound(ctx context.Context, feedID string) (pricefeed.Round, error)
	ListPriceRounds(ctx context.Context, feedID string, limit int) ([]pricefeed.Round, error)
	UpdatePriceRound(ctx context.Context, round pricefeed.Round) (pricefeed.Round, error)
	CreatePriceObservation(ctx context.Context, obs pricefeed.Observation) (pricefeed.Observation, error)
	ListPriceObservations(ctx context.Context, feedID string, roundID int64, limit int) ([]pricefeed.Observation, error)
}

// DataFeedStore persists centralized Chainlink data feed configs and updates.
type DataFeedStore interface {
	CreateDataFeed(ctx context.Context, feed datafeeds.Feed) (datafeeds.Feed, error)
	UpdateDataFeed(ctx context.Context, feed datafeeds.Feed) (datafeeds.Feed, error)
	GetDataFeed(ctx context.Context, id string) (datafeeds.Feed, error)
	ListDataFeeds(ctx context.Context, accountID string) ([]datafeeds.Feed, error)

	CreateDataFeedUpdate(ctx context.Context, upd datafeeds.Update) (datafeeds.Update, error)
	ListDataFeedUpdates(ctx context.Context, feedID string, limit int) ([]datafeeds.Update, error)
	ListDataFeedUpdatesByRound(ctx context.Context, feedID string, roundID int64) ([]datafeeds.Update, error)
	GetLatestDataFeedUpdate(ctx context.Context, feedID string) (datafeeds.Update, error)
}

// VRFStore persists VRF keys and requests.
type VRFStore interface {
	CreateVRFKey(ctx context.Context, key vrf.Key) (vrf.Key, error)
	UpdateVRFKey(ctx context.Context, key vrf.Key) (vrf.Key, error)
	GetVRFKey(ctx context.Context, id string) (vrf.Key, error)
	ListVRFKeys(ctx context.Context, accountID string) ([]vrf.Key, error)

	CreateVRFRequest(ctx context.Context, req vrf.Request) (vrf.Request, error)
	GetVRFRequest(ctx context.Context, id string) (vrf.Request, error)
	ListVRFRequests(ctx context.Context, accountID string, limit int) ([]vrf.Request, error)
}

// DataStreamStore persists data stream configs and frames.
type DataStreamStore interface {
	CreateStream(ctx context.Context, stream datastreams.Stream) (datastreams.Stream, error)
	UpdateStream(ctx context.Context, stream datastreams.Stream) (datastreams.Stream, error)
	GetStream(ctx context.Context, id string) (datastreams.Stream, error)
	ListStreams(ctx context.Context, accountID string) ([]datastreams.Stream, error)

	CreateFrame(ctx context.Context, frame datastreams.Frame) (datastreams.Frame, error)
	ListFrames(ctx context.Context, streamID string, limit int) ([]datastreams.Frame, error)
	GetLatestFrame(ctx context.Context, streamID string) (datastreams.Frame, error)
}

// DataLinkStore persists datalink channels and deliveries.
type DataLinkStore interface {
	CreateChannel(ctx context.Context, ch datalink.Channel) (datalink.Channel, error)
	UpdateChannel(ctx context.Context, ch datalink.Channel) (datalink.Channel, error)
	GetChannel(ctx context.Context, id string) (datalink.Channel, error)
	ListChannels(ctx context.Context, accountID string) ([]datalink.Channel, error)

	CreateDelivery(ctx context.Context, del datalink.Delivery) (datalink.Delivery, error)
	GetDelivery(ctx context.Context, id string) (datalink.Delivery, error)
	ListDeliveries(ctx context.Context, accountID string, limit int) ([]datalink.Delivery, error)
}

// DTAStore persists DTA products and orders.
type DTAStore interface {
	CreateProduct(ctx context.Context, product dta.Product) (dta.Product, error)
	UpdateProduct(ctx context.Context, product dta.Product) (dta.Product, error)
	GetProduct(ctx context.Context, id string) (dta.Product, error)
	ListProducts(ctx context.Context, accountID string) ([]dta.Product, error)

	CreateOrder(ctx context.Context, order dta.Order) (dta.Order, error)
	GetOrder(ctx context.Context, id string) (dta.Order, error)
	ListOrders(ctx context.Context, accountID string, limit int) ([]dta.Order, error)
}

// ConfidentialStore persists enclave + sealed key metadata.
type ConfidentialStore interface {
	CreateEnclave(ctx context.Context, enclave confidential.Enclave) (confidential.Enclave, error)
	UpdateEnclave(ctx context.Context, enclave confidential.Enclave) (confidential.Enclave, error)
	GetEnclave(ctx context.Context, id string) (confidential.Enclave, error)
	ListEnclaves(ctx context.Context, accountID string) ([]confidential.Enclave, error)

	CreateSealedKey(ctx context.Context, key confidential.SealedKey) (confidential.SealedKey, error)
	ListSealedKeys(ctx context.Context, accountID, enclaveID string, limit int) ([]confidential.SealedKey, error)

	CreateAttestation(ctx context.Context, att confidential.Attestation) (confidential.Attestation, error)
	ListAttestations(ctx context.Context, accountID, enclaveID string, limit int) ([]confidential.Attestation, error)
	ListAccountAttestations(ctx context.Context, accountID string, limit int) ([]confidential.Attestation, error)
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
	ListRequests(ctx context.Context, accountID string, limit int, status string) ([]oracle.Request, error)
	ListPendingRequests(ctx context.Context) ([]oracle.Request, error)
}

// SecretStore persists account secrets.
type SecretStore interface {
	CreateSecret(ctx context.Context, sec secret.Secret) (secret.Secret, error)
	UpdateSecret(ctx context.Context, sec secret.Secret) (secret.Secret, error)
	GetSecret(ctx context.Context, accountID, name string) (secret.Secret, error)
	ListSecrets(ctx context.Context, accountID string) ([]secret.Secret, error)
	DeleteSecret(ctx context.Context, accountID, name string) error
}

// CREStore persists Chainlink CRE assets.
type CREStore interface {
	CreatePlaybook(ctx context.Context, pb cre.Playbook) (cre.Playbook, error)
	UpdatePlaybook(ctx context.Context, pb cre.Playbook) (cre.Playbook, error)
	GetPlaybook(ctx context.Context, id string) (cre.Playbook, error)
	ListPlaybooks(ctx context.Context, accountID string) ([]cre.Playbook, error)

	CreateRun(ctx context.Context, run cre.Run) (cre.Run, error)
	UpdateRun(ctx context.Context, run cre.Run) (cre.Run, error)
	GetRun(ctx context.Context, id string) (cre.Run, error)
	ListRuns(ctx context.Context, accountID string, limit int) ([]cre.Run, error)

	CreateExecutor(ctx context.Context, exec cre.Executor) (cre.Executor, error)
	UpdateExecutor(ctx context.Context, exec cre.Executor) (cre.Executor, error)
	GetExecutor(ctx context.Context, id string) (cre.Executor, error)
	ListExecutors(ctx context.Context, accountID string) ([]cre.Executor, error)
}

// WorkspaceWalletStore exposes wallet records per workspace.
type WorkspaceWalletStore interface {
	CreateWorkspaceWallet(ctx context.Context, wallet account.WorkspaceWallet) (account.WorkspaceWallet, error)
	GetWorkspaceWallet(ctx context.Context, id string) (account.WorkspaceWallet, error)
	ListWorkspaceWallets(ctx context.Context, workspaceID string) ([]account.WorkspaceWallet, error)
	FindWorkspaceWalletByAddress(ctx context.Context, workspaceID, wallet string) (account.WorkspaceWallet, error)
}

// CCIPStore persists CCIP lanes and messages.
type CCIPStore interface {
	CreateLane(ctx context.Context, lane ccip.Lane) (ccip.Lane, error)
	UpdateLane(ctx context.Context, lane ccip.Lane) (ccip.Lane, error)
	GetLane(ctx context.Context, id string) (ccip.Lane, error)
	ListLanes(ctx context.Context, accountID string) ([]ccip.Lane, error)

	CreateMessage(ctx context.Context, msg ccip.Message) (ccip.Message, error)
	UpdateMessage(ctx context.Context, msg ccip.Message) (ccip.Message, error)
	GetMessage(ctx context.Context, id string) (ccip.Message, error)
	ListMessages(ctx context.Context, accountID string, limit int) ([]ccip.Message, error)
}
