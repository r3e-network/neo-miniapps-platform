package gasbank

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

type stubResolver struct {
	mu    sync.Mutex
	calls map[string]int
}

func (s *stubResolver) Resolve(ctx context.Context, tx Transaction) (bool, bool, string, time.Duration, error) {
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
	store := newMockStore()
	acct, err := store.CreateAccount(context.Background(), "owner")
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
	if updated.Status != StatusCompleted {
		t.Fatalf("expected completed status, got %s", updated.Status)
	}
}

func TestTimeoutResolver_New(t *testing.T) {
	// Test default timeout
	r := NewTimeoutResolver(0)
	if r == nil {
		t.Fatalf("expected resolver")
	}
	// Test custom timeout
	r2 := NewTimeoutResolver(5 * time.Minute)
	if r2 == nil {
		t.Fatalf("expected resolver")
	}
}

func TestTimeoutResolver_Resolve(t *testing.T) {
	r := NewTimeoutResolver(50 * time.Millisecond)

	tx := Transaction{ID: "test-tx", Status: StatusPending}

	// First call stores the tx
	done, success, msg, retryAfter, err := r.Resolve(context.Background(), tx)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if done {
		t.Fatalf("expected not done on first call")
	}
	if success {
		t.Fatalf("expected not success on first call")
	}
	if retryAfter == 0 {
		t.Fatalf("expected retry after")
	}

	// Second call before timeout - should return not done
	done, _, _, _, err = r.Resolve(context.Background(), tx)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if done {
		t.Fatalf("expected not done before timeout")
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// After timeout, should return done and failed
	done, success, msg, _, err = r.Resolve(context.Background(), tx)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !done {
		t.Fatalf("expected done after timeout")
	}
	if success {
		t.Fatalf("expected not success after timeout")
	}
	if msg == "" {
		t.Fatalf("expected error message")
	}
}

func TestSettlementPoller_WithTracer(t *testing.T) {
	poller := NewSettlementPoller(nil, nil, nil, nil)
	// With nil tracer
	poller.WithTracer(nil)
	// With actual tracer
	poller.WithTracer(mockTracer{})
}

type mockTracer struct{}

func (mockTracer) StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, func(error)) {
	return ctx, func(error) {}
}

func TestSettlementPoller_WithObservationHooks(t *testing.T) {
	poller := NewSettlementPoller(nil, nil, nil, nil)
	poller.WithObservationHooks(core.ObservationHooks{})
}

func TestSettlementPoller_WithRetryPolicy(t *testing.T) {
	poller := NewSettlementPoller(nil, nil, nil, nil)
	poller.WithRetryPolicy(3, 10*time.Second)
	// Test with zero values - should not change
	poller.WithRetryPolicy(0, 0)
}

func TestSettlementPoller_Domain(t *testing.T) {
	poller := NewSettlementPoller(nil, nil, nil, nil)
	if poller.Domain() != "gasbank" {
		t.Fatalf("expected domain gasbank")
	}
}

func TestSettlementPoller_Name(t *testing.T) {
	poller := NewSettlementPoller(nil, nil, nil, nil)
	if poller.Name() != "gasbank-settlement" {
		t.Fatalf("expected name gasbank-settlement")
	}
}

func TestSettlementPoller_Ready_NoResolver(t *testing.T) {
	poller := NewSettlementPoller(nil, nil, nil, nil)
	err := poller.Ready(context.Background())
	if err == nil {
		t.Fatalf("expected error for no resolver")
	}
}

func TestSettlementPoller_Ready_NotRunning(t *testing.T) {
	resolver := &stubResolver{}
	poller := NewSettlementPoller(nil, nil, resolver, nil)
	err := poller.Ready(context.Background())
	if err == nil {
		t.Fatalf("expected error when not running")
	}
}

func TestSettlementPoller_StartStop_NoResolver(t *testing.T) {
	poller := NewSettlementPoller(nil, nil, nil, nil)
	// Should not error, just warn and return
	if err := poller.Start(context.Background()); err != nil {
		t.Fatalf("start without resolver: %v", err)
	}
	// Stop when not running
	if err := poller.Stop(context.Background()); err != nil {
		t.Fatalf("stop when not running: %v", err)
	}
}

