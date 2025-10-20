package functions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/automation"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	automationsvc "github.com/R3E-Network/service_layer/internal/app/services/automation"
	gasbanksvc "github.com/R3E-Network/service_layer/internal/app/services/gasbank"
	oraclesvc "github.com/R3E-Network/service_layer/internal/app/services/oracle"
	pricefeedsvc "github.com/R3E-Network/service_layer/internal/app/services/pricefeed"
	"github.com/R3E-Network/service_layer/internal/app/services/triggers"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service manages function definitions.
type Service struct {
	accounts   storage.AccountStore
	store      storage.FunctionStore
	log        *logger.Logger
	triggers   *triggers.Service
	automation *automationsvc.Service
	priceFeeds *pricefeedsvc.Service
	oracle     *oraclesvc.Service
	gasBank    *gasbanksvc.Service
	executor   FunctionExecutor
}

// New constructs a function service.
func New(accounts storage.AccountStore, store storage.FunctionStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("functions")
	}
	return &Service{accounts: accounts, store: store, log: log}
}

// AttachDependencies wires auxiliary services so function workflows can drive
// cross-domain actions (triggers, automation, oracle, price feeds, gas bank).
func (s *Service) AttachDependencies(
	triggers *triggers.Service,
	automation *automationsvc.Service,
	priceFeeds *pricefeedsvc.Service,
	oracle *oraclesvc.Service,
	gasBank *gasbanksvc.Service,
) {
	s.triggers = triggers
	s.automation = automation
	s.priceFeeds = priceFeeds
	s.oracle = oracle
	s.gasBank = gasBank
}

// AttachExecutor injects a function executor implementation.
func (s *Service) AttachExecutor(exec FunctionExecutor) {
	s.executor = exec
}

// Create registers a new function definition.
func (s *Service) Create(ctx context.Context, def function.Definition) (function.Definition, error) {
	if def.AccountID == "" {
		return function.Definition{}, fmt.Errorf("account_id is required")
	}
	if def.Name == "" {
		return function.Definition{}, fmt.Errorf("name is required")
	}
	if def.Source == "" {
		return function.Definition{}, fmt.Errorf("source is required")
	}

	if s.accounts != nil {
		if _, err := s.accounts.GetAccount(ctx, def.AccountID); err != nil {
			return function.Definition{}, fmt.Errorf("account validation failed: %w", err)
		}
	}

	created, err := s.store.CreateFunction(ctx, def)
	if err != nil {
		return function.Definition{}, err
	}
	s.log.Infof("function %s created", created.ID)
	return created, nil
}

// Update overwrites mutable fields of a function definition.
func (s *Service) Update(ctx context.Context, def function.Definition) (function.Definition, error) {
	existing, err := s.store.GetFunction(ctx, def.ID)
	if err != nil {
		return function.Definition{}, err
	}

	if def.Name == "" {
		def.Name = existing.Name
	}
	if def.Description == "" {
		def.Description = existing.Description
	}
	if def.Source == "" {
		def.Source = existing.Source
	}
	if len(def.Secrets) == 0 {
		def.Secrets = existing.Secrets
	}
	def.AccountID = existing.AccountID

	updated, err := s.store.UpdateFunction(ctx, def)
	if err != nil {
		return function.Definition{}, err
	}
	s.log.Infof("function %s updated", def.ID)
	return updated, nil
}

// Get retrieves a function by identifier.
func (s *Service) Get(ctx context.Context, id string) (function.Definition, error) {
	return s.store.GetFunction(ctx, id)
}

// List returns functions belonging to an account.
func (s *Service) List(ctx context.Context, accountID string) ([]function.Definition, error) {
	return s.store.ListFunctions(ctx, accountID)
}

var errDependencyUnavailable = errors.New("dependent service not configured")

// RegisterTrigger delegates trigger creation to the trigger service while
// preserving the function-centric surface area.
func (s *Service) RegisterTrigger(ctx context.Context, trg trigger.Trigger) (trigger.Trigger, error) {
	if s.triggers == nil {
		return trigger.Trigger{}, fmt.Errorf("register trigger: %w", errDependencyUnavailable)
	}
	return s.triggers.Register(ctx, trg)
}

