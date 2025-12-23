// Package neosimulation provides simulation service for automated transaction testing.
// This service simulates real user transactions by:
// - Requesting accounts from the pool
// - Simulating transactions (payGAS with random amounts)
// - Recording transactions to Supabase
// - Releasing accounts back to the pool
package neosimulation

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/infrastructure/chain"
	"github.com/R3E-Network/service_layer/infrastructure/database"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
	"github.com/R3E-Network/service_layer/infrastructure/runtime"
	commonservice "github.com/R3E-Network/service_layer/infrastructure/service"
	neoaccountsclient "github.com/R3E-Network/service_layer/infrastructure/accountpool/client"
)

// Service implements the simulation service.
type Service struct {
	*commonservice.BaseService
	mu sync.RWMutex

	chainClient    *chain.Client
	db             database.RepositoryInterface
	accountPoolURL string
	poolClient     *neoaccountsclient.Client

	// Contract invoker for smart contract calls
	contractInvoker *ContractInvoker

	// MiniApp simulator for workflow simulation
	miniAppSimulator *MiniAppSimulator

	// Simulation configuration
	miniApps      []string
	minInterval   time.Duration
	maxInterval   time.Duration
	minAmount     int64
	maxAmount     int64
	workersPerApp int

	// Simulation state
	running   bool
	stopCh    chan struct{}
	startedAt *time.Time
	txCounts  map[string]int64
	lastTxTimes map[string]time.Time
	rng       *rand.Rand
}

