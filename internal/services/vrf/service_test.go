package vrf

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domainvrf "github.com/R3E-Network/service_layer/internal/app/domain/vrf"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
	core "github.com/R3E-Network/service_layer/internal/services/core"
)

const (
	testVRFWallet    = "0xabc123abc123abc123abc123abc123abc123abcd"
	testVRFBadWallet = "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
)

func TestService_CreateKeyAndList(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{
		WorkspaceID:   acct.ID,
		WalletAddress: testVRFWallet,
		Status:        "active",
	}); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	key, err := svc.CreateKey(context.Background(), domainvrf.Key{
		AccountID:     acct.ID,
		PublicKey:     "pk",
		Label:         "Label",
		WalletAddress: testVRFWallet,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if key.Status != domainvrf.KeyStatusInactive {
		t.Fatalf("expected inactive status")
	}

	keys, err := svc.ListKeys(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key")
	}
}

func TestService_UpdateKeyOwnership(t *testing.T) {
	store := memory.New()
	acct1, err := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	if err != nil {
		t.Fatalf("create account1: %v", err)
	}
	acct2, err := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	if err != nil {
		t.Fatalf("create account2: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct1.ID, WalletAddress: testVRFWallet}); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	key, err := svc.CreateKey(context.Background(), domainvrf.Key{
		AccountID:     acct1.ID,
		PublicKey:     "pk",
		WalletAddress: testVRFWallet,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	_, err = svc.UpdateKey(context.Background(), acct2.ID, domainvrf.Key{ID: key.ID, AccountID: acct2.ID, PublicKey: "pk", WalletAddress: testVRFWallet})
	if err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_CreateRequestDispatcher(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testVRFWallet}); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	key, err := svc.CreateKey(context.Background(), domainvrf.Key{AccountID: acct.ID, PublicKey: "pk", WalletAddress: testVRFWallet, Status: domainvrf.KeyStatusActive})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	called := false
	svc.WithDispatcher(DispatcherFunc(func(ctx context.Context, req domainvrf.Request, k domainvrf.Key) error {
		called = true
		if req.KeyID != k.ID {
			t.Fatalf("dispatcher key mismatch")
		}
		return nil
	}))

	req, err := svc.CreateRequest(context.Background(), acct.ID, key.ID, "consumer", "seed", map[string]string{"env": "prod"})
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if !called {
		t.Fatalf("expected dispatcher call")
	}
	got, err := svc.GetRequest(context.Background(), acct.ID, req.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if got.ID != req.ID {
		t.Fatalf("request mismatch")
	}
}

func TestService_CreateRequestRejectsForeignKey(t *testing.T) {
	store := memory.New()
	acct1, err := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	if err != nil {
		t.Fatalf("create account1: %v", err)
	}
	acct2, err := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	if err != nil {
		t.Fatalf("create account2: %v", err)
	}
	svc := New(store, store, nil)
	key, err := svc.CreateKey(context.Background(), domainvrf.Key{
		AccountID:     acct1.ID,
		PublicKey:     "pk",
		WalletAddress: testVRFWallet,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	if _, err := svc.CreateRequest(context.Background(), acct2.ID, key.ID, "consumer", "seed", nil); err == nil {
		t.Fatalf("expected ownership check for foreign key")
	}
}

func TestService_CreateKeyRequiresWalletRegistration(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)

	if _, err := svc.CreateKey(context.Background(), domainvrf.Key{
		AccountID:     acct.ID,
		PublicKey:     "pk",
		WalletAddress: testVRFBadWallet,
	}); err == nil {
		t.Fatalf("expected error when wallet not registered")
	}
}

func TestService_GetKey(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testVRFWallet})
	key, _ := svc.CreateKey(context.Background(), domainvrf.Key{AccountID: acct.ID, PublicKey: "pk", WalletAddress: testVRFWallet})

	got, err := svc.GetKey(context.Background(), acct.ID, key.ID)
	if err != nil {
		t.Fatalf("get key: %v", err)
	}
	if got.ID != key.ID {
		t.Fatalf("key mismatch")
	}
}

func TestService_GetKeyOwnership(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct1.ID, WalletAddress: testVRFWallet})
	key, _ := svc.CreateKey(context.Background(), domainvrf.Key{AccountID: acct1.ID, PublicKey: "pk", WalletAddress: testVRFWallet})

	if _, err := svc.GetKey(context.Background(), acct2.ID, key.ID); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_UpdateKey(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testVRFWallet})
	key, _ := svc.CreateKey(context.Background(), domainvrf.Key{AccountID: acct.ID, PublicKey: "pk", WalletAddress: testVRFWallet})

	updated, err := svc.UpdateKey(context.Background(), acct.ID, domainvrf.Key{ID: key.ID, AccountID: acct.ID, PublicKey: "pk2", WalletAddress: testVRFWallet, Label: "updated", Status: domainvrf.KeyStatusActive})
	if err != nil {
		t.Fatalf("update key: %v", err)
	}
	if updated.Label != "updated" {
		t.Fatalf("expected updated label")
	}
}

