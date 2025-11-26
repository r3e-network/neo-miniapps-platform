package gasbank

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domain "github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

func TestService_DepositWithdraw(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, store, nil)
	gasAcct, err := svc.EnsureAccount(context.Background(), acct.ID, " wallet1 ")
	if err != nil {
		t.Fatalf("ensure gas account: %v", err)
	}
	if gasAcct.WalletAddress != "wallet1" {
		t.Fatalf("wallet not normalised: %s", gasAcct.WalletAddress)
	}

	updated, tx, err := svc.Deposit(context.Background(), gasAcct.ID, 10, "tx1", "from", "to")
	if err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if updated.Available < 9.999 {
		t.Fatalf("unexpected balance: %v", updated.Available)
	}
	if updated.Pending != 0 {
		t.Fatalf("pending should be zero after deposit: %v", updated.Pending)
	}
	if tx.Type != "deposit" {
		t.Fatalf("unexpected tx type: %s", tx.Type)
	}

	updated, tx, err = svc.Withdraw(context.Background(), acct.ID, gasAcct.ID, 5, "to-wallet")
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if updated.Available < 4.999 {
		t.Fatalf("balance not reduced: %v", updated.Available)
	}
	if updated.Pending < 4.999 || updated.Pending > 5.001 {
		t.Fatalf("pending not tracked: %v", updated.Pending)
	}
	if updated.Balance < 9.999 {
		t.Fatalf("total balance should remain until settlement: %v", updated.Balance)
	}
	if tx.Type != "withdrawal" {
		t.Fatalf("unexpected tx type: %s", tx.Type)
	}

	settled, settledTx, err := svc.CompleteWithdrawal(context.Background(), tx.ID, true, "")
	if err != nil {
		t.Fatalf("complete withdrawal: %v", err)
	}
	if settled.Pending > Epsilon {
		t.Fatalf("pending not cleared: %v", settled.Pending)
	}
	if math.Abs(settled.Balance-5.0) > 1e-3 {
		t.Fatalf("balance not reduced: %v", settled.Balance)
	}
	if settledTx.Status != domain.StatusCompleted {
		t.Fatalf("unexpected status: %s", settledTx.Status)
	}

	secondAcct, secondTx, err := svc.Withdraw(context.Background(), acct.ID, gasAcct.ID, 2, "addr")
	if err != nil {
		t.Fatalf("second withdraw: %v", err)
	}
	if secondAcct.Pending < 1.999 {
		t.Fatalf("second pending incorrect: %v", secondAcct.Pending)
	}

	failureAcct, failureTx, err := svc.CompleteWithdrawal(context.Background(), secondTx.ID, false, "insufficient balance")
	if err != nil {
		t.Fatalf("complete withdrawal failure: %v", err)
	}
	if math.Abs(failureAcct.Available-5.0) > 1e-3 {
		t.Fatalf("available not restored: %v", failureAcct.Available)
	}
	if failureTx.Status != domain.StatusFailed {
		t.Fatalf("unexpected failure status: %s", failureTx.Status)
	}
}

func TestService_PreventDuplicateWallets(t *testing.T) {
	store := memory.New()
	acct1, err := store.CreateAccount(context.Background(), account.Account{Owner: "a"})
	if err != nil {
		t.Fatalf("create account 1: %v", err)
	}
	acct2, err := store.CreateAccount(context.Background(), account.Account{Owner: "b"})
	if err != nil {
		t.Fatalf("create account 2: %v", err)
	}

	svc := New(store, store, nil)
	if _, err := svc.EnsureAccount(context.Background(), acct1.ID, "WalletX"); err != nil {
		t.Fatalf("ensure wallet for account1: %v", err)
	}
	if _, err := svc.EnsureAccount(context.Background(), acct2.ID, "walletx"); err == nil || !errors.Is(err, ErrWalletInUse) {
		t.Fatalf("expected duplicate wallet error, got %v", err)
	}
}

