package datalink

import (
	"context"
	"testing"

	core "github.com/R3E-Network/service_layer/system/framework/core"
)

const testChannelSigner = "0xdddd111122223333444455556666777788889999"

func setupTest() (*MemoryStore, *MockAccountChecker, *MockWalletChecker) {
	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	wallets := NewMockWalletChecker()
	return store, accounts, wallets
}

func TestService_CreateChannel(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccount("acct-1", "")
	wallets.AddWallet("acct-1", testChannelSigner)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	ch, err := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if ch.Status != ChannelStatusInactive {
		t.Fatalf("expected default inactive")
	}
	channels, err := svc.ListChannels(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("list channels: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("expected one channel")
	}
}

func TestService_CreateDelivery(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccount("acct-1", "")
	wallets.AddWallet("acct-1", testChannelSigner)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	ch, err := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	called := false
	svc.WithDispatcher(DispatcherFunc(func(ctx context.Context, del Delivery, channel Channel) error {
		called = true
		if del.ChannelID != channel.ID {
			t.Fatalf("dispatcher mismatch")
		}
		return nil
	}))

	del, err := svc.CreateDelivery(context.Background(), "acct-1", ch.ID, map[string]any{"payload": "x"}, map[string]string{"env": "prod"})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	if !called {
		t.Fatalf("expected dispatcher call")
	}
	got, err := svc.GetDelivery(context.Background(), "acct-1", del.ID)
	if err != nil || got.ID != del.ID {
		t.Fatalf("get delivery mismatch")
	}
}

func TestService_ChannelRequiresSignerWallet(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccount("acct-1", "")

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	if _, err := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Name: "Provider", Endpoint: "https://example", SignerSet: []string{"unknown"}}); err == nil {
		t.Fatalf("expected signer validation error")
	}
}

func TestService_UpdateChannel(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccount("acct-1", "")
	wallets.AddWallet("acct-1", testChannelSigner)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	ch, _ := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})

	updated, err := svc.UpdateChannel(context.Background(), Channel{ID: ch.ID, AccountID: "acct-1", Name: "Updated", Endpoint: "https://updated", SignerSet: []string{testChannelSigner}, Status: ChannelStatusActive})
	if err != nil {
		t.Fatalf("update channel: %v", err)
	}
	if updated.Name != "Updated" {
		t.Fatalf("expected updated name")
	}
	if updated.Status != ChannelStatusActive {
		t.Fatalf("expected active status")
	}
}

func TestService_UpdateChannelOwnership(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccount("acct-1", "")
	accounts.AddAccount("acct-2", "")
	wallets.AddWallet("acct-1", testChannelSigner)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	ch, _ := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})

	if _, err := svc.UpdateChannel(context.Background(), Channel{ID: ch.ID, AccountID: "acct-2", Name: "Hacked", Endpoint: "https://hacked", SignerSet: []string{testChannelSigner}}); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_GetChannel(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccount("acct-1", "")
	wallets.AddWallet("acct-1", testChannelSigner)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	ch, _ := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})

	got, err := svc.GetChannel(context.Background(), "acct-1", ch.ID)
	if err != nil {
		t.Fatalf("get channel: %v", err)
	}
	if got.ID != ch.ID {
		t.Fatalf("channel mismatch")
	}
}

func TestService_GetChannelOwnership(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccount("acct-1", "")
	accounts.AddAccount("acct-2", "")
	wallets.AddWallet("acct-1", testChannelSigner)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	ch, _ := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})

	if _, err := svc.GetChannel(context.Background(), "acct-2", ch.ID); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_ListDeliveries(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccount("acct-1", "")
	wallets.AddWallet("acct-1", testChannelSigner)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	ch, _ := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})
	svc.CreateDelivery(context.Background(), "acct-1", ch.ID, map[string]any{"test": "data"}, nil)

	deliveries, err := svc.ListDeliveries(context.Background(), "acct-1", 10)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected one delivery")
	}
}

func TestService_GetDeliveryOwnership(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccount("acct-1", "")
	accounts.AddAccount("acct-2", "")
	wallets.AddWallet("acct-1", testChannelSigner)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	ch, _ := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})
	del, _ := svc.CreateDelivery(context.Background(), "acct-1", ch.ID, map[string]any{"test": "data"}, nil)

	if _, err := svc.GetDelivery(context.Background(), "acct-2", del.ID); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_ChannelValidation(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccount("acct-1", "")
	wallets.AddWallet("acct-1", testChannelSigner)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)

	// Missing name
	if _, err := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Endpoint: "https://example", SignerSet: []string{testChannelSigner}}); err == nil {
		t.Fatalf("expected name required error")
	}
	// Missing endpoint
	if _, err := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Name: "Provider", SignerSet: []string{testChannelSigner}}); err == nil {
		t.Fatalf("expected endpoint required error")
	}
	// Missing signer set
	if _, err := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Name: "Provider", Endpoint: "https://example"}); err == nil {
		t.Fatalf("expected signer_set required error")
	}
	// Invalid status
	if _, err := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}, Status: "invalid"}); err == nil {
		t.Fatalf("expected invalid status error")
	}
}

func TestService_Publish(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccount("acct-1", "")
	wallets.AddWallet("acct-1", testChannelSigner)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	ch, _ := svc.CreateChannel(context.Background(), Channel{AccountID: "acct-1", Name: "Provider", Endpoint: "https://example", SignerSet: []string{testChannelSigner}})

	err := svc.Publish(context.Background(), "delivery", map[string]any{
		"account_id": "acct-1",
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
