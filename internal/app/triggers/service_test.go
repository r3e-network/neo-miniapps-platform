package triggers

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	"github.com/R3E-Network/service_layer/internal/app/storage"
)

func TestTriggerService(t *testing.T) {
	store := storage.NewMemory()

	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "user"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	fn, err := store.CreateFunction(context.Background(), function.Definition{AccountID: acct.ID, Name: "f", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	svc := NewService(store, store, store, nil)
	trg, err := svc.Register(context.Background(), trigger.Trigger{AccountID: acct.ID, FunctionID: fn.ID, Rule: "cron:0 * * * *"})
	if err != nil {
		t.Fatalf("register trigger: %v", err)
	}
	if !trg.Enabled {
		t.Fatalf("expected trigger to be enabled")
	}

	if _, err := svc.SetEnabled(context.Background(), trg.ID, false); err != nil {
		t.Fatalf("disable trigger: %v", err)
	}

	list, err := svc.List(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list triggers: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one trigger, got %d", len(list))
	}
	if list[0].Enabled {
		t.Fatalf("expected trigger to be disabled")
	}
}
