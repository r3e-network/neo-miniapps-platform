package gasbank

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domain "github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
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

	updated, tx, err = svc.Withdraw(context.Background(), gasAcct.ID, 5, "to-wallet")
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

	secondAcct, secondTx, err := svc.Withdraw(context.Background(), gasAcct.ID, 2, "addr")
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
