package memory

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/domain/account"
	"github.com/R3E-Network/service_layer/domain/automation"
	"github.com/R3E-Network/service_layer/domain/datafeeds"
	"github.com/R3E-Network/service_layer/domain/datalink"
	"github.com/R3E-Network/service_layer/domain/datastreams"
	"github.com/R3E-Network/service_layer/domain/function"
	"github.com/R3E-Network/service_layer/domain/gasbank"
	"github.com/R3E-Network/service_layer/domain/oracle"
	"github.com/R3E-Network/service_layer/domain/secret"
	"github.com/R3E-Network/service_layer/domain/vrf"
)

func TestStoreCreateAccountAndFunction(t *testing.T) {
	store := New()

	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	fn, err := store.CreateFunction(context.Background(), function.Definition{AccountID: acct.ID, Name: "hello", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}
	if fn.AccountID != acct.ID {
		t.Fatalf("expected function to retain account id")
	}

	exec, err := store.CreateExecution(context.Background(), function.Execution{AccountID: acct.ID, FunctionID: fn.ID})
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}

	list, err := store.ListFunctionExecutions(context.Background(), fn.ID, 0)
	if err != nil || len(list) != 1 || list[0].ID != exec.ID {
		t.Fatalf("expected execution to be listed, got %#v err=%v", list, err)
	}
}

// AccountStore tests

func TestAccountStore_CRUD(t *testing.T) {
	ctx := context.Background()
	store := New()

	// Create
	acct, err := store.CreateAccount(ctx, account.Account{
		Owner:    "test-owner",
		Metadata: map[string]string{"key": "value"},
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if acct.ID == "" {
		t.Fatal("expected account ID to be set")
	}
	if acct.Owner != "test-owner" {
		t.Fatalf("expected owner 'test-owner', got %q", acct.Owner)
	}
	if acct.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}

	// Get
	retrieved, err := store.GetAccount(ctx, acct.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if retrieved.Owner != acct.Owner {
		t.Fatalf("expected owner %q, got %q", acct.Owner, retrieved.Owner)
	}

	// Update
	acct.Owner = "updated-owner"
	updated, err := store.UpdateAccount(ctx, acct)
	if err != nil {
		t.Fatalf("update account: %v", err)
	}
	if updated.Owner != "updated-owner" {
		t.Fatalf("expected updated owner, got %q", updated.Owner)
	}

	// List
	accounts, err := store.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}

	// Delete
	if err := store.DeleteAccount(ctx, acct.ID); err != nil {
		t.Fatalf("delete account: %v", err)
	}

	// Verify deleted
	_, err = store.GetAccount(ctx, acct.ID)
	if err == nil {
		t.Fatal("expected error getting deleted account")
	}
}

func TestAccountStore_Errors(t *testing.T) {
	ctx := context.Background()
	store := New()

	// Get non-existent
	_, err := store.GetAccount(ctx, "non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent account")
	}

	// Update non-existent
	_, err = store.UpdateAccount(ctx, account.Account{ID: "non-existent"})
	if err == nil {
		t.Fatal("expected error updating non-existent account")
	}

	// Delete non-existent
	err = store.DeleteAccount(ctx, "non-existent")
	if err == nil {
		t.Fatal("expected error deleting non-existent account")
	}

	// Duplicate ID
	acct, _ := store.CreateAccount(ctx, account.Account{ID: "dup-id", Owner: "owner1"})
	_, err = store.CreateAccount(ctx, account.Account{ID: acct.ID, Owner: "owner2"})
	if err == nil {
		t.Fatal("expected error creating duplicate account")
	}
}

// FunctionStore tests

func TestFunctionStore_CRUD(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})

	fn, err := store.CreateFunction(ctx, function.Definition{
		AccountID: acct.ID,
		Name:      "test-fn",
		Source:    "() => 42",
		Secrets:   []string{"secret1"},
	})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}
	if fn.ID == "" {
		t.Fatal("expected function ID")
	}

	retrieved, err := store.GetFunction(ctx, fn.ID)
	if err != nil {
		t.Fatalf("get function: %v", err)
	}
	if retrieved.Name != "test-fn" {
		t.Fatalf("expected name 'test-fn', got %q", retrieved.Name)
	}

	fn.Name = "updated-fn"
	updated, err := store.UpdateFunction(ctx, fn)
	if err != nil {
		t.Fatalf("update function: %v", err)
	}
	if updated.Name != "updated-fn" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}

	fns, err := store.ListFunctions(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list functions: %v", err)
	}
	if len(fns) != 1 {
		t.Fatalf("expected 1 function, got %d", len(fns))
	}
}

