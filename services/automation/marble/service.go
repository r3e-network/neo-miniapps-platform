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
	"strings"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/infrastructure/chain"
	"github.com/R3E-Network/service_layer/infrastructure/database"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
	"github.com/R3E-Network/service_layer/infrastructure/runtime"
	commonservice "github.com/R3E-Network/service_layer/infrastructure/service"
	txproxytypes "github.com/R3E-Network/service_layer/infrastructure/txproxy/types"
	neoflowsupabase "github.com/R3E-Network/service_layer/services/automation/supabase"
)

const (
	ServiceID   = "neoflow"
	ServiceName = "NeoFlow Service"
	Version     = "2.0.0"

	// Polling intervals
	SchedulerInterval    = time.Second
	AnchoredTaskInterval = 5 * time.Second

	// Service fee per trigger execution (in GAS smallest unit)
	ServiceFeePerExecution = 50000 // 0.0005 GAS
)

// Service implements the NeoFlow service.
type Service struct {
	*commonservice.BaseService
	scheduler *Scheduler

	// Service-specific repository
	repo neoflowsupabase.RepositoryInterface

	// Optional chain interaction for anchored tasks (platform contracts).
	chainClient          *chain.Client
	priceFeedHash        string
	priceFeed            *chain.PriceFeedContract
	automationAnchorHash string
	automationAnchor     *chain.AutomationAnchorContract
	txProxy              txproxytypes.Invoker
	eventListener        *chain.EventListener
	enableChainExec      bool
}

// Scheduler manages trigger execution.
type Scheduler struct {
	mu            sync.RWMutex
	triggers      map[string]*neoflowsupabase.Trigger
	anchoredTasks map[string]*anchoredTaskState // Platform AutomationAnchor tasks by key
}

// Config holds NeoFlow service configuration.
type Config struct {
	Marble      *marble.Marble
	DB          database.RepositoryInterface
	NeoFlowRepo neoflowsupabase.RepositoryInterface

	// Optional chain configuration for anchored tasks (platform AutomationAnchor + PriceFeed).
	ChainClient          *chain.Client
	PriceFeedHash        string
	AutomationAnchorHash string
	TxProxy              txproxytypes.Invoker
	EventListener        *chain.EventListener
	EnableChainExec      bool
}

// New creates a new NeoFlow service.
func New(cfg Config) (*Service, error) { //nolint:gocritic // cfg is read once at startup.
	if cfg.Marble == nil {
		return nil, fmt.Errorf("marble is required")
	}

	strict := runtime.StrictIdentityMode() || cfg.Marble.IsEnclave()

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
			anchoredTasks: make(map[string]*anchoredTaskState),
		},
		chainClient:          cfg.ChainClient,
		priceFeedHash:        cfg.PriceFeedHash,
		automationAnchorHash: cfg.AutomationAnchorHash,
		txProxy:              cfg.TxProxy,
		eventListener:        cfg.EventListener,
		enableChainExec:      cfg.EnableChainExec,
	}

	if s.chainClient != nil && s.priceFeedHash != "" {
		s.priceFeed = chain.NewPriceFeedContract(s.chainClient, s.priceFeedHash)
	}
	if s.chainClient != nil && s.automationAnchorHash != "" {
		s.automationAnchor = chain.NewAutomationAnchorContract(s.chainClient, s.automationAnchorHash)
	}

	if s.enableChainExec && strings.TrimSpace(s.automationAnchorHash) == "" {
		if strict {
			return nil, fmt.Errorf("neoflow: EnableChainExec requires AutomationAnchorHash configured")
		}
		s.Logger().WithFields(nil).Warn("EnableChainExec enabled but AutomationAnchorHash not configured; disabling on-chain automation")
		s.enableChainExec = false
	}

	if s.enableChainExec && s.automationAnchorHash != "" && s.automationAnchor == nil {
		if strict {
			return nil, fmt.Errorf("neoflow: EnableChainExec requires chain client configured")
		}
		s.Logger().WithFields(nil).Warn("EnableChainExec enabled but chain client not configured; disabling on-chain automation")
		s.enableChainExec = false
	}

	if s.enableChainExec && s.automationAnchorHash != "" && s.txProxy == nil {
		if strict {
			return nil, fmt.Errorf("neoflow: EnableChainExec requires TxProxy configured")
		}
		s.Logger().WithFields(nil).Warn("EnableChainExec enabled but TxProxy not configured; disabling on-chain automation")
		s.enableChainExec = false
	}

	// Hydrate scheduler cache and register periodic workers.
	base.WithHydrate(s.hydrateSchedulerCache)
	base.AddTickerWorker(SchedulerInterval, func(ctx context.Context) error {
		s.checkAndExecuteTriggers(ctx)
		return nil
	}, commonservice.WithTickerWorkerName("scheduler"))
	base.AddTickerWorker(AnchoredTaskInterval, func(ctx context.Context) error {
		s.checkAndExecuteAnchoredTasks(ctx)
		return nil
	}, commonservice.WithTickerWorkerName("anchored-task-checker"))

	if s.enableChainExec && s.automationAnchor != nil {
		base.WithHydrate(s.hydrateAnchoredTasks)
		s.setupAutomationAnchorListener()
		base.AddWorker(s.runEventListener)
	}

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
	anchoredTasks := len(s.scheduler.anchoredTasks)
	for _, t := range s.scheduler.anchoredTasks {
		if t == nil {
			continue
		}
		t.mu.Lock()
		totalExecutions += t.executionCount
		t.mu.Unlock()
	}
	s.scheduler.mu.RUnlock()

	return map[string]any{
		"active_triggers":  activeTriggers,
		"anchored_tasks":   anchoredTasks,
		"total_executions": totalExecutions,
		"service_fee":      ServiceFeePerExecution,
		"trigger_types": map[string]string{
			"cron":           "Cron-based time triggers (stored in Supabase)",
			"anchored_cron":  "Cron triggers anchored via AutomationAnchor",
			"anchored_price": "Price threshold triggers using PriceFeed + AutomationAnchor",
		},
	}
}
