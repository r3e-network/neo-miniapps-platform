package functions

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
)

func TestService(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, store, nil)
	fn := function.Definition{AccountID: acct.ID, Name: "hello", Source: "() => 1"}
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
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	svc.AttachExecutor(&mockExecutor{})

	created, err := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "echo", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	result, err := svc.Execute(context.Background(), created.ID, map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Output["foo"] != "bar" {
		t.Fatalf("unexpected output: %v", result.Output)
	}
}

type mockExecutor struct{}

func (m *mockExecutor) Execute(ctx context.Context, def function.Definition, payload map[string]any) (function.ExecutionResult, error) {
	return function.ExecutionResult{FunctionID: def.ID, Output: payload}, nil
}