func TestFunctionStore_Executions(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})
	fn, _ := store.CreateFunction(ctx, function.Definition{AccountID: acct.ID, Name: "fn"})

	for i := 0; i < 5; i++ {
		_, err := store.CreateExecution(ctx, function.Execution{
			AccountID:  acct.ID,
			FunctionID: fn.ID,
			StartedAt:  time.Now().Add(time.Duration(i) * time.Minute),
		})
		if err != nil {
			t.Fatalf("create execution %d: %v", i, err)
		}
	}

	execs, err := store.ListFunctionExecutions(ctx, fn.ID, 3)
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(execs) != 3 {
		t.Fatalf("expected 3 executions, got %d", len(execs))
	}

	exec, err := store.GetExecution(ctx, execs[0].ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if exec.FunctionID != fn.ID {
		t.Fatalf("expected function ID %q, got %q", fn.ID, exec.FunctionID)
	}
}

// GasBankStore tests

func TestGasBankStore_Accounts(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})

	gasAcct, err := store.CreateGasAccount(ctx, gasbank.Account{
		AccountID:     acct.ID,
		WalletAddress: "0x123",
		Balance:       1000.0,
	})
	if err != nil {
		t.Fatalf("create gas account: %v", err)
	}

	retrieved, err := store.GetGasAccount(ctx, gasAcct.ID)
	if err != nil {
		t.Fatalf("get gas account: %v", err)
	}
	if retrieved.Balance != 1000.0 {
		t.Fatalf("expected balance 1000, got %v", retrieved.Balance)
	}

	byWallet, err := store.GetGasAccountByWallet(ctx, "0x123")
	if err != nil {
		t.Fatalf("get gas account by wallet: %v", err)
	}
	if byWallet.ID != gasAcct.ID {
		t.Fatalf("expected ID %q, got %q", gasAcct.ID, byWallet.ID)
	}

	gasAcct.Balance = 2000.0
	updated, err := store.UpdateGasAccount(ctx, gasAcct)
	if err != nil {
		t.Fatalf("update gas account: %v", err)
	}
	if updated.Balance != 2000.0 {
		t.Fatalf("expected updated balance, got %v", updated.Balance)
	}

	accounts, err := store.ListGasAccounts(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list gas accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 gas account, got %d", len(accounts))
	}
}

func TestGasBankStore_Transactions(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})
	gasAcct, _ := store.CreateGasAccount(ctx, gasbank.Account{AccountID: acct.ID, WalletAddress: "0x123"})

	tx, err := store.CreateGasTransaction(ctx, gasbank.Transaction{
		AccountID: gasAcct.ID,
		Type:      gasbank.TransactionDeposit,
		Amount:    100.0,
		Status:    gasbank.StatusPending,
	})
	if err != nil {
		t.Fatalf("create gas transaction: %v", err)
	}

	retrieved, err := store.GetGasTransaction(ctx, tx.ID)
	if err != nil {
		t.Fatalf("get gas transaction: %v", err)
	}
	if retrieved.Amount != 100.0 {
		t.Fatalf("expected amount 100, got %v", retrieved.Amount)
	}

	tx.Status = gasbank.StatusCompleted
	updated, err := store.UpdateGasTransaction(ctx, tx)
	if err != nil {
		t.Fatalf("update gas transaction: %v", err)
	}
	if updated.Status != gasbank.StatusCompleted {
		t.Fatalf("expected status 'completed', got %q", updated.Status)
	}

	txs, err := store.ListGasTransactions(ctx, gasAcct.ID, 10)
	if err != nil {
		t.Fatalf("list gas transactions: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txs))
	}
}

