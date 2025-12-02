package lottery

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/pkg/logger"
)

func TestLotteryService_FullLifecycle(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("lottery-test")

	// Setup
	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	accounts.AddAccount("test-account")
	accounts.AddAccount("player-1")
	accounts.AddAccount("player-2")

	svc := New(accounts, store, log)
	vrfService := NewMockVRFService()
	svc.WithVRF(vrfService)

	// Test 1: Create lottery config
	t.Run("CreateConfig", func(t *testing.T) {
		config, err := svc.CreateConfig(ctx, "test-account", LotteryConfig{
			Name:        "Test Mega Lottery",
			Description: "A test lottery",
			VRFKeyID:    "test-vrf-key",
		})
		if err != nil {
			t.Fatalf("CreateConfig failed: %v", err)
		}
		if config.ID == "" {
			t.Error("Config ID should not be empty")
		}
		if config.TicketPrice != DefaultTicketPrice {
			t.Errorf("Expected ticket price %d, got %d", DefaultTicketPrice, config.TicketPrice)
		}
		if config.MainNumberMax != DefaultMainNumberMax {
			t.Errorf("Expected main number max %d, got %d", DefaultMainNumberMax, config.MainNumberMax)
		}
	})

	// Test 2: Start a round
	var roundID string
	t.Run("StartRound", func(t *testing.T) {
		round, err := svc.StartRound(ctx, "test-account")
		if err != nil {
			t.Fatalf("StartRound failed: %v", err)
		}
		roundID = round.ID
		if round.Status != RoundStatusActive {
			t.Errorf("Expected status %s, got %s", RoundStatusActive, round.Status)
		}
		if round.RoundNumber != 1 {
			t.Errorf("Expected round number 1, got %d", round.RoundNumber)
		}
	})

	// Test 3: Buy tickets with valid numbers
	var ticketIDs []string
	t.Run("BuyTicket_ValidNumbers", func(t *testing.T) {
		numbers := []int{5, 12, 23, 45, 67}
		megaNumber := 15

		ticket, err := svc.BuyTicket(ctx, "player-1", roundID, numbers, megaNumber)
		if err != nil {
			t.Fatalf("BuyTicket failed: %v", err)
		}
		ticketIDs = append(ticketIDs, ticket.ID)

		if ticket.AccountID != "player-1" {
			t.Errorf("Expected account player-1, got %s", ticket.AccountID)
		}
		if len(ticket.Numbers) != 5 {
			t.Errorf("Expected 5 numbers, got %d", len(ticket.Numbers))
		}
		if ticket.MegaNumber != megaNumber {
			t.Errorf("Expected mega number %d, got %d", megaNumber, ticket.MegaNumber)
		}
	})

	// Test 4: Buy ticket with invalid numbers
	t.Run("BuyTicket_InvalidNumbers", func(t *testing.T) {
		// Duplicate numbers
		_, err := svc.BuyTicket(ctx, "player-1", roundID, []int{5, 5, 23, 45, 67}, 15)
		if err == nil {
			t.Error("Expected error for duplicate numbers")
		}

		// Out of range
		_, err = svc.BuyTicket(ctx, "player-1", roundID, []int{5, 12, 23, 45, 100}, 15)
		if err == nil {
			t.Error("Expected error for out of range number")
		}

		// Invalid mega number
		_, err = svc.BuyTicket(ctx, "player-1", roundID, []int{5, 12, 23, 45, 67}, 30)
		if err == nil {
			t.Error("Expected error for invalid mega number")
		}
	})

	// Test 5: Quick pick
	t.Run("QuickPick", func(t *testing.T) {
		ticket, err := svc.QuickPick(ctx, "player-2", roundID)
		if err != nil {
			t.Fatalf("QuickPick failed: %v", err)
		}
		ticketIDs = append(ticketIDs, ticket.ID)

		if !ticket.QuickPick {
			t.Error("Expected QuickPick to be true")
		}
		if len(ticket.Numbers) != 5 {
			t.Errorf("Expected 5 numbers, got %d", len(ticket.Numbers))
		}

		// Verify numbers are unique
		seen := make(map[int]bool)
		for _, n := range ticket.Numbers {
			if seen[n] {
				t.Error("Quick pick generated duplicate numbers")
			}
			seen[n] = true
			if n < DefaultMainNumberMin || n > DefaultMainNumberMax {
				t.Errorf("Number %d out of range", n)
			}
		}
		if ticket.MegaNumber < DefaultMegaNumberMin || ticket.MegaNumber > DefaultMegaNumberMax {
			t.Errorf("Mega number %d out of range", ticket.MegaNumber)
		}
	})

	// Test 6: Buy more tickets to meet minimum
	t.Run("BuyMoreTickets", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			ticket, err := svc.QuickPick(ctx, "player-1", roundID)
			if err != nil {
				t.Fatalf("QuickPick %d failed: %v", i, err)
			}
			ticketIDs = append(ticketIDs, ticket.ID)
		}
	})

	// Test 7: Verify round state
	t.Run("VerifyRoundState", func(t *testing.T) {
		round, err := svc.GetRound(ctx, roundID)
		if err != nil {
			t.Fatalf("GetRound failed: %v", err)
		}
		if round.TicketCount != 12 {
			t.Errorf("Expected 12 tickets, got %d", round.TicketCount)
		}
		if round.PrizePool != 12*DefaultTicketPrice {
			t.Errorf("Expected prize pool %d, got %d", 12*DefaultTicketPrice, round.PrizePool)
		}
	})

	// Test 8: Execute automated draw
	var drawResult DrawResult
	t.Run("ExecuteAutomatedDraw", func(t *testing.T) {
		result, err := svc.ExecuteAutomatedDraw(ctx, roundID)
		if err != nil {
			t.Fatalf("ExecuteAutomatedDraw failed: %v", err)
		}
		drawResult = result

		if len(result.WinningNumbers) != 5 {
			t.Errorf("Expected 5 winning numbers, got %d", len(result.WinningNumbers))
		}
		if result.MegaNumber < DefaultMegaNumberMin || result.MegaNumber > DefaultMegaNumberMax {
			t.Errorf("Mega number %d out of range", result.MegaNumber)
		}

		// Verify winning numbers are sorted
		for i := 1; i < len(result.WinningNumbers); i++ {
			if result.WinningNumbers[i] < result.WinningNumbers[i-1] {
				t.Error("Winning numbers should be sorted")
			}
		}
	})

	// Test 9: Verify round completed
	t.Run("VerifyRoundCompleted", func(t *testing.T) {
		round, err := svc.GetRound(ctx, roundID)
		if err != nil {
			t.Fatalf("GetRound failed: %v", err)
		}
		if round.Status != RoundStatusCompleted {
			t.Errorf("Expected status %s, got %s", RoundStatusCompleted, round.Status)
		}
		if len(round.WinningNumbers) != 5 {
			t.Errorf("Expected 5 winning numbers, got %d", len(round.WinningNumbers))
		}
	})

	// Test 10: List tickets and check for winners
	t.Run("ListTicketsAndCheckWinners", func(t *testing.T) {
		tickets, err := svc.ListTickets(ctx, roundID, 100)
		if err != nil {
			t.Fatalf("ListTickets failed: %v", err)
		}
		if len(tickets) != 12 {
			t.Errorf("Expected 12 tickets, got %d", len(tickets))
		}

		// Log draw results
		t.Logf("Winning numbers: %v, Mega: %d", drawResult.WinningNumbers, drawResult.MegaNumber)
		t.Logf("Total winners: %d", len(drawResult.Winners))
		for _, w := range drawResult.Winners {
			t.Logf("  Winner: tier=%d, prize=%d", w.PrizeTier, w.PrizeAmount)
		}
	})

	// Test 11: Claim prize (if any winners)
	t.Run("ClaimPrize", func(t *testing.T) {
		if len(drawResult.Winners) == 0 {
			t.Skip("No winners to claim")
		}

		winner := drawResult.Winners[0]
		ticket, err := svc.ClaimPrize(ctx, winner.AccountID, winner.TicketID)
		if err != nil {
			t.Fatalf("ClaimPrize failed: %v", err)
		}
		if !ticket.Claimed {
			t.Error("Ticket should be marked as claimed")
		}

		// Try to claim again
		_, err = svc.ClaimPrize(ctx, winner.AccountID, winner.TicketID)
		if err != ErrAlreadyClaimed {
			t.Errorf("Expected ErrAlreadyClaimed, got %v", err)
		}
	})

	// Test 12: Get stats
	t.Run("GetStats", func(t *testing.T) {
		stats, err := svc.GetStats(ctx, "test-account")
		if err != nil {
			t.Fatalf("GetStats failed: %v", err)
		}
		if stats.TotalRounds != 1 {
			t.Errorf("Expected 1 round, got %d", stats.TotalRounds)
		}
		if stats.TotalTicketsSold != 12 {
			t.Errorf("Expected 12 tickets sold, got %d", stats.TotalTicketsSold)
		}
	})

	// Test 13: Start new round after completion
	t.Run("StartNewRoundAfterCompletion", func(t *testing.T) {
		round, err := svc.StartRound(ctx, "test-account")
		if err != nil {
			t.Fatalf("StartRound failed: %v", err)
		}
		if round.RoundNumber != 2 {
			t.Errorf("Expected round number 2, got %d", round.RoundNumber)
		}
	})
}