func TestSettlementPoller_DoubleStart(t *testing.T) {
	store := newMockStore()
	resolver := &stubResolver{}
	poller := NewSettlementPoller(store, nil, resolver, nil)
	poller.interval = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := poller.Start(ctx); err != nil {
		t.Fatalf("first start: %v", err)
	}
	defer poller.Stop(context.Background())

	// Second start should be no-op
	if err := poller.Start(ctx); err != nil {
		t.Fatalf("second start: %v", err)
	}
}

func TestSettlementPollerHonoursNextAttemptFromStore(t *testing.T) {
	store := newMockStore()
	acct, _ := store.CreateAccount(context.Background(), "owner")
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

// errorResolver always returns an error
type errorResolver struct {
	mu    sync.Mutex
	calls int
}

func (e *errorResolver) Resolve(ctx context.Context, tx Transaction) (bool, bool, string, time.Duration, error) {
	e.mu.Lock()
	e.calls++
	e.mu.Unlock()
	return false, false, "", 10 * time.Millisecond, fmt.Errorf("simulated error")
}

// retryResolver returns not done until max attempts
type retryResolver struct {
	mu       sync.Mutex
	calls    map[string]int
	maxCalls int
}

func (r *retryResolver) Resolve(ctx context.Context, tx Transaction) (bool, bool, string, time.Duration, error) {
	r.mu.Lock()
	if r.calls == nil {
		r.calls = make(map[string]int)
	}
	r.calls[tx.ID]++
	count := r.calls[tx.ID]
	r.mu.Unlock()

	if count >= r.maxCalls {
		return true, true, "", 0, nil
	}
	return false, false, "retry", 10 * time.Millisecond, nil
}

func TestSettlementPoller_WithErrorResolver(t *testing.T) {
	store := newMockStore()
	acct, _ := store.CreateAccount(context.Background(), "owner")
	svc := New(store, store, nil)
	gasAcct, _ := svc.EnsureAccount(context.Background(), acct.ID, "wallet-err")
	svc.Deposit(context.Background(), gasAcct.ID, 5, "tx", "from", "to")
	_, _, _ = svc.Withdraw(context.Background(), acct.ID, gasAcct.ID, 1, "addr")

	resolver := &errorResolver{}
	poller := NewSettlementPoller(store, svc, resolver, nil)
	poller.WithRetryPolicy(2, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := poller.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer poller.Stop(context.Background())

	// Wait for resolver to be called multiple times
	time.Sleep(100 * time.Millisecond)

	resolver.mu.Lock()
	calls := resolver.calls
	resolver.mu.Unlock()
	if calls < 2 {
		t.Fatalf("expected multiple resolver calls, got %d", calls)
	}
}

func TestSettlementPoller_WithRetryResolver(t *testing.T) {
	store := newMockStore()
	acct, _ := store.CreateAccount(context.Background(), "owner")
	svc := New(store, store, nil)
	gasAcct, _ := svc.EnsureAccount(context.Background(), acct.ID, "wallet-retry")
	svc.Deposit(context.Background(), gasAcct.ID, 5, "tx", "from", "to")
	_, tx, _ := svc.Withdraw(context.Background(), acct.ID, gasAcct.ID, 1, "addr")

	// Use stubResolver which settles immediately on first call
	resolver := &stubResolver{}
	poller := NewSettlementPoller(store, svc, resolver, nil)
	poller.interval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := poller.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer poller.Stop(context.Background())

	// Wait for resolver to be called and settle
	time.Sleep(100 * time.Millisecond)

	updated, _ := store.GetGasTransaction(context.Background(), tx.ID)
	if updated.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", updated.Status)
	}
}

func TestSettlementPoller_Ready_Running(t *testing.T) {
	store := newMockStore()
	resolver := &stubResolver{}
	poller := NewSettlementPoller(store, nil, resolver, nil)
	poller.interval = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := poller.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer poller.Stop(context.Background())

	if err := poller.Ready(ctx); err != nil {
		t.Fatalf("expected ready: %v", err)
	}
}