func TestGasBankStore_WithdrawalApprovals(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})
	gasAcct, _ := store.CreateGasAccount(ctx, gasbank.Account{AccountID: acct.ID, WalletAddress: "0x123"})
	tx, _ := store.CreateGasTransaction(ctx, gasbank.Transaction{AccountID: gasAcct.ID, Type: gasbank.TransactionWithdrawal})

	approval, err := store.UpsertWithdrawalApproval(ctx, gasbank.WithdrawalApproval{
		TransactionID: tx.ID,
		Approver:      "0xapprover1",
		Status:        gasbank.ApprovalApproved,
	})
	if err != nil {
		t.Fatalf("upsert withdrawal approval: %v", err)
	}
	if approval.TransactionID != tx.ID {
		t.Fatalf("expected transaction ID %q, got %q", tx.ID, approval.TransactionID)
	}

	approvals, err := store.ListWithdrawalApprovals(ctx, tx.ID)
	if err != nil {
		t.Fatalf("list withdrawal approvals: %v", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("expected 1 approval, got %d", len(approvals))
	}
}

func TestGasBankStore_WithdrawalSchedules(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})
	gasAcct, _ := store.CreateGasAccount(ctx, gasbank.Account{AccountID: acct.ID, WalletAddress: "0x123"})
	tx, _ := store.CreateGasTransaction(ctx, gasbank.Transaction{AccountID: gasAcct.ID, Type: gasbank.TransactionWithdrawal})

	scheduleAt := time.Now().Add(-time.Hour)

	schedule, err := store.SaveWithdrawalSchedule(ctx, gasbank.WithdrawalSchedule{
		TransactionID: tx.ID,
		ScheduleAt:    scheduleAt,
	})
	if err != nil {
		t.Fatalf("save withdrawal schedule: %v", err)
	}

	retrieved, err := store.GetWithdrawalSchedule(ctx, tx.ID)
	if err != nil {
		t.Fatalf("get withdrawal schedule: %v", err)
	}
	if retrieved.TransactionID != tx.ID {
		t.Fatalf("expected transaction ID %q, got %q", tx.ID, retrieved.TransactionID)
	}

	due, err := store.ListDueWithdrawalSchedules(ctx, time.Now(), 10)
	if err != nil {
		t.Fatalf("list due schedules: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("expected 1 due schedule, got %d", len(due))
	}

	if err := store.DeleteWithdrawalSchedule(ctx, schedule.TransactionID); err != nil {
		t.Fatalf("delete schedule: %v", err)
	}

	_, err = store.GetWithdrawalSchedule(ctx, tx.ID)
	if err == nil {
		t.Fatal("expected error getting deleted schedule")
	}
}

func TestGasBankStore_SettlementAttempts(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})
	gasAcct, _ := store.CreateGasAccount(ctx, gasbank.Account{AccountID: acct.ID, WalletAddress: "0x123"})
	tx, _ := store.CreateGasTransaction(ctx, gasbank.Transaction{AccountID: gasAcct.ID, Type: gasbank.TransactionWithdrawal})

	attempt, err := store.RecordSettlementAttempt(ctx, gasbank.SettlementAttempt{
		TransactionID: tx.ID,
		Attempt:       1,
		Status:        "failed",
		Error:         "network error",
	})
	if err != nil {
		t.Fatalf("record settlement attempt: %v", err)
	}
	if attempt.Attempt != 1 {
		t.Fatalf("expected attempt number 1, got %d", attempt.Attempt)
	}

	attempts, err := store.ListSettlementAttempts(ctx, tx.ID, 10)
	if err != nil {
		t.Fatalf("list settlement attempts: %v", err)
	}
	if len(attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(attempts))
	}
}

func TestGasBankStore_DeadLetters(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})
	gasAcct, _ := store.CreateGasAccount(ctx, gasbank.Account{AccountID: acct.ID, WalletAddress: "0x123"})
	tx, _ := store.CreateGasTransaction(ctx, gasbank.Transaction{AccountID: gasAcct.ID, Type: gasbank.TransactionWithdrawal})

	dl, err := store.UpsertDeadLetter(ctx, gasbank.DeadLetter{
		TransactionID: tx.ID,
		AccountID:     acct.ID,
		Reason:        "max retries exceeded",
	})
	if err != nil {
		t.Fatalf("upsert dead letter: %v", err)
	}

	retrieved, err := store.GetDeadLetter(ctx, tx.ID)
	if err != nil {
		t.Fatalf("get dead letter: %v", err)
	}
	if retrieved.Reason != "max retries exceeded" {
		t.Fatalf("expected reason, got %q", retrieved.Reason)
	}

	dls, err := store.ListDeadLetters(ctx, acct.ID, 10)
	if err != nil {
		t.Fatalf("list dead letters: %v", err)
	}
	if len(dls) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(dls))
	}

	if err := store.RemoveDeadLetter(ctx, dl.TransactionID); err != nil {
		t.Fatalf("remove dead letter: %v", err)
	}

	_, err = store.GetDeadLetter(ctx, tx.ID)
	if err == nil {
		t.Fatal("expected error getting removed dead letter")
	}
}

