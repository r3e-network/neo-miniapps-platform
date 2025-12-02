package lottery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/sandbox"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Service provides lottery management with VRF randomness and automated draws.
type Service struct {
	*framework.SandboxedServiceEngine
	store      Store
	vrf        VRFService
	automation AutomationService
}

// New constructs a lottery service.
func New(accounts framework.AccountChecker, store Store, log *logger.Logger) *Service {
	return &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:         "lottery",
				Domain:       "lottery",
				Description:  "Mega Millions style lottery with VRF and automation",
				DependsOn:    []string{"store", "svc-accounts", "svc-vrf", "svc-automation"},
				RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceEvent},
				Capabilities: []string{"lottery"},
				Accounts:     accounts,
				Logger:       log,
			},
			SecurityLevel: sandbox.SecurityLevelPrivileged,
			RequestedCapabilities: []sandbox.Capability{
				sandbox.CapStorageRead,
				sandbox.CapStorageWrite,
				sandbox.CapDatabaseRead,
				sandbox.CapDatabaseWrite,
				sandbox.CapBusPublish,
				sandbox.CapServiceCall,
			},
			StorageQuota: 50 * 1024 * 1024, // 50MB
		}),
		store: store,
	}
}

// WithVRF sets the VRF service for random number generation.
func (s *Service) WithVRF(vrf VRFService) {
	s.vrf = vrf
}

// WithAutomation sets the automation service for scheduled draws.
func (s *Service) WithAutomation(automation AutomationService) {
	s.automation = automation
}

// Errors
var (
	ErrInvalidNumbers     = errors.New("invalid lottery numbers")
	ErrRoundNotActive     = errors.New("round is not active")
	ErrRoundNotFound      = errors.New("round not found")
	ErrTicketNotFound     = errors.New("ticket not found")
	ErrAlreadyClaimed     = errors.New("prize already claimed")
	ErrNotWinner          = errors.New("ticket is not a winner")
	ErrInsufficientTickets = errors.New("insufficient tickets for draw")
	ErrVRFNotConfigured   = errors.New("VRF service not configured")
	ErrDrawInProgress     = errors.New("draw already in progress")
)

// CreateConfig creates a new lottery configuration.
func (s *Service) CreateConfig(ctx context.Context, accountID string, config LotteryConfig) (LotteryConfig, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return LotteryConfig{}, fmt.Errorf("account validation: %w", err)
	}

	config.AccountID = accountID
	if config.TicketPrice == 0 {
		config.TicketPrice = DefaultTicketPrice
	}
	if config.MainNumberMin == 0 {
		config.MainNumberMin = DefaultMainNumberMin
	}
	if config.MainNumberMax == 0 {
		config.MainNumberMax = DefaultMainNumberMax
	}
	if config.MainNumberCount == 0 {
		config.MainNumberCount = DefaultMainNumberCount
	}
	if config.MegaNumberMin == 0 {
		config.MegaNumberMin = DefaultMegaNumberMin
	}
	if config.MegaNumberMax == 0 {
		config.MegaNumberMax = DefaultMegaNumberMax
	}
	if config.MinTicketsForDraw == 0 {
		config.MinTicketsForDraw = DefaultMinTicketsForDraw
	}
	if config.DrawSchedule == "" {
		config.DrawSchedule = DefaultDrawSchedule
	}
	config.Enabled = true

	created, err := s.store.CreateConfig(ctx, config)
	if err != nil {
		return LotteryConfig{}, fmt.Errorf("create config: %w", err)
	}

	s.Logger().WithField("config_id", created.ID).
		WithField("account_id", accountID).
		Info("lottery config created")
	s.LogCreated("lottery_config", created.ID, accountID)

	return created, nil
}