func TestService_WithdrawRejectsForeignAccount(t *testing.T) {
	store := memory.New()
	owner, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	other, err := store.CreateAccount(context.Background(), account.Account{Owner: "other"})
	if err != nil {
		t.Fatalf("create other: %v", err)
	}

	svc := New(store, store, nil)
	gasAcct, err := svc.EnsureAccount(context.Background(), owner.ID, "wallet-123")
	if err != nil {
		t.Fatalf("ensure account: %v", err)
	}
	if _, _, err := svc.Deposit(context.Background(), gasAcct.ID, 5, "tx", "", ""); err != nil {
		t.Fatalf("seed deposit: %v", err)
	}

	if _, _, err := svc.Withdraw(context.Background(), other.ID, gasAcct.ID, 1, "dest"); err == nil {
		t.Fatalf("expected withdraw to reject foreign account")
	}
}

func TestService_DepositRollsBackOnTransactionFailure(t *testing.T) {
	store := &failingGasBankStore{Store: memory.New(), failCreateTx: true}
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	gasAcct, err := store.CreateGasAccount(context.Background(), domain.Account{AccountID: acct.ID})
	if err != nil {
		t.Fatalf("create gas account: %v", err)
	}

	svc := New(store, store, nil)
	if _, _, err := svc.Deposit(context.Background(), gasAcct.ID, 15, "tx", "", ""); err == nil {
		t.Fatalf("expected deposit to fail")
	}

	stored, err := store.GetGasAccount(context.Background(), gasAcct.ID)
	if err != nil {
		t.Fatalf("get gas account: %v", err)
	}
	if stored.Balance != 0 || stored.Available != 0 || stored.Pending != 0 {
		t.Fatalf("expected balances to rollback, got %+v", stored)
	}
}

func TestService_Summary(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, store, logger.NewDefault("test"))
	gasAcct, err := svc.EnsureAccount(context.Background(), acct.ID, "wallet-summary")
	if err != nil {
		t.Fatalf("ensure gas account: %v", err)
	}
	if _, _, err := svc.Deposit(context.Background(), gasAcct.ID, 25, "tx-summary", "from", "to"); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if _, _, err := svc.Withdraw(context.Background(), acct.ID, gasAcct.ID, 5, "dest"); err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	summary, err := svc.Summary(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if len(summary.Accounts) != 1 {
		t.Fatalf("expected 1 account in summary, got %d", len(summary.Accounts))
	}
	if summary.TotalBalance < 24.999 || summary.TotalBalance > 25.001 {
		t.Fatalf("unexpected total balance: %v", summary.TotalBalance)
	}
	if summary.PendingWithdrawals != 1 {
		t.Fatalf("expected 1 pending withdrawal, got %d", summary.PendingWithdrawals)
	}
	if summary.PendingAmount < 4.999 || summary.PendingAmount > 5.001 {
		t.Fatalf("unexpected pending amount: %v", summary.PendingAmount)
	}
	if summary.LastDeposit == nil || summary.LastDeposit.ID == "" {
		t.Fatalf("expected last deposit info")
	}
	if summary.LastWithdrawal == nil || summary.LastWithdrawal.ID == "" {
		t.Fatalf("expected last withdrawal info")
	}
}

