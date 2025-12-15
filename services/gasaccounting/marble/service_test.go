// Package gasaccounting provides GAS ledger and accounting service.
package gasaccounting

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/services/gasaccounting/supabase"
)

func newTestService(t *testing.T) (*Service, *supabase.MockRepository) {
	repo := supabase.NewMockRepository()
	svc, err := New(Config{Repository: repo})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	return svc, repo
}

func TestNew(t *testing.T) {
	repo := supabase.NewMockRepository()
	svc, err := New(Config{Repository: repo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if svc == nil {
		t.Fatal("New() returned nil service")
	}
}

func TestDeposit(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *DepositRequest
		wantErr bool
	}{
		{
			name: "valid deposit",
			req: &DepositRequest{
				UserID: 1,
				Amount: 100000,
				TxHash: "0xabc123",
			},
			wantErr: false,
		},
		{
			name: "invalid user_id",
			req: &DepositRequest{
				UserID: 0,
				Amount: 100000,
				TxHash: "0xabc123",
			},
			wantErr: true,
		},
		{
			name: "negative user_id",
			req: &DepositRequest{
				UserID: -1,
				Amount: 100000,
				TxHash: "0xabc123",
			},
			wantErr: true,
		},
		{
			name: "zero amount",
			req: &DepositRequest{
				UserID: 1,
				Amount: 0,
				TxHash: "0xabc123",
			},
			wantErr: true,
		},
		{
			name: "negative amount",
			req: &DepositRequest{
				UserID: 1,
				Amount: -100,
				TxHash: "0xabc123",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.Deposit(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Deposit() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && resp == nil {
				t.Error("Deposit() returned nil response")
			}
		})
	}
}

func TestConsume(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	// Pre-populate balance
	repo.UpdateBalance(ctx, 1, 100000, 0)

	tests := []struct {
		name    string
		req     *ConsumeRequest
		wantErr bool
	}{
		{
			name: "valid consume",
			req: &ConsumeRequest{
				UserID:      1,
				Amount:      5000,
				ServiceID:   "neorand",
				RequestID:   "req-001",
				Description: "VRF request",
			},
			wantErr: false,
		},
		{
			name: "invalid user_id",
			req: &ConsumeRequest{
				UserID:    0,
				Amount:    5000,
				ServiceID: "neorand",
				RequestID: "req-002",
			},
			wantErr: true,
		},
		{
			name: "zero amount",
			req: &ConsumeRequest{
				UserID:    1,
				Amount:    0,
				ServiceID: "neorand",
				RequestID: "req-003",
			},
			wantErr: true,
		},
		{
			name: "insufficient balance",
			req: &ConsumeRequest{
				UserID:    1,
				Amount:    999999999,
				ServiceID: "neorand",
				RequestID: "req-004",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.Consume(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Consume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && resp == nil {
				t.Error("Consume() returned nil response")
			}
		})
	}
}

func TestGetBalance(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	// Test non-existent user (should return zero balance)
	resp, err := svc.GetBalance(ctx, 999)
	if err != nil {
		t.Errorf("GetBalance() error = %v", err)
	}
	if resp.AvailableBalance != 0 {
		t.Errorf("Expected 0 balance for non-existent user, got %d", resp.AvailableBalance)
	}

	// Pre-populate and test
	repo.UpdateBalance(ctx, 1, 50000, 10000)
	resp, err = svc.GetBalance(ctx, 1)
	if err != nil {
		t.Errorf("GetBalance() error = %v", err)
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

func TestReserve(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	// Pre-populate balance
	repo.UpdateBalance(ctx, 1, 100000, 0)

	tests := []struct {
		name    string
		req     *ReserveRequest
		wantErr bool
	}{
		{
			name: "valid reserve",
			req: &ReserveRequest{
				UserID:    1,
				Amount:    10000,
				ServiceID: "neovault",
				RequestID: "mix-001",
				TTL:       5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "reserve with default TTL",
			req: &ReserveRequest{
				UserID:    1,
				Amount:    5000,
				ServiceID: "neovault",
				RequestID: "mix-002",
				TTL:       0, // Should use default
			},
			wantErr: false,
		},
		{
			name: "invalid user_id",
			req: &ReserveRequest{
				UserID:    0,
				Amount:    10000,
				ServiceID: "neovault",
				RequestID: "mix-003",
			},
			wantErr: true,
		},
		{
			name: "zero amount",
			req: &ReserveRequest{
				UserID:    1,
				Amount:    0,
				ServiceID: "neovault",
				RequestID: "mix-004",
			},
			wantErr: true,
		},
		{
			name: "insufficient balance",
			req: &ReserveRequest{
				UserID:    1,
				Amount:    999999999,
				ServiceID: "neovault",
				RequestID: "mix-005",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.Reserve(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Reserve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if resp == nil {
					t.Error("Reserve() returned nil response")
				} else if resp.ReservationID == "" {
					t.Error("Reserve() returned empty reservation ID")
				}
			}
		})
	}
}

func TestRelease(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	// Pre-populate balance
	repo.UpdateBalance(ctx, 1, 100000, 0)

	// Create a reservation first
	reserveResp, err := svc.Reserve(ctx, &ReserveRequest{
		UserID:    1,
		Amount:    10000,
		ServiceID: "neovault",
		RequestID: "mix-001",
		TTL:       5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}

	// Test release without consume
	releaseResp, err := svc.Release(ctx, &ReleaseRequest{
		ReservationID: reserveResp.ReservationID,
		Consume:       false,
	})
	if err != nil {
		t.Errorf("Release() error = %v", err)
	}
	if releaseResp.Released != 10000 {
		t.Errorf("Expected released 10000, got %d", releaseResp.Released)
	}
	if releaseResp.Consumed != 0 {
		t.Errorf("Expected consumed 0, got %d", releaseResp.Consumed)
	}

	// Test release with consume
	repo.UpdateBalance(ctx, 1, 100000, 0)
	reserveResp2, _ := svc.Reserve(ctx, &ReserveRequest{
		UserID:    1,
		Amount:    10000,
		ServiceID: "neovault",
		RequestID: "mix-002",
		TTL:       5 * time.Minute,
	})

	releaseResp2, err := svc.Release(ctx, &ReleaseRequest{
		ReservationID: reserveResp2.ReservationID,
		Consume:       true,
		ActualAmount:  8000,
	})
	if err != nil {
		t.Errorf("Release() error = %v", err)
	}
	if releaseResp2.Consumed != 8000 {
		t.Errorf("Expected consumed 8000, got %d", releaseResp2.Consumed)
	}
	if releaseResp2.Released != 2000 {
		t.Errorf("Expected released 2000, got %d", releaseResp2.Released)
	}

	// Test release non-existent reservation
	_, err = svc.Release(ctx, &ReleaseRequest{
		ReservationID: "non-existent",
		Consume:       false,
	})
	if err == nil {
		t.Error("Expected error for non-existent reservation")
	}
}

func TestReleaseConsumeFullAmount(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	repo.UpdateBalance(ctx, 1, 100000, 0)

	reserveResp, _ := svc.Reserve(ctx, &ReserveRequest{
		UserID:    1,
		Amount:    10000,
		ServiceID: "neovault",
		RequestID: "mix-003",
		TTL:       5 * time.Minute,
	})

	// Consume with zero actual amount (should use full reservation)
	releaseResp, err := svc.Release(ctx, &ReleaseRequest{
		ReservationID: reserveResp.ReservationID,
		Consume:       true,
		ActualAmount:  0,
	})
	if err != nil {
		t.Errorf("Release() error = %v", err)
	}
	if releaseResp.Consumed != 10000 {
		t.Errorf("Expected consumed 10000, got %d", releaseResp.Consumed)
	}
}

func TestReleaseConsumeOverAmount(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	repo.UpdateBalance(ctx, 1, 100000, 0)

	reserveResp, _ := svc.Reserve(ctx, &ReserveRequest{
		UserID:    1,
		Amount:    10000,
		ServiceID: "neovault",
		RequestID: "mix-004",
		TTL:       5 * time.Minute,
	})

	// Consume with amount > reservation (should cap at reservation)
	releaseResp, err := svc.Release(ctx, &ReleaseRequest{
		ReservationID: reserveResp.ReservationID,
		Consume:       true,
		ActualAmount:  99999,
	})
	if err != nil {
		t.Errorf("Release() error = %v", err)
	}
	if releaseResp.Consumed != 10000 {
		t.Errorf("Expected consumed 10000 (capped), got %d", releaseResp.Consumed)
	}
}

func TestGetHistory(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	// Create some entries
	svc.Deposit(ctx, &DepositRequest{
		UserID: 1,
		Amount: 100000,
		TxHash: "0xabc123",
	})

	repo.UpdateBalance(ctx, 1, 100000, 0)

	svc.Consume(ctx, &ConsumeRequest{
		UserID:    1,
		Amount:    5000,
		ServiceID: "neorand",
		RequestID: "req-001",
	})

	// Get history
	resp, err := svc.GetHistory(ctx, &LedgerHistoryRequest{
		UserID: 1,
		Limit:  10,
	})
	if err != nil {
		t.Errorf("GetHistory() error = %v", err)
	}
	if len(resp.Entries) < 2 {
		t.Errorf("Expected at least 2 entries, got %d", len(resp.Entries))
	}
}

func TestGetHistoryWithFilters(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// Get history with entry type filter
	entryType := EntryTypeDeposit
	resp, err := svc.GetHistory(ctx, &LedgerHistoryRequest{
		UserID:    1,
		EntryType: &entryType,
		Limit:     10,
		Offset:    0,
	})
	if err != nil {
		t.Errorf("GetHistory() error = %v", err)
	}
	if resp == nil {
		t.Error("GetHistory() returned nil response")
	}
}

func TestStatistics(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	// Make some operations
	svc.Deposit(ctx, &DepositRequest{
		UserID: 1,
		Amount: 100000,
		TxHash: "0xabc123",
	})

	repo.UpdateBalance(ctx, 1, 100000, 0)

	svc.Consume(ctx, &ConsumeRequest{
		UserID:    1,
		Amount:    5000,
		ServiceID: "neorand",
		RequestID: "req-001",
	})

	stats := svc.statistics()
	if stats == nil {
		t.Fatal("statistics() returned nil")
	}

	if stats["total_deposits"].(int64) != 100000 {
		t.Errorf("Expected total_deposits 100000, got %v", stats["total_deposits"])
	}
	if stats["total_consumed"].(int64) != 5000 {
		t.Errorf("Expected total_consumed 5000, got %v", stats["total_consumed"])
	}
}

func TestCleanupExpiredReservations(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	repo.UpdateBalance(ctx, 1, 100000, 0)

	// Create a reservation with very short TTL
	svc.Reserve(ctx, &ReserveRequest{
		UserID:    1,
		Amount:    10000,
		ServiceID: "neovault",
		RequestID: "mix-expire",
		TTL:       1 * time.Millisecond,
	})

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Run cleanup
	err := svc.cleanupExpiredReservations(ctx)
	if err != nil {
		t.Errorf("cleanupExpiredReservations() error = %v", err)
	}

	// Verify reservation was cleaned up
	svc.mu.RLock()
	count := len(svc.reservations)
	svc.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 reservations after cleanup, got %d", count)
	}
}

func TestHydrate(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// Test hydrate with nil repo
	svc.repo = nil
	err := svc.hydrate(ctx)
	if err != nil {
		t.Errorf("hydrate() with nil repo error = %v", err)
	}
}

func TestLogger(t *testing.T) {
	svc, _ := newTestService(t)
	logger := svc.Logger()
	if logger == nil {
		t.Error("Logger() returned nil")
	}
}

func TestDepositMultiple(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// Multiple deposits should accumulate
	svc.Deposit(ctx, &DepositRequest{UserID: 1, Amount: 50000, TxHash: "0x1"})
	svc.Deposit(ctx, &DepositRequest{UserID: 1, Amount: 30000, TxHash: "0x2"})

	resp, _ := svc.GetBalance(ctx, 1)
	if resp.AvailableBalance != 80000 {
		t.Errorf("Expected 80000, got %d", resp.AvailableBalance)
	}
}

func TestConsumeNoBalance(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// User with no balance
	_, err := svc.Consume(ctx, &ConsumeRequest{
		UserID:    999,
		Amount:    100,
		ServiceID: "test",
		RequestID: "req-999",
	})
	if err == nil {
		t.Error("Expected error for user with no balance")
	}
}

func TestReserveNoBalance(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// User with no balance
	_, err := svc.Reserve(ctx, &ReserveRequest{
		UserID:    999,
		Amount:    100,
		ServiceID: "test",
		RequestID: "req-999",
	})
	if err == nil {
		t.Error("Expected error for user with no balance")
	}
}

func TestReleaseGetBalanceError(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	repo.UpdateBalance(ctx, 1, 100000, 0)

	// Create reservation
	reserveResp, _ := svc.Reserve(ctx, &ReserveRequest{
		UserID:    1,
		Amount:    10000,
		ServiceID: "test",
		RequestID: "req-err",
	})

	// Clear balance to simulate error path
	delete(repo.GetBalancesMap(), 1)

	// Release should fail when getting balance
	_, err := svc.Release(ctx, &ReleaseRequest{
		ReservationID: reserveResp.ReservationID,
		Consume:       true,
	})
	if err == nil {
		t.Error("Expected error when balance not found")
	}
}

func TestHydrateWithReservations(t *testing.T) {
	repo := supabase.NewMockRepository()
	ctx := context.Background()

	// Pre-create a reservation in repo
	repo.CreateReservation(ctx, &supabase.Reservation{
		ID:        "test-res-1",
		UserID:    1,
		Amount:    5000,
		ServiceID: "test",
		RequestID: "req-hydrate",
		ExpiresAt: time.Now().Add(10 * time.Minute),
		CreatedAt: time.Now(),
	})

	svc, err := New(Config{Repository: repo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Hydrate should load the reservation
	err = svc.hydrate(ctx)
	if err != nil {
		t.Errorf("hydrate() error = %v", err)
	}

	svc.mu.RLock()
	count := len(svc.reservations)
	svc.mu.RUnlock()

	if count != 1 {
		t.Errorf("Expected 1 reservation after hydrate, got %d", count)
	}
}