// New creates a new simulation service.
func New(cfg Config) (*Service, error) {
	if cfg.Marble == nil {
		return nil, fmt.Errorf("neosimulation: marble is required")
	}

	marble, ok := cfg.Marble.(*marble.Marble)
	if !ok {
		return nil, fmt.Errorf("neosimulation: invalid marble type")
	}

	strict := runtime.StrictIdentityMode() || marble.IsEnclave()

	if strict && cfg.ChainClient == nil {
		return nil, fmt.Errorf("neosimulation: chain client is required in strict/enclave mode")
	}

	// Type assert DB if provided
	var db database.RepositoryInterface
	if cfg.DB != nil {
		var ok bool
		db, ok = cfg.DB.(database.RepositoryInterface)
		if !ok {
			return nil, fmt.Errorf("neosimulation: DB must implement database.RepositoryInterface")
		}
	}

	base := commonservice.NewBase(&commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  marble,
		DB:      db,
	})

	// Get account pool URL
	accountPoolURL := strings.TrimSpace(cfg.AccountPoolURL)
	if accountPoolURL == "" {
		accountPoolURL = strings.TrimSpace(os.Getenv("NEOACCOUNTS_SERVICE_URL"))
	}
	if accountPoolURL == "" {
		accountPoolURL = "http://neoaccounts:8080" // Default service mesh URL
	}

	// Get MiniApps list
	miniApps := cfg.MiniApps
	if len(miniApps) == 0 {
		miniAppsEnv := strings.TrimSpace(os.Getenv("SIMULATION_MINIAPPS"))
		if miniAppsEnv != "" {
			miniApps = strings.Split(miniAppsEnv, ",")
			for i := range miniApps {
				miniApps[i] = strings.TrimSpace(miniApps[i])
			}
		}
	}
	if len(miniApps) == 0 {
		miniApps = []string{"builtin-lottery", "builtin-coin-flip", "builtin-dice-game"}
	}

	// Get interval configuration
	minIntervalMS := cfg.MinIntervalMS
	if minIntervalMS == 0 {
		if envVal := os.Getenv("SIMULATION_TX_INTERVAL_MIN_MS"); envVal != "" {
			fmt.Sscanf(envVal, "%d", &minIntervalMS)
		}
	}
	if minIntervalMS == 0 {
		minIntervalMS = DefaultMinIntervalMS
	}

	maxIntervalMS := cfg.MaxIntervalMS
	if maxIntervalMS == 0 {
		if envVal := os.Getenv("SIMULATION_TX_INTERVAL_MAX_MS"); envVal != "" {
			fmt.Sscanf(envVal, "%d", &maxIntervalMS)
		}
	}
	if maxIntervalMS == 0 {
		maxIntervalMS = DefaultMaxIntervalMS
	}

	// Get amount configuration
	minAmount := cfg.MinAmount
	if minAmount == 0 {
		minAmount = DefaultMinAmount
	}
	maxAmount := cfg.MaxAmount
	if maxAmount == 0 {
		maxAmount = DefaultMaxAmount
	}

	// Get workers per app configuration
	workersPerApp := cfg.WorkersPerApp
	if workersPerApp == 0 {
		if envVal := os.Getenv("SIMULATION_WORKERS_PER_APP"); envVal != "" {
			fmt.Sscanf(envVal, "%d", &workersPerApp)
		}
	}
	if workersPerApp <= 0 {
		workersPerApp = DefaultWorkersPerApp
	}

	// Initialize account pool client
	poolClient, err := neoaccountsclient.New(neoaccountsclient.Config{
		BaseURL:   accountPoolURL,
		ServiceID: ServiceID,
	})
	if err != nil {
		return nil, fmt.Errorf("neosimulation: failed to create account pool client: %w", err)
	}

	var chainClient *chain.Client
	if cfg.ChainClient != nil {
		chainClient = cfg.ChainClient.(*chain.Client)
	}

	// Initialize contract invoker for smart contract calls using pool accounts
	// All signing happens inside the TEE via the account pool service
	var contractInvoker *ContractInvoker
	invoker, err := NewContractInvokerFromEnv(poolClient)
	if err != nil {
		// Log warning but don't fail - contract invocation is optional
		fmt.Printf("neosimulation: contract invoker disabled: %v\n", err)
	} else {
		contractInvoker = invoker
		fmt.Println("neosimulation: contract invoker initialized (using pool accounts)")
	}

	// Initialize MiniApp simulator if contract invoker is available
	var miniAppSimulator *MiniAppSimulator
	if contractInvoker != nil {
		miniAppSimulator = NewMiniAppSimulator(contractInvoker)
		fmt.Println("neosimulation: MiniApp simulator initialized for all 7 apps")
	}

	s := &Service{
		BaseService:      base,
		chainClient:      chainClient,
		db:               db,
		accountPoolURL:   accountPoolURL,
		poolClient:       poolClient,
		contractInvoker:  contractInvoker,
		miniAppSimulator: miniAppSimulator,
		miniApps:         miniApps,
		minInterval:    time.Duration(minIntervalMS) * time.Millisecond,
		maxInterval:    time.Duration(maxIntervalMS) * time.Millisecond,
		minAmount:      minAmount,
		maxAmount:      maxAmount,
		workersPerApp:  workersPerApp,
		running:        false,
		txCounts:       make(map[string]int64),
		lastTxTimes:    make(map[string]time.Time),
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Register statistics provider for /info endpoint
	base.WithStats(s.statistics)

	// Register standard routes (/health, /info) plus service-specific routes
	base.RegisterStandardRoutes()
	s.registerRoutes()

	// Auto-start if configured
	if cfg.AutoStart || strings.ToLower(os.Getenv("SIMULATION_ENABLED")) == "true" {
		go func() {
			time.Sleep(2 * time.Second) // Wait for service to fully initialize
			if err := s.Start(context.Background()); err != nil {
				s.Logger().WithError(err).Warn("failed to auto-start simulation")
			}
		}()
	}

	return s, nil
}

// statistics returns runtime statistics for the /info endpoint.
func (s *Service) statistics() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]any{
		"running":           s.running,
		"mini_apps":         s.miniApps,
		"workers_per_app":   s.workersPerApp,
		"min_interval_ms":   s.minInterval.Milliseconds(),
		"max_interval_ms":   s.maxInterval.Milliseconds(),
		"min_amount":        s.minAmount,
		"max_amount":        s.maxAmount,
		"tx_counts":         s.txCounts,
		"contract_invoker":  s.contractInvoker != nil,
		"miniapp_simulator": s.miniAppSimulator != nil,
	}

	// Add contract invocation stats if available
	if s.contractInvoker != nil {
		stats["contract_stats"] = s.contractInvoker.GetStats()
	}

	// Add MiniApp workflow stats if available
	if s.miniAppSimulator != nil {
		stats["miniapp_workflow_stats"] = s.miniAppSimulator.GetStats()
	}

	if s.startedAt != nil {
		stats["started_at"] = s.startedAt.Format(time.RFC3339)
		stats["uptime"] = time.Since(*s.startedAt).String()
	}

	return stats
}

