package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/services/accounts"
	automationsvc "github.com/R3E-Network/service_layer/internal/app/services/automation"
	"github.com/R3E-Network/service_layer/internal/app/services/functions"
	gasbanksvc "github.com/R3E-Network/service_layer/internal/app/services/gasbank"
	oraclesvc "github.com/R3E-Network/service_layer/internal/app/services/oracle"
	pricefeedsvc "github.com/R3E-Network/service_layer/internal/app/services/pricefeed"
	randomsvc "github.com/R3E-Network/service_layer/internal/app/services/random"
	"github.com/R3E-Network/service_layer/internal/app/services/secrets"
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
	Secrets    storage.SecretStore
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
	Secrets    *secrets.Service
	Random     *randomsvc.Service
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
	if stores.Secrets == nil {
		stores.Secrets = mem
	}

	manager := system.NewManager()

	acctService := accounts.New(stores.Accounts, log)
	funcService := functions.New(stores.Accounts, stores.Functions, log)
	secretsService := secrets.New(stores.Secrets, log)
	teeMode := strings.ToLower(strings.TrimSpace(os.Getenv("TEE_MODE")))
	var executor functions.FunctionExecutor
	switch teeMode {
	case "mock", "disabled", "off":
		log.Warn("TEE_MODE set to mock; using mock TEE executor")
		executor = functions.NewMockTEEExecutor()
	default:
		executor = functions.NewTEEExecutor(secretsService)
	}
	funcService.AttachExecutor(executor)
	funcService.AttachSecretResolver(secretsService)
	trigService := triggers.New(stores.Accounts, stores.Functions, stores.Triggers, log)
	gasService := gasbanksvc.New(stores.Accounts, stores.GasBank, log)
	automationService := automationsvc.New(stores.Accounts, stores.Functions, stores.Automation, log)
	priceFeedService := pricefeedsvc.New(stores.Accounts, stores.PriceFeeds, log)
	oracleService := oraclesvc.New(stores.Accounts, stores.Oracle, log)
	randomService := randomsvc.New(log)

	httpClient := &http.Client{Timeout: 10 * time.Second}

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
	if endpoint := strings.TrimSpace(os.Getenv("PRICEFEED_FETCH_URL")); endpoint != "" {
		fetcher, err := pricefeedsvc.NewHTTPFetcher(httpClient, endpoint, os.Getenv("PRICEFEED_FETCH_KEY"), log)
		if err != nil {
			log.WithError(err).Warn("configure price feed fetcher")
		} else {
			priceRunner.WithFetcher(fetcher)
		}
	} else {
		log.Warn("PRICEFEED_FETCH_URL not set; price feed refresher disabled")
	}

	oracleRunner := oraclesvc.NewDispatcher(oracleService, log)
	if endpoint := strings.TrimSpace(os.Getenv("ORACLE_RESOLVER_URL")); endpoint != "" {
		resolver, err := oraclesvc.NewHTTPResolver(httpClient, endpoint, os.Getenv("ORACLE_RESOLVER_KEY"), log)
		if err != nil {
			log.WithError(err).Warn("configure oracle resolver")
		} else {
			oracleRunner.WithResolver(resolver)
		}
	} else {
		log.Warn("ORACLE_RESOLVER_URL not set; oracle dispatcher disabled")
	}

	var settlement system.Service
	if endpoint := strings.TrimSpace(os.Getenv("GASBANK_RESOLVER_URL")); endpoint != "" {
		resolver, err := gasbanksvc.NewHTTPWithdrawalResolver(httpClient, endpoint, os.Getenv("GASBANK_RESOLVER_KEY"), log)
		if err != nil {
			log.WithError(err).Warn("configure gas bank resolver")
		} else {
			settlement = gasbanksvc.NewSettlementPoller(stores.GasBank, gasService, resolver, log)
		}
	} else {
		log.Warn("GASBANK_RESOLVER_URL not set; gas bank settlement disabled")
	}

	services := []system.Service{autoRunner, priceRunner, oracleRunner}
	if settlement != nil {
		services = append(services, settlement)
	}

	for _, svc := range services {
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
		Secrets:    secretsService,
		Random:     randomService,
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
