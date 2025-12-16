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
package neoflow

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/infrastructure/chain"
	"github.com/R3E-Network/service_layer/infrastructure/database"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
	commonservice "github.com/R3E-Network/service_layer/infrastructure/service"
	neoflowsupabase "github.com/R3E-Network/service_layer/services/automation/supabase"
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

// Service implements the NeoFlow service.
type Service struct {
	*commonservice.BaseService
	scheduler *Scheduler

	// Service-specific repository
	repo neoflowsupabase.RepositoryInterface

	// Chain interaction for trigger execution
	chainClient      *chain.Client
	teeFulfiller     *chain.TEEFulfiller
	neoflowHash      string
	neoFeedsContract *chain.NeoFeedsContract
	eventListener    *chain.EventListener
	enableChainExec  bool
}

// Scheduler manages trigger execution.
type Scheduler struct {
	mu            sync.RWMutex
	triggers      map[string]*neoflowsupabase.Trigger
	chainTriggers map[uint64]*chain.Trigger // On-chain triggers by ID
}

// Config holds NeoFlow service configuration.
type Config struct {
	Marble      *marble.Marble
	DB          database.RepositoryInterface
	NeoFlowRepo neoflowsupabase.RepositoryInterface

	// Chain configuration for trigger execution
	ChainClient      *chain.Client
	TEEFulfiller     *chain.TEEFulfiller
	NeoFlowHash      string                  // Contract hash for NeoFlowService
	NeoFeedsContract *chain.NeoFeedsContract // For price-based triggers
	EventListener    *chain.EventListener    // For event-based triggers
	EnableChainExec  bool                    // Enable on-chain trigger execution
}

// New creates a new NeoFlow service.
func New(cfg Config) (*Service, error) { //nolint:gocritic // cfg is read once at startup.
	if cfg.Marble == nil {
		return nil, fmt.Errorf("marble is required")
	}

	base := commonservice.NewBase(&commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	s := &Service{
		BaseService: base,
		repo:        cfg.NeoFlowRepo,
		scheduler: &Scheduler{
			triggers:      make(map[string]*neoflowsupabase.Trigger),
			chainTriggers: make(map[uint64]*chain.Trigger),
		},
		chainClient:      cfg.ChainClient,
		teeFulfiller:     cfg.TEEFulfiller,
		neoflowHash:      cfg.NeoFlowHash,
		neoFeedsContract: cfg.NeoFeedsContract,
		eventListener:    cfg.EventListener,
		enableChainExec:  cfg.EnableChainExec,
	}

	// Hydrate scheduler cache and register periodic workers.
	base.WithHydrate(s.hydrateSchedulerCache)
	base.AddTickerWorker(SchedulerInterval, func(ctx context.Context) error {
		s.checkAndExecuteTriggers(ctx)
		return nil
	}, commonservice.WithTickerWorkerName("scheduler"))
	base.AddTickerWorker(ChainTriggerInterval, func(ctx context.Context) error {
		s.checkChainTriggers(ctx)
		return nil
	}, commonservice.WithTickerWorkerName("chain-trigger-checker"))

	// Setup event listeners for on-chain triggers.
	s.SetupEventTriggerListener()
	base.AddWorker(s.runEventListener)

	// Register statistics provider for /info endpoint
	base.WithStats(s.statistics)

	// Register standard routes (/health, /info) plus service-specific routes
	base.RegisterStandardRoutes()
	s.registerRoutes()

	return s, nil
}

func (s *Service) runEventListener(ctx context.Context) {
	if s.eventListener == nil {
		return
	}

	if err := s.eventListener.Start(ctx); err != nil {
		s.Logger().WithContext(ctx).WithError(err).Warn("failed to start event listener")
	}
}

func (s *Service) hydrateSchedulerCache(ctx context.Context) error {
	if s.repo == nil {
		return nil
	}

	userID := s.schedulerHydrationUserID()
	if userID == "" {
		return nil
	}

	triggers, err := s.repo.GetTriggers(ctx, userID)
	if err != nil {
		return nil
	}

	s.scheduler.mu.Lock()
	for i := range triggers {
		trigger := &triggers[i]
		if trigger.Enabled {
			s.scheduler.triggers[trigger.ID] = trigger
		} else {
			delete(s.scheduler.triggers, trigger.ID)
		}
	}
	s.scheduler.mu.Unlock()

	return nil
}

func (s *Service) schedulerHydrationUserID() string {
	if s == nil {
		return ""
	}

	if marbleInstance := s.Marble(); marbleInstance != nil {
		if value, ok := marbleInstance.Secret("NEOFLOW_SCHEDULER_USER_ID"); ok && len(value) > 0 {
			return string(value)
		}
	}

	return os.Getenv("NEOFLOW_SCHEDULER_USER_ID")
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