// AutomationStore tests

func TestAutomationStore_CRUD(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})

	job, err := store.CreateAutomationJob(ctx, automation.Job{
		AccountID:  acct.ID,
		Name:       "test-job",
		Schedule:   "0 * * * *",
		FunctionID: "fn-1",
	})
	if err != nil {
		t.Fatalf("create automation job: %v", err)
	}
	if job.ID == "" {
		t.Fatal("expected job ID")
	}

	retrieved, err := store.GetAutomationJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("get automation job: %v", err)
	}
	if retrieved.Name != "test-job" {
		t.Fatalf("expected name 'test-job', got %q", retrieved.Name)
	}

	job.Name = "updated-job"
	updated, err := store.UpdateAutomationJob(ctx, job)
	if err != nil {
		t.Fatalf("update automation job: %v", err)
	}
	if updated.Name != "updated-job" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}

	jobs, err := store.ListAutomationJobs(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list automation jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
}

// OracleStore tests

func TestOracleStore_Sources(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})

	source, err := store.CreateDataSource(ctx, oracle.DataSource{
		AccountID: acct.ID,
		Name:      "test-source",
		URL:       "https://api.example.com/data",
	})
	if err != nil {
		t.Fatalf("create oracle source: %v", err)
	}

	retrieved, err := store.GetDataSource(ctx, source.ID)
	if err != nil {
		t.Fatalf("get oracle source: %v", err)
	}
	if retrieved.Name != "test-source" {
		t.Fatalf("expected name 'test-source', got %q", retrieved.Name)
	}

	source.URL = "https://api.example.com/v2/data"
	updated, err := store.UpdateDataSource(ctx, source)
	if err != nil {
		t.Fatalf("update oracle source: %v", err)
	}
	if updated.URL != "https://api.example.com/v2/data" {
		t.Fatalf("expected updated URL, got %q", updated.URL)
	}

	sources, err := store.ListDataSources(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list oracle sources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
}

func TestOracleStore_Requests(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})

	req, err := store.CreateRequest(ctx, oracle.Request{
		AccountID:    acct.ID,
		DataSourceID: "source-1",
		Status:       oracle.StatusPending,
	})
	if err != nil {
		t.Fatalf("create oracle request: %v", err)
	}

	retrieved, err := store.GetRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("get oracle request: %v", err)
	}
	if retrieved.Status != oracle.StatusPending {
		t.Fatalf("expected status 'pending', got %q", retrieved.Status)
	}

	req.Status = oracle.StatusSucceeded
	req.Result = "test-result"
	_, err = store.UpdateRequest(ctx, req)
	if err != nil {
		t.Fatalf("update oracle request: %v", err)
	}

	requests, err := store.ListRequests(ctx, acct.ID, 10, "")
	if err != nil {
		t.Fatalf("list oracle requests: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
}

// SecretStore tests

func TestSecretStore_CRUD(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})

	sec, err := store.CreateSecret(ctx, secret.Secret{
		AccountID: acct.ID,
		Name:      "api-key",
		Value:     "encrypted-value",
	})
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}

	retrieved, err := store.GetSecret(ctx, acct.ID, "api-key")
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if retrieved.Name != "api-key" {
		t.Fatalf("expected name 'api-key', got %q", retrieved.Name)
	}
	if retrieved.ID != sec.ID {
		t.Fatalf("expected ID %q, got %q", sec.ID, retrieved.ID)
	}

	sec.Value = "new-encrypted-value"
	updated, err := store.UpdateSecret(ctx, sec)
	if err != nil {
		t.Fatalf("update secret: %v", err)
	}
	if updated.Value != "new-encrypted-value" {
		t.Fatalf("expected updated value, got %q", updated.Value)
	}

	secrets, err := store.ListSecrets(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list secrets: %v", err)
	}
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}

	if err := store.DeleteSecret(ctx, acct.ID, "api-key"); err != nil {
		t.Fatalf("delete secret: %v", err)
	}
}

