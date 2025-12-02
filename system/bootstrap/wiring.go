// Package bootstrap provides component wiring for the event system and user API.
// This file connects the IndexerBridge, Dispatcher, RequestRouter, and UserService.
//
// DEPRECATED: This file contains legacy service-specific wiring code (automation,
// gasbank managers). New services should be self-contained in packages/ and use
// the ServiceRouter pattern for HTTP endpoints. This file will be refactored to
// remove service-specific dependencies.
package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/api"
	"github.com/R3E-Network/service_layer/system/engine/events"
)

// EventSystemConfig configures the event processing system.
type EventSystemConfig struct {
	// Database connection for neo-indexer and request storage
	DB *sql.DB

	// Logger for all components
	Logger *logger.Logger

	// Contract hash to type mappings (e.g., "0x1234..." -> "oraclehub")
	ContractTypes map[string]string

	// Worker pool sizes
	DispatcherWorkers int
	RouterWorkers     int
}

// EventSystem holds all event processing components.
type EventSystem struct {
	Dispatcher *events.Dispatcher
	Router     *events.RequestRouter
	Bridge     *events.IndexerBridge
	Store      *events.PostgresRequestStore

	log *logger.Logger
}

// NewEventSystem creates and wires all event processing components.
func NewEventSystem(cfg EventSystemConfig) (*EventSystem, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("database connection required")
	}

	log := cfg.Logger
	if log == nil {
		log = logger.NewDefault("event-system")
	}

	// Create request store
	store := events.NewPostgresRequestStore(cfg.DB)

	// Create dispatcher
	dispatcherWorkers := cfg.DispatcherWorkers
	if dispatcherWorkers <= 0 {
		dispatcherWorkers = 4
	}
	dispatcher := events.NewDispatcher(events.DispatcherConfig{
		QueueSize:   1000,
		WorkerCount: dispatcherWorkers,
		Logger:      log,
	})

	// Create request router
	routerWorkers := cfg.RouterWorkers
	if routerWorkers <= 0 {
		routerWorkers = 4
	}
	router := events.NewRequestRouter(events.RouterConfig{
		Store:       store,
		WorkerCount: routerWorkers,
		Logger:      log,
	})

	// Create indexer bridge
	bridge := events.NewIndexerBridge(events.IndexerBridgeConfig{
		DB:            cfg.DB,
		Dispatcher:    dispatcher,
		Router:        router,
		Logger:        log,
		ContractTypes: cfg.ContractTypes,
	})

	return &EventSystem{
		Dispatcher: dispatcher,
		Router:     router,
		Bridge:     bridge,
		Store:      store,
		log:        log,
	}, nil
}

// Start initializes all event system components.
func (es *EventSystem) Start(ctx context.Context) error {
	// Ensure database schema
	if err := es.Store.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("ensure request store schema: %w", err)
	}

	// Start dispatcher (workerCount is configured in DispatcherConfig)
	if err := es.Dispatcher.Start(ctx, 0); err != nil {
		return fmt.Errorf("start dispatcher: %w", err)
	}

	// Start router
	if err := es.Router.Start(ctx); err != nil {
		es.Dispatcher.Stop()
		return fmt.Errorf("start router: %w", err)
	}

	// Start bridge
	if err := es.Bridge.Start(ctx); err != nil {
		es.Router.Stop()
		es.Dispatcher.Stop()
		return fmt.Errorf("start bridge: %w", err)
	}

	es.log.Info("event system started")
	return nil
}

// Stop gracefully shuts down all event system components.
func (es *EventSystem) Stop() {
	es.Bridge.Stop()
	es.Router.Stop()
	es.Dispatcher.Stop()
	es.log.Info("event system stopped")
}

// Health returns the health status of the event system.
type EventSystemHealth struct {
	Healthy     bool              `json:"healthy"`
	Dispatcher  ComponentHealth   `json:"dispatcher"`
	Router      ComponentHealth   `json:"router"`
	Bridge      ComponentHealth   `json:"bridge"`
}