func TestService_SubmitApproval(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, store, nil)
	gasAcct, err := svc.EnsureAccount(context.Background(), acct.ID, "wallet-approval")
	if err != nil {
		t.Fatalf("ensure gas account: %v", err)
	}
	gasAcct.RequiredApprovals = 2
	if _, err := store.UpdateGasAccount(context.Background(), gasAcct); err != nil {
		t.Fatalf("update gas account: %v", err)
	}
	if _, _, err := svc.Deposit(context.Background(), gasAcct.ID, 10, "tx-deposit", "", ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}

	updated, tx, err := svc.Withdraw(context.Background(), acct.ID, gasAcct.ID, 4, "dest")
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if tx.Status != domain.StatusAwaitingApproval {
		t.Fatalf("expected awaiting approval status, got %s", tx.Status)
	}
	if math.Abs(updated.Available-6) > 1e-6 {
		t.Fatalf("unexpected available balance: %v", updated.Available)
	}

	if _, _, err := svc.SubmitApproval(context.Background(), tx.ID, "approver-1", "", "", true); err != nil {
		t.Fatalf("submit first approval: %v", err)
	}
	txAfterFirst, err := store.GetGasTransaction(context.Background(), tx.ID)
	if err != nil {
		t.Fatalf("get transaction: %v", err)
	}
	if txAfterFirst.Status != domain.StatusAwaitingApproval {
		t.Fatalf("expected awaiting approval after first approval, got %s", txAfterFirst.Status)
	}

	if _, _, err := svc.SubmitApproval(context.Background(), tx.ID, "approver-2", "", "", true); err != nil {
		t.Fatalf("submit second approval: %v", err)
	}
	txAfterSecond, err := store.GetGasTransaction(context.Background(), tx.ID)
	if err != nil {
		t.Fatalf("get transaction after second approval: %v", err)
	}
	if txAfterSecond.Status != domain.StatusPending {
		t.Fatalf("expected pending status after threshold met, got %s", txAfterSecond.Status)
	}
	approvals, err := svc.ListApprovals(context.Background(), tx.ID)
	if err != nil {
		t.Fatalf("list approvals: %v", err)
	}
	if len(approvals) != 2 {
		t.Fatalf("expected 2 approvals, got %d", len(approvals))
	}

	cancelAcct, newTx, err := svc.Withdraw(context.Background(), acct.ID, gasAcct.ID, 2, "dest")
	if err != nil {
		t.Fatalf("withdraw for rejection: %v", err)
	}
	if _, _, err := svc.SubmitApproval(context.Background(), newTx.ID, "approver-3", "", "", false); err != nil {
		t.Fatalf("reject approval: %v", err)
	}
	finalAcct, err := store.GetGasAccount(context.Background(), cancelAcct.ID)
	if err != nil {
		t.Fatalf("get gas account: %v", err)
	}
	if math.Abs(finalAcct.Available-6) > 1e-6 {
		t.Fatalf("expected available balance restored after rejection, got %v", finalAcct.Available)
	}
	rejectedTx, err := store.GetGasTransaction(context.Background(), newTx.ID)
	if err != nil {
		t.Fatalf("get rejected transaction: %v", err)
	}
	if rejectedTx.Status != domain.StatusCancelled {
		t.Fatalf("expected cancelled status, got %s", rejectedTx.Status)
	}
}

func TestService_EnsureAccountWithOptions(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "opt-owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	minBalance := 10.0
	dailyLimit := 25.0
	notify := 5.0
	required := 3
	ensured, err := svc.EnsureAccountWithOptions(context.Background(), acct.ID, EnsureAccountOptions{
		WalletAddress:         "WALLET",
		MinBalance:            &minBalance,
		DailyLimit:            &dailyLimit,
		NotificationThreshold: &notify,
		RequiredApprovals:     &required,
	})
	if err != nil {
		t.Fatalf("ensure with options: %v", err)
	}
	if ensured.MinBalance != minBalance {
		t.Fatalf("expected min balance %.1f, got %.1f", minBalance, ensured.MinBalance)
	}
	if ensured.DailyLimit != dailyLimit {
		t.Fatalf("expected daily limit %.1f, got %.1f", dailyLimit, ensured.DailyLimit)
	}
	if ensured.NotificationThreshold != notify {
		t.Fatalf("expected notification threshold %.1f, got %.1f", notify, ensured.NotificationThreshold)
	}
	if ensured.RequiredApprovals != required {
		t.Fatalf("expected required approvals %d, got %d", required, ensured.RequiredApprovals)
	}

	zero := 0.0
	reqZero := 0
	updated, err := svc.EnsureAccountWithOptions(context.Background(), acct.ID, EnsureAccountOptions{
		MinBalance:        &zero,
		RequiredApprovals: &reqZero,
	})
	if err != nil {
		t.Fatalf("update ensure options: %v", err)
	}
	if updated.MinBalance != 0 {
		t.Fatalf("expected min balance reset to 0, got %.2f", updated.MinBalance)
	}
	if updated.RequiredApprovals != 0 {
		t.Fatalf("expected required approvals 0, got %d", updated.RequiredApprovals)
	}
}