// ScheduleAutomationJob creates a job through the automation service.
func (s *Service) ScheduleAutomationJob(ctx context.Context, accountID, functionID, name, schedule, description string) (automation.Job, error) {
	if s.automation == nil {
		return automation.Job{}, fmt.Errorf("create automation job: %w", errDependencyUnavailable)
	}
	return s.automation.CreateJob(ctx, accountID, functionID, name, schedule, description)
}

// UpdateAutomationJob updates an automation job via the automation service.
func (s *Service) UpdateAutomationJob(ctx context.Context, jobID string, name, schedule, description *string) (automation.Job, error) {
	if s.automation == nil {
		return automation.Job{}, fmt.Errorf("update automation job: %w", errDependencyUnavailable)
	}
	return s.automation.UpdateJob(ctx, jobID, name, schedule, description, nil)
}

// SetAutomationEnabled toggles a job's enabled flag.
func (s *Service) SetAutomationEnabled(ctx context.Context, jobID string, enabled bool) (automation.Job, error) {
	if s.automation == nil {
		return automation.Job{}, fmt.Errorf("set automation enabled: %w", errDependencyUnavailable)
	}
	return s.automation.SetEnabled(ctx, jobID, enabled)
}

// CreatePriceFeed provisions a feed via the price feed service.
func (s *Service) CreatePriceFeed(ctx context.Context, accountID, baseAsset, quoteAsset, updateInterval, heartbeat string, deviation float64) (pricefeed.Feed, error) {
	if s.priceFeeds == nil {
		return pricefeed.Feed{}, fmt.Errorf("create price feed: %w", errDependencyUnavailable)
	}
	return s.priceFeeds.CreateFeed(ctx, accountID, baseAsset, quoteAsset, updateInterval, heartbeat, deviation)
}

// RecordPriceSnapshot records a snapshot via the price feed service.
func (s *Service) RecordPriceSnapshot(ctx context.Context, feedID string, price float64, source string, collectedAt time.Time) (pricefeed.Snapshot, error) {
	if s.priceFeeds == nil {
		return pricefeed.Snapshot{}, fmt.Errorf("record price snapshot: %w", errDependencyUnavailable)
	}
	return s.priceFeeds.RecordSnapshot(ctx, feedID, price, source, collectedAt)
}

// CreateOracleRequest creates a request via the oracle service.
func (s *Service) CreateOracleRequest(ctx context.Context, accountID, dataSourceID, payload string) (oracle.Request, error) {
	if s.oracle == nil {
		return oracle.Request{}, fmt.Errorf("create oracle request: %w", errDependencyUnavailable)
	}
	return s.oracle.CreateRequest(ctx, accountID, dataSourceID, payload)
}

// CompleteOracleRequest marks an oracle request as completed.
func (s *Service) CompleteOracleRequest(ctx context.Context, requestID, result string) (oracle.Request, error) {
	if s.oracle == nil {
		return oracle.Request{}, fmt.Errorf("complete oracle request: %w", errDependencyUnavailable)
	}
	return s.oracle.CompleteRequest(ctx, requestID, result)
}

// EnsureGasAccount ensures the gas bank has an account for the owner.
func (s *Service) EnsureGasAccount(ctx context.Context, accountID, wallet string) (gasbank.Account, error) {
	if s.gasBank == nil {
		return gasbank.Account{}, fmt.Errorf("ensure gas account: %w", errDependencyUnavailable)
	}
	return s.gasBank.EnsureAccount(ctx, accountID, wallet)
}

// Execute runs the specified function definition with the provided payload.
func (s *Service) Execute(ctx context.Context, id string, payload map[string]any) (function.ExecutionResult, error) {
	if s.executor == nil {
		return function.ExecutionResult{}, fmt.Errorf("execute function: %w", errDependencyUnavailable)
	}
	def, err := s.store.GetFunction(ctx, id)
	if err != nil {
		return function.ExecutionResult{}, err
	}
	if s.accounts != nil {
		if _, err := s.accounts.GetAccount(ctx, def.AccountID); err != nil {
			return function.ExecutionResult{}, fmt.Errorf("account validation failed: %w", err)
		}
	}
	return s.executor.Execute(ctx, def, payload)
}

// FunctionExecutor executes function definitions.
type FunctionExecutor interface {
	Execute(ctx context.Context, def function.Definition, payload map[string]any) (function.ExecutionResult, error)
}