// StartRound starts a new lottery round.
func (s *Service) StartRound(ctx context.Context, accountID string) (Round, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return Round{}, fmt.Errorf("account validation: %w", err)
	}

	config, err := s.store.GetConfigByAccount(ctx, accountID)
	if err != nil {
		return Round{}, fmt.Errorf("get config: %w", err)
	}

	// Check for existing active round
	existing, err := s.store.GetActiveRound(ctx, accountID)
	if err == nil && existing.ID != "" {
		return Round{}, fmt.Errorf("active round already exists: %s", existing.ID)
	}

	// Get next round number
	rounds, err := s.store.ListRounds(ctx, accountID, 1)
	roundNumber := int64(1)
	if err == nil && len(rounds) > 0 {
		roundNumber = rounds[0].RoundNumber + 1
	}

	now := time.Now().UTC()
	round := Round{
		AccountID:    accountID,
		RoundNumber:  roundNumber,
		Status:       RoundStatusActive,
		PrizePool:    0,
		TicketCount:  0,
		TicketPrice:  config.TicketPrice,
		DrawSchedule: config.DrawSchedule,
		StartedAt:    now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	created, err := s.store.CreateRound(ctx, round)
	if err != nil {
		return Round{}, fmt.Errorf("create round: %w", err)
	}

	s.Logger().WithField("round_id", created.ID).
		WithField("round_number", created.RoundNumber).
		WithField("account_id", accountID).
		Info("lottery round started")
	s.LogCreated("lottery_round", created.ID, accountID)
	s.IncrementCounter("lottery_rounds_started_total", map[string]string{"account_id": accountID})

	return created, nil
}

// BuyTicket purchases a lottery ticket with chosen numbers.
func (s *Service) BuyTicket(ctx context.Context, accountID, roundID string, numbers []int, megaNumber int) (Ticket, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return Ticket{}, fmt.Errorf("account validation: %w", err)
	}

	round, err := s.store.GetRound(ctx, roundID)
	if err != nil {
		return Ticket{}, ErrRoundNotFound
	}

	if round.Status != RoundStatusActive {
		return Ticket{}, ErrRoundNotActive
	}

	// Validate numbers
	if err := s.validateNumbers(numbers, megaNumber); err != nil {
		return Ticket{}, err
	}

	// Sort numbers for consistent storage
	sortedNumbers := make([]int, len(numbers))
	copy(sortedNumbers, numbers)
	sort.Ints(sortedNumbers)

	now := time.Now().UTC()
	ticket := Ticket{
		AccountID:   accountID,
		RoundID:     roundID,
		Numbers:     sortedNumbers,
		MegaNumber:  megaNumber,
		QuickPick:   false,
		PurchasedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	created, err := s.store.CreateTicket(ctx, ticket)
	if err != nil {
		return Ticket{}, fmt.Errorf("create ticket: %w", err)
	}

	// Update round prize pool and ticket count
	round.PrizePool += round.TicketPrice
	round.TicketCount++
	round.UpdatedAt = now
	if _, err := s.store.UpdateRound(ctx, round); err != nil {
		s.Logger().WithError(err).Warn("failed to update round after ticket purchase")
	}

	s.Logger().WithField("ticket_id", created.ID).
		WithField("round_id", roundID).
		WithField("account_id", accountID).
		Info("lottery ticket purchased")
	s.LogCreated("lottery_ticket", created.ID, accountID)
	s.IncrementCounter("lottery_tickets_sold_total", map[string]string{"account_id": round.AccountID})

	return created, nil
}

// QuickPick generates random numbers and purchases a ticket.
func (s *Service) QuickPick(ctx context.Context, accountID, roundID string) (Ticket, error) {
	numbers, megaNumber := s.generateRandomNumbers()

	ticket, err := s.BuyTicket(ctx, accountID, roundID, numbers, megaNumber)
	if err != nil {
		return Ticket{}, err
	}

	// Mark as quick pick
	ticket.QuickPick = true
	ticket.UpdatedAt = time.Now().UTC()
	updated, err := s.store.UpdateTicket(ctx, ticket)
	if err != nil {
		return ticket, nil // Return original ticket if update fails
	}

	return updated, nil
}

