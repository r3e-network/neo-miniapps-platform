package accounts

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
)

func TestService(t *testing.T) {
	store := memory.New()
	svc := New(store, nil)

	acct, err := svc.Create(context.Background(), "alice", map[string]string{"tier": "pro"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if acct.ID == "" {
		t.Fatalf("expected id to be generated")
	}

	updated, err := svc.UpdateMetadata(context.Background(), acct.ID, map[string]string{"tier": "enterprise"})
	if err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	if updated.Metadata["tier"] != "enterprise" {
		t.Fatalf("metadata not updated")
	}

	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 account, got %d", len(list))
	}
}
