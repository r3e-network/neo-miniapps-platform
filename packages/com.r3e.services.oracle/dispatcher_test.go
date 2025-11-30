package oracle

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/domain/account"
	"github.com/R3E-Network/service_layer/pkg/storage/memory"
)

type stubResolver struct {
	done    bool
	success bool
	result  string
	errMsg  string
	retry   time.Duration
}

func (s stubResolver) Resolve(ctx context.Context, req Request) (bool, bool, string, string, time.Duration, error) {
	return s.done, s.success, s.result, s.errMsg, s.retry, nil
}

func TestDispatcher_ResolveSuccess(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, NewStoreAdapter(store), nil)
	src, err := svc.CreateSource(context.Background(), acct.ID, "prices", "https://api.example.com", "GET", "", nil, "")
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	req, err := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	dispatcher := NewDispatcher(svc, nil)
	dispatcher.WithResolver(stubResolver{done: true, success: true, result: `{"price":10}`})
	dispatcher.tick(context.Background())

	updated, err := svc.GetRequest(context.Background(), req.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if updated.Status != StatusSucceeded {
		t.Fatalf("expected succeeded status, got %s", updated.Status)
	}
	if updated.Attempts != 1 {
		t.Fatalf("expected attempts incremented, got %d", updated.Attempts)
	}
}

func TestDispatcher_ResolveFailure(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "prices", "https://api.example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	dispatcher := NewDispatcher(svc, nil)
	dispatcher.WithResolver(stubResolver{done: true, success: false, errMsg: "failed"})
	dispatcher.tick(context.Background())

	updated, err := svc.GetRequest(context.Background(), req.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if updated.Status != StatusFailed {
		t.Fatalf("expected failed status, got %s", updated.Status)
	}
	if updated.Error == "" {
		t.Fatalf("expected error message")
	}
	if updated.Attempts != 1 {
		t.Fatalf("expected attempt recorded, got %d", updated.Attempts)
	}
}

func TestDispatcher_ExpiresTTL(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "prices", "https://api.example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	dispatcher := NewDispatcher(svc, nil)
	dispatcher.WithRetryPolicy(0, 0, time.Nanosecond)
	dispatcher.WithResolver(stubResolver{done: true, success: true, result: `{}`})
	dispatcher.tick(context.Background())

	updated, _ := svc.GetRequest(context.Background(), req.ID)
	if updated.Status != StatusFailed {
		t.Fatalf("expected failed due to ttl, got %s", updated.Status)
	}
	if updated.Error == "" {
		t.Fatalf("expected ttl error message")
	}
}

func TestDispatcher_MaxAttempts(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "prices", "https://api.example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	dispatcher := NewDispatcher(svc, nil)
	dispatcher.WithResolver(stubResolver{done: false, success: false, errMsg: "retry", retry: 0})
	dispatcher.WithRetryPolicy(1, time.Millisecond, 0)

	dispatcher.tick(context.Background())
	time.Sleep(2 * time.Millisecond)
	dispatcher.tick(context.Background())

	updated, _ := svc.GetRequest(context.Background(), req.ID)
	if updated.Status != StatusFailed {
		t.Fatalf("expected failed after max attempts, got %s", updated.Status)
	}
	if updated.Attempts < 1 {
		t.Fatalf("expected attempts incremented")
	}
}

func TestDispatcher_Lifecycle(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	svc.CreateSource(context.Background(), acct.ID, "src", "https://example.com", "GET", "", nil, "")

	dispatcher := NewDispatcher(svc, nil)
	dispatcher.WithResolver(stubResolver{done: true, success: true, result: "{}"})
	dispatcher.interval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := dispatcher.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Wait for a tick
	time.Sleep(100 * time.Millisecond)

	if err := dispatcher.Ready(ctx); err != nil {
		t.Fatalf("ready: %v", err)
	}

	if err := dispatcher.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if err := dispatcher.Ready(context.Background()); err == nil {
		t.Fatalf("expected not ready after stop")
	}
}

func TestDispatcher_StartWithoutResolver(t *testing.T) {
	dispatcher := NewDispatcher(nil, nil)

	// Start without resolver should not error, just log warning
	if err := dispatcher.Start(context.Background()); err != nil {
		t.Fatalf("start without resolver: %v", err)
	}

	// Ready should fail when not running
	if err := dispatcher.Ready(context.Background()); err == nil {
		t.Fatalf("expected not ready without resolver")
	}

	// Stop should be no-op
	if err := dispatcher.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestDispatcher_DoubleStart(t *testing.T) {
	store := memory.New()
	svc := New(store, NewStoreAdapter(store), nil)

	dispatcher := NewDispatcher(svc, nil)
	dispatcher.WithResolver(stubResolver{done: true, success: true, result: "{}"})
	dispatcher.interval = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := dispatcher.Start(ctx); err != nil {
		t.Fatalf("first start: %v", err)
	}
	defer dispatcher.Stop(context.Background())

	// Second start should be no-op
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatalf("second start: %v", err)
	}
}

func TestDispatcher_Descriptor(t *testing.T) {
	dispatcher := NewDispatcher(nil, nil)
	d := dispatcher.Descriptor()
	if d.Name != "oracle-dispatcher" {
		t.Fatalf("expected name oracle-dispatcher")
	}
	if d.Domain != "oracle" {
		t.Fatalf("expected domain oracle")
	}
}

func TestDispatcher_WithTracer(t *testing.T) {
	dispatcher := NewDispatcher(nil, nil)
	// With nil tracer
	dispatcher.WithTracer(nil)
	// With actual tracer
	dispatcher.WithTracer(mockTracer{})
}

type mockTracer struct{}

func (mockTracer) StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, func(error)) {
	return ctx, func(error) {}
}

