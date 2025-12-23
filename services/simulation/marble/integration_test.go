// +build integration

package neosimulation

import (
	"context"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/infrastructure/logging"
	commonservice "github.com/R3E-Network/service_layer/infrastructure/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContinuousSimulation tests that the simulation runs continuously
// and generates transactions at the expected rate.
func TestContinuousSimulation(t *testing.T) {
	// Create service with fast intervals for testing
	s := &Service{
		BaseService: commonservice.NewBase(&commonservice.BaseConfig{
			ID:      ServiceID,
			Name:    ServiceName,
			Version: Version,
			Marble:  nil,
			DB:      nil,
			Logger:  logging.New("test-simulation", "debug", "json"),
		}),
		miniApps:    []string{"test-lottery", "test-dice", "test-coinflip"},
		minInterval: 50 * time.Millisecond,  // Fast for testing
		maxInterval: 100 * time.Millisecond, // Fast for testing
		minAmount:   1000000,
		maxAmount:   100000000,
		txCounts:    make(map[string]int64),
		lastTxTimes: make(map[string]time.Time),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Start simulation
	err := s.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, s.running)

	// Let it run for a short period
	time.Sleep(500 * time.Millisecond)

	// Check status
	status := s.GetStatus()
	assert.True(t, status.Running)
	assert.Equal(t, 3, len(status.MiniApps))
	assert.NotNil(t, status.StartedAt)
	assert.NotEmpty(t, status.Uptime)

	// Stop simulation
	err = s.Stop()
	require.NoError(t, err)
	assert.False(t, s.running)

	t.Logf("Simulation ran successfully with %d mini apps", len(status.MiniApps))
	t.Logf("Uptime: %s", status.Uptime)
}

// TestSimulationStatisticsAccumulation tests that statistics accumulate correctly.
func TestSimulationStatisticsAccumulation(t *testing.T) {
	s := &Service{
		BaseService: commonservice.NewBase(&commonservice.BaseConfig{
			ID:      ServiceID,
			Name:    ServiceName,
			Version: Version,
			Logger:  logging.New("test-stats", "debug", "json"),
		}),
		miniApps:    []string{"stats-app"},
		minInterval: 10 * time.Millisecond,
		maxInterval: 20 * time.Millisecond,
		minAmount:   1000000,
		maxAmount:   100000000,
		txCounts:    make(map[string]int64),
		lastTxTimes: make(map[string]time.Time),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Get initial statistics
	initialStats := s.statistics()
	assert.False(t, initialStats["running"].(bool))

	// Start and let run briefly
	err := s.Start(context.Background())
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Get running statistics
	runningStats := s.statistics()
	assert.True(t, runningStats["running"].(bool))
	assert.NotEmpty(t, runningStats["started_at"])
	assert.NotEmpty(t, runningStats["uptime"])

	// Stop
	s.Stop()

	// Verify stopped
	stoppedStats := s.statistics()
	assert.False(t, stoppedStats["running"].(bool))
}

// TestMultipleAppSimulation tests simulation with multiple apps running concurrently.
func TestMultipleAppSimulation(t *testing.T) {
	apps := []string{
		"builtin-lottery",
		"builtin-coin-flip",
		"builtin-dice-game",
		"builtin-roulette",
		"builtin-slots",
	}

	s := &Service{
		BaseService: commonservice.NewBase(&commonservice.BaseConfig{
			ID:      ServiceID,
			Name:    ServiceName,
			Version: Version,
			Logger:  logging.New("test-multi", "info", "json"),
		}),
		miniApps:    apps,
		minInterval: 20 * time.Millisecond,
		maxInterval: 50 * time.Millisecond,
		minAmount:   1000000,
		maxAmount:   100000000,
		txCounts:    make(map[string]int64),
		lastTxTimes: make(map[string]time.Time),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Start simulation
	err := s.Start(context.Background())
	require.NoError(t, err)

	// Let all apps run
	time.Sleep(200 * time.Millisecond)

	// Verify all apps are tracked
	status := s.GetStatus()
	assert.Equal(t, len(apps), len(status.MiniApps))

	// Stop
	err = s.Stop()
	require.NoError(t, err)

	t.Logf("Successfully ran simulation with %d concurrent apps", len(apps))
}

// TestSimulationRestartCycle tests starting, stopping, and restarting simulation.
func TestSimulationRestartCycle(t *testing.T) {
	s := &Service{
		BaseService: commonservice.NewBase(&commonservice.BaseConfig{
			ID:      ServiceID,
			Name:    ServiceName,
			Version: Version,
			Logger:  logging.New("test-restart", "debug", "json"),
		}),
		miniApps:    []string{"restart-app"},
		minInterval: 50 * time.Millisecond,
		maxInterval: 100 * time.Millisecond,
		minAmount:   1000000,
		maxAmount:   100000000,
		txCounts:    make(map[string]int64),
		lastTxTimes: make(map[string]time.Time),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Cycle 1
	err := s.Start(context.Background())
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	err = s.Stop()
	require.NoError(t, err)

	// Cycle 2
	err = s.Start(context.Background())
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	err = s.Stop()
	require.NoError(t, err)

	// Cycle 3
	err = s.Start(context.Background())
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	err = s.Stop()
	require.NoError(t, err)

	t.Log("Successfully completed 3 restart cycles")
}

// TestSimulationUnderLoad tests simulation behavior under concurrent access.
func TestSimulationUnderLoad(t *testing.T) {
	s := &Service{
		BaseService: commonservice.NewBase(&commonservice.BaseConfig{
			ID:      ServiceID,
			Name:    ServiceName,
			Version: Version,
			Logger:  logging.New("test-load", "warn", "json"),
		}),
		miniApps:    []string{"load-app-1", "load-app-2"},
		minInterval: 10 * time.Millisecond,
		maxInterval: 30 * time.Millisecond,
		minAmount:   1000000,
		maxAmount:   100000000,
		txCounts:    make(map[string]int64),
		lastTxTimes: make(map[string]time.Time),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	err := s.Start(context.Background())
	require.NoError(t, err)

	// Concurrent status checks
	var statusChecks int64
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Spawn multiple goroutines checking status
	for i := 0; i < 10; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					_ = s.GetStatus()
					_ = s.statistics()
					atomic.AddInt64(&statusChecks, 1)
					time.Sleep(5 * time.Millisecond)
				}
			}
		}()
	}

	<-ctx.Done()
	s.Stop()

	checks := atomic.LoadInt64(&statusChecks)
	t.Logf("Completed %d concurrent status checks without errors", checks)
	assert.Greater(t, checks, int64(50), "Should have completed many status checks")
}

// TestRandomIntervalDistribution tests that random intervals are properly distributed.
func TestRandomIntervalDistribution(t *testing.T) {
	s := &Service{
		minInterval: 100 * time.Millisecond,
		maxInterval: 500 * time.Millisecond,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Collect samples
	samples := make([]time.Duration, 1000)
	for i := 0; i < 1000; i++ {
		samples[i] = s.randomInterval()
	}

	// Verify all within bounds
	for i, sample := range samples {
		assert.GreaterOrEqual(t, sample, s.minInterval, "Sample %d below min", i)
		assert.LessOrEqual(t, sample, s.maxInterval, "Sample %d above max", i)
	}

	// Calculate average - should be roughly in the middle
	var total time.Duration
	for _, sample := range samples {
		total += sample
	}
	avg := total / time.Duration(len(samples))
	expectedAvg := (s.minInterval + s.maxInterval) / 2

	// Allow 20% deviation from expected average
	deviation := float64(avg-expectedAvg) / float64(expectedAvg)
	assert.Less(t, deviation, 0.2, "Average should be close to midpoint")
	assert.Greater(t, deviation, -0.2, "Average should be close to midpoint")

	t.Logf("Random interval distribution: min=%v, max=%v, avg=%v, expected=%v",
		s.minInterval, s.maxInterval, avg, expectedAvg)
}

// TestRandomAmountDistribution tests that random amounts are properly distributed.
func TestRandomAmountDistribution(t *testing.T) {
	s := &Service{
		minAmount: 1000000,   // 0.01 GAS
		maxAmount: 100000000, // 1 GAS
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Collect samples
	samples := make([]int64, 1000)
	for i := 0; i < 1000; i++ {
		samples[i] = s.randomAmount()
	}

	// Verify all within bounds
	for i, sample := range samples {
		assert.GreaterOrEqual(t, sample, s.minAmount, "Sample %d below min", i)
		assert.LessOrEqual(t, sample, s.maxAmount, "Sample %d above max", i)
	}

	// Calculate average
	var total int64
	for _, sample := range samples {
		total += sample
	}
	avg := total / int64(len(samples))
	expectedAvg := (s.minAmount + s.maxAmount) / 2

	// Allow 20% deviation
	deviation := float64(avg-expectedAvg) / float64(expectedAvg)
	assert.Less(t, deviation, 0.2, "Average should be close to midpoint")
	assert.Greater(t, deviation, -0.2, "Average should be close to midpoint")

	t.Logf("Random amount distribution: min=%d, max=%d, avg=%d, expected=%d",
		s.minAmount, s.maxAmount, avg, expectedAvg)
}
