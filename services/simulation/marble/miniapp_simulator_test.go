// Package neosimulation provides simulation service for automated transaction testing.
package neosimulation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// AllMiniApps Tests
// =============================================================================

func TestAllMiniApps(t *testing.T) {
	apps := AllMiniApps()

	assert.Len(t, apps, 7)

	// Verify all expected apps are present
	appIDs := make(map[string]bool)
	for _, app := range apps {
		appIDs[app.AppID] = true
	}

	assert.True(t, appIDs["builtin-lottery"])
	assert.True(t, appIDs["builtin-coin-flip"])
	assert.True(t, appIDs["builtin-dice-game"])
	assert.True(t, appIDs["builtin-scratch-card"])
	assert.True(t, appIDs["builtin-prediction-market"])
	assert.True(t, appIDs["builtin-flashloan"])
	assert.True(t, appIDs["builtin-price-ticker"])
}

func TestAllMiniApps_Categories(t *testing.T) {
	apps := AllMiniApps()

	gaming := 0
	defi := 0
	for _, app := range apps {
		switch app.Category {
		case "gaming":
			gaming++
		case "defi":
			defi++
		}
	}

	assert.Equal(t, 4, gaming) // lottery, coin-flip, dice-game, scratch-card
	assert.Equal(t, 3, defi)   // prediction-market, flashloan, price-ticker
}

func TestAllMiniApps_BetAmounts(t *testing.T) {
	apps := AllMiniApps()

	for _, app := range apps {
		if app.AppID == "builtin-price-ticker" {
			assert.Equal(t, int64(0), app.BetAmount, "price-ticker should have 0 bet amount")
		} else {
			assert.Greater(t, app.BetAmount, int64(0), "%s should have positive bet amount", app.AppID)
		}
	}
}

// =============================================================================
// NewMiniAppSimulator Tests
// =============================================================================

func TestNewMiniAppSimulator(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	sim := NewMiniAppSimulator(mockInvoker)

	require.NotNil(t, sim)
	assert.NotNil(t, sim.invoker)
}

// =============================================================================
// SimulateLottery Tests
// =============================================================================

func TestMiniAppSimulator_SimulateLottery_Success(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulateLottery(ctx)

	require.NoError(t, err)

	// Verify PayToApp was called for lottery
	payToAppCalls := mockInvoker.getPayToAppCalls()
	require.Len(t, payToAppCalls, 1)
	assert.Equal(t, "builtin-lottery", payToAppCalls[0].AppID)
	assert.Contains(t, payToAppCalls[0].Memo, "lottery:round:")

	// Verify stats updated
	stats := sim.GetStats()
	lotteryStats := stats["lottery"].(map[string]int64)
	assert.Greater(t, lotteryStats["tickets_bought"], int64(0))
}

func TestMiniAppSimulator_SimulateLottery_PaymentError(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	mockInvoker.payToAppErr = errors.New("payment failed")
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulateLottery(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "buy tickets")

	// Verify error counter incremented
	stats := sim.GetStats()
	assert.Equal(t, int64(1), stats["errors"])
}

func TestMiniAppSimulator_SimulateLottery_DrawTriggered(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()

	// Run multiple times to trigger a draw (every 5 tickets)
	for i := 0; i < 10; i++ {
		_ = sim.SimulateLottery(ctx)
	}

	// Verify RecordRandomness was called for draws
	randomnessCalls := mockInvoker.getRecordRandomnessCalls()
	assert.Greater(t, len(randomnessCalls), 0)

	stats := sim.GetStats()
	lotteryStats := stats["lottery"].(map[string]int64)
	assert.Greater(t, lotteryStats["draws"], int64(0))
}

// =============================================================================
// SimulateCoinFlip Tests
// =============================================================================