// ComponentHealth represents the health of a single component.
type ComponentHealth struct {
	Running bool   `json:"running"`
	Status  string `json:"status"`
}

// Health returns the current health status of the event system.
func (es *EventSystem) Health() EventSystemHealth {
	bridgeStats := es.Bridge.Stats()
	routerStats := es.Router.Stats()
	dispatcherStats := es.Dispatcher.Stats()

	health := EventSystemHealth{
		Dispatcher: ComponentHealth{
			Running: dispatcherStats.Running,
			Status:  "ok",
		},
		Router: ComponentHealth{
			Running: routerStats.Running,
			Status:  "ok",
		},
		Bridge: ComponentHealth{
			Running: bridgeStats.Running,
			Status:  "ok",
		},
	}

	if !dispatcherStats.Running {
		health.Dispatcher.Status = "stopped"
	}
	if !routerStats.Running {
		health.Router.Status = "stopped"
	}
	if !bridgeStats.Running {
		health.Bridge.Status = "stopped"
	}

	health.Healthy = health.Dispatcher.Running && health.Router.Running && health.Bridge.Running
	return health
}

// RegisterContract registers a contract hash with its type for event routing.
func (es *EventSystem) RegisterContract(hash, contractType string) {
	es.Bridge.RegisterContract(hash, contractType)
}

// RegisterEventHandler registers an event handler with the dispatcher.
func (es *EventSystem) RegisterEventHandler(id string, handler events.EventHandler) {
	es.Dispatcher.RegisterHandler(id, handler)
}

// RegisterServiceHandler registers a service handler with the router.
func (es *EventSystem) RegisterServiceHandler(handler events.ServiceHandler) {
	es.Router.RegisterHandler(handler)
}

// UserAPIConfig configures the user-facing API.
type UserAPIConfig struct {
	// Database connection
	DB *sql.DB

	// Encryption key for secrets (32 bytes recommended)
	SecretsEncryptKey []byte

	// Request router for service requests
	Router *events.RequestRouter

	// Logger
	Logger *logger.Logger
}

// UserAPI holds all user-facing API components.
type UserAPI struct {
	Service *api.UserService
	Handler *api.HTTPHandler

	accountMgr    *api.PostgresAccountManager
	secretsMgr    *api.PostgresSecretsManager
	contractMgr   *api.PostgresContractManager
	automationMgr *api.PostgresAutomationManager
	gasbankMgr    *api.PostgresGasBankManager

	log *logger.Logger
}

// NewUserAPI creates and wires all user API components.
func NewUserAPI(cfg UserAPIConfig) (*UserAPI, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("database connection required")
	}

	log := cfg.Logger
	if log == nil {
		log = logger.NewDefault("user-api")
	}

	// Create managers
	accountMgr := api.NewPostgresAccountManager(cfg.DB, log)
	secretsMgr := api.NewPostgresSecretsManager(cfg.DB, cfg.SecretsEncryptKey, log)
	contractMgr := api.NewPostgresContractManager(cfg.DB, log)
	automationMgr := api.NewPostgresAutomationManager(cfg.DB, log)
	gasbankMgr := api.NewPostgresGasBankManager(cfg.DB, log)

	// Create user service
	svc := api.NewUserService(api.UserServiceConfig{
		Accounts:   accountMgr,
		Secrets:    secretsMgr,
		Contracts:  contractMgr,
		Automation: automationMgr,
		GasBank:    gasbankMgr,
		Router:     cfg.Router,
		Logger:     log,
	})

	// Create HTTP handler
	handler := api.NewHTTPHandler(svc, log)

	return &UserAPI{
		Service:       svc,
		Handler:       handler,
		accountMgr:    accountMgr,
		secretsMgr:    secretsMgr,
		contractMgr:   contractMgr,
		automationMgr: automationMgr,
		gasbankMgr:    gasbankMgr,
		log:           log,
	}, nil
}