func TestLotteryService_PrizeTiers(t *testing.T) {
	svc := &Service{}

	testCases := []struct {
		mainMatches int
		megaMatch   bool
		expectedTier int
	}{
		{5, true, 1},   // Jackpot
		{5, false, 2},  // Match 5
		{4, true, 3},   // Match 4+Mega
		{4, false, 4},  // Match 4
		{3, true, 5},   // Match 3+Mega
		{3, false, 6},  // Match 3
		{2, true, 7},   // Match 2+Mega
		{1, true, 8},   // Match 1+Mega
		{0, true, 9},   // Match Mega only
		{2, false, 0},  // No prize
		{1, false, 0},  // No prize
		{0, false, 0},  // No prize
	}

	for _, tc := range testCases {
		tier := svc.getPrizeTier(tc.mainMatches, tc.megaMatch)
		if tier != tc.expectedTier {
			t.Errorf("getPrizeTier(%d, %v) = %d, expected %d",
				tc.mainMatches, tc.megaMatch, tier, tc.expectedTier)
		}
	}
}

func TestLotteryService_CountMatches(t *testing.T) {
	svc := &Service{}

	testCases := []struct {
		player   []int
		winning  []int
		expected int
	}{
		{[]int{1, 2, 3, 4, 5}, []int{1, 2, 3, 4, 5}, 5},
		{[]int{1, 2, 3, 4, 5}, []int{6, 7, 8, 9, 10}, 0},
		{[]int{1, 2, 3, 4, 5}, []int{1, 2, 6, 7, 8}, 2},
		{[]int{10, 20, 30, 40, 50}, []int{10, 25, 30, 45, 50}, 3},
	}

	for _, tc := range testCases {
		matches := svc.countMatches(tc.player, tc.winning)
		if matches != tc.expected {
			t.Errorf("countMatches(%v, %v) = %d, expected %d",
				tc.player, tc.winning, matches, tc.expected)
		}
	}
}