func TestService_WithdrawWithScheduleAndLimits(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "limits"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	gasAcct, err := svc.EnsureAccount(context.Background(), acct.ID, "wallet-limits")
	if err != nil {
		t.Fatalf("ensure account: %v", err)
	}
	gasAcct.MinBalance = 3
	gasAcct.DailyLimit = 5
	if _, err := store.UpdateGasAccount(context.Background(), gasAcct); err != nil {
		t.Fatalf("update account: %v", err)
	}
	if _, _, err := svc.Deposit(context.Background(), gasAcct.ID, 10, "tx-limit", "", ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}

	// Min balance violation
	if _, _, err := svc.Withdraw(context.Background(), acct.ID, gasAcct.ID, 8, "dest"); !errors.Is(err, errMinBalance) {
		t.Fatalf("expected min balance error, got %v", err)
	}

	// Valid withdraw within daily limit
	if _, _, err := svc.Withdraw(context.Background(), acct.ID, gasAcct.ID, 2, "dest"); err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	// Exceed daily limit
	if _, _, err := svc.Withdraw(context.Background(), acct.ID, gasAcct.ID, 4, "dest"); !errors.Is(err, errDailyLimit) {
		t.Fatalf("expected daily limit error, got %v", err)
	}

	// Schedule future withdrawal
	future := time.Now().Add(time.Hour)
	opts := WithdrawOptions{
		Amount:     1,
		ToAddress:  "dest",
		ScheduleAt: &future,
	}
	_, scheduledTx, err := svc.WithdrawWithOptions(context.Background(), acct.ID, gasAcct.ID, opts)
	if err != nil {
		t.Fatalf("scheduled withdraw: %v", err)
	}
	if scheduledTx.Status != domain.StatusScheduled {
		t.Fatalf("expected scheduled status, got %s", scheduledTx.Status)
	}

	// Force schedule due and activate
	due := time.Now().Add(-time.Minute)
	if _, err := store.SaveWithdrawalSchedule(context.Background(), domain.WithdrawalSchedule{
		TransactionID: scheduledTx.ID,
		ScheduleAt:    due,
		NextRunAt:     due,
		CreatedAt:     due,
		UpdatedAt:     due,
	}); err != nil {
		t.Fatalf("save due schedule: %v", err)
	}

	if err := svc.ActivateDueSchedules(context.Background(), 10); err != nil {
		t.Fatalf("activate schedules: %v", err)
	}
	tx, err := store.GetGasTransaction(context.Background(), scheduledTx.ID)
	if err != nil {
		t.Fatalf("get activated transaction: %v", err)
	}
	if tx.Status != domain.StatusPending {
		t.Fatalf("expected pending status after activation, got %s", tx.Status)
	}
}

func TestService_WithdrawRejectsCronExpressions(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	gasAcct, _ := store.CreateGasAccount(context.Background(), domain.Account{AccountID: acct.ID})

	svc := New(store, store, nil)
	_, _, err := svc.Deposit(context.Background(), gasAcct.ID, 10, "tx", "", "")
	if err != nil {
		t.Fatalf("deposit: %v", err)
	}

	_, _, err = svc.WithdrawWithOptions(context.Background(), acct.ID, gasAcct.ID, WithdrawOptions{
		Amount:         1,
		ToAddress:      "dest",
		CronExpression: "0 * * * *",
	})
	if err == nil || !errors.Is(err, errCronUnsupported) {
		t.Fatalf("expected cron unsupported error, got %v", err)
	}
}