// InitiateDraw starts the draw process for a round.
func (s *Service) InitiateDraw(ctx context.Context, roundID string) (Round, error) {
	round, err := s.store.GetRound(ctx, roundID)
	if err != nil {
		return Round{}, ErrRoundNotFound
	}

	if round.Status != RoundStatusActive {
		return Round{}, ErrRoundNotActive
	}

	if round.Status == RoundStatusDrawing {
		return Round{}, ErrDrawInProgress
	}

	config, err := s.store.GetConfigByAccount(ctx, round.AccountID)
	if err != nil {
		return Round{}, fmt.Errorf("get config: %w", err)
	}

	if round.TicketCount < int64(config.MinTicketsForDraw) {
		return Round{}, ErrInsufficientTickets
	}

	// Update status to drawing
	round.Status = RoundStatusDrawing
	round.UpdatedAt = time.Now().UTC()
	updated, err := s.store.UpdateRound(ctx, round)
	if err != nil {
		return Round{}, fmt.Errorf("update round: %w", err)
	}

	// Request VRF randomness if configured
	if s.vrf != nil && config.VRFKeyID != "" {
		seed := s.generateDrawSeed(round)
		vrfReq, err := s.vrf.CreateRequest(ctx, round.AccountID, config.VRFKeyID, "lottery-draw", seed, map[string]string{
			"round_id": round.ID,
			"type":     "lottery_draw",
		})
		if err != nil {
			s.Logger().WithError(err).Warn("failed to create VRF request, using fallback")
		} else {
			updated.VRFRequestID = vrfReq.ID
			updated, _ = s.store.UpdateRound(ctx, updated)
		}
	}

	s.Logger().WithField("round_id", roundID).
		WithField("ticket_count", round.TicketCount).
		Info("lottery draw initiated")
	s.IncrementCounter("lottery_draws_initiated_total", map[string]string{"account_id": round.AccountID})

	return updated, nil
}

// CompleteDraw completes the draw with winning numbers.
func (s *Service) CompleteDraw(ctx context.Context, roundID string, winningNumbers []int, megaNumber int) (DrawResult, error) {
	round, err := s.store.GetRound(ctx, roundID)
	if err != nil {
		return DrawResult{}, ErrRoundNotFound
	}

	if round.Status != RoundStatusDrawing {
		return DrawResult{}, fmt.Errorf("round is not in drawing state")
	}

	// Validate winning numbers
	if err := s.validateNumbers(winningNumbers, megaNumber); err != nil {
		return DrawResult{}, err
	}

	// Sort winning numbers
	sortedWinning := make([]int, len(winningNumbers))
	copy(sortedWinning, winningNumbers)
	sort.Ints(sortedWinning)

	// Update round with winning numbers
	now := time.Now().UTC()
	round.WinningNumbers = sortedWinning
	round.MegaNumber = megaNumber
	round.Status = RoundStatusCompleted
	round.CompletedAt = now
	round.UpdatedAt = now

	// Find all winners
	tickets, err := s.store.ListTickets(ctx, roundID, 10000)
	if err != nil {
		return DrawResult{}, fmt.Errorf("list tickets: %w", err)
	}

	var winners []Winner
	for _, ticket := range tickets {
		mainMatches := s.countMatches(ticket.Numbers, sortedWinning)
		megaMatch := ticket.MegaNumber == megaNumber
		tier := s.getPrizeTier(mainMatches, megaMatch)

		if tier > 0 {
			prize := s.calculatePrize(round.PrizePool, tier, mainMatches, megaMatch)
			winners = append(winners, Winner{
				TicketID:    ticket.ID,
				AccountID:   ticket.AccountID,
				PrizeTier:   tier,
				PrizeAmount: prize,
				MainMatches: mainMatches,
				MegaMatch:   megaMatch,
			})

			// Update ticket with prize info
			ticket.PrizeTier = tier
			ticket.PrizeAmount = prize
			ticket.UpdatedAt = now
			s.store.UpdateTicket(ctx, ticket)

			if tier == 1 {
				round.JackpotWinners++
			}
		}
	}

	updated, err := s.store.UpdateRound(ctx, round)
	if err != nil {
		return DrawResult{}, fmt.Errorf("update round: %w", err)
	}

	result := DrawResult{
		RoundID:        roundID,
		WinningNumbers: sortedWinning,
		MegaNumber:     megaNumber,
		TotalPrizePool: round.PrizePool,
		Winners:        winners,
		DrawnAt:        now,
	}

	s.Logger().WithField("round_id", roundID).
		WithField("winning_numbers", sortedWinning).
		WithField("mega_number", megaNumber).
		WithField("winner_count", len(winners)).
		Info("lottery draw completed")
	s.IncrementCounter("lottery_draws_completed_total", map[string]string{"account_id": updated.AccountID})

	// Publish event
	eventPayload := map[string]any{
		"round_id":        roundID,
		"winning_numbers": sortedWinning,
		"mega_number":     megaNumber,
		"winner_count":    len(winners),
		"prize_pool":      round.PrizePool,
	}
	if err := s.PublishEvent(ctx, "lottery.draw.completed", eventPayload); err != nil {
		if errors.Is(err, core.ErrBusUnavailable) {
			s.Logger().WithError(err).Warn("bus unavailable for lottery draw event")
		}
	}

	return result, nil
}

