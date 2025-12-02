package dta

import (
	"context"
	"testing"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

const (
	testOrderWalletLower = "0xabc123abc123abc123abc123abc123abc123abcd"
	testOrderWalletUpper = "0xABC123ABC123ABC123ABC123ABC123ABC123ABCD"
)

func TestService_CreateProduct(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_CreateOrder(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_CreateOrderRequiresWallet(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_UpdateProduct(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_UpdateProductOwnership(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_GetProduct(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_GetProductOwnership(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_ListOrders(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_GetOrderOwnership(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_ProductValidation(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_OrderValidation(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
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
	t.Skipf("test requires database; run with integration test suite")
}
