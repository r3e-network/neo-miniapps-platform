// Package gasaccounting provides GAS ledger and accounting service.
package gasaccounting

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/logging"
	"github.com/R3E-Network/service_layer/internal/marble"
	commonservice "github.com/R3E-Network/service_layer/services/common/service"
	"github.com/R3E-Network/service_layer/services/gasaccounting/supabase"
)

// =============================================================================
// Service Definition
// =============================================================================

// Service implements the GasAccounting service.
type Service struct {
	*commonservice.BaseService
	mu sync.RWMutex

	// Repository
	repo supabase.Repository

	// Reservations (in-memory cache with DB backing)
	reservations map[string]*Reservation

	// Metrics
	totalDeposits    int64
	totalWithdrawals int64
	totalConsumed    int64
	startTime        time.Time
}

// Reservation represents a GAS reservation.
type Reservation struct {
	ID        string
	UserID    int64
	Amount    int64
	ServiceID string
	RequestID string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Config holds GasAccounting service configuration.
type Config struct {
	Marble     *marble.Marble
	DB         database.RepositoryInterface
	Repository supabase.Repository
}

// =============================================================================
// Constructor
// =============================================================================

// New creates a new GasAccounting service.
func New(cfg Config) (*Service, error) {
	base := commonservice.NewBase(&commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	s := &Service{
		BaseService:  base,
		repo:         cfg.Repository,
		reservations: make(map[string]*Reservation),
		startTime:    time.Now(),
	}

	// Set up hydration
	s.WithHydrate(s.hydrate)

	// Set up statistics provider
	s.WithStats(s.statistics)

	// Add reservation cleanup worker
	s.AddTickerWorker(1*time.Minute, s.cleanupExpiredReservations)

	return s, nil
}

// =============================================================================
// Lifecycle
// =============================================================================

// hydrate loads state from the database.
func (s *Service) hydrate(ctx context.Context) error {
	s.Logger().Info(ctx, "Hydrating GasAccounting state...", nil)

	if s.repo == nil {
		return nil
	}

	// Load active reservations
	reservations, err := s.repo.ListActiveReservations(ctx)
	if err != nil {
		s.Logger().Warn(ctx, "Failed to load reservations", map[string]interface{}{"error": err.Error()})
		return nil
	}

	s.mu.Lock()
	for _, r := range reservations {
		s.reservations[r.ID] = &Reservation{
			ID:        r.ID,
			UserID:    r.UserID,
			Amount:    r.Amount,
			ServiceID: r.ServiceID,
			RequestID: r.RequestID,
			ExpiresAt: r.ExpiresAt,
			CreatedAt: r.CreatedAt,
		}
	}
	s.mu.Unlock()

	s.Logger().Info(ctx, "Loaded reservations", map[string]interface{}{"count": len(reservations)})
	return nil
}

// statistics returns service statistics.
func (s *Service) statistics() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]any{
		"total_deposits":      s.totalDeposits,
		"total_withdrawals":   s.totalWithdrawals,
		"total_consumed":      s.totalConsumed,
		"active_reservations": len(s.reservations),
		"uptime":              time.Since(s.startTime).String(),
	}
}

// =============================================================================
// Core Operations
// =============================================================================

// Deposit records a GAS deposit.
func (s *Service) Deposit(ctx context.Context, req *DepositRequest) (*DepositResponse, error) {
	if req.UserID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}
	if req.Amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	// Get current balance
	balance, err := s.repo.GetBalance(ctx, req.UserID)
	if err != nil {
		balance = &supabase.AccountBalance{UserID: req.UserID}
	}

	newBalance := balance.AvailableBalance + req.Amount

	// Create ledger entry
	entry := &supabase.LedgerEntry{
		UserID:         req.UserID,
		EntryType:      string(EntryTypeDeposit),
		Amount:         req.Amount,
		BalanceAfter:   newBalance,
		ReferenceID:    req.TxHash,
		ReferenceType:  "tx",
		Description:    "GAS deposit",
		IdempotencyKey: fmt.Sprintf("deposit:%s", req.TxHash),
	}

	entryID, err := s.repo.CreateEntry(ctx, entry)
	if err != nil {
		return nil, fmt.Errorf("create entry: %w", err)
	}

	// Update balance
	if err := s.repo.UpdateBalance(ctx, req.UserID, newBalance, balance.ReservedBalance); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	s.mu.Lock()
	s.totalDeposits += req.Amount
	s.mu.Unlock()

	return &DepositResponse{
		EntryID:     entryID,
		NewBalance:  newBalance,
		DepositedAt: time.Now(),
	}, nil
}