// EnsureSchema creates all required database tables.
func (ua *UserAPI) EnsureSchema(ctx context.Context) error {
	if err := ua.accountMgr.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("ensure account schema: %w", err)
	}
	if err := ua.secretsMgr.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("ensure secrets schema: %w", err)
	}
	if err := ua.contractMgr.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("ensure contract schema: %w", err)
	}
	if err := ua.automationMgr.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("ensure automation schema: %w", err)
	}
	if err := ua.gasbankMgr.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("ensure gasbank schema: %w", err)
	}
	ua.log.Info("user API schema ensured")
	return nil
}

// RegisterRoutes registers all API routes on the given mux.
func (ua *UserAPI) RegisterRoutes(mux *http.ServeMux) {
	ua.Handler.RegisterRoutes(mux)
	ua.log.Info("user API routes registered")
}

// FullSystemConfig configures the complete service layer system.
type FullSystemConfig struct {
	// Database connection
	DB *sql.DB

	// Logger
	Logger *logger.Logger

	// Contract hash to type mappings
	ContractTypes map[string]string

	// Encryption key for secrets
	SecretsEncryptKey []byte

	// Worker pool sizes
	DispatcherWorkers int
	RouterWorkers     int
}

// FullSystem holds the complete service layer system.
type FullSystem struct {
	Events  *EventSystem
	UserAPI *UserAPI

	log *logger.Logger
}

// NewFullSystem creates and wires the complete service layer system.
func NewFullSystem(cfg FullSystemConfig) (*FullSystem, error) {
	log := cfg.Logger
	if log == nil {
		log = logger.NewDefault("service-layer")
	}

	// Create event system
	eventSystem, err := NewEventSystem(EventSystemConfig{
		DB:                cfg.DB,
		Logger:            log,
		ContractTypes:     cfg.ContractTypes,
		DispatcherWorkers: cfg.DispatcherWorkers,
		RouterWorkers:     cfg.RouterWorkers,
	})
	if err != nil {
		return nil, fmt.Errorf("create event system: %w", err)
	}

	// Create user API with router from event system
	userAPI, err := NewUserAPI(UserAPIConfig{
		DB:                cfg.DB,
		SecretsEncryptKey: cfg.SecretsEncryptKey,
		Router:            eventSystem.Router,
		Logger:            log,
	})
	if err != nil {
		return nil, fmt.Errorf("create user API: %w", err)
	}

	return &FullSystem{
		Events:  eventSystem,
		UserAPI: userAPI,
		log:     log,
	}, nil
}

// Start initializes all system components.
func (fs *FullSystem) Start(ctx context.Context) error {
	// Ensure all schemas
	if err := fs.Events.Store.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("ensure event store schema: %w", err)
	}
	if err := fs.UserAPI.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("ensure user API schema: %w", err)
	}

	// Start event system
	if err := fs.Events.Start(ctx); err != nil {
		return fmt.Errorf("start event system: %w", err)
	}

	fs.log.Info("full system started")
	return nil
}

// Stop gracefully shuts down all system components.
func (fs *FullSystem) Stop() {
	fs.Events.Stop()
	fs.log.Info("full system stopped")
}

// RegisterRoutes registers all API routes on the given mux.
func (fs *FullSystem) RegisterRoutes(mux *http.ServeMux) {
	fs.UserAPI.RegisterRoutes(mux)
}

// RegisterContract registers a contract hash with its type.
func (fs *FullSystem) RegisterContract(hash, contractType string) {
	fs.Events.RegisterContract(hash, contractType)
}

// RegisterEventHandler registers an event handler.
func (fs *FullSystem) RegisterEventHandler(id string, handler events.EventHandler) {
	fs.Events.RegisterEventHandler(id, handler)
}

// RegisterServiceHandler registers a service handler.
func (fs *FullSystem) RegisterServiceHandler(handler events.ServiceHandler) {
	fs.Events.RegisterServiceHandler(handler)
}
