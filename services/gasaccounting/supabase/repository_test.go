// Package supabase provides data access for GasAccounting service.
package supabase

import (
	"context"
	"testing"
	"time"
)

func TestNewMockRepository(t *testing.T) {
	repo := NewMockRepository()
	if repo == nil {
		t.Fatal("NewMockRepository() returned nil")
	}
	if repo.entries == nil {
		t.Error("entries map not initialized")
	}
	if repo.balances == nil {
		t.Error("balances map not initialized")
	}
	if repo.reservations == nil {
		t.Error("reservations map not initialized")
	}
}

func TestMockRepository_CreateEntry(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	entry := &LedgerEntry{
		UserID:         1,
		EntryType:      "deposit",
		Amount:         100000,
		BalanceAfter:   100000,
		IdempotencyKey: "test-key-1",
	}

	id, err := repo.CreateEntry(ctx, entry)
	if err != nil {
		t.Errorf("CreateEntry() error = %v", err)
	}
	if id <= 0 {
		t.Errorf("CreateEntry() returned invalid id = %d", id)
	}

	// Test idempotency
	id2, err := repo.CreateEntry(ctx, entry)
	if err != nil {
		t.Errorf("CreateEntry() idempotent error = %v", err)
	}
	if id2 != id {
		t.Errorf("CreateEntry() idempotent id = %d, want %d", id2, id)
	}
}

func TestMockRepository_CreateEntryNoIdempotencyKey(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	entry1 := &LedgerEntry{UserID: 1, EntryType: "deposit", Amount: 100}
	entry2 := &LedgerEntry{UserID: 1, EntryType: "deposit", Amount: 200}

	id1, _ := repo.CreateEntry(ctx, entry1)
	id2, _ := repo.CreateEntry(ctx, entry2)

	if id1 == id2 {
		t.Error("Entries without idempotency key should have different IDs")
	}
}

func TestMockRepository_GetEntry(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	entry := &LedgerEntry{
		UserID:    1,
		EntryType: "deposit",
		Amount:    100000,
	}

	id, _ := repo.CreateEntry(ctx, entry)

	// Get existing entry
	got, err := repo.GetEntry(ctx, id)
	if err != nil {
		t.Errorf("GetEntry() error = %v", err)
	}
	if got.Amount != entry.Amount {
		t.Errorf("GetEntry() amount = %d, want %d", got.Amount, entry.Amount)
	}

	// Get non-existent entry
	_, err = repo.GetEntry(ctx, 99999)
	if err == nil {
		t.Error("GetEntry() expected error for non-existent entry")
	}
}

