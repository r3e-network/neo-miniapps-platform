// Package neosimulation provides MiniApp workflow simulation.
package neosimulation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync/atomic"
	"time"
)

// MiniAppSimulator simulates all MiniApp workflows.
type MiniAppSimulator struct {
	invoker ContractInvokerInterface

	// Statistics per MiniApp
	lotteryTickets     int64
	lotteryDraws       int64
	coinFlipBets       int64
	coinFlipWins       int64
	diceGameBets       int64
	diceGameWins       int64
	scratchCardBuys    int64
	scratchCardWins    int64
	predictionBets     int64
	predictionResolves int64
	flashloanBorrows   int64
	flashloanRepays    int64
	priceQueries       int64
	simulationErrors   int64
}

// MiniAppConfig holds configuration for each MiniApp.
type MiniAppConfig struct {
	AppID       string
	Name        string
	Category    string
	Interval    time.Duration
	BetAmount   int64 // in 8 decimals (1 GAS = 100000000)
	Description string
}

// AllMiniApps returns configuration for all builtin MiniApps.
func AllMiniApps() []MiniAppConfig {
	return []MiniAppConfig{
		{
			AppID:       "builtin-lottery",
			Name:        "Neo Lottery",
			Category:    "gaming",
			Interval:    5 * time.Second,
			BetAmount:   10000000, // 0.1 GAS per ticket
			Description: "Buy lottery tickets, draw winners",
		},
		{
			AppID:       "builtin-coin-flip",
			Name:        "Neo Coin Flip",
			Category:    "gaming",
			Interval:    3 * time.Second,
			BetAmount:   5000000, // 0.05 GAS per flip
			Description: "50/50 coin flip, double or nothing",
		},
		{
			AppID:       "builtin-dice-game",
			Name:        "Neo Dice",
			Category:    "gaming",
			Interval:    4 * time.Second,
			BetAmount:   8000000, // 0.08 GAS per roll
			Description: "Roll dice, win up to 6x",
		},
		{
			AppID:       "builtin-scratch-card",
			Name:        "Neo Scratch Cards",
			Category:    "gaming",
			Interval:    6 * time.Second,
			BetAmount:   2000000, // 0.02 GAS per card
			Description: "Instant win scratch cards",
		},
		{
			AppID:       "builtin-prediction-market",
			Name:        "Neo Predictions",
			Category:    "defi",
			Interval:    10 * time.Second,
			BetAmount:   20000000, // 0.2 GAS per prediction
			Description: "Bet on price movements",
		},
		{
			AppID:       "builtin-flashloan",
			Name:        "Neo FlashLoan",
			Category:    "defi",
			Interval:    15 * time.Second,
			BetAmount:   100000000, // 1 GAS flash loan
			Description: "Instant borrow and repay",
		},
		{
			AppID:       "builtin-price-ticker",
			Name:        "Neo Price Ticker",
			Category:    "defi",
			Interval:    8 * time.Second,
			BetAmount:   0, // Read-only, no payment
			Description: "Query price feeds",
		},
	}
}

// NewMiniAppSimulator creates a new MiniApp simulator.
func NewMiniAppSimulator(invoker ContractInvokerInterface) *MiniAppSimulator {
	return &MiniAppSimulator{
		invoker: invoker,
	}
}

// GetStats returns simulation statistics for all MiniApps.
func (s *MiniAppSimulator) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"lottery": map[string]int64{
			"tickets_bought": atomic.LoadInt64(&s.lotteryTickets),
			"draws":          atomic.LoadInt64(&s.lotteryDraws),
		},
		"coin_flip": map[string]int64{
			"bets":  atomic.LoadInt64(&s.coinFlipBets),
			"wins":  atomic.LoadInt64(&s.coinFlipWins),
		},
		"dice_game": map[string]int64{
			"bets":  atomic.LoadInt64(&s.diceGameBets),
			"wins":  atomic.LoadInt64(&s.diceGameWins),
		},
		"scratch_card": map[string]int64{
			"cards_bought": atomic.LoadInt64(&s.scratchCardBuys),
			"wins":         atomic.LoadInt64(&s.scratchCardWins),
		},
		"prediction_market": map[string]int64{
			"bets":     atomic.LoadInt64(&s.predictionBets),
			"resolves": atomic.LoadInt64(&s.predictionResolves),
		},
		"flashloan": map[string]int64{
			"borrows": atomic.LoadInt64(&s.flashloanBorrows),
			"repays":  atomic.LoadInt64(&s.flashloanRepays),
		},
		"price_ticker": map[string]int64{
			"queries": atomic.LoadInt64(&s.priceQueries),
		},
		"errors": atomic.LoadInt64(&s.simulationErrors),
	}
}

