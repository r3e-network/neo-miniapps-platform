package memory

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
)

func TestStoreCreateAccountAndFunction(t *testing.T) {
	store := New()

	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	fn, err := store.CreateFunction(context.Background(), function.Definition{AccountID: acct.ID, Name: "hello", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}
	if fn.AccountID != acct.ID {
		t.Fatalf("expected function to retain account id")
	}

	exec, err := store.CreateExecution(context.Background(), function.Execution{AccountID: acct.ID, FunctionID: fn.ID})
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}

	list, err := store.ListFunctionExecutions(context.Background(), fn.ID, 0)
	if err != nil || len(list) != 1 || list[0].ID != exec.ID {
		t.Fatalf("expected execution to be listed, got %#v err=%v", list, err)
	}
}