// Start starts the simulation.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("simulation already running")
	}

	s.running = true
	s.stopCh = make(chan struct{})
	now := time.Now()
	s.startedAt = &now

	// Start multiple workers for each MiniApp to achieve target transaction rate
	// With N workers per app, each targeting interval I, effective rate is I/N per app
	for _, appID := range s.miniApps {
		for workerID := 0; workerID < s.workersPerApp; workerID++ {
			go s.simulateApp(appID, workerID)
		}
	}

	// Start contract invocation workers if contract invoker is available
	if s.contractInvoker != nil {
		// PriceFeed updater (every 5 seconds per symbol)
		go s.runPriceFeedUpdater()

		// RandomnessLog recorder (every 10 seconds)
		go s.runRandomnessRecorder()

		// Auto top-up worker for pool accounts with low GAS balance (every 30 seconds)
		go s.runAutoTopUp()

		s.Logger().WithContext(ctx).Info("contract invocation workers started")
	}

	// Start MiniApp workflow simulators if MiniApp simulator is available
	if s.miniAppSimulator != nil {
		// Gaming MiniApps
		go s.runMiniAppWorkflow("lottery", 5*time.Second, s.miniAppSimulator.SimulateLottery)
		go s.runMiniAppWorkflow("coin-flip", 3*time.Second, s.miniAppSimulator.SimulateCoinFlip)
		go s.runMiniAppWorkflow("dice-game", 4*time.Second, s.miniAppSimulator.SimulateDiceGame)
		go s.runMiniAppWorkflow("scratch-card", 6*time.Second, s.miniAppSimulator.SimulateScratchCard)

		// DeFi MiniApps
		go s.runMiniAppWorkflow("prediction-market", 10*time.Second, s.miniAppSimulator.SimulatePredictionMarket)
		go s.runMiniAppWorkflow("flashloan", 15*time.Second, s.miniAppSimulator.SimulateFlashLoan)
		go s.runMiniAppWorkflow("price-ticker", 8*time.Second, s.miniAppSimulator.SimulatePriceTicker)

		s.Logger().WithContext(ctx).Info("MiniApp workflow simulators started for all 7 apps")
	}

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"mini_apps":          s.miniApps,
		"workers_per_app":    s.workersPerApp,
		"total_workers":      len(s.miniApps) * s.workersPerApp,
		"contract_invoker":   s.contractInvoker != nil,
		"miniapp_simulator":  s.miniAppSimulator != nil,
	}).Info("simulation started")

	return nil
}

// Stop stops the simulation.
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("simulation not running")
	}

	s.running = false
	close(s.stopCh)
	s.startedAt = nil

	s.Logger().WithContext(context.Background()).Info("simulation stopped")

	return nil
}

// GetStatus returns the current simulation status.
func (s *Service) GetStatus() *SimulationStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &SimulationStatus{
		Running:       s.running,
		MiniApps:      s.miniApps,
		MinIntervalMS: int(s.minInterval.Milliseconds()),
		MaxIntervalMS: int(s.maxInterval.Milliseconds()),
		TxCounts:      make(map[string]int64),
		LastTxTimes:   make(map[string]string),
		StartedAt:     s.startedAt,
	}

	for appID, count := range s.txCounts {
		status.TxCounts[appID] = count
	}

	for appID, t := range s.lastTxTimes {
		status.LastTxTimes[appID] = t.Format(time.RFC3339)
	}

	if s.startedAt != nil {
		status.Uptime = time.Since(*s.startedAt).String()
	}

	return status
}