func TestService_ListRequests(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testVRFWallet})
	key, _ := svc.CreateKey(context.Background(), domainvrf.Key{AccountID: acct.ID, PublicKey: "pk", WalletAddress: testVRFWallet})
	svc.CreateRequest(context.Background(), acct.ID, key.ID, "consumer", "seed", nil)

	requests, err := svc.ListRequests(context.Background(), acct.ID, 10)
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected 1 request")
	}
}

func TestService_GetRequestOwnership(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct1.ID, WalletAddress: testVRFWallet})
	key, _ := svc.CreateKey(context.Background(), domainvrf.Key{AccountID: acct1.ID, PublicKey: "pk", WalletAddress: testVRFWallet})
	req, _ := svc.CreateRequest(context.Background(), acct1.ID, key.ID, "consumer", "seed", nil)

	if _, err := svc.GetRequest(context.Background(), acct2.ID, req.ID); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_KeyValidation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)

	// Missing public key
	if _, err := svc.CreateKey(context.Background(), domainvrf.Key{AccountID: acct.ID, WalletAddress: testVRFWallet}); err == nil {
		t.Fatalf("expected public_key required error")
	}
	// Missing wallet address
	if _, err := svc.CreateKey(context.Background(), domainvrf.Key{AccountID: acct.ID, PublicKey: "pk"}); err == nil {
		t.Fatalf("expected wallet_address required error")
	}
	// Invalid status
	if _, err := svc.CreateKey(context.Background(), domainvrf.Key{AccountID: acct.ID, PublicKey: "pk", WalletAddress: testVRFWallet, Status: "invalid"}); err == nil {
		t.Fatalf("expected invalid status error")
	}
}

func TestService_RequestValidation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testVRFWallet})
	key, _ := svc.CreateKey(context.Background(), domainvrf.Key{AccountID: acct.ID, PublicKey: "pk", WalletAddress: testVRFWallet})

	// Missing consumer
	if _, err := svc.CreateRequest(context.Background(), acct.ID, key.ID, "", "seed", nil); err == nil {
		t.Fatalf("expected consumer required error")
	}
	// Missing seed
	if _, err := svc.CreateRequest(context.Background(), acct.ID, key.ID, "consumer", "", nil); err == nil {
		t.Fatalf("expected seed required error")
	}
}

func TestService_Lifecycle(t *testing.T) {
	svc := New(nil, nil, nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := svc.Ready(context.Background()); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if svc.Ready(context.Background()) == nil {
		t.Fatalf("expected not ready after stop")
	}
}

func TestService_Manifest(t *testing.T) {
	svc := New(nil, nil, nil)
	m := svc.Manifest()
	if m.Name != "vrf" {
		t.Fatalf("expected name vrf")
	}
	if m.Domain != "vrf" {
		t.Fatalf("expected domain vrf")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "vrf" {
		t.Fatalf("expected name vrf")
	}
}

func TestService_WithDispatcherRetry(t *testing.T) {
	svc := New(nil, nil, nil)
	svc.WithDispatcherRetry(core.RetryPolicy{Attempts: 0})
}

func TestService_WithDispatcherHooks(t *testing.T) {
	svc := New(nil, nil, nil)
	svc.WithDispatcherHooks(core.DispatchHooks{})
}

func TestService_WithTracer(t *testing.T) {
	svc := New(nil, nil, nil)
	svc.WithTracer(core.NoopTracer)
}
