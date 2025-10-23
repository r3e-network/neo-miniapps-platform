package gasbank

import "testing"

func TestTransactionConstants(t *testing.T) {
	if TransactionDeposit != "deposit" || TransactionWithdrawal != "withdrawal" {
		t.Fatalf("unexpected transaction constants")
	}
	if StatusPending != "pending" || StatusCompleted != "completed" || StatusFailed != "failed" {
		t.Fatalf("unexpected status constants")
	}
}

func TestAccountBalances(t *testing.T) {
	acct := Account{Balance: 10.5, Available: 8.5, Pending: 2.0}
	if acct.Balance != acct.Available+acct.Pending {
		t.Fatalf("expected balance to equal available + pending")
	}
}
