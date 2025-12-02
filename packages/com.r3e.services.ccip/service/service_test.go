package ccip

import (
	"context"
	"testing"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

const testLaneWallet = "0xaaaabbbbccccddddeeeeffffaaaabbbbccccdddd"

func setupTest() (*MemoryStore, *MockAccountChecker, *MockWalletChecker) {
	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	wallets := NewMockWalletChecker()
	return store, accounts, wallets
}

func TestService_CreateLane(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")
	wallets.AddWallet("acct-1", testLaneWallet)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	lane, err := svc.CreateLane(context.Background(), Lane{
		AccountID:   "acct-1",
		Name:        "Primary Lane",
		SourceChain: "Ethereum",
		DestChain:   "Neo",
		SignerSet:   []string{testLaneWallet},
	})
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}
	if lane.SourceChain != "ethereum" {
		t.Fatalf("expected normalized source chain, got %s", lane.SourceChain)
	}

	lanes, err := svc.ListLanes(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("list lanes: %v", err)
	}
	if len(lanes) != 1 {
		t.Fatalf("expected 1 lane, got %d", len(lanes))
	}
}

func TestService_CreateLaneValidation(t *testing.T) {
	store, accounts, _ := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")

	svc := New(accounts, store, nil)
	if _, err := svc.CreateLane(context.Background(), Lane{AccountID: "acct-1"}); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestService_SendMessageOwnership(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")
	accounts.AddAccountWithTenant("acct-2", "")
	wallets.AddWallet("acct-1", testLaneWallet)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	lane, err := svc.CreateLane(context.Background(), Lane{
		AccountID:   "acct-1",
		Name:        "Lane",
		SourceChain: "eth",
		DestChain:   "neo",
		SignerSet:   []string{testLaneWallet},
	})
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	if _, err := svc.SendMessage(context.Background(), "acct-2", lane.ID, nil, nil, nil, nil); err == nil {
		t.Fatalf("expected ownership error when sending with foreign account")
	}
}

func TestService_SendMessageDispatch(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")
	wallets.AddWallet("acct-1", testLaneWallet)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	lane, err := svc.CreateLane(context.Background(), Lane{
		AccountID:   "acct-1",
		Name:        "Lane",
		SourceChain: "eth",
		DestChain:   "neo",
		SignerSet:   []string{testLaneWallet},
	})
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}
	called := false
	svc.WithDispatcher(DispatcherFunc(func(ctx context.Context, msg Message, ln Lane) error {
		called = true
		if msg.LaneID != ln.ID {
			t.Fatalf("dispatcher lane mismatch")
		}
		return nil
	}))

	msg, err := svc.SendMessage(context.Background(), "acct-1", lane.ID, map[string]any{"hello": "world"}, []TokenTransfer{{Token: "eth", Amount: "1", Recipient: "addr"}}, map[string]string{"Env": "Prod"}, []string{"Priority"})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if len(msg.TokenTransfers) != 1 || msg.TokenTransfers[0].Token != "eth" {
		t.Fatalf("expected normalized token transfer")
	}
	if !called {
		t.Fatalf("expected dispatcher to be called")
	}

	msgs, err := svc.ListMessages(context.Background(), "acct-1", 10)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message")
	}
}

func TestService_UpdateLane(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")
	wallets.AddWallet("acct-1", testLaneWallet)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	lane, _ := svc.CreateLane(context.Background(), Lane{AccountID: "acct-1", Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})

	updated, err := svc.UpdateLane(context.Background(), Lane{ID: lane.ID, AccountID: "acct-1", Name: "Updated", SourceChain: "bsc", DestChain: "polygon", SignerSet: []string{testLaneWallet}})
	if err != nil {
		t.Fatalf("update lane: %v", err)
	}
	if updated.Name != "Updated" {
		t.Fatalf("expected updated name")
	}
	if updated.SourceChain != "bsc" {
		t.Fatalf("expected bsc source chain")
	}
}

func TestService_UpdateLaneOwnership(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")
	accounts.AddAccountWithTenant("acct-2", "")
	wallets.AddWallet("acct-1", testLaneWallet)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	lane, _ := svc.CreateLane(context.Background(), Lane{AccountID: "acct-1", Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})

	if _, err := svc.UpdateLane(context.Background(), Lane{ID: lane.ID, AccountID: "acct-2", Name: "Hacked", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}}); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_GetLane(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")
	wallets.AddWallet("acct-1", testLaneWallet)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	lane, _ := svc.CreateLane(context.Background(), Lane{AccountID: "acct-1", Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})

	got, err := svc.GetLane(context.Background(), "acct-1", lane.ID)
	if err != nil {
		t.Fatalf("get lane: %v", err)
	}
	if got.ID != lane.ID {
		t.Fatalf("lane mismatch")
	}
}

