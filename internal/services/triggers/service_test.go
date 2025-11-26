package triggers

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
	"github.com/R3E-Network/service_layer/internal/domain/account"
	"github.com/R3E-Network/service_layer/internal/domain/function"
	"github.com/R3E-Network/service_layer/internal/domain/trigger"
	"github.com/R3E-Network/service_layer/pkg/logger"
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
	trg := trigger.Trigger{AccountID: acct.ID, FunctionID: fn.ID, Rule: "cron:@hourly", Enabled: true}
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
	evt := trigger.Trigger{AccountID: acct.ID, FunctionID: fn.ID, Type: trigger.TypeEvent, Rule: "user.created", Enabled: true}
	if _, err := svc.Register(context.Background(), evt); err != nil {
		t.Fatalf("register event trigger: %v", err)
	}
	// Webhook trigger
	wh := trigger.Trigger{
		AccountID:  acct.ID,
		FunctionID: fn.ID,
		Type:       trigger.TypeWebhook,
		Config:     map[string]string{"url": "https://callback"},
		Enabled:    true,
	}
	if _, err := svc.Register(context.Background(), wh); err != nil {
		t.Fatalf("register webhook trigger: %v", err)
	}
}

func TestService_RegisterRejectsForeignFunction(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	fn, _ := store.CreateFunction(context.Background(), function.Definition{AccountID: acct1.ID, Name: "fn", Source: "() => 1"})

	svc := New(store, store, store, nil)
	trg := trigger.Trigger{AccountID: acct2.ID, FunctionID: fn.ID, Rule: "cron:@hourly", Enabled: true}
	if _, err := svc.Register(context.Background(), trg); err == nil {
		t.Fatalf("expected register to fail when function belongs to different account")
	}
}

func TestService_RegisterRespectsDisabledFlag(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	fn, _ := store.CreateFunction(context.Background(), function.Definition{AccountID: acct.ID, Name: "fn", Source: "() => 1"})

	svc := New(store, store, store, nil)
	trg := trigger.Trigger{AccountID: acct.ID, FunctionID: fn.ID, Rule: "cron:@hourly", Enabled: false}

	created, err := svc.Register(context.Background(), trg)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if created.Enabled {
		t.Fatalf("expected trigger to remain disabled")
	}
}

func ExampleService_Register() {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	fn, _ := store.CreateFunction(context.Background(), function.Definition{AccountID: acct.ID, Name: "fn", Source: "() => 1"})
	log := logger.NewDefault("example-triggers")
	log.SetOutput(io.Discard)
	svc := New(store, store, store, log)

	trg, _ := svc.Register(context.Background(), trigger.Trigger{
		AccountID:  acct.ID,
		FunctionID: fn.ID,
		Type:       trigger.TypeEvent,
		Rule:       "user.created",
		Enabled:    true,
	})
	fmt.Println(trg.Type, trg.Enabled)
	// Output:
	// event true
}