// simulateApp runs the simulation loop for a single MiniApp worker.
// It submits REAL transactions to the Neo N3 blockchain by transferring
// small amounts of GAS from the master account to pool accounts.
// Target rate: 1 transaction every 1-3 seconds per miniapp (achieved via multiple workers).
func (s *Service) simulateApp(appID string, workerID int) {
	ctx := context.Background()
	logger := s.Logger().WithFields(map[string]interface{}{"app_id": appID, "worker_id": workerID})

	logger.WithContext(ctx).Info("starting simulation worker for app")

	for {
		// Record start time to calculate remaining sleep after transaction
		iterationStart := time.Now()
		targetInterval := s.randomInterval()

		// Check if simulation is stopped
		select {
		case <-s.stopCh:
			logger.WithContext(ctx).Info("stopping simulation worker for app")
			return
		default:
		}

		// Check if pool client is available
		if s.poolClient == nil {
			// Simulate without actual pool - just log and update stats
			amount := s.randomAmount()
			s.mu.Lock()
			s.txCounts[appID]++
			s.lastTxTimes[appID] = time.Now()
			s.mu.Unlock()

			logger.WithFields(map[string]interface{}{
				"tx_type": "payGAS",
				"amount":  amount,
				"mode":    "simulated",
			}).Debug("simulated transaction (no pool client)")

			// Sleep remaining time to hit target interval
			elapsed := time.Since(iterationStart)
			if remaining := targetInterval - elapsed; remaining > 0 {
				time.Sleep(remaining)
			}
			continue
		}

		// Request ONE account from the pool (destination for funding)
		resp, err := s.poolClient.RequestAccounts(ctx, 1, fmt.Sprintf("simulation-%s", appID))
		if err != nil {
			logger.WithError(err).Warn("failed to request accounts from pool")
			// Sleep remaining time before retry
			elapsed := time.Since(iterationStart)
			if remaining := targetInterval - elapsed; remaining > 0 {
				time.Sleep(remaining)
			}
			continue
		}

		if len(resp.Accounts) < 1 {
			logger.Warn("no accounts available in pool")
			// Sleep remaining time before retry
			elapsed := time.Since(iterationStart)
			if remaining := targetInterval - elapsed; remaining > 0 {
				time.Sleep(remaining)
			}
			continue
		}

		destAccount := resp.Accounts[0]

		// Use a small amount for the transfer (0.00001 GAS = 1000 in 8 decimals)
		// This is small enough to allow many transactions without draining the master account
		amount := int64(1000) // 0.00001 GAS
		txType := "fund"
		txHash := ""
		status := "pending"

		// Submit REAL transaction to the blockchain using Fund (from master account)
		fundResp, err := s.poolClient.FundAccount(ctx, destAccount.Address, amount)
		if err != nil {
			logger.WithError(err).WithFields(map[string]interface{}{
				"destination": destAccount.Address,
				"amount":      amount,
			}).Warn("failed to submit blockchain transaction")
			status = "failed"
		} else {
			txHash = fundResp.TxHash
			status = "confirmed"
			logger.WithFields(map[string]interface{}{
				"destination": destAccount.Address,
				"amount":      amount,
				"tx_hash":     txHash,
			}).Info("blockchain transaction submitted successfully")
		}

		// Record transaction to database
		if s.db != nil {
			tx := &SimulationTx{
				AppID:          appID,
				AccountAddress: destAccount.Address,
				TxType:         txType,
				Amount:         amount,
				Status:         status,
				TxHash:         txHash,
				CreatedAt:      time.Now(),
			}

			if err := s.recordTransaction(ctx, tx); err != nil {
				logger.WithError(err).Warn("failed to record transaction")
			}
		}

		// Update statistics
		s.mu.Lock()
		s.txCounts[appID]++
		s.lastTxTimes[appID] = time.Now()
		s.mu.Unlock()

		// Release the account back to the pool (do this in background to not delay next tx)
		go func(accountID string) {
			_, err := s.poolClient.ReleaseAccounts(ctx, []string{accountID})
			if err != nil {
				logger.WithError(err).Warn("failed to release account")
			}
		}(destAccount.ID)

		// Sleep remaining time to hit target interval (1-3 seconds from iteration start)
		elapsed := time.Since(iterationStart)
		if remaining := targetInterval - elapsed; remaining > 0 {
			time.Sleep(remaining)
		}
	}
}