func TestMiniAppSimulator_SimulateCoinFlip_Success(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulateCoinFlip(ctx)

	require.NoError(t, err)

	// Verify PayToApp was called
	payToAppCalls := mockInvoker.getPayToAppCalls()
	require.Len(t, payToAppCalls, 1)
	assert.Equal(t, "builtin-coin-flip", payToAppCalls[0].AppID)
	assert.Equal(t, int64(5000000), payToAppCalls[0].Amount) // 0.05 GAS

	// Verify RecordRandomness was called
	randomnessCalls := mockInvoker.getRecordRandomnessCalls()
	assert.Len(t, randomnessCalls, 1)

	// Verify stats updated
	stats := sim.GetStats()
	coinFlipStats := stats["coin_flip"].(map[string]int64)
	assert.Equal(t, int64(1), coinFlipStats["bets"])
}

func TestMiniAppSimulator_SimulateCoinFlip_PaymentError(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	mockInvoker.payToAppErr = errors.New("payment failed")
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulateCoinFlip(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "place bet")
}

func TestMiniAppSimulator_SimulateCoinFlip_RandomnessError(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	mockInvoker.recordRandomnessErr = errors.New("randomness failed")
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulateCoinFlip(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "flip randomness")
}

// =============================================================================
// SimulateDiceGame Tests
// =============================================================================

func TestMiniAppSimulator_SimulateDiceGame_Success(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulateDiceGame(ctx)

	require.NoError(t, err)

	// Verify PayToApp was called
	payToAppCalls := mockInvoker.getPayToAppCalls()
	require.Len(t, payToAppCalls, 1)
	assert.Equal(t, "builtin-dice-game", payToAppCalls[0].AppID)
	assert.Equal(t, int64(8000000), payToAppCalls[0].Amount) // 0.08 GAS

	// Verify RecordRandomness was called
	randomnessCalls := mockInvoker.getRecordRandomnessCalls()
	assert.Len(t, randomnessCalls, 1)

	// Verify stats updated
	stats := sim.GetStats()
	diceStats := stats["dice_game"].(map[string]int64)
	assert.Equal(t, int64(1), diceStats["bets"])
}

func TestMiniAppSimulator_SimulateDiceGame_PaymentError(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	mockInvoker.payToAppErr = errors.New("payment failed")
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulateDiceGame(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "place bet")
}

// =============================================================================
// SimulateScratchCard Tests
// =============================================================================

func TestMiniAppSimulator_SimulateScratchCard_Success(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulateScratchCard(ctx)

	require.NoError(t, err)

	// Verify PayToApp was called
	payToAppCalls := mockInvoker.getPayToAppCalls()
	require.Len(t, payToAppCalls, 1)
	assert.Equal(t, "builtin-scratch-card", payToAppCalls[0].AppID)

	// Verify RecordRandomness was called
	randomnessCalls := mockInvoker.getRecordRandomnessCalls()
	assert.Len(t, randomnessCalls, 1)

	// Verify stats updated
	stats := sim.GetStats()
	scratchStats := stats["scratch_card"].(map[string]int64)
	assert.Equal(t, int64(1), scratchStats["cards_bought"])
}

func TestMiniAppSimulator_SimulateScratchCard_PaymentError(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	mockInvoker.payToAppErr = errors.New("payment failed")
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulateScratchCard(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "buy card")
}

// =============================================================================
// SimulatePredictionMarket Tests
// =============================================================================

func TestMiniAppSimulator_SimulatePredictionMarket_Success(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulatePredictionMarket(ctx)

	require.NoError(t, err)

	// Verify UpdatePriceFeed was called twice (query + resolve)
	priceFeedCalls := mockInvoker.getUpdatePriceFeedCalls()
	assert.Len(t, priceFeedCalls, 2)

	// Verify PayToApp was called
	payToAppCalls := mockInvoker.getPayToAppCalls()
	require.Len(t, payToAppCalls, 1)
	assert.Equal(t, "builtin-prediction-market", payToAppCalls[0].AppID)
	assert.Equal(t, int64(20000000), payToAppCalls[0].Amount) // 0.2 GAS

	// Verify stats updated
	stats := sim.GetStats()
	predictionStats := stats["prediction_market"].(map[string]int64)
	assert.Equal(t, int64(1), predictionStats["bets"])
	assert.Equal(t, int64(1), predictionStats["resolves"])
}