// ExecuteAutomatedDraw performs a complete automated draw using VRF.
func (s *Service) ExecuteAutomatedDraw(ctx context.Context, roundID string) (DrawResult, error) {
	// Initiate draw
	round, err := s.InitiateDraw(ctx, roundID)
	if err != nil {
		return DrawResult{}, err
	}

	// Generate winning numbers
	var winningNumbers []int
	var megaNumber int

	// Try to use VRF result if available
	if round.VRFRequestID != "" && s.vrf != nil {
		vrfReq, err := s.vrf.GetRequest(ctx, round.AccountID, round.VRFRequestID)
		if err == nil && vrfReq.Status == "fulfilled" && vrfReq.Result != "" {
			winningNumbers, megaNumber = s.parseVRFResult(vrfReq.Result)
		}
	}

	// Fallback to pseudo-random if VRF not available
	if len(winningNumbers) == 0 {
		winningNumbers, megaNumber = s.generateRandomNumbers()
	}

	return s.CompleteDraw(ctx, roundID, winningNumbers, megaNumber)
}

// ClaimPrize claims a prize for a winning ticket.
func (s *Service) ClaimPrize(ctx context.Context, accountID, ticketID string) (Ticket, error) {
	ticket, err := s.store.GetTicket(ctx, ticketID)
	if err != nil {
		return Ticket{}, ErrTicketNotFound
	}

	if err := core.EnsureOwnership(ticket.AccountID, accountID, "ticket", ticketID); err != nil {
		return Ticket{}, err
	}

	if ticket.Claimed {
		return Ticket{}, ErrAlreadyClaimed
	}

	if ticket.PrizeTier == 0 {
		return Ticket{}, ErrNotWinner
	}

	// Mark as claimed
	ticket.Claimed = true
	ticket.ClaimedAt = time.Now().UTC()
	ticket.UpdatedAt = ticket.ClaimedAt

	updated, err := s.store.UpdateTicket(ctx, ticket)
	if err != nil {
		return Ticket{}, fmt.Errorf("update ticket: %w", err)
	}

	s.Logger().WithField("ticket_id", ticketID).
		WithField("prize_tier", ticket.PrizeTier).
		WithField("prize_amount", ticket.PrizeAmount).
		Info("lottery prize claimed")
	s.IncrementCounter("lottery_prizes_claimed_total", map[string]string{"account_id": accountID})

	return updated, nil
}

// GetRound retrieves a round by ID.
func (s *Service) GetRound(ctx context.Context, roundID string) (Round, error) {
	return s.store.GetRound(ctx, roundID)
}

// GetActiveRound retrieves the active round for an account.
func (s *Service) GetActiveRound(ctx context.Context, accountID string) (Round, error) {
	return s.store.GetActiveRound(ctx, accountID)
}

// ListRounds lists rounds for an account.
func (s *Service) ListRounds(ctx context.Context, accountID string, limit int) ([]Round, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.store.ListRounds(ctx, accountID, limit)
}

// GetTicket retrieves a ticket by ID.
func (s *Service) GetTicket(ctx context.Context, accountID, ticketID string) (Ticket, error) {
	ticket, err := s.store.GetTicket(ctx, ticketID)
	if err != nil {
		return Ticket{}, ErrTicketNotFound
	}
	if err := core.EnsureOwnership(ticket.AccountID, accountID, "ticket", ticketID); err != nil {
		return Ticket{}, err
	}
	return ticket, nil
}

// ListTickets lists tickets for a round.
func (s *Service) ListTickets(ctx context.Context, roundID string, limit int) ([]Ticket, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.store.ListTickets(ctx, roundID, limit)
}

// ListMyTickets lists tickets for an account.
func (s *Service) ListMyTickets(ctx context.Context, accountID string, limit int) ([]Ticket, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.store.ListTicketsByAccount(ctx, accountID, limit)
}

