package app

import (
	"context"
	"fmt"

	"github.com/R3E-Network/service_layer/internal/app/services/accounts"
	automationsvc "github.com/R3E-Network/service_layer/internal/app/services/automation"
	"github.com/R3E-Network/service_layer/internal/app/services/functions"
	gasbanksvc "github.com/R3E-Network/service_layer/internal/app/services/gasbank"
	oraclesvc "github.com/R3E-Network/service_layer/internal/app/services/oracle"
	pricefeedsvc "github.com/R3E-Network/service_layer/internal/app/services/pricefeed"
	"github.com/R3E-Network/service_layer/internal/app/services/triggers"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
	"github.com/R3E-Network/service_layer/internal/app/system"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Stores encapsulates persistence dependencies. Nil stores default to the
// in-memory implementation.
type Stores struct {
	Accounts   storage.AccountStore
	Functions  storage.FunctionStore
	Triggers   storage.TriggerStore
	GasBank    storage.GasBankStore
	Automation storage.AutomationStore
	PriceFeeds storage.PriceFeedStore
	Oracle     storage.OracleStore
}

// Application ties domain services together and manages their lifecycle.
type Application struct {
	manager *system.Manager
	log     *logger.Logger

	Accounts   *accounts.Service
	Functions  *functions.Service
	Triggers   *triggers.Service
	GasBank    *gasbanksvc.Service
	Automation *automationsvc.Service
	PriceFeeds *pricefeedsvc.Service
	Oracle     *oraclesvc.Service
}

// New builds a fully initialised application with the provided stores.
func New(stores Stores, log *logger.Logger) (*Application, error) {
	if log == nil {
		log = logger.NewDefault("app")
	}

	mem := memory.New()
	if stores.Accounts == nil {
		stores.Accounts = mem
	}
	if stores.Functions == nil {
		stores.Functions = mem
	}
	if stores.Triggers == nil {
		stores.Triggers = mem
	}
	if stores.GasBank == nil {
		stores.GasBank = mem
	}
	if stores.Automation == nil {
		stores.Automation = mem
	}
	if stores.PriceFeeds == nil {
		stores.PriceFeeds = mem
	}
	if stores.Oracle == nil {
		stores.Oracle = mem
	}

	manager := system.NewManager()

	acctService := accounts.New(stores.Accounts, log)
	funcService := functions.New(stores.Accounts, stores.Functions, log)
	funcService.AttachExecutor(functions.NewSimpleExecutor())
	trigService := triggers.New(stores.Accounts, stores.Functions, stores.Triggers, log)
	gasService := gasbanksvc.New(stores.Accounts, stores.GasBank, log)
	settlement := gasbanksvc.NewSettlementPoller(stores.GasBank, gasService, nil, log)
	automationService := automationsvc.New(stores.Accounts, stores.Functions, stores.Automation, log)
	priceFeedService := pricefeedsvc.New(stores.Accounts, stores.PriceFeeds, log)
	oracleService := oraclesvc.New(stores.Accounts, stores.Oracle, log)

	funcService.AttachDependencies(trigService, automationService, priceFeedService, oracleService, gasService)

	for _, name := range []string{"accounts", "functions", "triggers"} {
		if err := manager.Register(system.NoopService{ServiceName: name}); err != nil {
			return nil, fmt.Errorf("register %s service: %w", name, err)
		}
	}

	autoRunner := automationsvc.NewScheduler(automationService, log)
	autoRunner.WithDispatcher(automationsvc.NewFunctionDispatcher(automationsvc.FunctionRunnerFunc(func(ctx context.Context, functionID string, payload map[string]any) error {
		_, err := funcService.Execute(ctx, functionID, payload)
		return err
	}), automationService, log))
	priceRunner := pricefeedsvc.NewRefresher(priceFeedService, log)
	priceRunner.WithFetcher(pricefeedsvc.NewRandomFetcher(log))
	oracleRunner := oraclesvc.NewDispatcher(oracleService, log)

	for _, svc := range []system.Service{settlement, autoRunner, priceRunner, oracleRunner} {
		if err := manager.Register(svc); err != nil {
			return nil, fmt.Errorf("register %s: %w", svc.Name(), err)
		}
	}

	return &Application{
		manager:    manager,
		log:        log,
		Accounts:   acctService,
		Functions:  funcService,
		Triggers:   trigService,
		GasBank:    gasService,
		Automation: automationService,
		PriceFeeds: priceFeedService,
		Oracle:     oracleService,
	}, nil
}

// Attach registers an additional lifecycle-managed service. Call before Start.
func (a *Application) Attach(service system.Service) error {
	return a.manager.Register(service)
}

// Start begins all registered services.
func (a *Application) Start(ctx context.Context) error {
	return a.manager.Start(ctx)
}

// Stop stops all services.
func (a *Application) Stop(ctx context.Context) error {
	return a.manager.Stop(ctx)
}