// randomInterval returns a random interval between minInterval and maxInterval.
func (s *Service) randomInterval() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	minMS := s.minInterval.Milliseconds()
	maxMS := s.maxInterval.Milliseconds()

	if minMS >= maxMS {
		return s.minInterval
	}

	randomMS := minMS + s.rng.Int63n(maxMS-minMS+1)
	return time.Duration(randomMS) * time.Millisecond
}

// randomAmount returns a random amount between minAmount and maxAmount.
func (s *Service) randomAmount() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.minAmount >= s.maxAmount {
		return s.minAmount
	}

	return s.minAmount + s.rng.Int63n(s.maxAmount-s.minAmount+1)
}

// recordTransaction records a simulated transaction to the database.
func (s *Service) recordTransaction(ctx context.Context, tx *SimulationTx) error {
	if s.db == nil {
		// No database configured - just log the transaction
		s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
			"app_id":  tx.AppID,
			"address": tx.AccountAddress,
			"tx_type": tx.TxType,
			"amount":  tx.Amount,
		}).Debug("simulated transaction (no db)")
		return nil
	}

	// Type assert to *database.Repository for GenericCreate
	repo, ok := s.db.(*database.Repository)
	if !ok {
		return fmt.Errorf("database is not *database.Repository")
	}

	// Use Supabase generic create
	type SimulationTxDB struct {
		AppID          string    `json:"app_id"`
		AccountAddress string    `json:"account_address"`
		TxType         string    `json:"tx_type"`
		Amount         int64     `json:"amount"`
		Status         string    `json:"status"`
		TxHash         string    `json:"tx_hash,omitempty"`
		CreatedAt      time.Time `json:"created_at"`
	}

	record := SimulationTxDB{
		AppID:          tx.AppID,
		AccountAddress: tx.AccountAddress,
		TxType:         tx.TxType,
		Amount:         tx.Amount,
		Status:         tx.Status,
		TxHash:         tx.TxHash,
		CreatedAt:      tx.CreatedAt,
	}

	// Insert using generic repository method
	err := database.GenericCreate(repo, ctx, "simulation_txs", &record, nil)
	if err != nil {
		return fmt.Errorf("insert simulation_txs: %w", err)
	}

	return nil
}

// runPriceFeedUpdater runs the PriceFeed update loop.
func (s *Service) runPriceFeedUpdater() {
	ctx := context.Background()
	logger := s.Logger().WithFields(map[string]interface{}{"worker": "pricefeed"})

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			logger.WithContext(ctx).Info("stopping PriceFeed updater")
			return
		case <-ticker.C:
			for _, symbol := range s.contractInvoker.GetPriceSymbols() {
				txHash, err := s.contractInvoker.UpdatePriceFeed(ctx, symbol)
				if err != nil {
					logger.WithError(err).WithField("symbol", symbol).Warn("PriceFeed update failed")
				} else {
					logger.WithFields(map[string]interface{}{
						"symbol":  symbol,
						"tx_hash": txHash[:16] + "...",
					}).Debug("PriceFeed updated")
				}
				time.Sleep(500 * time.Millisecond) // Small delay between updates
			}
		}
	}
}

// runRandomnessRecorder runs the RandomnessLog record loop.
func (s *Service) runRandomnessRecorder() {
	ctx := context.Background()
	logger := s.Logger().WithFields(map[string]interface{}{"worker": "randomness"})

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			logger.WithContext(ctx).Info("stopping RandomnessLog recorder")
			return
		case <-ticker.C:
			txHash, err := s.contractInvoker.RecordRandomness(ctx)
			if err != nil {
				logger.WithError(err).Warn("RandomnessLog record failed")
			} else {
				logger.WithFields(map[string]interface{}{
					"tx_hash": txHash[:16] + "...",
				}).Debug("RandomnessLog recorded")
			}
		}
	}
}