func TestDispatcher_EnableDeadLetter(t *testing.T) {
	dispatcher := NewDispatcher(nil, nil)
	dispatcher.EnableDeadLetter(true)
	dispatcher.EnableDeadLetter(false)
}

func TestDispatcher_TickWithNilService(t *testing.T) {
	dispatcher := NewDispatcher(nil, nil)
	dispatcher.WithResolver(stubResolver{done: true, success: true})
	// Should not panic with nil service
	dispatcher.tick(context.Background())
}

func TestDispatcher_RetryWithDelay(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "retry-src", "https://example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	// Resolver returns not done with retry duration
	dispatcher := NewDispatcher(svc, nil)
	dispatcher.WithResolver(stubResolver{done: false, retry: 100 * time.Millisecond})

	// First tick should start processing
	dispatcher.tick(context.Background())

	// Request should now be running
	updated, _ := svc.GetRequest(context.Background(), req.ID)
	if updated.Status != StatusRunning {
		t.Fatalf("expected running status after first tick, got %s", updated.Status)
	}

	// Second tick should respect retry delay
	dispatcher.tick(context.Background())
	// Third tick after delay
	time.Sleep(150 * time.Millisecond)
	dispatcher.tick(context.Background())
}

func TestDispatcher_DeadLetterDisabled(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "dl-src", "https://example.com", "GET", "", nil, "")
	svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	dispatcher := NewDispatcher(svc, nil)
	dispatcher.WithResolver(stubResolver{done: false, retry: time.Millisecond})
	dispatcher.WithRetryPolicy(1, time.Millisecond, 0)
	dispatcher.EnableDeadLetter(false)

	// First tick starts the request
	dispatcher.tick(context.Background())
	time.Sleep(5 * time.Millisecond)
	// Second tick should exceed max attempts
	dispatcher.tick(context.Background())
}

func TestDispatcher_RequestResolverFunc(t *testing.T) {
	// Test RequestResolverFunc adapter
	fn := RequestResolverFunc(func(ctx context.Context, req Request) (bool, bool, string, string, time.Duration, error) {
		return true, true, "result", "", 0, nil
	})

	done, success, result, errMsg, retry, err := fn.Resolve(context.Background(), Request{})
	if !done || !success || result != "result" || errMsg != "" || retry != 0 || err != nil {
		t.Fatalf("unexpected values from resolver func")
	}
}
