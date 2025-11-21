package oracle

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
)

type stubResolver struct {
	done    bool
	success bool
	result  string
	errMsg  string
	retry   time.Duration
}

func (s stubResolver) Resolve(ctx context.Context, req oracle.Request) (bool, bool, string, string, time.Duration, error) {
	return s.done, s.success, s.result, s.errMsg, s.retry, nil
}

func TestDispatcher_ResolveSuccess(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
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
	if updated.Status != oracle.StatusSucceeded {
		t.Fatalf("expected succeeded status, got %s", updated.Status)
	}
	if updated.Attempts != 1 {
		t.Fatalf("expected attempts incremented, got %d", updated.Attempts)
	}
}

func TestDispatcher_ResolveFailure(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "prices", "https://api.example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	dispatcher := NewDispatcher(svc, nil)
	dispatcher.WithResolver(stubResolver{done: true, success: false, errMsg: "failed"})
	dispatcher.tick(context.Background())

	updated, err := svc.GetRequest(context.Background(), req.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if updated.Status != oracle.StatusFailed {
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
	svc := New(store, store, nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "prices", "https://api.example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	dispatcher := NewDispatcher(svc, nil)
	dispatcher.WithRetryPolicy(0, 0, time.Nanosecond)
	dispatcher.WithResolver(stubResolver{done: true, success: true, result: `{}`})
	dispatcher.tick(context.Background())

	updated, _ := svc.GetRequest(context.Background(), req.ID)
	if updated.Status != oracle.StatusFailed {
		t.Fatalf("expected failed due to ttl, got %s", updated.Status)
	}
	if updated.Error == "" {
		t.Fatalf("expected ttl error message")
	}
}

func TestDispatcher_MaxAttempts(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "prices", "https://api.example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	dispatcher := NewDispatcher(svc, nil)
	dispatcher.WithResolver(stubResolver{done: false, success: false, errMsg: "retry", retry: 0})
	dispatcher.WithRetryPolicy(1, time.Millisecond, 0)

	dispatcher.tick(context.Background())
	time.Sleep(2 * time.Millisecond)
	dispatcher.tick(context.Background())

	updated, _ := svc.GetRequest(context.Background(), req.ID)
	if updated.Status != oracle.StatusFailed {
		t.Fatalf("expected failed after max attempts, got %s", updated.Status)
	}
	if updated.Attempts < 1 {
		t.Fatalf("expected attempts incremented")
	}
}
