// Package integration provides integration tests for the service layer.
package integration

import (
	"context"
	"testing"
	"time"

	gasaccounting "github.com/R3E-Network/service_layer/services/gasaccounting/marble"
	"github.com/R3E-Network/service_layer/services/gasaccounting/supabase"
)

// =============================================================================
// GasAccounting Service Tests
// =============================================================================

func TestGasAccountingDeposit(t *testing.T) {
	repo := supabase.NewMockRepository()
	svc, err := gasaccounting.New(gasaccounting.Config{
		Repository: repo,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()

	// Test deposit
	resp, err := svc.Deposit(ctx, &gasaccounting.DepositRequest{
		UserID: 1,
		Amount: 100000,
		TxHash: "0xabc123",
	})
	if err != nil {
		t.Fatalf("Deposit failed: %v", err)
	}

	if resp.NewBalance != 100000 {
		t.Errorf("Expected balance 100000, got %d", resp.NewBalance)
	}
}

func TestGasAccountingConsume(t *testing.T) {
	repo := supabase.NewMockRepository()

	// Pre-populate balance
	ctx := context.Background()
	repo.UpdateBalance(ctx, 1, 100000, 0)

	svc, err := gasaccounting.New(gasaccounting.Config{
		Repository: repo,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Test consume
	resp, err := svc.Consume(ctx, &gasaccounting.ConsumeRequest{
		UserID:      1,
		Amount:      5000,
		ServiceID:   "neorand",
		RequestID:   "req-001",
		Description: "VRF request",
	})
	if err != nil {
		t.Fatalf("Consume failed: %v", err)
	}

	if resp.NewBalance != 95000 {
		t.Errorf("Expected balance 95000, got %d", resp.NewBalance)
	}
}

func TestGasAccountingInsufficientBalance(t *testing.T) {
	repo := supabase.NewMockRepository()

	// Pre-populate with small balance
	ctx := context.Background()
	repo.UpdateBalance(ctx, 1, 1000, 0)

	svc, err := gasaccounting.New(gasaccounting.Config{
		Repository: repo,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Try to consume more than available
	_, err = svc.Consume(ctx, &gasaccounting.ConsumeRequest{
		UserID:    1,
		Amount:    5000,
		ServiceID: "neorand",
		RequestID: "req-001",
	})
	if err == nil {
		t.Error("Expected insufficient balance error")
	}
}

func TestGasAccountingReservation(t *testing.T) {
	repo := supabase.NewMockRepository()

	// Pre-populate balance
	ctx := context.Background()
	repo.UpdateBalance(ctx, 1, 100000, 0)

	svc, err := gasaccounting.New(gasaccounting.Config{
		Repository: repo,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Reserve
	reserveResp, err := svc.Reserve(ctx, &gasaccounting.ReserveRequest{
		UserID:    1,
		Amount:    10000,
		ServiceID: "neovault",
		RequestID: "mix-001",
		TTL:       5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}

	if reserveResp.Amount != 10000 {
		t.Errorf("Expected reserved amount 10000, got %d", reserveResp.Amount)
	}

	if reserveResp.NewAvailable != 90000 {
		t.Errorf("Expected available 90000, got %d", reserveResp.NewAvailable)
	}

	// Release with consume
	releaseResp, err := svc.Release(ctx, &gasaccounting.ReleaseRequest{
		ReservationID: reserveResp.ReservationID,
		Consume:       true,
		ActualAmount:  8000,
	})
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	if releaseResp.Consumed != 8000 {
		t.Errorf("Expected consumed 8000, got %d", releaseResp.Consumed)
	}

	if releaseResp.Released != 2000 {
		t.Errorf("Expected released 2000, got %d", releaseResp.Released)
	}
}

func TestGasAccountingReservationRelease(t *testing.T) {
	repo := supabase.NewMockRepository()

	// Pre-populate balance
	ctx := context.Background()
	repo.UpdateBalance(ctx, 1, 100000, 0)

	svc, err := gasaccounting.New(gasaccounting.Config{
		Repository: repo,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Reserve
	reserveResp, err := svc.Reserve(ctx, &gasaccounting.ReserveRequest{
		UserID:    1,
		Amount:    10000,
		ServiceID: "neovault",
		RequestID: "mix-002",
		TTL:       5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}

	// Release without consume (operation failed)
	releaseResp, err := svc.Release(ctx, &gasaccounting.ReleaseRequest{
		ReservationID: reserveResp.ReservationID,
		Consume:       false,
	})
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	if releaseResp.Released != 10000 {
		t.Errorf("Expected released 10000, got %d", releaseResp.Released)
	}

	if releaseResp.Consumed != 0 {
		t.Errorf("Expected consumed 0, got %d", releaseResp.Consumed)
	}

	// Balance should be restored
	if releaseResp.NewAvailable != 100000 {
		t.Errorf("Expected available 100000, got %d", releaseResp.NewAvailable)
	}
}

func TestGasAccountingGetBalance(t *testing.T) {
	repo := supabase.NewMockRepository()

	ctx := context.Background()
	repo.UpdateBalance(ctx, 1, 50000, 10000)

	svc, err := gasaccounting.New(gasaccounting.Config{
		Repository: repo,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	resp, err := svc.GetBalance(ctx, 1)
	if err != nil {
		t.Fatalf("GetBalance failed: %v", err)
	}

	if resp.AvailableBalance != 50000 {
		t.Errorf("Expected available 50000, got %d", resp.AvailableBalance)
	}

	if resp.ReservedBalance != 10000 {
		t.Errorf("Expected reserved 10000, got %d", resp.ReservedBalance)
	}

	if resp.TotalBalance != 60000 {
		t.Errorf("Expected total 60000, got %d", resp.TotalBalance)
	}
}

func TestGasAccountingHistory(t *testing.T) {
	repo := supabase.NewMockRepository()
	svc, err := gasaccounting.New(gasaccounting.Config{
		Repository: repo,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()

	// Create some entries
	svc.Deposit(ctx, &gasaccounting.DepositRequest{
		UserID: 1,
		Amount: 100000,
		TxHash: "0xabc123",
	})

	repo.UpdateBalance(ctx, 1, 100000, 0)

	svc.Consume(ctx, &gasaccounting.ConsumeRequest{
		UserID:    1,
		Amount:    5000,
		ServiceID: "neorand",
		RequestID: "req-001",
	})

	// Get history
	resp, err := svc.GetHistory(ctx, &gasaccounting.LedgerHistoryRequest{
		UserID: 1,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(resp.Entries) < 2 {
		t.Errorf("Expected at least 2 entries, got %d", len(resp.Entries))
	}
}

// =============================================================================
// Common Service Infrastructure Tests
// =============================================================================

func TestServiceMetrics(t *testing.T) {
	// Import would be: commonservice "github.com/R3E-Network/service_layer/services/common/service"
	// Test metrics collection
	t.Skip("Metrics test requires service instantiation")
}

func TestServiceProbes(t *testing.T) {
	// Test Kubernetes probes
	t.Skip("Probes test requires HTTP server")
}

func TestDeepHealthCheck(t *testing.T) {
	// Test deep health check framework
	t.Skip("Health check test requires service dependencies")
}
