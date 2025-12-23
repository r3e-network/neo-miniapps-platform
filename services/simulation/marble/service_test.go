package neosimulation

import (
	"context"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/infrastructure/logging"
	commonservice "github.com/R3E-Network/service_layer/infrastructure/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBaseService creates a minimal base service for testing.
func mockBaseService() *commonservice.BaseService {
	return commonservice.NewBase(&commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  nil,
		DB:      nil,
		Logger:  logging.New("test", "debug", "json"),
	})
}

// TestNew tests service creation.
func TestNew(t *testing.T) {
	t.Run("requires marble", func(t *testing.T) {
		_, err := New(Config{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "marble is required")
	})

	t.Run("invalid marble type", func(t *testing.T) {
		_, err := New(Config{
			Marble: "invalid",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid marble type")
	})

	t.Run("invalid DB type", func(t *testing.T) {
		// This would require a valid marble mock
		t.Skip("requires mock marble setup")
	})
}

// TestRandomInterval tests random interval generation.
func TestRandomInterval(t *testing.T) {
	s := &Service{
		minInterval: 1000 * time.Millisecond,
		maxInterval: 3000 * time.Millisecond,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	for i := 0; i < 100; i++ {
		interval := s.randomInterval()
		assert.GreaterOrEqual(t, interval, s.minInterval)
		assert.LessOrEqual(t, interval, s.maxInterval)
	}
}

// TestRandomIntervalEqualMinMax tests when min equals max.
func TestRandomIntervalEqualMinMax(t *testing.T) {
	s := &Service{
		minInterval: 1000 * time.Millisecond,
		maxInterval: 1000 * time.Millisecond,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	interval := s.randomInterval()
	assert.Equal(t, s.minInterval, interval)
}

// TestRandomIntervalMinGreaterThanMax tests when min > max.
func TestRandomIntervalMinGreaterThanMax(t *testing.T) {
	s := &Service{
		minInterval: 3000 * time.Millisecond,
		maxInterval: 1000 * time.Millisecond,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	interval := s.randomInterval()
	assert.Equal(t, s.minInterval, interval)
}

// TestRandomAmount tests random amount generation.
func TestRandomAmount(t *testing.T) {
	s := &Service{
		minAmount: 1000000,   // 0.01 GAS
		maxAmount: 100000000, // 1 GAS
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	for i := 0; i < 100; i++ {
		amount := s.randomAmount()
		assert.GreaterOrEqual(t, amount, s.minAmount)
		assert.LessOrEqual(t, amount, s.maxAmount)
	}
}

// TestRandomAmountEqualMinMax tests when min equals max.
func TestRandomAmountEqualMinMax(t *testing.T) {
	s := &Service{
		minAmount: 1000000,
		maxAmount: 1000000,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	amount := s.randomAmount()
	assert.Equal(t, s.minAmount, amount)
}

// TestRandomAmountMinGreaterThanMax tests when min > max.
func TestRandomAmountMinGreaterThanMax(t *testing.T) {
	s := &Service{
		minAmount: 100000000,
		maxAmount: 1000000,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	amount := s.randomAmount()
	assert.Equal(t, s.minAmount, amount)
}

// TestStartStop tests starting and stopping simulation.
func TestStartStop(t *testing.T) {
	s := &Service{
		BaseService: mockBaseService(),
		miniApps:    []string{"test-app"},
		minInterval: 100 * time.Millisecond,
		maxInterval: 200 * time.Millisecond,
		txCounts:    make(map[string]int64),
		lastTxTimes: make(map[string]time.Time),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Test start
	err := s.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, s.running)
	assert.NotNil(t, s.startedAt)
	assert.NotNil(t, s.stopCh)

	// Test start when already running
	err = s.Start(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Test stop
	err = s.Stop()
	require.NoError(t, err)
	assert.False(t, s.running)
	assert.Nil(t, s.startedAt)

	// Test stop when not running
	err = s.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

// TestStartMultipleApps tests starting simulation with multiple apps.
func TestStartMultipleApps(t *testing.T) {
	s := &Service{
		BaseService: mockBaseService(),
		miniApps:    []string{"app1", "app2", "app3"},
		minInterval: 100 * time.Millisecond,
		maxInterval: 200 * time.Millisecond,
		txCounts:    make(map[string]int64),
		lastTxTimes: make(map[string]time.Time),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	err := s.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, s.running)

	// Give goroutines time to start
	time.Sleep(50 * time.Millisecond)

	err = s.Stop()
	require.NoError(t, err)
	assert.False(t, s.running)
}

// TestGetStatus tests status retrieval.
func TestGetStatus(t *testing.T) {
	now := time.Now()
	s := &Service{
		running:     true,
		miniApps:    []string{"app1", "app2"},
		minInterval: 1000 * time.Millisecond,
		maxInterval: 3000 * time.Millisecond,
		startedAt:   &now,
		txCounts: map[string]int64{
			"app1": 10,
			"app2": 20,
		},
		lastTxTimes: map[string]time.Time{
			"app1": now.Add(-5 * time.Second),
			"app2": now.Add(-3 * time.Second),
		},
	}

	status := s.GetStatus()
	assert.True(t, status.Running)
	assert.Equal(t, []string{"app1", "app2"}, status.MiniApps)
	assert.Equal(t, 1000, status.MinIntervalMS)
	assert.Equal(t, 3000, status.MaxIntervalMS)
	assert.Equal(t, int64(10), status.TxCounts["app1"])
	assert.Equal(t, int64(20), status.TxCounts["app2"])
	assert.NotEmpty(t, status.LastTxTimes["app1"])
	assert.NotEmpty(t, status.LastTxTimes["app2"])
	assert.NotNil(t, status.StartedAt)
	assert.NotEmpty(t, status.Uptime)
}

// TestGetStatusNotRunning tests status when simulation is not running.
func TestGetStatusNotRunning(t *testing.T) {
	s := &Service{
		running:     false,
		miniApps:    []string{"app1"},
		minInterval: 1000 * time.Millisecond,
		maxInterval: 3000 * time.Millisecond,
		startedAt:   nil,
		txCounts:    make(map[string]int64),
		lastTxTimes: make(map[string]time.Time),
	}

	status := s.GetStatus()
	assert.False(t, status.Running)
	assert.Nil(t, status.StartedAt)
	assert.Empty(t, status.Uptime)
}

// TestStatistics tests statistics generation.
func TestStatistics(t *testing.T) {
	now := time.Now()
	s := &Service{
		running:     true,
		miniApps:    []string{"app1"},
		minInterval: 1000 * time.Millisecond,
		maxInterval: 3000 * time.Millisecond,
		minAmount:   1000000,
		maxAmount:   100000000,
		startedAt:   &now,
		txCounts: map[string]int64{
			"app1": 5,
		},
	}

	stats := s.statistics()
	assert.True(t, stats["running"].(bool))
	assert.Equal(t, []string{"app1"}, stats["mini_apps"])
	assert.Equal(t, int64(1000), stats["min_interval_ms"])
	assert.Equal(t, int64(3000), stats["max_interval_ms"])
	assert.Equal(t, int64(1000000), stats["min_amount"])
	assert.Equal(t, int64(100000000), stats["max_amount"])
	assert.NotEmpty(t, stats["started_at"])
	assert.NotEmpty(t, stats["uptime"])
}

// TestStatisticsNotStarted tests statistics when not started.
func TestStatisticsNotStarted(t *testing.T) {
	s := &Service{
		running:     false,
		miniApps:    []string{"app1"},
		minInterval: 1000 * time.Millisecond,
		maxInterval: 3000 * time.Millisecond,
		minAmount:   1000000,
		maxAmount:   100000000,
		startedAt:   nil,
		txCounts:    make(map[string]int64),
	}

	stats := s.statistics()
	assert.False(t, stats["running"].(bool))
	_, hasStartedAt := stats["started_at"]
	assert.False(t, hasStartedAt)
	_, hasUptime := stats["uptime"]
	assert.False(t, hasUptime)
}

// TestRecordTransactionNoDatabase tests transaction recording without database.
func TestRecordTransactionNoDatabase(t *testing.T) {
	s := &Service{
		BaseService: mockBaseService(),
		db:          nil,
	}

	tx := &SimulationTx{
		AppID:          "test-app",
		AccountAddress: "NAddr123",
		TxType:         "payGAS",
		Amount:         1000000,
		Status:         "simulated",
		CreatedAt:      time.Now(),
	}

	// Should not error when db is nil - just logs
	err := s.recordTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

// TestConcurrentStatusAccess tests concurrent access to status.
func TestConcurrentStatusAccess(t *testing.T) {
	now := time.Now()
	s := &Service{
		running:     true,
		miniApps:    []string{"app1"},
		minInterval: 1000 * time.Millisecond,
		maxInterval: 3000 * time.Millisecond,
		startedAt:   &now,
		txCounts:    make(map[string]int64),
		lastTxTimes: make(map[string]time.Time),
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.GetStatus()
		}()
	}
	wg.Wait()
}

// TestConcurrentStatisticsAccess tests concurrent access to statistics.
func TestConcurrentStatisticsAccess(t *testing.T) {
	now := time.Now()
	s := &Service{
		running:     true,
		miniApps:    []string{"app1"},
		minInterval: 1000 * time.Millisecond,
		maxInterval: 3000 * time.Millisecond,
		minAmount:   1000000,
		maxAmount:   100000000,
		startedAt:   &now,
		txCounts:    make(map[string]int64),
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.statistics()
		}()
	}
	wg.Wait()
}

// TestConcurrentRandomInterval tests concurrent random interval generation.
func TestConcurrentRandomInterval(t *testing.T) {
	s := &Service{
		minInterval: 1000 * time.Millisecond,
		maxInterval: 3000 * time.Millisecond,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			interval := s.randomInterval()
			assert.GreaterOrEqual(t, interval, s.minInterval)
			assert.LessOrEqual(t, interval, s.maxInterval)
		}()
	}
	wg.Wait()
}

// TestConcurrentRandomAmount tests concurrent random amount generation.
func TestConcurrentRandomAmount(t *testing.T) {
	s := &Service{
		minAmount: 1000000,
		maxAmount: 100000000,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			amount := s.randomAmount()
			assert.GreaterOrEqual(t, amount, s.minAmount)
			assert.LessOrEqual(t, amount, s.maxAmount)
		}()
	}
	wg.Wait()
}

// TestHTTPHandlers tests HTTP route handlers.
func TestHTTPHandlers(t *testing.T) {
	s := &Service{
		BaseService: mockBaseService(),
		running:     false,
		miniApps:    []string{"app1"},
		minInterval: 1000 * time.Millisecond,
		maxInterval: 3000 * time.Millisecond,
		minAmount:   1000000,
		maxAmount:   100000000,
		txCounts:    make(map[string]int64),
		lastTxTimes: make(map[string]time.Time),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Register routes
	s.registerRoutes()

	t.Run("GET /status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("POST /start with JSON body", func(t *testing.T) {
		body := strings.NewReader(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/start", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, s.running)
	})

	t.Run("POST /start when already running", func(t *testing.T) {
		body := strings.NewReader(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/start", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("POST /stop", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/stop", nil)
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.False(t, s.running)
	})

	t.Run("POST /stop when not running", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/stop", nil)
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestHTTPHandlersWithConfig tests HTTP handlers with configuration override.
func TestHTTPHandlersWithConfig(t *testing.T) {
	s := &Service{
		BaseService: mockBaseService(),
		running:     false,
		miniApps:    []string{"app1"},
		minInterval: 1000 * time.Millisecond,
		maxInterval: 3000 * time.Millisecond,
		minAmount:   1000000,
		maxAmount:   100000000,
		txCounts:    make(map[string]int64),
		lastTxTimes: make(map[string]time.Time),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	s.registerRoutes()

	t.Run("POST /start with custom mini_apps", func(t *testing.T) {
		body := strings.NewReader(`{"mini_apps":["custom-app1","custom-app2"]}`)
		req := httptest.NewRequest(http.MethodPost, "/start", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, s.running)
		assert.Equal(t, []string{"custom-app1", "custom-app2"}, s.miniApps)
		s.Stop()
	})

	t.Run("POST /start with custom intervals", func(t *testing.T) {
		body := strings.NewReader(`{"min_interval_ms":500,"max_interval_ms":1500}`)
		req := httptest.NewRequest(http.MethodPost, "/start", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, s.running)
		assert.Equal(t, 500*time.Millisecond, s.minInterval)
		assert.Equal(t, 1500*time.Millisecond, s.maxInterval)
		s.Stop()
	})

	t.Run("POST /start with all config options", func(t *testing.T) {
		body := strings.NewReader(`{"mini_apps":["full-config-app"],"min_interval_ms":200,"max_interval_ms":800}`)
		req := httptest.NewRequest(http.MethodPost, "/start", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, s.running)
		assert.Equal(t, []string{"full-config-app"}, s.miniApps)
		assert.Equal(t, 200*time.Millisecond, s.minInterval)
		assert.Equal(t, 800*time.Millisecond, s.maxInterval)
		s.Stop()
	})

	t.Run("POST /start with invalid JSON", func(t *testing.T) {
		body := strings.NewReader(`{invalid json}`)
		req := httptest.NewRequest(http.MethodPost, "/start", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestSimulationTxStruct tests SimulationTx struct.
func TestSimulationTxStruct(t *testing.T) {
	now := time.Now()
	tx := &SimulationTx{
		ID:             123,
		AppID:          "test-app",
		AccountAddress: "NAddr123",
		TxType:         "payGAS",
		Amount:         1000000,
		Status:         "simulated",
		CreatedAt:      now,
	}

	assert.Equal(t, int64(123), tx.ID)
	assert.Equal(t, "test-app", tx.AppID)
	assert.Equal(t, "NAddr123", tx.AccountAddress)
	assert.Equal(t, "payGAS", tx.TxType)
	assert.Equal(t, int64(1000000), tx.Amount)
	assert.Equal(t, "simulated", tx.Status)
	assert.Equal(t, now, tx.CreatedAt)
}

// TestSimulationStatusStruct tests SimulationStatus struct.
func TestSimulationStatusStruct(t *testing.T) {
	now := time.Now()
	status := &SimulationStatus{
		Running:       true,
		MiniApps:      []string{"app1", "app2"},
		MinIntervalMS: 1000,
		MaxIntervalMS: 3000,
		TxCounts:      map[string]int64{"app1": 10},
		LastTxTimes:   map[string]string{"app1": now.Format(time.RFC3339)},
		StartedAt:     &now,
		Uptime:        "1h30m",
	}

	assert.True(t, status.Running)
	assert.Len(t, status.MiniApps, 2)
	assert.Equal(t, 1000, status.MinIntervalMS)
	assert.Equal(t, 3000, status.MaxIntervalMS)
	assert.Equal(t, int64(10), status.TxCounts["app1"])
	assert.NotEmpty(t, status.LastTxTimes["app1"])
	assert.NotNil(t, status.StartedAt)
	assert.Equal(t, "1h30m", status.Uptime)
}

// TestConfigDefaults tests default configuration values.
func TestConfigDefaults(t *testing.T) {
	assert.Equal(t, 2000, DefaultMinIntervalMS)
	assert.Equal(t, 6000, DefaultMaxIntervalMS)
	assert.Equal(t, int64(1000000), int64(DefaultMinAmount))
	assert.Equal(t, int64(100000000), int64(DefaultMaxAmount))
}

// TestServiceConstants tests service constants.
func TestServiceConstants(t *testing.T) {
	assert.Equal(t, "neosimulation", ServiceID)
	assert.Equal(t, "Neo Simulation Service", ServiceName)
	assert.Equal(t, "1.0.0", Version)
}

// Benchmark tests
func BenchmarkRandomInterval(b *testing.B) {
	s := &Service{
		minInterval: 1000 * time.Millisecond,
		maxInterval: 3000 * time.Millisecond,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.randomInterval()
	}
}

func BenchmarkRandomAmount(b *testing.B) {
	s := &Service{
		minAmount: 1000000,
		maxAmount: 100000000,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.randomAmount()
	}
}

func BenchmarkGetStatus(b *testing.B) {
	now := time.Now()
	s := &Service{
		running:     true,
		miniApps:    []string{"app1", "app2"},
		minInterval: 1000 * time.Millisecond,
		maxInterval: 3000 * time.Millisecond,
		startedAt:   &now,
		txCounts:    map[string]int64{"app1": 10, "app2": 20},
		lastTxTimes: map[string]time.Time{"app1": now, "app2": now},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.GetStatus()
	}
}

func BenchmarkStatistics(b *testing.B) {
	now := time.Now()
	s := &Service{
		running:     true,
		miniApps:    []string{"app1"},
		minInterval: 1000 * time.Millisecond,
		maxInterval: 3000 * time.Millisecond,
		minAmount:   1000000,
		maxAmount:   100000000,
		startedAt:   &now,
		txCounts:    map[string]int64{"app1": 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.statistics()
	}
}
