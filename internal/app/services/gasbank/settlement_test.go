package gasbank

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domain "github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
)

type stubResolver struct {
	mu    sync.Mutex
	calls map[string]int
}

func (s *stubResolver) Resolve(ctx context.Context, tx domain.Transaction) (bool, bool, string, time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls == nil {
		s.calls = make(map[string]int)
	}
	s.calls[tx.ID]++
	// settle immediately on first call as success
	return true, true, "", 0, nil
}

func TestSettlementPoller(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, store, nil)
	gasAcct, err := svc.EnsureAccount(context.Background(), acct.ID, "wallet")
	if err != nil {
		t.Fatalf("ensure account: %v", err)
	}
	if _, _, err := svc.Deposit(context.Background(), gasAcct.ID, 5, "tx", "from", "to"); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	acctState, err := store.GetGasAccount(context.Background(), gasAcct.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	t.Logf("account after deposit: %+v", acctState)
	if acctState.Available < 4.999 {
		t.Fatalf("expected balance after deposit, got %v", acctState.Available)
	}
	pending, tx, err := svc.Withdraw(context.Background(), gasAcct.ID, 1, "addr")
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if pending.Pending < 0.999 {
		t.Fatalf("expected pending funds")
	}

	resolver := &stubResolver{}
	poller := NewSettlementPoller(store, svc, resolver, nil)
	poller.interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := poller.Start(ctx); err != nil {
		t.Fatalf("start poller: %v", err)
	}
	defer poller.Stop(context.Background())

	time.Sleep(50 * time.Millisecond)

	updated, err := store.GetGasTransaction(context.Background(), tx.ID)
	if err != nil {
		t.Fatalf("get tx: %v", err)
	}
	if updated.Status != domain.StatusCompleted {
		t.Fatalf("expected completed status, got %s", updated.Status)
	}
}
