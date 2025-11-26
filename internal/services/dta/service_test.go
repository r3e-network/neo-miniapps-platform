package dta

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domaindta "github.com/R3E-Network/service_layer/internal/app/domain/dta"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
	core "github.com/R3E-Network/service_layer/internal/services/core"
)

const (
	testOrderWalletLower = "0xabc123abc123abc123abc123abc123abc123abcd"
	testOrderWalletUpper = "0xABC123ABC123ABC123ABC123ABC123ABC123ABCD"
)

func TestService_CreateProduct(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	prod, err := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct.ID, Name: "Fund", Symbol: "FND", Type: "open"})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if prod.Status != domaindta.ProductStatusInactive {
		t.Fatalf("expected inactive")
	}
	products, err := svc.ListProducts(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list products: %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("expected one product")
	}
}

func TestService_CreateOrder(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testOrderWalletLower}); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	prod, err := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct.ID, Name: "Fund", Symbol: "FND", Type: "open"})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	order, err := svc.CreateOrder(context.Background(), acct.ID, prod.ID, domaindta.OrderTypeSubscription, "100", testOrderWalletUpper, map[string]string{"tier": "gold"})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if order.Type != domaindta.OrderTypeSubscription {
		t.Fatalf("order type mismatch")
	}
	got, err := svc.GetOrder(context.Background(), acct.ID, order.ID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if got.ID != order.ID {
		t.Fatalf("order mismatch")
	}
}

func TestService_CreateOrderRequiresWallet(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	prod, err := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct.ID, Name: "Fund", Symbol: "FND", Type: "open"})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if _, err := svc.CreateOrder(context.Background(), acct.ID, prod.ID, domaindta.OrderTypeSubscription, "100", "", nil); err == nil {
		t.Fatalf("expected wallet error")
	}
}

func TestService_UpdateProduct(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	prod, _ := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct.ID, Name: "Fund", Symbol: "FND", Type: "open"})

	updated, err := svc.UpdateProduct(context.Background(), domaindta.Product{ID: prod.ID, AccountID: acct.ID, Name: "Updated", Symbol: "UPD", Status: domaindta.ProductStatusActive})
	if err != nil {
		t.Fatalf("update product: %v", err)
	}
	if updated.Name != "Updated" {
		t.Fatalf("expected updated name")
	}
	if updated.Status != domaindta.ProductStatusActive {
		t.Fatalf("expected active status")
	}
}

func TestService_UpdateProductOwnership(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	prod, _ := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct1.ID, Name: "Fund", Symbol: "FND"})

	if _, err := svc.UpdateProduct(context.Background(), domaindta.Product{ID: prod.ID, AccountID: acct2.ID, Name: "Hacked"}); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_GetProduct(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	prod, _ := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct.ID, Name: "Fund", Symbol: "FND"})

	got, err := svc.GetProduct(context.Background(), acct.ID, prod.ID)
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	if got.ID != prod.ID {
		t.Fatalf("product mismatch")
	}
}

func TestService_GetProductOwnership(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	prod, _ := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct1.ID, Name: "Fund", Symbol: "FND"})

	if _, err := svc.GetProduct(context.Background(), acct2.ID, prod.ID); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_ListOrders(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testOrderWalletLower})
	prod, _ := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct.ID, Name: "Fund", Symbol: "FND"})
	svc.CreateOrder(context.Background(), acct.ID, prod.ID, domaindta.OrderTypeSubscription, "100", testOrderWalletLower, nil)

	orders, err := svc.ListOrders(context.Background(), acct.ID, 10)
	if err != nil {
		t.Fatalf("list orders: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 order")
	}
}

func TestService_GetOrderOwnership(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct1.ID, WalletAddress: testOrderWalletLower})
	prod, _ := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct1.ID, Name: "Fund", Symbol: "FND"})
	order, _ := svc.CreateOrder(context.Background(), acct1.ID, prod.ID, domaindta.OrderTypeSubscription, "100", testOrderWalletLower, nil)

	if _, err := svc.GetOrder(context.Background(), acct2.ID, order.ID); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_ProductValidation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)

	// Missing name
	if _, err := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct.ID, Symbol: "FND"}); err == nil {
		t.Fatalf("expected name required error")
	}
	// Missing symbol
	if _, err := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct.ID, Name: "Fund"}); err == nil {
		t.Fatalf("expected symbol required error")
	}
	// Invalid status
	if _, err := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct.ID, Name: "Fund", Symbol: "FND", Status: "invalid"}); err == nil {
		t.Fatalf("expected invalid status error")
	}
}

func TestService_OrderValidation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testOrderWalletLower})
	prod, _ := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct.ID, Name: "Fund", Symbol: "FND"})

	// Invalid order type
	if _, err := svc.CreateOrder(context.Background(), acct.ID, prod.ID, "invalid", "100", testOrderWalletLower, nil); err == nil {
		t.Fatalf("expected invalid type error")
	}
	// Missing amount
	if _, err := svc.CreateOrder(context.Background(), acct.ID, prod.ID, domaindta.OrderTypeSubscription, "", testOrderWalletLower, nil); err == nil {
		t.Fatalf("expected amount required error")
	}
	// Missing wallet
	if _, err := svc.CreateOrder(context.Background(), acct.ID, prod.ID, domaindta.OrderTypeSubscription, "100", "", nil); err == nil {
		t.Fatalf("expected wallet required error")
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
	if m.Name != "dta" {
		t.Fatalf("expected name dta")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "dta" {
		t.Fatalf("expected name dta")
	}
}

func TestService_WithObservationHooks(t *testing.T) {
	svc := New(nil, nil, nil)
	// With nil hooks
	svc.WithObservationHooks(core.ObservationHooks{})
}

func TestService_RedemptionOrder(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testOrderWalletLower})
	prod, _ := svc.CreateProduct(context.Background(), domaindta.Product{AccountID: acct.ID, Name: "Fund", Symbol: "FND"})

	order, err := svc.CreateOrder(context.Background(), acct.ID, prod.ID, domaindta.OrderTypeRedemption, "50", testOrderWalletLower, nil)
	if err != nil {
		t.Fatalf("create redemption order: %v", err)
	}
	if order.Type != domaindta.OrderTypeRedemption {
		t.Fatalf("expected redemption type")
	}
}
