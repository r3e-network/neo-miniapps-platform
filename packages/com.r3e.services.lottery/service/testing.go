package lottery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore provides an in-memory implementation of Store for testing.
type MemoryStore struct {
	mu       sync.RWMutex
	rounds   map[string]Round
	tickets  map[string]Ticket
	configs  map[string]LotteryConfig
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		rounds:  make(map[string]Round),
		tickets: make(map[string]Ticket),
		configs: make(map[string]LotteryConfig),
	}
}

// Round operations

func (s *MemoryStore) CreateRound(ctx context.Context, round Round) (Round, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	round.ID = uuid.New().String()
	now := time.Now().UTC()
	round.CreatedAt = now
	round.UpdatedAt = now
	s.rounds[round.ID] = round
	return round, nil
}

func (s *MemoryStore) GetRound(ctx context.Context, roundID string) (Round, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	round, ok := s.rounds[roundID]
	if !ok {
		return Round{}, fmt.Errorf("round not found: %s", roundID)
	}
	return round, nil
}

func (s *MemoryStore) GetActiveRound(ctx context.Context, accountID string) (Round, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, round := range s.rounds {
		if round.AccountID == accountID && round.Status == RoundStatusActive {
			return round, nil
		}
	}
	return Round{}, fmt.Errorf("no active round found")
}

func (s *MemoryStore) UpdateRound(ctx context.Context, round Round) (Round, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rounds[round.ID]; !ok {
		return Round{}, fmt.Errorf("round not found: %s", round.ID)
	}
	round.UpdatedAt = time.Now().UTC()
	s.rounds[round.ID] = round
	return round, nil
}