func TestLotteryService_ValidateNumbers(t *testing.T) {
	svc := &Service{}

	testCases := []struct {
		name       string
		numbers    []int
		megaNumber int
		wantErr    bool
	}{
		{"valid", []int{1, 20, 35, 50, 70}, 15, false},
		{"too few numbers", []int{1, 2, 3, 4}, 15, true},
		{"too many numbers", []int{1, 2, 3, 4, 5, 6}, 15, true},
		{"duplicate numbers", []int{1, 1, 3, 4, 5}, 15, true},
		{"number too low", []int{0, 2, 3, 4, 5}, 15, true},
		{"number too high", []int{1, 2, 3, 4, 71}, 15, true},
		{"mega too low", []int{1, 2, 3, 4, 5}, 0, true},
		{"mega too high", []int{1, 2, 3, 4, 5}, 26, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.validateNumbers(tc.numbers, tc.megaNumber)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateNumbers() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestLotteryService_GenerateRandomNumbers(t *testing.T) {
	svc := &Service{}

	for i := 0; i < 100; i++ {
		numbers, megaNumber := svc.generateRandomNumbers()

		// Check count
		if len(numbers) != DefaultMainNumberCount {
			t.Errorf("Expected %d numbers, got %d", DefaultMainNumberCount, len(numbers))
		}

		// Check uniqueness
		seen := make(map[int]bool)
		for _, n := range numbers {
			if seen[n] {
				t.Errorf("Duplicate number found: %d", n)
			}
			seen[n] = true

			// Check range
			if n < DefaultMainNumberMin || n > DefaultMainNumberMax {
				t.Errorf("Number %d out of range [%d, %d]", n, DefaultMainNumberMin, DefaultMainNumberMax)
			}
		}

		// Check mega number range
		if megaNumber < DefaultMegaNumberMin || megaNumber > DefaultMegaNumberMax {
			t.Errorf("Mega number %d out of range [%d, %d]", megaNumber, DefaultMegaNumberMin, DefaultMegaNumberMax)
		}

		// Check sorted
		for j := 1; j < len(numbers); j++ {
			if numbers[j] < numbers[j-1] {
				t.Errorf("Numbers not sorted: %v", numbers)
				break
			}
		}
	}
}

func TestLotteryService_CalculatePrize(t *testing.T) {
	svc := &Service{}
	prizePool := int64(1_000_000_000) // 10 GAS

	testCases := []struct {
		tier     int
		expected int64
	}{
		{1, 500_000_000},  // 50% of pool
		{2, 150_000_000},  // 15% of pool
		{3, 100_000_000},  // 10% of pool
		{4, 50_000_000},   // Fixed 0.5 GAS
		{5, 20_000_000},   // Fixed 0.2 GAS
		{6, 10_000_000},   // Fixed 0.1 GAS
		{7, 5_000_000},    // Fixed 0.05 GAS
		{8, 2_000_000},    // Fixed 0.02 GAS
		{9, 1_000_000},    // Fixed 0.01 GAS
		{0, 0},            // No prize
	}

	for _, tc := range testCases {
		prize := svc.calculatePrize(prizePool, tc.tier, 0, false)
		if prize != tc.expected {
			t.Errorf("calculatePrize(tier=%d) = %d, expected %d", tc.tier, prize, tc.expected)
		}
	}
}
