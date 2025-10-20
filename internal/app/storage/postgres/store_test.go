package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	_ "github.com/lib/pq"
)

func TestStoreIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; skipping postgres integration test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store := New(db)

	ctx := context.Background()
	acct, err := store.CreateAccount(ctx, account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	fn, err := store.CreateFunction(ctx, function.Definition{AccountID: acct.ID, Name: "fn", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	trg := trigger.Trigger{AccountID: acct.ID, FunctionID: fn.ID, Rule: "cron:@hourly", Enabled: true}
	if _, err := store.CreateTrigger(ctx, trg); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
}