// VRFStore tests

func TestVRFStore_Keys(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})

	key, err := store.CreateVRFKey(ctx, vrf.Key{
		AccountID: acct.ID,
		PublicKey: "pk-123",
	})
	if err != nil {
		t.Fatalf("create VRF key: %v", err)
	}

	retrieved, err := store.GetVRFKey(ctx, key.ID)
	if err != nil {
		t.Fatalf("get VRF key: %v", err)
	}
	if retrieved.PublicKey != "pk-123" {
		t.Fatalf("expected public key 'pk-123', got %q", retrieved.PublicKey)
	}

	keys, err := store.ListVRFKeys(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list VRF keys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
}

func TestVRFStore_Requests(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})

	req, err := store.CreateVRFRequest(ctx, vrf.Request{
		AccountID: acct.ID,
		KeyID:     "key-1",
		Seed:      "seed-123",
	})
	if err != nil {
		t.Fatalf("create VRF request: %v", err)
	}

	retrieved, err := store.GetVRFRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("get VRF request: %v", err)
	}
	if retrieved.Seed != "seed-123" {
		t.Fatalf("expected seed 'seed-123', got %q", retrieved.Seed)
	}

	requests, err := store.ListVRFRequests(ctx, acct.ID, 10)
	if err != nil {
		t.Fatalf("list VRF requests: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
}

// DataFeedStore tests

func TestDataFeedStore_CRUD(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})

	feed, err := store.CreateDataFeed(ctx, datafeeds.Feed{
		AccountID:   acct.ID,
		Pair:        "BTC/USD",
		Description: "Bitcoin price feed",
	})
	if err != nil {
		t.Fatalf("create data feed: %v", err)
	}

	retrieved, err := store.GetDataFeed(ctx, feed.ID)
	if err != nil {
		t.Fatalf("get data feed: %v", err)
	}
	if retrieved.Pair != "BTC/USD" {
		t.Fatalf("expected pair 'BTC/USD', got %q", retrieved.Pair)
	}

	feeds, err := store.ListDataFeeds(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list data feeds: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("expected 1 feed, got %d", len(feeds))
	}
}

func TestDataFeedStore_Updates(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})
	feed, _ := store.CreateDataFeed(ctx, datafeeds.Feed{AccountID: acct.ID, Pair: "ETH/USD"})

	upd, err := store.CreateDataFeedUpdate(ctx, datafeeds.Update{
		FeedID:  feed.ID,
		RoundID: 1,
		Price:   "100.50",
	})
	if err != nil {
		t.Fatalf("create data feed update: %v", err)
	}
	if upd.ID == "" {
		t.Fatal("expected update ID")
	}

	latest, err := store.GetLatestDataFeedUpdate(ctx, feed.ID)
	if err != nil {
		t.Fatalf("get latest data feed update: %v", err)
	}
	if latest.Price != "100.50" {
		t.Fatalf("expected price '100.50', got %q", latest.Price)
	}

	updates, err := store.ListDataFeedUpdates(ctx, feed.ID, 10)
	if err != nil {
		t.Fatalf("list data feed updates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
}

// DataStreamStore tests

func TestDataStreamStore_CRUD(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})

	stream, err := store.CreateStream(ctx, datastreams.Stream{
		AccountID: acct.ID,
		Name:      "test-stream",
	})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}

	retrieved, err := store.GetStream(ctx, stream.ID)
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	if retrieved.Name != "test-stream" {
		t.Fatalf("expected name 'test-stream', got %q", retrieved.Name)
	}

	stream.Name = "updated-stream"
	updated, err := store.UpdateStream(ctx, stream)
	if err != nil {
		t.Fatalf("update stream: %v", err)
	}
	if updated.Name != "updated-stream" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}

	streams, err := store.ListStreams(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list streams: %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(streams))
	}
}

func TestDataStreamStore_Frames(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})
	stream, _ := store.CreateStream(ctx, datastreams.Stream{AccountID: acct.ID, Name: "test"})

	for i := 0; i < 3; i++ {
		_, err := store.CreateFrame(ctx, datastreams.Frame{
			StreamID: stream.ID,
			Sequence: int64(i + 1),
			Payload:  map[string]any{"value": float64(i * 100)},
		})
		if err != nil {
			t.Fatalf("create frame %d: %v", i, err)
		}
	}

	latest, err := store.GetLatestFrame(ctx, stream.ID)
	if err != nil {
		t.Fatalf("get latest frame: %v", err)
	}
	if latest.StreamID != stream.ID {
		t.Fatalf("expected stream ID %q, got %q", stream.ID, latest.StreamID)
	}

	frames, err := store.ListFrames(ctx, stream.ID, 10)
	if err != nil {
		t.Fatalf("list frames: %v", err)
	}
	if len(frames) != 3 {
		t.Fatalf("expected 3 frames, got %d", len(frames))
	}
}