// SimulateLottery simulates the lottery workflow:
// 1. Buy tickets (payment)
// 2. Request randomness for draw
// 3. Determine winner
func (s *MiniAppSimulator) SimulateLottery(ctx context.Context) error {
	appID := "builtin-lottery"
	ticketCount := randomInt(1, 5) // Buy 1-5 tickets
	amount := int64(ticketCount) * 10000000 // 0.1 GAS per ticket

	// Step 1: Buy tickets (payment to PaymentHub)
	memo := fmt.Sprintf("lottery:round:%d:tickets:%d", time.Now().Unix(), ticketCount)
	_, err := s.invoker.PayToApp(ctx, appID, amount, memo)
	if err != nil {
		atomic.AddInt64(&s.simulationErrors, 1)
		return fmt.Errorf("buy tickets: %w", err)
	}
	atomic.AddInt64(&s.lotteryTickets, int64(ticketCount))

	// Step 2: Request randomness for draw (every 5th ticket triggers a draw)
	if atomic.LoadInt64(&s.lotteryTickets)%5 == 0 {
		_, err = s.invoker.RecordRandomness(ctx)
		if err != nil {
			atomic.AddInt64(&s.simulationErrors, 1)
			return fmt.Errorf("draw randomness: %w", err)
		}
		atomic.AddInt64(&s.lotteryDraws, 1)
	}

	return nil
}

// SimulateCoinFlip simulates the coin flip workflow:
// 1. Place bet (payment)
// 2. Request randomness
// 3. Resolve outcome (50% win rate)
func (s *MiniAppSimulator) SimulateCoinFlip(ctx context.Context) error {
	appID := "builtin-coin-flip"
	amount := int64(5000000) // 0.05 GAS bet
	choice := randomInt(0, 1) // 0 = heads, 1 = tails

	// Step 1: Place bet
	memo := fmt.Sprintf("coinflip:bet:%d:choice:%d", time.Now().UnixNano(), choice)
	_, err := s.invoker.PayToApp(ctx, appID, amount, memo)
	if err != nil {
		atomic.AddInt64(&s.simulationErrors, 1)
		return fmt.Errorf("place bet: %w", err)
	}
	atomic.AddInt64(&s.coinFlipBets, 1)

	// Step 2: Request randomness and resolve
	_, err = s.invoker.RecordRandomness(ctx)
	if err != nil {
		atomic.AddInt64(&s.simulationErrors, 1)
		return fmt.Errorf("flip randomness: %w", err)
	}

	// Step 3: Simulate outcome (50% win rate)
	if randomInt(0, 1) == choice {
		atomic.AddInt64(&s.coinFlipWins, 1)
	}

	return nil
}

// SimulateDiceGame simulates the dice game workflow:
// 1. Place bet on number (1-6)
// 2. Request randomness
// 3. Roll dice and resolve
func (s *MiniAppSimulator) SimulateDiceGame(ctx context.Context) error {
	appID := "builtin-dice-game"
	amount := int64(8000000) // 0.08 GAS bet
	chosenNumber := randomInt(1, 6)

	// Step 1: Place bet
	memo := fmt.Sprintf("dice:bet:%d:number:%d", time.Now().UnixNano(), chosenNumber)
	_, err := s.invoker.PayToApp(ctx, appID, amount, memo)
	if err != nil {
		atomic.AddInt64(&s.simulationErrors, 1)
		return fmt.Errorf("place bet: %w", err)
	}
	atomic.AddInt64(&s.diceGameBets, 1)

	// Step 2: Request randomness and roll
	_, err = s.invoker.RecordRandomness(ctx)
	if err != nil {
		atomic.AddInt64(&s.simulationErrors, 1)
		return fmt.Errorf("roll randomness: %w", err)
	}

	// Step 3: Simulate outcome (1/6 win rate)
	rolledNumber := randomInt(1, 6)
	if rolledNumber == chosenNumber {
		atomic.AddInt64(&s.diceGameWins, 1)
	}

	return nil
}

// SimulateScratchCard simulates the scratch card workflow:
// 1. Buy scratch card (payment)
// 2. Request randomness for reveal
// 3. Reveal and determine prize
func (s *MiniAppSimulator) SimulateScratchCard(ctx context.Context) error {
	appID := "builtin-scratch-card"
	cardType := randomInt(1, 3) // 3 card types with different prices
	amount := int64(cardType) * 2000000 // 0.02-0.06 GAS per card

	// Step 1: Buy card
	memo := fmt.Sprintf("scratch:buy:%d:type:%d", time.Now().UnixNano(), cardType)
	_, err := s.invoker.PayToApp(ctx, appID, amount, memo)
	if err != nil {
		atomic.AddInt64(&s.simulationErrors, 1)
		return fmt.Errorf("buy card: %w", err)
	}
	atomic.AddInt64(&s.scratchCardBuys, 1)

	// Step 2: Request randomness for reveal
	_, err = s.invoker.RecordRandomness(ctx)
	if err != nil {
		atomic.AddInt64(&s.simulationErrors, 1)
		return fmt.Errorf("reveal randomness: %w", err)
	}

	// Step 3: Simulate outcome (20% win rate)
	if randomInt(1, 5) == 1 {
		atomic.AddInt64(&s.scratchCardWins, 1)
	}

	return nil
}