func TestService_WithdrawRollsBackOnTransactionFailure(t *testing.T) {
	store := &failingGasBankStore{Store: memory.New()}
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	gasAcct, err := store.CreateGasAccount(context.Background(), domain.Account{AccountID: acct.ID})
	if err != nil {
		t.Fatalf("create gas account: %v", err)
	}

	svc := New(store, store, nil)
	if _, _, err := svc.Deposit(context.Background(), gasAcct.ID, 20, "tx", "", ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}

	store.failCreateTx = true
	if _, _, err := svc.Withdraw(context.Background(), acct.ID, gasAcct.ID, 7, "dest"); err == nil {
		t.Fatalf("expected withdraw to fail")
	}

	stored, err := store.GetGasAccount(context.Background(), gasAcct.ID)
	if err != nil {
		t.Fatalf("get gas account: %v", err)
	}
	if math.Abs(stored.Available-20) > 1e-9 {
		t.Fatalf("available balance should rollback to 20, got %v", stored.Available)
	}
	if stored.Pending != 0 {
		t.Fatalf("pending should be zero after rollback, got %v", stored.Pending)
	}
}

type failingGasBankStore struct {
	*memory.Store
	failCreateTx bool
}

func (s *failingGasBankStore) CreateGasTransaction(ctx context.Context, tx domain.Transaction) (domain.Transaction, error) {
	if s.failCreateTx {
		return domain.Transaction{}, fmt.Errorf("stub create gas transaction failure")
	}
	created, err := s.Store.CreateGasTransaction(ctx, tx)
	if err == nil {
		// Reset failure flag so subsequent operations can toggle explicitly.
		s.failCreateTx = false
	}
	return created, err
}

func ExampleService_Deposit() {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	gasAcct, _ := store.CreateGasAccount(context.Background(), domain.Account{AccountID: acct.ID})

	log := logger.NewDefault("example-gasbank")
	log.SetOutput(io.Discard)
	svc := New(store, store, log)
	accountWithFunds, tx, _ := svc.Deposit(context.Background(), gasAcct.ID, 10, "tx123", "walletA", "walletB")
	fmt.Printf("balance:%.0f status:%s\n", accountWithFunds.Available, tx.Status)
	// Output:
	// balance:10 status:completed
}

func TestService_Lifecycle(t *testing.T) {
	svc := New(nil, nil, nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := svc.Ready(context.Background()); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if svc.Ready(context.Background()) == nil {
		t.Fatalf("expected not ready after stop")
	}
}

func TestService_Manifest(t *testing.T) {
	svc := New(nil, nil, nil)
	m := svc.Manifest()
	if m.Name != "gasbank" {
		t.Fatalf("expected name gasbank")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "gasbank" {
		t.Fatalf("expected name gasbank")
	}
}

func TestService_GetAccount(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	gasAcct, _ := svc.EnsureAccount(context.Background(), acct.ID, "wallet-get")

	got, err := svc.GetAccount(context.Background(), gasAcct.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if got.ID != gasAcct.ID {
		t.Fatalf("account mismatch")
	}
}

func TestService_ListAccounts(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	svc.EnsureAccount(context.Background(), acct.ID, "wallet1")

	accounts, err := svc.ListAccounts(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
}

func TestService_ListTransactions(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	gasAcct, _ := svc.EnsureAccount(context.Background(), acct.ID, "wallet-tx")
	svc.Deposit(context.Background(), gasAcct.ID, 10, "tx1", "from", "to")

	txs, err := svc.ListTransactions(context.Background(), gasAcct.ID, 10)
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txs))
	}
}
