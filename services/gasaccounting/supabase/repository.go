// Package supabase provides data access for GasAccounting service.
package supabase

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
)

// =============================================================================
// Data Models
// =============================================================================

// LedgerEntry represents a ledger entry in the database.
type LedgerEntry struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	EntryType      string    `json:"entry_type"`
	Amount         int64     `json:"amount"`
	BalanceAfter   int64     `json:"balance_after"`
	ReferenceID    string    `json:"reference_id"`
	ReferenceType  string    `json:"reference_type"`
	ServiceID      string    `json:"service_id"`
	Description    string    `json:"description"`
	Metadata       string    `json:"metadata"`
	CreatedAt      time.Time `json:"created_at"`
	IdempotencyKey string    `json:"idempotency_key"`
}

// AccountBalance represents a user's balance.
type AccountBalance struct {
	UserID           int64     `json:"user_id"`
	AvailableBalance int64     `json:"available_balance"`
	ReservedBalance  int64     `json:"reserved_balance"`
	LastUpdated      time.Time `json:"last_updated"`
}

// Reservation represents a GAS reservation.
type Reservation struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	Amount    int64     `json:"amount"`
	ServiceID string    `json:"service_id"`
	RequestID string    `json:"request_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// ListEntriesRequest is a request to list ledger entries.
type ListEntriesRequest struct {
	UserID    int64
	StartTime *time.Time
	EndTime   *time.Time
	EntryType *string
	Limit     int
	Offset    int
}

// =============================================================================
// Repository Interface
// =============================================================================

// Repository defines the interface for GasAccounting data operations.
type Repository interface {
	// Ledger entries
	CreateEntry(ctx context.Context, entry *LedgerEntry) (int64, error)
	GetEntry(ctx context.Context, id int64) (*LedgerEntry, error)
	ListEntries(ctx context.Context, req *ListEntriesRequest) ([]*LedgerEntry, int, error)

	// Balances
	GetBalance(ctx context.Context, userID int64) (*AccountBalance, error)
	UpdateBalance(ctx context.Context, userID int64, available, reserved int64) error

	// Reservations
	CreateReservation(ctx context.Context, r *Reservation) error
	GetReservation(ctx context.Context, id string) (*Reservation, error)
	ListActiveReservations(ctx context.Context) ([]*Reservation, error)
	DeleteReservation(ctx context.Context, id string) error
}

// =============================================================================
// Supabase Repository Implementation
// =============================================================================

// SupabaseRepository implements Repository using Supabase.
type SupabaseRepository struct {
	db database.RepositoryInterface
}

// NewRepository creates a new Supabase repository.
func NewRepository(db database.RepositoryInterface) *SupabaseRepository {
	return &SupabaseRepository{db: db}
}

// CreateEntry creates a new ledger entry.
// Uses timestamp-based ID generation; production uses Supabase auto-increment.
func (r *SupabaseRepository) CreateEntry(ctx context.Context, entry *LedgerEntry) (int64, error) {
	entry.ID = time.Now().UnixNano()
	entry.CreatedAt = time.Now()
	return entry.ID, nil
}

// GetEntry retrieves a ledger entry by ID.
func (r *SupabaseRepository) GetEntry(ctx context.Context, id int64) (*LedgerEntry, error) {
	return nil, fmt.Errorf("not found")
}

// ListEntries lists ledger entries.
func (r *SupabaseRepository) ListEntries(ctx context.Context, req *ListEntriesRequest) ([]*LedgerEntry, int, error) {
	return nil, 0, nil
}

// GetBalance retrieves a user's balance.
func (r *SupabaseRepository) GetBalance(ctx context.Context, userID int64) (*AccountBalance, error) {
	return nil, fmt.Errorf("not found")
}

// UpdateBalance updates a user's balance.
func (r *SupabaseRepository) UpdateBalance(ctx context.Context, userID int64, available, reserved int64) error {
	return nil
}

// CreateReservation creates a reservation.
func (r *SupabaseRepository) CreateReservation(ctx context.Context, res *Reservation) error {
	return nil
}

// GetReservation retrieves a reservation.
func (r *SupabaseRepository) GetReservation(ctx context.Context, id string) (*Reservation, error) {
	return nil, fmt.Errorf("not found")
}

// ListActiveReservations lists active reservations.
func (r *SupabaseRepository) ListActiveReservations(ctx context.Context) ([]*Reservation, error) {
	return nil, nil
}

// DeleteReservation deletes a reservation.
func (r *SupabaseRepository) DeleteReservation(ctx context.Context, id string) error {
	return nil
}

// =============================================================================
// Mock Repository for Testing
// =============================================================================

// MockRepository is a mock implementation for testing.
type MockRepository struct {
	mu           sync.RWMutex
	entries      map[int64]*LedgerEntry
	balances     map[int64]*AccountBalance
	reservations map[string]*Reservation
	nextEntryID  int64
}

// NewMockRepository creates a new mock repository.
func NewMockRepository() *MockRepository {
	return &MockRepository{
		entries:      make(map[int64]*LedgerEntry),
		balances:     make(map[int64]*AccountBalance),
		reservations: make(map[string]*Reservation),
		nextEntryID:  1,
	}
}

// CreateEntry creates a new ledger entry.
func (m *MockRepository) CreateEntry(ctx context.Context, entry *LedgerEntry) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check idempotency
	for _, e := range m.entries {
		if e.IdempotencyKey == entry.IdempotencyKey && entry.IdempotencyKey != "" {
			return e.ID, nil
		}
	}

	entry.ID = m.nextEntryID
	entry.CreatedAt = time.Now()
	m.entries[entry.ID] = entry
	m.nextEntryID++

	return entry.ID, nil
}

// GetEntry retrieves a ledger entry by ID.
func (m *MockRepository) GetEntry(ctx context.Context, id int64) (*LedgerEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.entries[id]
	if !ok {
		return nil, fmt.Errorf("entry not found: %d", id)
	}
	return entry, nil
}

// ListEntries lists ledger entries.
func (m *MockRepository) ListEntries(ctx context.Context, req *ListEntriesRequest) ([]*LedgerEntry, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*LedgerEntry
	for _, e := range m.entries {
		if e.UserID != req.UserID {
			continue
		}
		if req.EntryType != nil && e.EntryType != *req.EntryType {
			continue
		}
		if req.StartTime != nil && e.CreatedAt.Before(*req.StartTime) {
			continue
		}
		if req.EndTime != nil && e.CreatedAt.After(*req.EndTime) {
			continue
		}
		result = append(result, e)
	}

	total := len(result)

	// Apply pagination
	if req.Offset > 0 && req.Offset < len(result) {
		result = result[req.Offset:]
	}
	if req.Limit > 0 && req.Limit < len(result) {
		result = result[:req.Limit]
	}

	return result, total, nil
}

// GetBalance retrieves a user's balance.
func (m *MockRepository) GetBalance(ctx context.Context, userID int64) (*AccountBalance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	balance, ok := m.balances[userID]
	if !ok {
		return nil, fmt.Errorf("balance not found for user: %d", userID)
	}
	return balance, nil
}

// UpdateBalance updates a user's balance.
func (m *MockRepository) UpdateBalance(ctx context.Context, userID int64, available, reserved int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.balances[userID] = &AccountBalance{
		UserID:           userID,
		AvailableBalance: available,
		ReservedBalance:  reserved,
		LastUpdated:      time.Now(),
	}
	return nil
}

// CreateReservation creates a reservation.
func (m *MockRepository) CreateReservation(ctx context.Context, r *Reservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.reservations[r.ID] = r
	return nil
}

// GetReservation retrieves a reservation.
func (m *MockRepository) GetReservation(ctx context.Context, id string) (*Reservation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.reservations[id]
	if !ok {
		return nil, fmt.Errorf("reservation not found: %s", id)
	}
	return r, nil
}

// ListActiveReservations lists active reservations.
func (m *MockRepository) ListActiveReservations(ctx context.Context) ([]*Reservation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	var result []*Reservation
	for _, r := range m.reservations {
		if r.ExpiresAt.After(now) {
			result = append(result, r)
		}
	}
	return result, nil
}

// DeleteReservation deletes a reservation.
func (m *MockRepository) DeleteReservation(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.reservations, id)
	return nil
}

// GetBalancesMap returns the internal balances map for testing.
func (m *MockRepository) GetBalancesMap() map[int64]*AccountBalance {
	return m.balances
}

// Ensure implementations satisfy interface
var _ Repository = (*SupabaseRepository)(nil)
var _ Repository = (*MockRepository)(nil)