func TestMiniAppSimulator_SimulatePredictionMarket_PriceQueryError(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	mockInvoker.updatePriceFeedErr = errors.New("price feed error")
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulatePredictionMarket(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query price")
}

func TestMiniAppSimulator_SimulatePredictionMarket_PaymentError(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	mockInvoker.payToAppErr = errors.New("payment failed")
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulatePredictionMarket(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "place prediction")
}

// =============================================================================
// SimulateFlashLoan Tests
// =============================================================================

func TestMiniAppSimulator_SimulateFlashLoan_Success(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulateFlashLoan(ctx)

	require.NoError(t, err)

	// Verify PayToApp was called twice (borrow fee + repay)
	payToAppCalls := mockInvoker.getPayToAppCalls()
	require.Len(t, payToAppCalls, 2)
	assert.Equal(t, "builtin-flashloan", payToAppCalls[0].AppID)
	assert.Equal(t, "builtin-flashloan", payToAppCalls[1].AppID)
	assert.Contains(t, payToAppCalls[0].Memo, "borrow")
	assert.Contains(t, payToAppCalls[1].Memo, "repay")

	// Verify stats updated
	stats := sim.GetStats()
	flashloanStats := stats["flashloan"].(map[string]int64)
	assert.Equal(t, int64(1), flashloanStats["borrows"])
	assert.Equal(t, int64(1), flashloanStats["repays"])
}

func TestMiniAppSimulator_SimulateFlashLoan_BorrowError(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	mockInvoker.payToAppErr = errors.New("borrow failed")
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulateFlashLoan(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "borrow")
}

// =============================================================================
// SimulatePriceTicker Tests
// =============================================================================

func TestMiniAppSimulator_SimulatePriceTicker_Success(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulatePriceTicker(ctx)

	require.NoError(t, err)

	// Verify UpdatePriceFeed was called for all 4 symbols
	priceFeedCalls := mockInvoker.getUpdatePriceFeedCalls()
	assert.Len(t, priceFeedCalls, 4)

	// Verify all symbols were queried
	symbols := make(map[string]bool)
	for _, call := range priceFeedCalls {
		symbols[call.Symbol] = true
	}
	assert.True(t, symbols["BTCUSD"])
	assert.True(t, symbols["ETHUSD"])
	assert.True(t, symbols["NEOUSD"])
	assert.True(t, symbols["GASUSD"])

	// Verify stats updated
	stats := sim.GetStats()
	priceStats := stats["price_ticker"].(map[string]int64)
	assert.Equal(t, int64(4), priceStats["queries"])
}

func TestMiniAppSimulator_SimulatePriceTicker_PriceError(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	mockInvoker.updatePriceFeedErr = errors.New("price feed error")
	sim := NewMiniAppSimulator(mockInvoker)

	ctx := context.Background()
	err := sim.SimulatePriceTicker(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query")
}

// =============================================================================
// GetStats Tests
// =============================================================================

func TestMiniAppSimulator_GetStats(t *testing.T) {
	mockInvoker := newMockContractInvoker()
	sim := NewMiniAppSimulator(mockInvoker)

	stats := sim.GetStats()

	// Verify all stat categories exist
	assert.Contains(t, stats, "lottery")
	assert.Contains(t, stats, "coin_flip")
	assert.Contains(t, stats, "dice_game")
	assert.Contains(t, stats, "scratch_card")
	assert.Contains(t, stats, "prediction_market")
	assert.Contains(t, stats, "flashloan")
	assert.Contains(t, stats, "price_ticker")
	assert.Contains(t, stats, "errors")
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestRandomInt(t *testing.T) {
	// Test range [1, 6] (dice roll)
	for i := 0; i < 100; i++ {
		result := randomInt(1, 6)
		assert.GreaterOrEqual(t, result, 1)
		assert.LessOrEqual(t, result, 6)
	}

	// Test range [0, 1] (coin flip)
	for i := 0; i < 100; i++ {
		result := randomInt(0, 1)
		assert.GreaterOrEqual(t, result, 0)
		assert.LessOrEqual(t, result, 1)
	}

	// Test edge case: min == max
	result := randomInt(5, 5)
	assert.Equal(t, 5, result)

	// Test edge case: min > max (should return min)
	result = randomInt(10, 5)
	assert.Equal(t, 10, result)
}

func TestGenerateGameID(t *testing.T) {
	id1 := generateGameID()
	id2 := generateGameID()

	assert.Len(t, id1, 16) // 8 bytes = 16 hex chars
	assert.Len(t, id2, 16)
	assert.NotEqual(t, id1, id2)
}

// =============================================================================
// Integration-style Tests (verify account pool usage patterns)
// =============================================================================

func TestMiniAppSimulator_VerifyPoolAccountUsage(t *testing.T) {
	// This test verifies that all MiniApps that make payments use pool accounts
	// (via PayToApp which uses InvokeContract with Global scope)

	mockInvoker := newMockContractInvoker()
	sim := NewMiniAppSimulator(mockInvoker)
	ctx := context.Background()

	// Run all MiniApp simulations
	_ = sim.SimulateLottery(ctx)
	_ = sim.SimulateCoinFlip(ctx)
	_ = sim.SimulateDiceGame(ctx)
	_ = sim.SimulateScratchCard(ctx)
	_ = sim.SimulatePredictionMarket(ctx)
	_ = sim.SimulateFlashLoan(ctx)
	_ = sim.SimulatePriceTicker(ctx)

	// Verify PayToApp was called for all payment-based MiniApps
	payToAppCalls := mockInvoker.getPayToAppCalls()

	// Expected: lottery(1) + coin-flip(1) + dice(1) + scratch(1) + prediction(1) + flashloan(2) = 7
	assert.GreaterOrEqual(t, len(payToAppCalls), 7)

	// Verify all expected apps made payments
	appPayments := make(map[string]int)
	for _, call := range payToAppCalls {
		appPayments[call.AppID]++
	}

	assert.Greater(t, appPayments["builtin-lottery"], 0)
	assert.Greater(t, appPayments["builtin-coin-flip"], 0)
	assert.Greater(t, appPayments["builtin-dice-game"], 0)
	assert.Greater(t, appPayments["builtin-scratch-card"], 0)
	assert.Greater(t, appPayments["builtin-prediction-market"], 0)
	assert.Equal(t, 2, appPayments["builtin-flashloan"]) // borrow + repay
}

func TestMiniAppSimulator_VerifyMasterAccountUsage(t *testing.T) {
	// This test verifies that PriceFeed and RandomnessLog use master account
	// (via UpdatePriceFeed and RecordRandomness which use InvokeMaster)

	mockInvoker := newMockContractInvoker()
	sim := NewMiniAppSimulator(mockInvoker)
	ctx := context.Background()

	// Run simulations that use master account
	_ = sim.SimulatePriceTicker(ctx)      // Uses UpdatePriceFeed
	_ = sim.SimulatePredictionMarket(ctx) // Uses UpdatePriceFeed
	_ = sim.SimulateLottery(ctx)          // May use RecordRandomness
	_ = sim.SimulateCoinFlip(ctx)         // Uses RecordRandomness

	// Verify UpdatePriceFeed was called (uses master account)
	priceFeedCalls := mockInvoker.getUpdatePriceFeedCalls()
	assert.Greater(t, len(priceFeedCalls), 0)

	// Verify RecordRandomness was called (uses master account)
	randomnessCalls := mockInvoker.getRecordRandomnessCalls()
	assert.Greater(t, len(randomnessCalls), 0)
}
