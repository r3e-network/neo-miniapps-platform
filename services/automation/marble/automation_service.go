// Package automation provides task automation service.
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
package automationmarble

import (
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	automationsupabase "github.com/R3E-Network/service_layer/services/automation/supabase"
	datafeedschain "github.com/R3E-Network/service_layer/services/datafeeds/chain"
)

const (
	ServiceID   = "automation"
	ServiceName = "Automation Service"
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

// Service implements the Automation service.
type Service struct {
	*marble.Service
	mu        sync.RWMutex
	scheduler *Scheduler

	// Service-specific repository
	repo automationsupabase.RepositoryInterface

	// Chain interaction for trigger execution
	chainClient       *chain.Client
	teeFulfiller      *chain.TEEFulfiller
	automationHash    string
	dataFeedsContract *datafeedschain.DataFeedsContract
	eventListener     *chain.EventListener
	enableChainExec   bool
}

// Scheduler manages trigger execution.
type Scheduler struct {
	mu            sync.RWMutex
	triggers      map[string]*automationsupabase.Trigger
	chainTriggers map[uint64]*chain.Trigger // On-chain triggers by ID
	stopCh        chan struct{}
}

// Config holds Automation service configuration.
type Config struct {
	Marble         *marble.Marble
	DB             database.RepositoryInterface
	AutomationRepo automationsupabase.RepositoryInterface

	// Chain configuration for trigger execution
	ChainClient       *chain.Client
	TEEFulfiller      *chain.TEEFulfiller
	AutomationHash    string                              // Contract hash for AutomationService
	DataFeedsContract *datafeedschain.DataFeedsContract   // For price-based triggers
	EventListener     *chain.EventListener                // For event-based triggers
	EnableChainExec   bool                                // Enable on-chain trigger execution
}

// New creates a new Automation service.
func New(cfg Config) (*Service, error) {
	base := marble.NewService(marble.ServiceConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	s := &Service{
		Service: base,
		repo:    cfg.AutomationRepo,
		scheduler: &Scheduler{
			triggers:      make(map[string]*automationsupabase.Trigger),
			chainTriggers: make(map[uint64]*chain.Trigger),
			stopCh:        make(chan struct{}),
		},
		chainClient:       cfg.ChainClient,
		teeFulfiller:      cfg.TEEFulfiller,
		automationHash:    cfg.AutomationHash,
		dataFeedsContract: cfg.DataFeedsContract,
		eventListener:     cfg.EventListener,
		enableChainExec:   cfg.EnableChainExec,
	}

	s.registerRoutes()
	return s, nil
}