// SimulatePredictionMarket simulates the prediction market workflow:
// 1. Query current price from PriceFeed
// 2. Place prediction bet (up/down)
// 3. Wait and resolve with new price
func (s *MiniAppSimulator) SimulatePredictionMarket(ctx context.Context) error {
	appID := "builtin-prediction-market"
	amount := int64(20000000) // 0.2 GAS bet
	symbol := []string{"BTCUSD", "ETHUSD", "NEOUSD", "GASUSD"}[randomInt(0, 3)]
	prediction := randomInt(0, 1) // 0 = down, 1 = up

	// Step 1: Query current price (simulated by updating price feed)
	_, err := s.invoker.UpdatePriceFeed(ctx, symbol)
	if err != nil {
		atomic.AddInt64(&s.simulationErrors, 1)
		return fmt.Errorf("query price: %w", err)
	}

	// Step 2: Place prediction bet
	memo := fmt.Sprintf("predict:%s:%d:dir:%d", symbol, time.Now().UnixNano(), prediction)
	_, err = s.invoker.PayToApp(ctx, appID, amount, memo)
	if err != nil {
		atomic.AddInt64(&s.simulationErrors, 1)
		return fmt.Errorf("place prediction: %w", err)
	}
	atomic.AddInt64(&s.predictionBets, 1)

	// Step 3: Resolve prediction (update price again to simulate time passing)
	_, err = s.invoker.UpdatePriceFeed(ctx, symbol)
	if err != nil {
		atomic.AddInt64(&s.simulationErrors, 1)
		return fmt.Errorf("resolve price: %w", err)
	}
	atomic.AddInt64(&s.predictionResolves, 1)

	return nil
}

// SimulateFlashLoan simulates the flash loan workflow:
// 1. Borrow GAS (payment request)
// 2. Execute arbitrage (simulated)
// 3. Repay with fee
func (s *MiniAppSimulator) SimulateFlashLoan(ctx context.Context) error {
	appID := "builtin-flashloan"
	borrowAmount := int64(100000000) // 1 GAS loan
	fee := borrowAmount * 9 / 10000  // 0.09% fee

	// Step 1: Request flash loan (borrow)
	memo := fmt.Sprintf("flashloan:borrow:%d:amount:%d", time.Now().UnixNano(), borrowAmount)
	_, err := s.invoker.PayToApp(ctx, appID, fee, memo) // Pay fee upfront
	if err != nil {
		atomic.AddInt64(&s.simulationErrors, 1)
		return fmt.Errorf("borrow: %w", err)
	}
	atomic.AddInt64(&s.flashloanBorrows, 1)

	// Step 2: Simulate arbitrage execution (just a delay)
	time.Sleep(100 * time.Millisecond)

	// Step 3: Repay loan (simulated - in real scenario this happens atomically)
	repayMemo := fmt.Sprintf("flashloan:repay:%d:amount:%d", time.Now().UnixNano(), borrowAmount)
	_, err = s.invoker.PayToApp(ctx, appID, borrowAmount, repayMemo)
	if err != nil {
		atomic.AddInt64(&s.simulationErrors, 1)
		return fmt.Errorf("repay: %w", err)
	}
	atomic.AddInt64(&s.flashloanRepays, 1)

	return nil
}

// SimulatePriceTicker simulates the price ticker workflow:
// 1. Query multiple price feeds
// 2. Display prices (simulated)
func (s *MiniAppSimulator) SimulatePriceTicker(ctx context.Context) error {
	symbols := []string{"BTCUSD", "ETHUSD", "NEOUSD", "GASUSD"}

	// Query all price feeds
	for _, symbol := range symbols {
		_, err := s.invoker.UpdatePriceFeed(ctx, symbol)
		if err != nil {
			atomic.AddInt64(&s.simulationErrors, 1)
			return fmt.Errorf("query %s: %w", symbol, err)
		}
		atomic.AddInt64(&s.priceQueries, 1)
		time.Sleep(200 * time.Millisecond) // Small delay between queries
	}

	return nil
}

// Helper function to generate random int in range [min, max]
func randomInt(min, max int) int {
	if min >= max {
		return min
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	return min + int(n.Int64())
}

// generateGameID generates a unique game ID
func generateGameID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