// GetBalance returns a user's balance.
func (s *Service) GetBalance(ctx context.Context, userID int64) (*BalanceResponse, error) {
	balance, err := s.repo.GetBalance(ctx, userID)
	if err != nil {
		return &BalanceResponse{
			UserID:           userID,
			AvailableBalance: 0,
			ReservedBalance:  0,
			TotalBalance:     0,
			AsOf:             time.Now(),
		}, nil
	}

	return &BalanceResponse{
		UserID:           balance.UserID,
		AvailableBalance: balance.AvailableBalance,
		ReservedBalance:  balance.ReservedBalance,
		TotalBalance:     balance.AvailableBalance + balance.ReservedBalance,
		AsOf:             time.Now(),
	}, nil
}

// Consume deducts GAS for a service operation.
func (s *Service) Consume(ctx context.Context, req *ConsumeRequest) (*ConsumeResponse, error) {
	if req.UserID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}
	if req.Amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	// Get current balance
	balance, err := s.repo.GetBalance(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}

	if balance.AvailableBalance < req.Amount {
		return nil, fmt.Errorf("insufficient balance: have %d, need %d", balance.AvailableBalance, req.Amount)
	}

	newBalance := balance.AvailableBalance - req.Amount

	// Create ledger entry
	entry := &supabase.LedgerEntry{
		UserID:         req.UserID,
		EntryType:      string(EntryTypeConsume),
		Amount:         -req.Amount, // Negative for debit
		BalanceAfter:   newBalance,
		ReferenceID:    req.RequestID,
		ReferenceType:  "request",
		ServiceID:      req.ServiceID,
		Description:    req.Description,
		IdempotencyKey: fmt.Sprintf("consume:%s:%s", req.ServiceID, req.RequestID),
	}

	entryID, err := s.repo.CreateEntry(ctx, entry)
	if err != nil {
		return nil, fmt.Errorf("create entry: %w", err)
	}

	// Update balance
	if err := s.repo.UpdateBalance(ctx, req.UserID, newBalance, balance.ReservedBalance); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	s.mu.Lock()
	s.totalConsumed += req.Amount
	s.mu.Unlock()

	return &ConsumeResponse{
		EntryID:    entryID,
		NewBalance: newBalance,
		ConsumedAt: time.Now(),
	}, nil
}

// Reserve reserves GAS for a pending operation.
func (s *Service) Reserve(ctx context.Context, req *ReserveRequest) (*ReserveResponse, error) {
	if req.UserID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}
	if req.Amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	// Get current balance
	balance, err := s.repo.GetBalance(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}

	if balance.AvailableBalance < req.Amount {
		return nil, fmt.Errorf("insufficient balance: have %d, need %d", balance.AvailableBalance, req.Amount)
	}

	// Create reservation
	reservationID := fmt.Sprintf("res:%s:%s:%d", req.ServiceID, req.RequestID, time.Now().UnixNano())
	ttl := req.TTL
	if ttl == 0 {
		ttl = 10 * time.Minute
	}
	expiresAt := time.Now().Add(ttl)

	reservation := &Reservation{
		ID:        reservationID,
		UserID:    req.UserID,
		Amount:    req.Amount,
		ServiceID: req.ServiceID,
		RequestID: req.RequestID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	// Update balances
	newAvailable := balance.AvailableBalance - req.Amount
	newReserved := balance.ReservedBalance + req.Amount

	if err := s.repo.UpdateBalance(ctx, req.UserID, newAvailable, newReserved); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	// Store reservation
	s.mu.Lock()
	s.reservations[reservationID] = reservation
	s.mu.Unlock()

	if err := s.repo.CreateReservation(ctx, &supabase.Reservation{
		ID:        reservationID,
		UserID:    req.UserID,
		Amount:    req.Amount,
		ServiceID: req.ServiceID,
		RequestID: req.RequestID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}); err != nil {
		s.Logger().Warn(ctx, "Failed to persist reservation", map[string]interface{}{"error": err.Error()})
	}

	return &ReserveResponse{
		ReservationID: reservationID,
		Amount:        req.Amount,
		ExpiresAt:     expiresAt,
		NewAvailable:  newAvailable,
	}, nil
}

