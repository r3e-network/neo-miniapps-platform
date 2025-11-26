package datalink

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domainlink "github.com/R3E-Network/service_layer/internal/app/domain/datalink"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
	core "github.com/R3E-Network/service_layer/internal/services/core"
)

const testChannelSigner = "0xdddd111122223333444455556666777788889999"

func TestService_CreateChannel(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testChannelSigner}); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	ch, err := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct.ID, Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if ch.Status != domainlink.ChannelStatusInactive {
		t.Fatalf("expected default inactive")
	}
	channels, err := svc.ListChannels(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list channels: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("expected one channel")
	}
}

func TestService_CreateDelivery(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testChannelSigner}); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	ch, err := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct.ID, Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	called := false
	svc.WithDispatcher(DispatcherFunc(func(ctx context.Context, del domainlink.Delivery, channel domainlink.Channel) error {
		called = true
		if del.ChannelID != channel.ID {
			t.Fatalf("dispatcher mismatch")
		}
		return nil
	}))

	del, err := svc.CreateDelivery(context.Background(), acct.ID, ch.ID, map[string]any{"payload": "x"}, map[string]string{"env": "prod"})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	if !called {
		t.Fatalf("expected dispatcher call")
	}
	got, err := svc.GetDelivery(context.Background(), acct.ID, del.ID)
	if err != nil || got.ID != del.ID {
		t.Fatalf("get delivery mismatch")
	}
}

func TestService_ChannelRequiresSignerWallet(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	if _, err := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct.ID, Name: "Provider", Endpoint: "https://example", SignerSet: []string{"unknown"}}); err == nil {
		t.Fatalf("expected signer validation error")
	}
}

func TestService_UpdateChannel(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testChannelSigner}); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	ch, _ := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct.ID, Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})

	updated, err := svc.UpdateChannel(context.Background(), domainlink.Channel{ID: ch.ID, AccountID: acct.ID, Name: "Updated", Endpoint: "https://updated", SignerSet: []string{testChannelSigner}, Status: domainlink.ChannelStatusActive})
	if err != nil {
		t.Fatalf("update channel: %v", err)
	}
	if updated.Name != "Updated" {
		t.Fatalf("expected updated name")
	}
	if updated.Status != domainlink.ChannelStatusActive {
		t.Fatalf("expected active status")
	}
}

func TestService_UpdateChannelOwnership(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct1.ID, WalletAddress: testChannelSigner})
	ch, _ := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct1.ID, Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})

	if _, err := svc.UpdateChannel(context.Background(), domainlink.Channel{ID: ch.ID, AccountID: acct2.ID, Name: "Hacked", Endpoint: "https://hacked", SignerSet: []string{testChannelSigner}}); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_GetChannel(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testChannelSigner})
	ch, _ := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct.ID, Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})

	got, err := svc.GetChannel(context.Background(), acct.ID, ch.ID)
	if err != nil {
		t.Fatalf("get channel: %v", err)
	}
	if got.ID != ch.ID {
		t.Fatalf("channel mismatch")
	}
}

func TestService_GetChannelOwnership(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct1.ID, WalletAddress: testChannelSigner})
	ch, _ := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct1.ID, Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})

	if _, err := svc.GetChannel(context.Background(), acct2.ID, ch.ID); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_ListDeliveries(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testChannelSigner})
	ch, _ := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct.ID, Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})
	svc.CreateDelivery(context.Background(), acct.ID, ch.ID, map[string]any{"test": "data"}, nil)

	deliveries, err := svc.ListDeliveries(context.Background(), acct.ID, 10)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected one delivery")
	}
}

func TestService_GetDeliveryOwnership(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct1.ID, WalletAddress: testChannelSigner})
	ch, _ := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct1.ID, Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})
	del, _ := svc.CreateDelivery(context.Background(), acct1.ID, ch.ID, map[string]any{"test": "data"}, nil)

	if _, err := svc.GetDelivery(context.Background(), acct2.ID, del.ID); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_ChannelValidation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testChannelSigner})

	// Missing name
	if _, err := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct.ID, Endpoint: "https://example", SignerSet: []string{testChannelSigner}}); err == nil {
		t.Fatalf("expected name required error")
	}
	// Missing endpoint
	if _, err := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct.ID, Name: "Provider", SignerSet: []string{testChannelSigner}}); err == nil {
		t.Fatalf("expected endpoint required error")
	}
	// Missing signer set
	if _, err := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct.ID, Name: "Provider", Endpoint: "https://example"}); err == nil {
		t.Fatalf("expected signer_set required error")
	}
	// Invalid status
	if _, err := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct.ID, Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}, Status: "invalid"}); err == nil {
		t.Fatalf("expected invalid status error")
	}
}

func TestService_Publish(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testChannelSigner})
	ch, _ := svc.CreateChannel(context.Background(), domainlink.Channel{AccountID: acct.ID, Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})

	err := svc.Publish(context.Background(), "delivery", map[string]any{
		"account_id": acct.ID,
		"channel_id": ch.ID,
		"payload":    map[string]any{"test": "data"},
		"metadata":   map[string]string{"env": "prod"},
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Test unsupported event
	if err := svc.Publish(context.Background(), "unsupported", nil); err == nil {
		t.Fatalf("expected unsupported event error")
	}

	// Test invalid payload
	if err := svc.Publish(context.Background(), "delivery", "invalid"); err == nil {
		t.Fatalf("expected payload type error")
	}
}

func TestService_Subscribe(t *testing.T) {
	svc := New(nil, nil, nil)
	// Subscribe not supported
	if err := svc.Subscribe(context.Background(), "delivery", func(ctx context.Context, data any) error { return nil }); err == nil {
		t.Fatalf("expected subscribe not supported error")
	}
	// Invalid event
	if err := svc.Subscribe(context.Background(), "invalid", nil); err == nil {
		t.Fatalf("expected unsupported event error")
	}
	// Nil handler
	if err := svc.Subscribe(context.Background(), "delivery", nil); err == nil {
		t.Fatalf("expected handler required error")
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
	if m.Name != "datalink" {
		t.Fatalf("expected name datalink")
	}
	if m.Domain != "datalink" {
		t.Fatalf("expected domain datalink")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "datalink" {
		t.Fatalf("expected name datalink")
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
