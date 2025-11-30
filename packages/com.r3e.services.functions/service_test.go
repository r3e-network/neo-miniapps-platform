package functions

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestService(t *testing.T) {
	store := NewMemoryStore()
	acct, err := store.CreateAccount(context.Background(), "owner")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, store, nil)
	fn := Definition{AccountID: acct.ID, Name: "hello", Source: "() => 1"}
	created, err := svc.Create(context.Background(), fn)
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	created.Description = "desc"
	updated, err := svc.Update(context.Background(), created)
	if err != nil {
		t.Fatalf("update function: %v", err)
	}
	if updated.Description != "desc" {
		t.Fatalf("expected description update")
	}

	list, err := svc.List(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list functions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 function, got %d", len(list))
	}
}

func TestService_Execute(t *testing.T) {
	store := NewMemoryStore()
	acct, _ := store.CreateAccount(context.Background(), "owner")
	svc := New(store, store, nil)
	svc.AttachExecutor(&mockExecutor{})

	created, err := svc.Create(context.Background(), Definition{AccountID: acct.ID, Name: "echo", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	result, err := svc.Execute(context.Background(), created.ID, map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Status != ExecutionStatusSucceeded {
		t.Fatalf("expected succeeded status, got %s", result.Status)
	}
	if result.Output["foo"] != "bar" {
		t.Fatalf("unexpected output: %v", result.Output)
	}

	execs, err := svc.ListExecutions(context.Background(), created.ID, 0)
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
}

func TestService_ExecuteFailureRecordsHistory(t *testing.T) {
	store := NewMemoryStore()
	acct, _ := store.CreateAccount(context.Background(), "owner")
	svc := New(store, store, nil)
	svc.AttachExecutor(&failingExecutor{})

	created, err := svc.Create(context.Background(), Definition{AccountID: acct.ID, Name: "fail", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	result, err := svc.Execute(context.Background(), created.ID, nil)
	if err == nil {
		t.Fatalf("expected execution error")
	}
	if result.Status != ExecutionStatusFailed {
		t.Fatalf("expected failed status, got %s", result.Status)
	}

	execs, err := svc.ListExecutions(context.Background(), created.ID, 0)
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
	if execs[0].Status != ExecutionStatusFailed {
		t.Fatalf("expected persisted execution to be failed")
	}
}

func TestService_ExecuteReturnsExecutorErrorWhenPersistenceFails(t *testing.T) {
	mem := NewMemoryStore()
	acct, _ := mem.CreateAccount(context.Background(), "owner")
	persistErr := errors.New("persist execution failure")
	execErr := errors.New("executor failure")
	store := &failingFunctionStore{Store: mem, err: persistErr}

	svc := New(mem, store, nil)
	svc.AttachExecutor(&erroringExecutor{err: execErr})

	fn, err := svc.Create(context.Background(), Definition{AccountID: acct.ID, Name: "boom", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	_, err = svc.Execute(context.Background(), fn.ID, nil)
	if err == nil {
		t.Fatalf("expected execution error")
	}
	if !errors.Is(err, execErr) {
		t.Fatalf("expected executor error to be preserved, got %v", err)
	}
	if !errors.Is(err, persistErr) {
		t.Fatalf("expected persistence error to be included, got %v", err)
	}
}

func TestService_UpdateValidatesSecrets(t *testing.T) {
	store := NewMemoryStore()
	acct, err := store.CreateAccount(context.Background(), "owner")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	resolver := &stubSecretResolver{allowed: map[string]bool{"foo": true}}

	svc := New(store, store, nil)
	svc.AttachSecretResolver(resolver)

	created, err := svc.Create(context.Background(), Definition{
		AccountID: acct.ID,
		Name:      "secret-fn",
		Source:    "() => 1",
		Secrets:   []string{"foo"},
	})
	if err != nil {
		t.Fatalf("create with secrets: %v", err)
	}

	created.Secrets = []string{"bar"}
	_, err = svc.Update(context.Background(), created)
	if err == nil {
		t.Fatalf("expected error for invalid secret")
	}
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
	if m.Name != "functions" {
		t.Fatalf("expected name functions")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "functions" {
		t.Fatalf("expected name functions")
	}
}

func TestService_Get(t *testing.T) {
	store := NewMemoryStore()
	acct, _ := store.CreateAccount(context.Background(), "owner")
	svc := New(store, store, nil)

	fn, _ := svc.Create(context.Background(), Definition{AccountID: acct.ID, Name: "test", Source: "() => 1"})

	got, err := svc.Get(context.Background(), fn.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != fn.ID {
		t.Fatalf("expected ID %s, got %s", fn.ID, got.ID)
	}
}

func TestService_GetExecution(t *testing.T) {
	store := NewMemoryStore()
	acct, _ := store.CreateAccount(context.Background(), "owner")
	svc := New(store, store, nil)
	svc.AttachExecutor(&mockExecutor{})

	fn, _ := svc.Create(context.Background(), Definition{AccountID: acct.ID, Name: "test", Source: "() => 1"})
	result, _ := svc.Execute(context.Background(), fn.ID, nil)

	got, err := svc.GetExecution(context.Background(), result.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if got.ID != result.ID {
		t.Fatalf("expected ID %s, got %s", result.ID, got.ID)
	}
}

// Test helpers

type mockExecutor struct{}

func (m *mockExecutor) Execute(ctx context.Context, def Definition, payload map[string]any) (ExecutionResult, error) {
	return ExecutionResult{
		FunctionID:  def.ID,
		Output:      map[string]any{"foo": payload["foo"]},
		Status:      ExecutionStatusSucceeded,
		StartedAt:   time.Now().UTC(),
		CompletedAt: time.Now().UTC(),
	}, nil
}

type failingExecutor struct{}

func (f *failingExecutor) Execute(ctx context.Context, def Definition, payload map[string]any) (ExecutionResult, error) {
	return ExecutionResult{
		FunctionID: def.ID,
		Output:     map[string]any{},
		Status:     ExecutionStatusFailed,
		StartedAt:  time.Now().UTC(),
	}, errors.New("boom")
}

type failingFunctionStore struct {
	Store
	err error
}

func (s *failingFunctionStore) CreateExecution(ctx context.Context, exec Execution) (Execution, error) {
	return Execution{}, s.err
}

type erroringExecutor struct {
	err error
}

func (e *erroringExecutor) Execute(ctx context.Context, def Definition, payload map[string]any) (ExecutionResult, error) {
	return ExecutionResult{
		FunctionID:  def.ID,
		Output:      map[string]any{},
		StartedAt:   time.Now().UTC(),
		CompletedAt: time.Now().UTC(),
	}, e.err
}

type stubSecretResolver struct {
	allowed map[string]bool
	last    []string
}

func (r *stubSecretResolver) ResolveSecrets(ctx context.Context, accountID string, names []string) (map[string]string, error) {
	r.last = append([]string(nil), names...)
	for _, name := range names {
		if !r.allowed[name] {
			return nil, fmt.Errorf("secret %s not found", name)
		}
	}
	return map[string]string{}, nil
}