// Release releases or consumes a reservation.
func (s *Service) Release(ctx context.Context, req *ReleaseRequest) (*ReleaseResponse, error) {
	s.mu.Lock()
	reservation, ok := s.reservations[req.ReservationID]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("reservation not found: %s", req.ReservationID)
	}
	delete(s.reservations, req.ReservationID)
	s.mu.Unlock()

	// Get current balance
	balance, err := s.repo.GetBalance(ctx, reservation.UserID)
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}

	var released, consumed int64
	var newAvailable, newReserved int64

	if req.Consume {
		// Consume the reservation
		actualAmount := req.ActualAmount
		if actualAmount <= 0 {
			actualAmount = reservation.Amount
		}
		if actualAmount > reservation.Amount {
			actualAmount = reservation.Amount
		}

		consumed = actualAmount
		released = reservation.Amount - actualAmount
		newAvailable = balance.AvailableBalance + released
		newReserved = balance.ReservedBalance - reservation.Amount

		// Create consume entry
		entry := &supabase.LedgerEntry{
			UserID:         reservation.UserID,
			EntryType:      string(EntryTypeConsume),
			Amount:         -consumed,
			BalanceAfter:   newAvailable,
			ReferenceID:    reservation.RequestID,
			ReferenceType:  "reservation",
			ServiceID:      reservation.ServiceID,
			Description:    "Reservation consumed",
			IdempotencyKey: fmt.Sprintf("consume:res:%s", req.ReservationID),
		}
		s.repo.CreateEntry(ctx, entry)

		s.mu.Lock()
		s.totalConsumed += consumed
		s.mu.Unlock()
	} else {
		// Release the reservation
		released = reservation.Amount
		newAvailable = balance.AvailableBalance + released
		newReserved = balance.ReservedBalance - reservation.Amount

		// Create release entry
		entry := &supabase.LedgerEntry{
			UserID:         reservation.UserID,
			EntryType:      string(EntryTypeRelease),
			Amount:         released,
			BalanceAfter:   newAvailable,
			ReferenceID:    reservation.RequestID,
			ReferenceType:  "reservation",
			ServiceID:      reservation.ServiceID,
			Description:    "Reservation released",
			IdempotencyKey: fmt.Sprintf("release:res:%s", req.ReservationID),
		}
		s.repo.CreateEntry(ctx, entry)
	}

	// Update balance
	if err := s.repo.UpdateBalance(ctx, reservation.UserID, newAvailable, newReserved); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	// Delete reservation from DB
	s.repo.DeleteReservation(ctx, req.ReservationID)

	return &ReleaseResponse{
		Released:     released,
		Consumed:     consumed,
		NewAvailable: newAvailable,
	}, nil
}

// GetHistory returns ledger history for a user.
func (s *Service) GetHistory(ctx context.Context, req *LedgerHistoryRequest) (*LedgerHistoryResponse, error) {
	entries, total, err := s.repo.ListEntries(ctx, &supabase.ListEntriesRequest{
		UserID:    req.UserID,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		EntryType: (*string)(req.EntryType),
		Limit:     req.Limit,
		Offset:    req.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}

	result := make([]*LedgerEntry, len(entries))
	for i, e := range entries {
		result[i] = &LedgerEntry{
			ID:             e.ID,
			UserID:         e.UserID,
			EntryType:      EntryType(e.EntryType),
			Amount:         e.Amount,
			BalanceAfter:   e.BalanceAfter,
			ReferenceID:    e.ReferenceID,
			ReferenceType:  e.ReferenceType,
			ServiceID:      e.ServiceID,
			Description:    e.Description,
			CreatedAt:      e.CreatedAt,
			IdempotencyKey: e.IdempotencyKey,
		}
	}

	return &LedgerHistoryResponse{
		Entries:    result,
		TotalCount: total,
		HasMore:    req.Offset+len(entries) < total,
	}, nil
}

// =============================================================================
// Background Workers
// =============================================================================

// cleanupExpiredReservations releases expired reservations.
func (s *Service) cleanupExpiredReservations(ctx context.Context) error {
	now := time.Now()
	var expired []*Reservation

	s.mu.Lock()
	for id, r := range s.reservations {
		if now.After(r.ExpiresAt) {
			expired = append(expired, r)
			delete(s.reservations, id)
		}
	}
	s.mu.Unlock()

	for _, r := range expired {
		s.Release(ctx, &ReleaseRequest{
			ReservationID: r.ID,
			Consume:       false,
		})
		s.Logger().Info(ctx, "Released expired reservation", map[string]interface{}{"id": r.ID, "user": r.UserID})
	}
	return nil
}

// Logger returns the service logger.
func (s *Service) Logger() *logging.Logger {
	return s.BaseService.Logger()
}