func TestMockRepository_ListEntries(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Create entries for user 1
	repo.CreateEntry(ctx, &LedgerEntry{UserID: 1, EntryType: "deposit", Amount: 100})
	repo.CreateEntry(ctx, &LedgerEntry{UserID: 1, EntryType: "consume", Amount: -50})
	repo.CreateEntry(ctx, &LedgerEntry{UserID: 2, EntryType: "deposit", Amount: 200})

	// List for user 1
	entries, total, err := repo.ListEntries(ctx, &ListEntriesRequest{UserID: 1, Limit: 10})
	if err != nil {
		t.Errorf("ListEntries() error = %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("ListEntries() count = %d, want 2", len(entries))
	}
	if total != 2 {
		t.Errorf("ListEntries() total = %d, want 2", total)
	}
}

func TestMockRepository_ListEntriesWithFilters(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	now := time.Now()
	repo.CreateEntry(ctx, &LedgerEntry{UserID: 1, EntryType: "deposit", Amount: 100})

	// Filter by entry type
	entryType := "deposit"
	entries, _, _ := repo.ListEntries(ctx, &ListEntriesRequest{
		UserID:    1,
		EntryType: &entryType,
		Limit:     10,
	})
	if len(entries) != 1 {
		t.Errorf("ListEntries() with type filter count = %d, want 1", len(entries))
	}

	// Filter by time range
	startTime := now.Add(-1 * time.Hour)
	endTime := now.Add(1 * time.Hour)
	entries, _, _ = repo.ListEntries(ctx, &ListEntriesRequest{
		UserID:    1,
		StartTime: &startTime,
		EndTime:   &endTime,
		Limit:     10,
	})
	if len(entries) != 1 {
		t.Errorf("ListEntries() with time filter count = %d, want 1", len(entries))
	}
}

func TestMockRepository_ListEntriesPagination(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Create 5 entries
	for i := 0; i < 5; i++ {
		repo.CreateEntry(ctx, &LedgerEntry{UserID: 1, EntryType: "deposit", Amount: int64(i * 100)})
	}

	// Test limit
	entries, total, _ := repo.ListEntries(ctx, &ListEntriesRequest{UserID: 1, Limit: 2})
	if len(entries) != 2 {
		t.Errorf("ListEntries() with limit count = %d, want 2", len(entries))
	}
	if total != 5 {
		t.Errorf("ListEntries() total = %d, want 5", total)
	}

	// Test offset
	entries, _, _ = repo.ListEntries(ctx, &ListEntriesRequest{UserID: 1, Offset: 3, Limit: 10})
	if len(entries) != 2 {
		t.Errorf("ListEntries() with offset count = %d, want 2", len(entries))
	}
}

func TestMockRepository_GetBalance(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Non-existent user
	_, err := repo.GetBalance(ctx, 999)
	if err == nil {
		t.Error("GetBalance() expected error for non-existent user")
	}

	// Create balance
	repo.UpdateBalance(ctx, 1, 50000, 10000)

	balance, err := repo.GetBalance(ctx, 1)
	if err != nil {
		t.Errorf("GetBalance() error = %v", err)
	}
	if balance.AvailableBalance != 50000 {
		t.Errorf("GetBalance() available = %d, want 50000", balance.AvailableBalance)
	}
	if balance.ReservedBalance != 10000 {
		t.Errorf("GetBalance() reserved = %d, want 10000", balance.ReservedBalance)
	}
}

func TestMockRepository_UpdateBalance(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	err := repo.UpdateBalance(ctx, 1, 100000, 5000)
	if err != nil {
		t.Errorf("UpdateBalance() error = %v", err)
	}

	balance, _ := repo.GetBalance(ctx, 1)
	if balance.AvailableBalance != 100000 {
		t.Errorf("UpdateBalance() available = %d, want 100000", balance.AvailableBalance)
	}
}

func TestMockRepository_CreateReservation(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	res := &Reservation{
		ID:        "res-1",
		UserID:    1,
		Amount:    10000,
		ServiceID: "test",
		RequestID: "req-1",
		ExpiresAt: time.Now().Add(10 * time.Minute),
		CreatedAt: time.Now(),
	}

	err := repo.CreateReservation(ctx, res)
	if err != nil {
		t.Errorf("CreateReservation() error = %v", err)
	}
}

func TestMockRepository_GetReservation(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Non-existent
	_, err := repo.GetReservation(ctx, "non-existent")
	if err == nil {
		t.Error("GetReservation() expected error for non-existent")
	}

	// Create and get
	res := &Reservation{ID: "res-1", UserID: 1, Amount: 10000}
	repo.CreateReservation(ctx, res)

	got, err := repo.GetReservation(ctx, "res-1")
	if err != nil {
		t.Errorf("GetReservation() error = %v", err)
	}
	if got.Amount != 10000 {
		t.Errorf("GetReservation() amount = %d, want 10000", got.Amount)
	}
}

func TestMockRepository_ListActiveReservations(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Create active and expired reservations
	repo.CreateReservation(ctx, &Reservation{
		ID:        "res-active",
		UserID:    1,
		Amount:    10000,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	})
	repo.CreateReservation(ctx, &Reservation{
		ID:        "res-expired",
		UserID:    1,
		Amount:    5000,
		ExpiresAt: time.Now().Add(-10 * time.Minute),
	})

	active, err := repo.ListActiveReservations(ctx)
	if err != nil {
		t.Errorf("ListActiveReservations() error = %v", err)
	}
	if len(active) != 1 {
		t.Errorf("ListActiveReservations() count = %d, want 1", len(active))
	}
}

func TestMockRepository_DeleteReservation(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	repo.CreateReservation(ctx, &Reservation{ID: "res-1", UserID: 1, Amount: 10000})

	err := repo.DeleteReservation(ctx, "res-1")
	if err != nil {
		t.Errorf("DeleteReservation() error = %v", err)
	}

	_, err = repo.GetReservation(ctx, "res-1")
	if err == nil {
		t.Error("DeleteReservation() reservation still exists")
	}
}

func TestMockRepository_GetBalancesMap(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	repo.UpdateBalance(ctx, 1, 100000, 0)

	balances := repo.GetBalancesMap()
	if balances == nil {
		t.Fatal("GetBalancesMap() returned nil")
	}
	if _, ok := balances[1]; !ok {
		t.Error("GetBalancesMap() missing user 1")
	}
}

func TestNewRepository(t *testing.T) {
	repo := NewRepository(nil)
	if repo == nil {
		t.Fatal("NewRepository() returned nil")
	}
}

func TestSupabaseRepository_Stubs(t *testing.T) {
	repo := NewRepository(nil)
	ctx := context.Background()

	// Test stub implementations
	_, err := repo.CreateEntry(ctx, &LedgerEntry{})
	if err != nil {
		t.Errorf("CreateEntry() stub error = %v", err)
	}

	_, err = repo.GetEntry(ctx, 1)
	if err == nil {
		t.Error("GetEntry() stub should return error")
	}

	_, _, err = repo.ListEntries(ctx, &ListEntriesRequest{})
	if err != nil {
		t.Errorf("ListEntries() stub error = %v", err)
	}

	_, err = repo.GetBalance(ctx, 1)
	if err == nil {
		t.Error("GetBalance() stub should return error")
	}

	err = repo.UpdateBalance(ctx, 1, 0, 0)
	if err != nil {
		t.Errorf("UpdateBalance() stub error = %v", err)
	}

	err = repo.CreateReservation(ctx, &Reservation{})
	if err != nil {
		t.Errorf("CreateReservation() stub error = %v", err)
	}

	_, err = repo.GetReservation(ctx, "id")
	if err == nil {
		t.Error("GetReservation() stub should return error")
	}

	_, err = repo.ListActiveReservations(ctx)
	if err != nil {
		t.Errorf("ListActiveReservations() stub error = %v", err)
	}

	err = repo.DeleteReservation(ctx, "id")
	if err != nil {
		t.Errorf("DeleteReservation() stub error = %v", err)
	}
}
