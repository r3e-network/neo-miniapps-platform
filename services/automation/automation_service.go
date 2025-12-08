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
package automation

import (
	"context"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
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

	// Chain interaction for trigger execution
	chainClient       *chain.Client
	teeFulfiller      *chain.TEEFulfiller
	automationHash    string
	dataFeedsContract *chain.DataFeedsContract
	eventListener     *chain.EventListener
	enableChainExec   bool
}

// Scheduler manages trigger execution.
type Scheduler struct {
	mu            sync.RWMutex
	triggers      map[string]*database.AutomationTrigger
	chainTriggers map[uint64]*chain.Trigger // On-chain triggers by ID
	stopCh        chan struct{}
}

// Config holds Automation service configuration.
type Config struct {
	Marble *marble.Marble
	DB     *database.Repository

	// Chain configuration for trigger execution
	ChainClient       *chain.Client
	TEEFulfiller      *chain.TEEFulfiller
	AutomationHash    string                   // Contract hash for AutomationService
	DataFeedsContract *chain.DataFeedsContract // For price-based triggers
	EventListener     *chain.EventListener     // For event-based triggers
	EnableChainExec   bool                     // Enable on-chain trigger execution
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
		scheduler: &Scheduler{
			triggers:      make(map[string]*database.AutomationTrigger),
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

func (s *Service) registerRoutes() {
	router := s.Router()
	router.HandleFunc("/health", marble.HealthHandler(s.Service)).Methods("GET")
	router.HandleFunc("/info", s.handleInfo).Methods("GET")
	router.HandleFunc("/triggers", s.handleListTriggers).Methods("GET")
	router.HandleFunc("/triggers", s.handleCreateTrigger).Methods("POST")
	router.HandleFunc("/triggers/{id}", s.handleGetTrigger).Methods("GET")
	router.HandleFunc("/triggers/{id}", s.handleUpdateTrigger).Methods("PUT")
	router.HandleFunc("/triggers/{id}", s.handleDeleteTrigger).Methods("DELETE")
	router.HandleFunc("/triggers/{id}/enable", s.handleEnableTrigger).Methods("POST")
	router.HandleFunc("/triggers/{id}/disable", s.handleDisableTrigger).Methods("POST")
	router.HandleFunc("/triggers/{id}/executions", s.handleListExecutions).Methods("GET")
	router.HandleFunc("/triggers/{id}/resume", s.handleResumeTrigger).Methods("POST")
}

// Start starts the automation scheduler.
func (s *Service) Start(ctx context.Context) error {
	if err := s.Service.Start(ctx); err != nil {
		return err
	}

	// Hydrate scheduler cache from DB (enabled triggers)
	if triggers, err := s.DB().GetAutomationTriggers(ctx, ""); err == nil {
		s.scheduler.mu.Lock()
		for i := range triggers {
			t := triggers[i]
			if t.Enabled {
				s.scheduler.triggers[t.ID] = &t
			}
		}
		s.scheduler.mu.Unlock()
	}

	// Setup event listeners for on-chain triggers
	s.SetupEventTriggerListener()

	// Start background workers
	go s.runScheduler(ctx)
	go s.runChainTriggerChecker(ctx)

	return nil
}

// Stop stops the automation service.
func (s *Service) Stop() error {
	close(s.scheduler.stopCh)
	return s.Service.Stop()
}

// runScheduler handles time-based triggers (cron).
func (s *Service) runScheduler(ctx context.Context) {
	ticker := time.NewTicker(SchedulerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.scheduler.stopCh:
			return
		case <-ticker.C:
			s.checkAndExecuteTriggers(ctx)
		}
	}
}

// runChainTriggerChecker handles on-chain triggers (price, threshold).
func (s *Service) runChainTriggerChecker(ctx context.Context) {
	ticker := time.NewTicker(ChainTriggerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.scheduler.stopCh:
			return
		case <-ticker.C:
			s.checkChainTriggers(ctx)
		}
	}
}