func TestService_GetLaneOwnership(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")
	accounts.AddAccountWithTenant("acct-2", "")
	wallets.AddWallet("acct-1", testLaneWallet)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	lane, _ := svc.CreateLane(context.Background(), Lane{AccountID: "acct-1", Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})

	if _, err := svc.GetLane(context.Background(), "acct-2", lane.ID); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_GetMessage(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")
	wallets.AddWallet("acct-1", testLaneWallet)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	lane, _ := svc.CreateLane(context.Background(), Lane{AccountID: "acct-1", Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})
	msg, _ := svc.SendMessage(context.Background(), "acct-1", lane.ID, nil, nil, nil, nil)

	got, err := svc.GetMessage(context.Background(), "acct-1", msg.ID)
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if got.ID != msg.ID {
		t.Fatalf("message mismatch")
	}
}

func TestService_GetMessageOwnership(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")
	accounts.AddAccountWithTenant("acct-2", "")
	wallets.AddWallet("acct-1", testLaneWallet)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	lane, _ := svc.CreateLane(context.Background(), Lane{AccountID: "acct-1", Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})
	msg, _ := svc.SendMessage(context.Background(), "acct-1", lane.ID, nil, nil, nil, nil)

	if _, err := svc.GetMessage(context.Background(), "acct-2", msg.ID); err == nil {
		t.Fatalf("expected ownership error")
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
	if m.Name != "ccip" {
		t.Fatalf("expected name ccip")
	}
	if m.Domain != "ccip" {
		t.Fatalf("expected domain ccip")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "ccip" {
		t.Fatalf("expected name ccip")
	}
}

func TestService_LaneValidation(t *testing.T) {
	store, accounts, _ := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")

	svc := New(accounts, store, nil)

	// Missing name
	if _, err := svc.CreateLane(context.Background(), Lane{AccountID: "acct-1", SourceChain: "eth", DestChain: "neo"}); err == nil {
		t.Fatalf("expected name required error")
	}
	// Missing source chain
	if _, err := svc.CreateLane(context.Background(), Lane{AccountID: "acct-1", Name: "Lane", DestChain: "neo"}); err == nil {
		t.Fatalf("expected source_chain required error")
	}
	// Missing dest chain
	if _, err := svc.CreateLane(context.Background(), Lane{AccountID: "acct-1", Name: "Lane", SourceChain: "eth"}); err == nil {
		t.Fatalf("expected dest_chain required error")
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

func TestService_TokenTransferNormalization(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")
	wallets.AddWallet("acct-1", testLaneWallet)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)
	lane, _ := svc.CreateLane(context.Background(), Lane{AccountID: "acct-1", Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})

	// Empty token transfer fields should be filtered
	msg, err := svc.SendMessage(context.Background(), "acct-1", lane.ID, nil, []TokenTransfer{
		{Token: "", Amount: "1", Recipient: "addr"},   // missing token
		{Token: "eth", Amount: "", Recipient: "addr"}, // missing amount
		{Token: "eth", Amount: "1", Recipient: ""},    // missing recipient
		{Token: "ETH", Amount: "1", Recipient: "addr"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if len(msg.TokenTransfers) != 1 {
		t.Fatalf("expected 1 valid token transfer, got %d", len(msg.TokenTransfers))
	}
}

func TestService_CreateLane_DuplicateSigners(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")
	wallets.AddWallet("acct-1", testLaneWallet)

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)

	// Test with duplicate signers - should deduplicate
	lane, err := svc.CreateLane(context.Background(), Lane{
		AccountID:   "acct-1",
		Name:        "Lane",
		SourceChain: "eth",
		DestChain:   "neo",
		SignerSet:   []string{testLaneWallet, testLaneWallet, "  " + testLaneWallet + "  "},
	})
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}
	if len(lane.SignerSet) != 1 {
		t.Fatalf("expected deduplicated signers, got %d", len(lane.SignerSet))
	}
}

func TestService_CreateLane_EmptySigners(t *testing.T) {
	store, accounts, wallets := setupTest()
	accounts.AddAccountWithTenant("acct-1", "")

	svc := New(accounts, store, nil)
	svc.WithWalletChecker(wallets)

	// Test with empty signers - should succeed
	lane, err := svc.CreateLane(context.Background(), Lane{
		AccountID:   "acct-1",
		Name:        "Lane",
		SourceChain: "eth",
		DestChain:   "neo",
		SignerSet:   []string{},
	})
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}
	if lane.ID == "" {
		t.Fatalf("expected lane ID")
	}
}

func TestService_ListLanes_MissingAccount(t *testing.T) {
	store, accounts, _ := setupTest()
	svc := New(accounts, store, nil)
	_, err := svc.ListLanes(context.Background(), "nonexistent")
	if err == nil {
		t.Fatalf("expected error for nonexistent account")
	}
}

func TestService_ListMessages_MissingAccount(t *testing.T) {
	store, accounts, _ := setupTest()
	svc := New(accounts, store, nil)
	_, err := svc.ListMessages(context.Background(), "nonexistent", 10)
	if err == nil {
		t.Fatalf("expected error for nonexistent account")
	}
}
