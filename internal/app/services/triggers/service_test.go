package triggers

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
)

func TestService(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	fn, err := store.CreateFunction(context.Background(), function.Definition{AccountID: acct.ID, Name: "fn", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	svc := New(store, store, store, nil)
	trg := trigger.Trigger{AccountID: acct.ID, FunctionID: fn.ID, Rule: "cron:@hourly"}
	created, err := svc.Register(context.Background(), trg)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if !created.Enabled {
		t.Fatalf("expected trigger enabled")
	}

	if _, err := svc.SetEnabled(context.Background(), created.ID, false); err != nil {
		t.Fatalf("disable trigger: %v", err)
	}

	list, err := svc.List(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list triggers: %v", err)
	}
	if len(list) != 1 || list[0].Enabled {
		t.Fatalf("expected one disabled trigger")
	}
}

func TestService_RegisterEventAndWebhook(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	fn, _ := store.CreateFunction(context.Background(), function.Definition{AccountID: acct.ID, Name: "fn", Source: "() => 1"})

	svc := New(store, store, store, nil)
	// Event trigger
	evt := trigger.Trigger{AccountID: acct.ID, FunctionID: fn.ID, Type: trigger.TypeEvent, Rule: "user.created"}
	if _, err := svc.Register(context.Background(), evt); err != nil {
		t.Fatalf("register event trigger: %v", err)
	}
	// Webhook trigger
	wh := trigger.Trigger{
		AccountID:  acct.ID,
		FunctionID: fn.ID,
		Type:       trigger.TypeWebhook,
		Config:     map[string]string{"url": "https://callback"},
	}
	if _, err := svc.Register(context.Background(), wh); err != nil {
		t.Fatalf("register webhook trigger: %v", err)
	}
}
