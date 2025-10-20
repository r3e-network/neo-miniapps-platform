package app

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
)

func TestApplicationLifecycle(t *testing.T) {
	application, err := New(Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}

	ctx := context.Background()
	if err := application.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	acct, err := application.Accounts.Create(ctx, "owner", nil)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	fn, err := application.Functions.Create(ctx, function.Definition{AccountID: acct.ID, Name: "hello", Source: "() => 'hi'"})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	if _, err := application.Triggers.Register(ctx, trigger.Trigger{AccountID: acct.ID, FunctionID: fn.ID, Rule: "cron:@hourly"}); err != nil {
		t.Fatalf("register trigger: %v", err)
	}

	if err := application.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
}
