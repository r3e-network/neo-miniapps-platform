// Package neoflow provides task neoflow service.
// This service implements the Trigger-Based pattern:
// - Users register triggers with conditions via Gateway
// - TEE monitors conditions continuously (time, price, events, thresholds)
// - When conditions are met, TEE executes callbacks on-chain
//
// Trigger Types:
// 1. Time-based: Cron expressions (e.g., "Every Friday 00:00 UTC")
// 2. Price-based: Price thresholds (e.g., "When BTC > $100,000")
// 3. Event-based: On-chain events (e.g., "When contract X emits event Y")
// 4. Threshold-based: Balance/value thresholds (e.g., "When balance < 10 GAS")
package neoflowmarble

import (
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	commonservice "github.com/R3E-Network/service_layer/services/common/service"
	neofeedschain "github.com/R3E-Network/service_layer/services/neofeeds/chain"
	neoflowsupabase "github.com/R3E-Network/service_layer/services/neoflow/supabase"
)

const (
	ServiceID   = "neoflow"
	ServiceName = "NeoFlow Service"
	Version     = "2.0.0"

	// Polling intervals
	SchedulerInterval    = time.Second
	ChainTriggerInterval = 5 * time.Second

	// Service fee per trigger execution (in GAS smallest unit)
	ServiceFeePerExecution = 50000 // 0.0005 GAS
)

// Trigger type constants (matching contract)
const (
	TriggerTypeTime      uint8 = 1 // Cron-based time trigger
	TriggerTypePrice     uint8 = 2 // Price threshold trigger
	TriggerTypeEvent     uint8 = 3 // On-chain event trigger
	TriggerTypeThreshold uint8 = 4 // Balance/value threshold
)

// Service implements the NeoFlow service.
type Service struct {
	*commonservice.BaseService
	mu        sync.RWMutex
	scheduler *Scheduler

	// Service-specific repository
	repo neoflowsupabase.RepositoryInterface

	// Chain interaction for trigger execution
	chainClient       *chain.Client
	teeFulfiller      *chain.TEEFulfiller
	neoflowHash    string
	neoFeedsContract *neofeedschain.NeoFeedsContract
	eventListener     *chain.EventListener
	enableChainExec   bool
}

// Scheduler manages trigger execution.
type Scheduler struct {
	mu            sync.RWMutex
	triggers      map[string]*neoflowsupabase.Trigger
	chainTriggers map[uint64]*chain.Trigger // On-chain triggers by ID
	stopCh        chan struct{}
}

// Config holds NeoFlow service configuration.
type Config struct {
	Marble         *marble.Marble
	DB             database.RepositoryInterface
	NeoFlowRepo neoflowsupabase.RepositoryInterface

	// Chain configuration for trigger execution
	ChainClient       *chain.Client
	TEEFulfiller      *chain.TEEFulfiller
	NeoFlowHash    string                            // Contract hash for NeoFlowService
	NeoFeedsContract *neofeedschain.NeoFeedsContract // For price-based triggers
	EventListener     *chain.EventListener              // For event-based triggers
	EnableChainExec   bool                              // Enable on-chain trigger execution
}

// New creates a new NeoFlow service.
func New(cfg Config) (*Service, error) {
	base := commonservice.NewBase(commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	s := &Service{
		BaseService: base,
		repo:    cfg.NeoFlowRepo,
		scheduler: &Scheduler{
			triggers:      make(map[string]*neoflowsupabase.Trigger),
			chainTriggers: make(map[uint64]*chain.Trigger),
			stopCh:        make(chan struct{}),
		},
		chainClient:       cfg.ChainClient,
		teeFulfiller:      cfg.TEEFulfiller,
		neoflowHash:    cfg.NeoFlowHash,
		neoFeedsContract: cfg.NeoFeedsContract,
		eventListener:     cfg.EventListener,
		enableChainExec:   cfg.EnableChainExec,
	}

	// Register statistics provider for /info endpoint
	base.WithStats(s.statistics)

	// Register standard routes (/health, /info) plus service-specific routes
	base.RegisterStandardRoutes()
	s.registerRoutes()

	return s, nil
}

// statistics returns runtime statistics for the /info endpoint.
func (s *Service) statistics() map[string]any {
	s.scheduler.mu.RLock()
	activeTriggers := 0
	totalExecutions := int64(0)
	for _, t := range s.scheduler.triggers {
		if t.Enabled {
			activeTriggers++
		}
	}
	chainTriggers := len(s.scheduler.chainTriggers)
	for _, t := range s.scheduler.chainTriggers {
		if t.ExecutionCount != nil {
			totalExecutions += t.ExecutionCount.Int64()
		}
	}
	s.scheduler.mu.RUnlock()

	return map[string]any{
		"active_triggers":  activeTriggers,
		"chain_triggers":   chainTriggers,
		"total_executions": totalExecutions,
		"service_fee":      ServiceFeePerExecution,
		"trigger_types": map[string]string{
			"time":      "Cron-based time triggers",
			"price":     "Price threshold triggers",
			"event":     "On-chain event triggers",
			"threshold": "Balance/value threshold triggers",
		},
	}
}