func (s *MemoryStore) ListRounds(ctx context.Context, accountID string, limit int) ([]Round, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Round
	for _, round := range s.rounds {
		if round.AccountID == accountID {
			result = append(result, round)
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// Ticket operations

func (s *MemoryStore) CreateTicket(ctx context.Context, ticket Ticket) (Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ticket.ID = uuid.New().String()
	now := time.Now().UTC()
	ticket.CreatedAt = now
	ticket.UpdatedAt = now
	s.tickets[ticket.ID] = ticket
	return ticket, nil
}

func (s *MemoryStore) GetTicket(ctx context.Context, ticketID string) (Ticket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ticket, ok := s.tickets[ticketID]
	if !ok {
		return Ticket{}, fmt.Errorf("ticket not found: %s", ticketID)
	}
	return ticket, nil
}

func (s *MemoryStore) UpdateTicket(ctx context.Context, ticket Ticket) (Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tickets[ticket.ID]; !ok {
		return Ticket{}, fmt.Errorf("ticket not found: %s", ticket.ID)
	}
	ticket.UpdatedAt = time.Now().UTC()
	s.tickets[ticket.ID] = ticket
	return ticket, nil
}

func (s *MemoryStore) ListTickets(ctx context.Context, roundID string, limit int) ([]Ticket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Ticket
	for _, ticket := range s.tickets {
		if ticket.RoundID == roundID {
			result = append(result, ticket)
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *MemoryStore) ListTicketsByAccount(ctx context.Context, accountID string, limit int) ([]Ticket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Ticket
	for _, ticket := range s.tickets {
		if ticket.AccountID == accountID {
			result = append(result, ticket)
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *MemoryStore) ListTicketsByRoundAndAccount(ctx context.Context, roundID, accountID string) ([]Ticket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Ticket
	for _, ticket := range s.tickets {
		if ticket.RoundID == roundID && ticket.AccountID == accountID {
			result = append(result, ticket)
		}
	}
	return result, nil
}

// Config operations

func (s *MemoryStore) CreateConfig(ctx context.Context, config LotteryConfig) (LotteryConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	config.ID = uuid.New().String()
	now := time.Now().UTC()
	config.CreatedAt = now
	config.UpdatedAt = now
	s.configs[config.ID] = config
	return config, nil
}

func (s *MemoryStore) GetConfig(ctx context.Context, configID string) (LotteryConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, ok := s.configs[configID]
	if !ok {
		return LotteryConfig{}, fmt.Errorf("config not found: %s", configID)
	}
	return config, nil
}

func (s *MemoryStore) GetConfigByAccount(ctx context.Context, accountID string) (LotteryConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, config := range s.configs {
		if config.AccountID == accountID {
			return config, nil
		}
	}
	return LotteryConfig{}, fmt.Errorf("config not found for account: %s", accountID)
}

func (s *MemoryStore) UpdateConfig(ctx context.Context, config LotteryConfig) (LotteryConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.configs[config.ID]; !ok {
		return LotteryConfig{}, fmt.Errorf("config not found: %s", config.ID)
	}
	config.UpdatedAt = time.Now().UTC()
	s.configs[config.ID] = config
	return config, nil
}

// Stats operations

func (s *MemoryStore) GetStats(ctx context.Context, accountID string) (LotteryStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stats LotteryStats
	var totalPrizes int64

	for _, round := range s.rounds {
		if round.AccountID == accountID {
			stats.TotalRounds++
			stats.TotalTicketsSold += round.TicketCount
			if round.PrizePool > stats.LargestJackpot {
				stats.LargestJackpot = round.PrizePool
			}
			if round.Status == RoundStatusActive {
				stats.ActiveRoundID = round.ID
				stats.CurrentJackpot = round.PrizePool
				stats.NextDrawAt = round.DrawAt
			}
		}
	}

	for _, ticket := range s.tickets {
		if ticket.Claimed {
			totalPrizes += ticket.PrizeAmount
		}
	}
	stats.TotalPrizesPaid = totalPrizes

	return stats, nil
}

// Winner operations

func (s *MemoryStore) GetWinnersByRound(ctx context.Context, roundID string) ([]Winner, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var winners []Winner
	for _, ticket := range s.tickets {
		if ticket.RoundID == roundID && ticket.PrizeTier > 0 {
			winners = append(winners, Winner{
				TicketID:    ticket.ID,
				AccountID:   ticket.AccountID,
				PrizeTier:   ticket.PrizeTier,
				PrizeAmount: ticket.PrizeAmount,
			})
		}
	}
	return winners, nil
}

// MockAccountChecker provides a mock account checker for testing.
// Implements framework.AccountChecker interface.
type MockAccountChecker struct {
	accounts map[string]string // accountID -> tenant
}

// NewMockAccountChecker creates a new mock account checker.
func NewMockAccountChecker() *MockAccountChecker {
	return &MockAccountChecker{
		accounts: make(map[string]string),
	}
}

// AddAccount adds an account to the mock.
func (m *MockAccountChecker) AddAccount(accountID string) {
	m.accounts[accountID] = ""
}

// AddAccountWithTenant adds an account with a tenant to the mock.
func (m *MockAccountChecker) AddAccountWithTenant(accountID, tenant string) {
	m.accounts[accountID] = tenant
}

// AccountExists checks if an account exists.
func (m *MockAccountChecker) AccountExists(ctx context.Context, accountID string) error {
	if _, ok := m.accounts[accountID]; !ok {
		return fmt.Errorf("account not found: %s", accountID)
	}
	return nil
}

// AccountTenant returns the tenant for an account.
func (m *MockAccountChecker) AccountTenant(ctx context.Context, accountID string) string {
	return m.accounts[accountID]
}

// MockVRFService provides a mock VRF service for testing.
type MockVRFService struct {
	requests map[string]VRFRequest
	mu       sync.RWMutex
}

// NewMockVRFService creates a new mock VRF service.
func NewMockVRFService() *MockVRFService {
	return &MockVRFService{
		requests: make(map[string]VRFRequest),
	}
}

// CreateRequest creates a mock VRF request.
func (m *MockVRFService) CreateRequest(ctx context.Context, accountID, keyID, consumer, seed string, metadata map[string]string) (VRFRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req := VRFRequest{
		ID:        uuid.New().String(),
		AccountID: accountID,
		KeyID:     keyID,
		Consumer:  consumer,
		Seed:      seed,
		Status:    "fulfilled",
		Result:    "0x" + seed[:32], // Use seed as mock result
	}
	m.requests[req.ID] = req
	return req, nil
}

// GetRequest retrieves a mock VRF request.
func (m *MockVRFService) GetRequest(ctx context.Context, accountID, requestID string) (VRFRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	req, ok := m.requests[requestID]
	if !ok {
		return VRFRequest{}, fmt.Errorf("request not found: %s", requestID)
	}
	return req, nil
}