// GetStats retrieves lottery statistics.
func (s *Service) GetStats(ctx context.Context, accountID string) (LotteryStats, error) {
	return s.store.GetStats(ctx, accountID)
}

// Helper methods

func (s *Service) validateNumbers(numbers []int, megaNumber int) error {
	if len(numbers) != DefaultMainNumberCount {
		return fmt.Errorf("%w: must provide exactly %d main numbers", ErrInvalidNumbers, DefaultMainNumberCount)
	}

	seen := make(map[int]bool)
	for _, n := range numbers {
		if n < DefaultMainNumberMin || n > DefaultMainNumberMax {
			return fmt.Errorf("%w: main numbers must be between %d and %d", ErrInvalidNumbers, DefaultMainNumberMin, DefaultMainNumberMax)
		}
		if seen[n] {
			return fmt.Errorf("%w: main numbers must be unique", ErrInvalidNumbers)
		}
		seen[n] = true
	}

	if megaNumber < DefaultMegaNumberMin || megaNumber > DefaultMegaNumberMax {
		return fmt.Errorf("%w: mega number must be between %d and %d", ErrInvalidNumbers, DefaultMegaNumberMin, DefaultMegaNumberMax)
	}

	return nil
}

func (s *Service) generateRandomNumbers() ([]int, int) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate 5 unique main numbers
	numbers := make([]int, 0, DefaultMainNumberCount)
	used := make(map[int]bool)
	for len(numbers) < DefaultMainNumberCount {
		n := r.Intn(DefaultMainNumberMax-DefaultMainNumberMin+1) + DefaultMainNumberMin
		if !used[n] {
			numbers = append(numbers, n)
			used[n] = true
		}
	}
	sort.Ints(numbers)

	// Generate mega number
	megaNumber := r.Intn(DefaultMegaNumberMax-DefaultMegaNumberMin+1) + DefaultMegaNumberMin

	return numbers, megaNumber
}

