package accounts

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/R3E-Network/service_layer/pkg/logger"
)

func TestNew(t *testing.T) {
	store := NewMemoryStore()

	// Test with nil logger - should use default
	svc := New(store, nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.Name() != "accounts" {
		t.Fatalf("expected name 'accounts', got %q", svc.Name())
	}

	// Test with custom logger
	log := logger.NewDefault("test-accounts")
	log.SetOutput(io.Discard)
	svc2 := New(store, log)
	if svc2.log != log {
		t.Fatal("expected custom logger to be set")
	}
}

func TestService_Manifest(t *testing.T) {
	store := NewMemoryStore()
	svc := New(store, nil)

	manifest := svc.Manifest()
	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}
	if manifest.Name != "accounts" {
		t.Fatalf("expected name 'accounts', got %q", manifest.Name)
	}
	if manifest.Domain != "accounts" {
		t.Fatalf("expected domain 'accounts', got %q", manifest.Domain)
	}
	if manifest.Layer != "service" {
		t.Fatalf("expected layer 'service', got %q", manifest.Layer)
	}
}

func TestService_Descriptor(t *testing.T) {
	store := NewMemoryStore()
	svc := New(store, nil)

	desc := svc.Descriptor()
	if desc.Name != "accounts" {
		t.Fatalf("expected name 'accounts', got %q", desc.Name)
	}
	if desc.Domain != "accounts" {
		t.Fatalf("expected domain 'accounts', got %q", desc.Domain)
	}
}

func TestService_StartStop(t *testing.T) {
	store := NewMemoryStore()
	svc := New(store, nil)
	ctx := context.Background()

	// Initially not ready
	err := svc.Ready(ctx)
	if err == nil {
		t.Fatal("expected not ready error initially")
	}

	// Start should mark ready
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := svc.Ready(ctx); err != nil {
		t.Fatalf("expected ready after start: %v", err)
	}

	// Stop should mark not ready
	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := svc.Ready(ctx); err == nil {
		t.Fatal("expected not ready after stop")
	}
}

func TestService_Create(t *testing.T) {
	store := NewMemoryStore()
	log := logger.NewDefault("test-accounts")
	log.SetOutput(io.Discard)
	svc := New(store, log)
	ctx := context.Background()

	// Empty owner should fail
	_, err := svc.Create(ctx, "", nil)
	if err == nil {
		t.Fatal("expected error for empty owner")
	}

	// Valid creation
	acct, err := svc.Create(ctx, "alice", map[string]string{"tier": "pro"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if acct.ID == "" {
		t.Fatal("expected id to be generated")
	}
	if acct.Owner != "alice" {
		t.Fatalf("expected owner 'alice', got %q", acct.Owner)
	}
	if acct.Metadata["tier"] != "pro" {
		t.Fatal("expected metadata to be preserved")
	}
}

func TestService_Get(t *testing.T) {
	store := NewMemoryStore()
	log := logger.NewDefault("test-accounts")
	log.SetOutput(io.Discard)
	svc := New(store, log)
	ctx := context.Background()

	// Get non-existent should fail
	_, err := svc.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent account")
	}

	// Get empty ID should fail
	_, err = svc.Get(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}

	// Create and get
	acct, _ := svc.Create(ctx, "bob", nil)
	retrieved, err := svc.Get(ctx, acct.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if retrieved.ID != acct.ID {
		t.Fatalf("expected ID %q, got %q", acct.ID, retrieved.ID)
	}
}

func TestService_UpdateMetadata(t *testing.T) {
	store := NewMemoryStore()
	log := logger.NewDefault("test-accounts")
	log.SetOutput(io.Discard)
	svc := New(store, log)
	ctx := context.Background()

	// Update non-existent should fail
	_, err := svc.UpdateMetadata(ctx, "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent account")
	}

	// Update empty ID should fail
	_, err = svc.UpdateMetadata(ctx, "", nil)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}

	// Create and update
	acct, _ := svc.Create(ctx, "charlie", map[string]string{"tier": "basic"})
	updated, err := svc.UpdateMetadata(ctx, acct.ID, map[string]string{"tier": "enterprise", "region": "us"})
	if err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	if updated.Metadata["tier"] != "enterprise" {
		t.Fatal("expected tier to be updated")
	}
	if updated.Metadata["region"] != "us" {
		t.Fatal("expected region to be added")
	}
}

func TestService_List(t *testing.T) {
	store := NewMemoryStore()
	log := logger.NewDefault("test-accounts")
	log.SetOutput(io.Discard)
	svc := New(store, log)
	ctx := context.Background()

	// List empty
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 accounts, got %d", len(list))
	}

	// Create some accounts and list
	svc.Create(ctx, "user1", nil)
	svc.Create(ctx, "user2", nil)
	svc.Create(ctx, "user3", nil)

	list, err = svc.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 accounts, got %d", len(list))
	}
}

func TestService_Delete(t *testing.T) {
	store := NewMemoryStore()
	log := logger.NewDefault("test-accounts")
	log.SetOutput(io.Discard)
	svc := New(store, log)
	ctx := context.Background()

	// Delete non-existent should fail
	err := svc.Delete(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent account")
	}

	// Delete empty ID should fail
	err = svc.Delete(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}

	// Create and delete
	acct, _ := svc.Create(ctx, "dave", nil)
	err = svc.Delete(ctx, acct.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Verify deletion
	_, err = svc.Get(ctx, acct.ID)
	if err == nil {
		t.Fatal("expected error getting deleted account")
	}
}

func TestService_DeleteWithWhitespace(t *testing.T) {
	store := NewMemoryStore()
	log := logger.NewDefault("test-accounts")
	log.SetOutput(io.Discard)
	svc := New(store, log)
	ctx := context.Background()

	acct, _ := svc.Create(ctx, "eve", nil)

	// Delete with whitespace around ID
	err := svc.Delete(ctx, "  "+acct.ID+"  ")
	if err != nil {
		t.Fatalf("delete with whitespace: %v", err)
	}

	// Verify deletion
	_, err = svc.Get(ctx, acct.ID)
	if err == nil {
		t.Fatal("expected error getting deleted account")
	}
}

func TestService_CreateAccount_EngineAPI(t *testing.T) {
	store := NewMemoryStore()
	log := logger.NewDefault("test-accounts")
	log.SetOutput(io.Discard)
	svc := New(store, log)
	ctx := context.Background()

	// Test engine API CreateAccount
	id, err := svc.CreateAccount(ctx, "frank", map[string]string{"env": "test"})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	// Verify through Get
	acct, err := svc.Get(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if acct.Owner != "frank" {
		t.Fatalf("expected owner 'frank', got %q", acct.Owner)
	}
}

func TestService_ListAccounts_EngineAPI(t *testing.T) {
	store := NewMemoryStore()
	log := logger.NewDefault("test-accounts")
	log.SetOutput(io.Discard)
	svc := New(store, log)
	ctx := context.Background()

	// Create some accounts
	svc.Create(ctx, "user-a", nil)
	svc.Create(ctx, "user-b", nil)

	// Test engine API ListAccounts
	list, err := svc.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(list))
	}
}

func TestService(t *testing.T) {
	store := NewMemoryStore()
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

func ExampleService_Create() {
	store := NewMemoryStore()
	log := logger.NewDefault("example-accounts")
	log.SetOutput(io.Discard)
	svc := New(store, log)
	acct, _ := svc.Create(context.Background(), "alice", map[string]string{"tier": "pro"})
	fmt.Println(acct.Owner, acct.Metadata["tier"])
	// Output:
	// alice pro
}
