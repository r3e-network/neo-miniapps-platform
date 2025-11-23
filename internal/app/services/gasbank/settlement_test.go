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
	pending, tx, err := svc.Withdraw(context.Background(), acct.ID, gasAcct.ID, 1, "addr")
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
	defer func() {
		if err := poller.Stop(context.Background()); err != nil {
			t.Fatalf("stop poller: %v", err)
		}
	}()

	time.Sleep(50 * time.Millisecond)

	updated, err := store.GetGasTransaction(context.Background(), tx.ID)
	if err != nil {
		t.Fatalf("get tx: %v", err)
	}
	if updated.Status != domain.StatusCompleted {
		t.Fatalf("expected completed status, got %s", updated.Status)
	}
}

func TestSettlementPollerHonoursNextAttemptFromStore(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	gasAcct, _ := svc.EnsureAccount(context.Background(), acct.ID, "wallet")
	_, _, _ = svc.Deposit(context.Background(), gasAcct.ID, 5, "tx", "from", "to")

	_, tx, err := svc.Withdraw(context.Background(), acct.ID, gasAcct.ID, 1, "addr")
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	// Simulate a persisted backoff before the next settlement attempt.
	tx.NextAttemptAt = time.Now().Add(200 * time.Millisecond)
	tx.UpdatedAt = time.Now().UTC()
	if _, err := store.UpdateGasTransaction(context.Background(), tx); err != nil {
		t.Fatalf("seed next attempt: %v", err)
	}

	resolver := &stubResolver{}
	poller := NewSettlementPoller(store, svc, resolver, nil)
	poller.interval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := poller.Start(ctx); err != nil {
		t.Fatalf("start poller: %v", err)
	}
	defer func() {
		if err := poller.Stop(context.Background()); err != nil {
			t.Fatalf("stop poller: %v", err)
		}
	}()

	// Immediately after start, resolver should not be called because next attempt is in the future.
	time.Sleep(50 * time.Millisecond)
	resolver.mu.Lock()
	callsEarly := resolver.calls[tx.ID]
	resolver.mu.Unlock()
	if callsEarly != 0 {
		t.Fatalf("expected no resolver calls before next attempt window, got %d", callsEarly)
	}

	// After the window passes, the resolver should be invoked and settle the withdrawal.
	time.Sleep(250 * time.Millisecond)
	resolver.mu.Lock()
	callsLater := resolver.calls[tx.ID]
	resolver.mu.Unlock()
	if callsLater == 0 {
		t.Fatalf("expected resolver to be invoked after delay")
	}
}
