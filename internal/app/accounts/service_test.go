package accounts

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/storage"
)

func TestAccountService(t *testing.T) {
	store := storage.NewMemory()
	svc := NewService(store, nil)

	acct, err := svc.Create(context.Background(), "owner", map[string]string{"tier": "pro"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	if acct.ID == "" {
		t.Fatalf("expected id to be generated")
	}

	acct2, err := svc.UpdateMetadata(context.Background(), acct.ID, map[string]string{"tier": "enterprise"})
	if err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	if acct2.Metadata["tier"] != "enterprise" {
		t.Fatalf("expected metadata update")
	}

	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one account, got %d", len(list))
	}

	if err := svc.Delete(context.Background(), acct.ID); err != nil {
		t.Fatalf("delete account: %v", err)
	}

	if _, err := svc.Get(context.Background(), acct.ID); err == nil {
		t.Fatalf("expected get to fail after delete")
	}
}
