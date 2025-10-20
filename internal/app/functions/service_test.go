package functions

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/storage"
)

func TestFunctionService(t *testing.T) {
	store := storage.NewMemory()
	acctSvc := storage.NewMemory()

	// Ensure account exists for validation.
	acct, err := acctSvc.CreateAccount(context.Background(), account.Account{Owner: "user"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := NewService(acctSvc, store, nil)
	def := function.Definition{AccountID: acct.ID, Name: "hello", Source: "() => 42"}
	created, err := svc.Create(context.Background(), def)
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	created.Description = "desc"
	updated, err := svc.Update(context.Background(), created)
	if err != nil {
		t.Fatalf("update function: %v", err)
	}
	if updated.Description != "desc" {
		t.Fatalf("expected description to be updated")
	}

	list, err := svc.List(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list functions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one function, got %d", len(list))
	}
}
