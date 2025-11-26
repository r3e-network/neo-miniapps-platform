package ccip

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domainccip "github.com/R3E-Network/service_layer/internal/app/domain/ccip"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
	core "github.com/R3E-Network/service_layer/internal/services/core"
)

const testLaneWallet = "0xaaaabbbbccccddddeeeeffffaaaabbbbccccdddd"

func TestService_CreateLane(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testLaneWallet}); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	lane, err := svc.CreateLane(context.Background(), domainccip.Lane{
		AccountID:   acct.ID,
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

	lanes, err := svc.ListLanes(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list lanes: %v", err)
	}
	if len(lanes) != 1 {
		t.Fatalf("expected 1 lane, got %d", len(lanes))
	}
}

func TestService_CreateLaneValidation(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	if _, err := svc.CreateLane(context.Background(), domainccip.Lane{AccountID: acct.ID}); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestService_SendMessageOwnership(t *testing.T) {
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
	if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct1.ID, WalletAddress: testLaneWallet}); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	lane, err := svc.CreateLane(context.Background(), domainccip.Lane{
		AccountID:   acct1.ID,
		Name:        "Lane",
		SourceChain: "eth",
		DestChain:   "neo",
		SignerSet:   []string{testLaneWallet},
	})
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	if _, err := svc.SendMessage(context.Background(), acct2.ID, lane.ID, nil, nil, nil, nil); err == nil {
		t.Fatalf("expected ownership error when sending with foreign account")
	}
}

func TestService_SendMessageDispatch(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testLaneWallet}); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	lane, err := svc.CreateLane(context.Background(), domainccip.Lane{
		AccountID:   acct.ID,
		Name:        "Lane",
		SourceChain: "eth",
		DestChain:   "neo",
		SignerSet:   []string{testLaneWallet},
	})
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}
	called := false
	svc.WithDispatcher(DispatcherFunc(func(ctx context.Context, msg domainccip.Message, ln domainccip.Lane) error {
		called = true
		if msg.LaneID != ln.ID {
			t.Fatalf("dispatcher lane mismatch")
		}
		return nil
	}))

	msg, err := svc.SendMessage(context.Background(), acct.ID, lane.ID, map[string]any{"hello": "world"}, []domainccip.TokenTransfer{{Token: "eth", Amount: "1", Recipient: "addr"}}, map[string]string{"Env": "Prod"}, []string{"Priority"})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if len(msg.TokenTransfers) != 1 || msg.TokenTransfers[0].Token != "eth" {
		t.Fatalf("expected normalized token transfer")
	}
	if !called {
		t.Fatalf("expected dispatcher to be called")
	}

	msgs, err := svc.ListMessages(context.Background(), acct.ID, 10)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message")
	}
}

func TestService_UpdateLane(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testLaneWallet})
	lane, _ := svc.CreateLane(context.Background(), domainccip.Lane{AccountID: acct.ID, Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})

	updated, err := svc.UpdateLane(context.Background(), domainccip.Lane{ID: lane.ID, AccountID: acct.ID, Name: "Updated", SourceChain: "bsc", DestChain: "polygon", SignerSet: []string{testLaneWallet}})
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
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct1.ID, WalletAddress: testLaneWallet})
	lane, _ := svc.CreateLane(context.Background(), domainccip.Lane{AccountID: acct1.ID, Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})

	if _, err := svc.UpdateLane(context.Background(), domainccip.Lane{ID: lane.ID, AccountID: acct2.ID, Name: "Hacked", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}}); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_GetLane(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testLaneWallet})
	lane, _ := svc.CreateLane(context.Background(), domainccip.Lane{AccountID: acct.ID, Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})

	got, err := svc.GetLane(context.Background(), acct.ID, lane.ID)
	if err != nil {
		t.Fatalf("get lane: %v", err)
	}
	if got.ID != lane.ID {
		t.Fatalf("lane mismatch")
	}
}

func TestService_GetLaneOwnership(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct1.ID, WalletAddress: testLaneWallet})
	lane, _ := svc.CreateLane(context.Background(), domainccip.Lane{AccountID: acct1.ID, Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})

	if _, err := svc.GetLane(context.Background(), acct2.ID, lane.ID); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_GetMessage(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testLaneWallet})
	lane, _ := svc.CreateLane(context.Background(), domainccip.Lane{AccountID: acct.ID, Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})
	msg, _ := svc.SendMessage(context.Background(), acct.ID, lane.ID, nil, nil, nil, nil)

	got, err := svc.GetMessage(context.Background(), acct.ID, msg.ID)
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if got.ID != msg.ID {
		t.Fatalf("message mismatch")
	}
}

func TestService_GetMessageOwnership(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct1.ID, WalletAddress: testLaneWallet})
	lane, _ := svc.CreateLane(context.Background(), domainccip.Lane{AccountID: acct1.ID, Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})
	msg, _ := svc.SendMessage(context.Background(), acct1.ID, lane.ID, nil, nil, nil, nil)

	if _, err := svc.GetMessage(context.Background(), acct2.ID, msg.ID); err == nil {
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
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)

	// Missing name
	if _, err := svc.CreateLane(context.Background(), domainccip.Lane{AccountID: acct.ID, SourceChain: "eth", DestChain: "neo"}); err == nil {
		t.Fatalf("expected name required error")
	}
	// Missing source chain
	if _, err := svc.CreateLane(context.Background(), domainccip.Lane{AccountID: acct.ID, Name: "Lane", DestChain: "neo"}); err == nil {
		t.Fatalf("expected source_chain required error")
	}
	// Missing dest chain
	if _, err := svc.CreateLane(context.Background(), domainccip.Lane{AccountID: acct.ID, Name: "Lane", SourceChain: "eth"}); err == nil {
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
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testLaneWallet})
	lane, _ := svc.CreateLane(context.Background(), domainccip.Lane{AccountID: acct.ID, Name: "Lane", SourceChain: "eth", DestChain: "neo", SignerSet: []string{testLaneWallet}})

	// Empty token transfer fields should be filtered
	msg, err := svc.SendMessage(context.Background(), acct.ID, lane.ID, nil, []domainccip.TokenTransfer{
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