// DataLinkStore tests

func TestDataLinkStore_Channels(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})

	ch, err := store.CreateChannel(ctx, datalink.Channel{
		AccountID: acct.ID,
		Name:      "test-channel",
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	retrieved, err := store.GetChannel(ctx, ch.ID)
	if err != nil {
		t.Fatalf("get channel: %v", err)
	}
	if retrieved.Name != "test-channel" {
		t.Fatalf("expected name 'test-channel', got %q", retrieved.Name)
	}

	ch.Name = "updated-channel"
	updated, err := store.UpdateChannel(ctx, ch)
	if err != nil {
		t.Fatalf("update channel: %v", err)
	}
	if updated.Name != "updated-channel" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}

	channels, err := store.ListChannels(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list channels: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
}

func TestDataLinkStore_Deliveries(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})
	ch, _ := store.CreateChannel(ctx, datalink.Channel{AccountID: acct.ID, Name: "test"})

	delivery, err := store.CreateDelivery(ctx, datalink.Delivery{
		AccountID: acct.ID,
		ChannelID: ch.ID,
		Status:    datalink.DeliveryStatusPending,
		Payload:   map[string]any{"message": "test-data"},
	})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}

	retrieved, err := store.GetDelivery(ctx, delivery.ID)
	if err != nil {
		t.Fatalf("get delivery: %v", err)
	}
	if retrieved.Status != datalink.DeliveryStatusPending {
		t.Fatalf("expected status 'pending', got %q", retrieved.Status)
	}

	deliveries, err := store.ListDeliveries(ctx, acct.ID, 10)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}
}

// WorkspaceWalletStore tests

func TestWorkspaceWalletStore_CRUD(t *testing.T) {
	ctx := context.Background()
	store := New()

	acct, _ := store.CreateAccount(ctx, account.Account{Owner: "owner"})

	wallet, err := store.CreateWorkspaceWallet(ctx, account.WorkspaceWallet{
		WorkspaceID:   acct.ID,
		WalletAddress: "0x1234567890abcdef1234567890abcdef12345678",
		Label:         "test-wallet",
	})
	if err != nil {
		t.Fatalf("create workspace wallet: %v", err)
	}

	retrieved, err := store.GetWorkspaceWallet(ctx, wallet.ID)
	if err != nil {
		t.Fatalf("get workspace wallet: %v", err)
	}
	if retrieved.WalletAddress != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Fatalf("expected address '0x1234567890abcdef1234567890abcdef12345678', got %q", retrieved.WalletAddress)
	}

	byAddr, err := store.FindWorkspaceWalletByAddress(ctx, acct.ID, "0x1234567890abcdef1234567890abcdef12345678")
	if err != nil {
		t.Fatalf("find workspace wallet by address: %v", err)
	}
	if byAddr.ID != wallet.ID {
		t.Fatalf("expected ID %q, got %q", wallet.ID, byAddr.ID)
	}

	wallets, err := store.ListWorkspaceWallets(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list workspace wallets: %v", err)
	}
	if len(wallets) != 1 {
		t.Fatalf("expected 1 wallet, got %d", len(wallets))
	}
}

// Concurrency test

func TestStore_Concurrency(t *testing.T) {
	ctx := context.Background()
	store := New()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				acct, err := store.CreateAccount(ctx, account.Account{Owner: "concurrent"})
				if err != nil {
					t.Errorf("goroutine %d, iteration %d: create account: %v", n, j, err)
					return
				}
				_, err = store.GetAccount(ctx, acct.ID)
				if err != nil {
					t.Errorf("goroutine %d, iteration %d: get account: %v", n, j, err)
					return
				}
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	accounts, _ := store.ListAccounts(ctx)
	if len(accounts) != 1000 {
		t.Fatalf("expected 1000 accounts, got %d", len(accounts))
	}
}