func (s *Service) generateDrawSeed(round Round) string {
	data := fmt.Sprintf("%s-%d-%d-%d-%d",
		round.ID,
		round.RoundNumber,
		round.TicketCount,
		round.PrizePool,
		time.Now().UnixNano(),
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (s *Service) parseVRFResult(result string) ([]int, int) {
	// Parse VRF output to generate numbers
	bytes, err := hex.DecodeString(strings.TrimPrefix(result, "0x"))
	if err != nil || len(bytes) < 6 {
		return s.generateRandomNumbers()
	}

	numbers := make([]int, 0, DefaultMainNumberCount)
	used := make(map[int]bool)
	idx := 0
	for len(numbers) < DefaultMainNumberCount && idx < len(bytes) {
		n := int(bytes[idx])%DefaultMainNumberMax + DefaultMainNumberMin
		if !used[n] {
			numbers = append(numbers, n)
			used[n] = true
		}
		idx++
	}

	// Fill remaining if needed
	for len(numbers) < DefaultMainNumberCount {
		extra, _ := s.generateRandomNumbers()
		for _, n := range extra {
			if !used[n] && len(numbers) < DefaultMainNumberCount {
				numbers = append(numbers, n)
				used[n] = true
			}
		}
	}

	sort.Ints(numbers)
	megaNumber := int(bytes[idx%len(bytes)])%DefaultMegaNumberMax + DefaultMegaNumberMin

	return numbers, megaNumber
}

func (s *Service) countMatches(playerNumbers, winningNumbers []int) int {
	matches := 0
	winningSet := make(map[int]bool)
	for _, n := range winningNumbers {
		winningSet[n] = true
	}
	for _, n := range playerNumbers {
		if winningSet[n] {
			matches++
		}
	}
	return matches
}

func (s *Service) getPrizeTier(mainMatches int, megaMatch bool) int {
	if mainMatches == 5 && megaMatch {
		return 1 // Jackpot
	}
	if mainMatches == 5 {
		return 2
	}
	if mainMatches == 4 && megaMatch {
		return 3
	}
	if mainMatches == 4 {
		return 4
	}
	if mainMatches == 3 && megaMatch {
		return 5
	}
	if mainMatches == 3 {
		return 6
	}
	if mainMatches == 2 && megaMatch {
		return 7
	}
	if mainMatches == 1 && megaMatch {
		return 8
	}
	if megaMatch {
		return 9
	}
	return 0 // No prize
}

func (s *Service) calculatePrize(prizePool int64, tier, mainMatches int, megaMatch bool) int64 {
	switch tier {
	case 1:
		return prizePool * 50 / 100 // 50% of pool
	case 2:
		return prizePool * 15 / 100 // 15% of pool
	case 3:
		return prizePool * 10 / 100 // 10% of pool
	case 4:
		return 50_000_000 // 0.5 GAS
	case 5:
		return 20_000_000 // 0.2 GAS
	case 6:
		return 10_000_000 // 0.1 GAS
	case 7:
		return 5_000_000 // 0.05 GAS
	case 8:
		return 2_000_000 // 0.02 GAS
	case 9:
		return 1_000_000 // 0.01 GAS
	default:
		return 0
	}
}

// HTTP API Methods

// HTTPGetRounds handles GET /rounds - list lottery rounds.
func (s *Service) HTTPGetRounds(ctx context.Context, req core.APIRequest) (any, error) {
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListRounds(ctx, req.AccountID, limit)
}

// HTTPPostRounds handles POST /rounds - start a new round.
func (s *Service) HTTPPostRounds(ctx context.Context, req core.APIRequest) (any, error) {
	return s.StartRound(ctx, req.AccountID)
}

// HTTPGetRoundsById handles GET /rounds/{id} - get a specific round.
func (s *Service) HTTPGetRoundsById(ctx context.Context, req core.APIRequest) (any, error) {
	roundID := req.PathParams["id"]
	return s.GetRound(ctx, roundID)
}

// HTTPGetRoundsActive handles GET /rounds/active - get active round.
func (s *Service) HTTPGetRoundsActive(ctx context.Context, req core.APIRequest) (any, error) {
	return s.GetActiveRound(ctx, req.AccountID)
}

// HTTPPostRoundsIdDraw handles POST /rounds/{id}/draw - initiate draw.
func (s *Service) HTTPPostRoundsIdDraw(ctx context.Context, req core.APIRequest) (any, error) {
	roundID := req.PathParams["id"]
	return s.ExecuteAutomatedDraw(ctx, roundID)
}

// HTTPGetTickets handles GET /tickets - list my tickets.
func (s *Service) HTTPGetTickets(ctx context.Context, req core.APIRequest) (any, error) {
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListMyTickets(ctx, req.AccountID, limit)
}

// HTTPPostTickets handles POST /tickets - buy a ticket.
func (s *Service) HTTPPostTickets(ctx context.Context, req core.APIRequest) (any, error) {
	roundID, _ := req.Body["round_id"].(string)
	quickPick, _ := req.Body["quick_pick"].(bool)

	if quickPick {
		return s.QuickPick(ctx, req.AccountID, roundID)
	}

	numbersRaw, _ := req.Body["numbers"].([]any)
	numbers := make([]int, 0, len(numbersRaw))
	for _, n := range numbersRaw {
		if num, ok := n.(float64); ok {
			numbers = append(numbers, int(num))
		}
	}

	megaNumber := 0
	if mn, ok := req.Body["mega_number"].(float64); ok {
		megaNumber = int(mn)
	}

	return s.BuyTicket(ctx, req.AccountID, roundID, numbers, megaNumber)
}

// HTTPGetTicketsById handles GET /tickets/{id} - get a specific ticket.
func (s *Service) HTTPGetTicketsById(ctx context.Context, req core.APIRequest) (any, error) {
	ticketID := req.PathParams["id"]
	return s.GetTicket(ctx, req.AccountID, ticketID)
}

// HTTPPostTicketsIdClaim handles POST /tickets/{id}/claim - claim prize.
func (s *Service) HTTPPostTicketsIdClaim(ctx context.Context, req core.APIRequest) (any, error) {
	ticketID := req.PathParams["id"]
	return s.ClaimPrize(ctx, req.AccountID, ticketID)
}

// HTTPGetStats handles GET /stats - get lottery statistics.
func (s *Service) HTTPGetStats(ctx context.Context, req core.APIRequest) (any, error) {
	return s.GetStats(ctx, req.AccountID)
}