// runPaymentHubPayer runs the PaymentHub payment loop for a MiniApp.
func (s *Service) runPaymentHubPayer(appID string) {
	ctx := context.Background()
	logger := s.Logger().WithFields(map[string]interface{}{"worker": "paymenthub", "app_id": appID})

	// Stagger start times
	time.Sleep(time.Duration(len(appID)%3) * time.Second)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	paymentCount := 0
	for {
		select {
		case <-s.stopCh:
			logger.WithContext(ctx).Info("stopping PaymentHub payer")
			return
		case <-ticker.C:
			paymentCount++
			amount := int64(100000) // 0.001 GAS
			memo := fmt.Sprintf("sim-payment-%d", paymentCount)

			txHash, err := s.contractInvoker.PayToApp(ctx, appID, amount, memo)
			if err != nil {
				logger.WithError(err).Warn("PaymentHub pay failed")
			} else {
				logger.WithFields(map[string]interface{}{
					"amount":  amount,
					"tx_hash": txHash[:16] + "...",
				}).Debug("PaymentHub payment sent")
			}
		}
	}
}

// runMiniAppWorkflow runs a MiniApp workflow simulation loop.
func (s *Service) runMiniAppWorkflow(appName string, interval time.Duration, workflowFn func(context.Context) error) {
	ctx := context.Background()
	logger := s.Logger().WithFields(map[string]interface{}{"worker": "miniapp", "app": appName})

	// Stagger start times based on app name
	time.Sleep(time.Duration(len(appName)%5) * time.Second)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.WithContext(ctx).WithField("interval", interval.String()).Info("starting MiniApp workflow simulator")

	for {
		select {
		case <-s.stopCh:
			logger.WithContext(ctx).Info("stopping MiniApp workflow simulator")
			return
		case <-ticker.C:
			err := workflowFn(ctx)
			if err != nil {
				logger.WithError(err).Warn("MiniApp workflow failed")
			} else {
				logger.WithContext(ctx).Debug("MiniApp workflow completed")
			}
		}
	}
}

// runAutoTopUp periodically checks for pool accounts with low GAS balance and funds them.
// This ensures pool accounts have enough GAS to pay for transaction fees.
func (s *Service) runAutoTopUp() {
	ctx := context.Background()
	logger := s.Logger().WithFields(map[string]interface{}{"worker": "auto-topup"})

	// Wait for initial setup
	time.Sleep(5 * time.Second)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	logger.WithContext(ctx).Info("starting auto top-up worker for pool accounts")

	// Minimum GAS balance threshold (0.1 GAS = 10000000 in 8 decimals)
	const minGASBalance int64 = 10000000
	// Amount to fund when balance is low (1 GAS = 100000000 in 8 decimals)
	const fundAmount int64 = 100000000

	for {
		select {
		case <-s.stopCh:
			logger.WithContext(ctx).Info("stopping auto top-up worker")
			return
		case <-ticker.C:
			// Get accounts with low GAS balance
			accounts, err := s.poolClient.ListLowBalanceAccounts(ctx, "GAS", minGASBalance, 10)
			if err != nil {
				logger.WithError(err).Warn("failed to list low balance accounts")
				continue
			}

			if len(accounts) == 0 {
				logger.WithContext(ctx).Debug("no accounts need top-up")
				continue
			}

			logger.WithContext(ctx).WithField("count", len(accounts)).Info("found accounts with low GAS balance")

			// Fund each account
			for _, acc := range accounts {
				_, err := s.poolClient.FundAccount(ctx, acc.Address, fundAmount)
				if err != nil {
					logger.WithError(err).WithFields(map[string]interface{}{
						"account_id": acc.ID,
						"address":    acc.Address,
					}).Warn("failed to fund account")
					continue
				}

				logger.WithFields(map[string]interface{}{
					"account_id": acc.ID,
					"address":    acc.Address,
					"amount":     fundAmount,
				}).Info("funded pool account")

				// Small delay between funding operations
				time.Sleep(2 * time.Second)
			}
		}
	}
}
