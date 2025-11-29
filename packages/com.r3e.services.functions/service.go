package functions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/applications/metrics"
	"github.com/R3E-Network/service_layer/applications/storage"
	"github.com/R3E-Network/service_layer/domain/automation"
	"github.com/R3E-Network/service_layer/domain/function"
	"github.com/R3E-Network/service_layer/domain/gasbank"
	"github.com/R3E-Network/service_layer/domain/oracle"
	automationsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.automation"
	datafeedsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datafeeds"
	datalinksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datalink"
	datastreamsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datastreams"
	gasbanksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.gasbank"
	oraclesvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.oracle"
	vrfsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.vrf"
	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Compile-time check: Service exposes Invoke for the core engine adapter.
type computeInvoker interface {
	Invoke(context.Context, any) (any, error)
}

var _ computeInvoker = (*Service)(nil)

// Service manages function definitions.
type Service struct {
	framework.ServiceBase
	base        *core.Base
	store       storage.FunctionStore
	log         *logger.Logger
	automation  *automationsvc.Service
	dataFeeds   *datafeedsvc.Service
	dataStreams *datastreamsvc.Service
	dataLink    *datalinksvc.Service
	oracle      *oraclesvc.Service
	gasBank     *gasbanksvc.Service
	vrf         *vrfsvc.Service
	executor    FunctionExecutor
	secrets     SecretResolver
}

// Name returns the stable engine module name.
func (s *Service) Name() string { return "functions" }

// Domain reports the service domain for engine grouping.
func (s *Service) Domain() string { return "functions" }

// Manifest describes the service contract for the engine OS.
func (s *Service) Manifest() *framework.Manifest {
	return &framework.Manifest{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Description:  "Function registry and execution runtime",
		Layer:        "service",
		DependsOn:    []string{"store", "svc-accounts"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceCompute, engine.APISurfaceEvent, engine.APISurfaceData},
		Capabilities: []string{"functions"},
		Quotas:       map[string]string{"compute": "function-exec"},
	}
}

// Descriptor advertises the service for system discovery.
func (s *Service) Descriptor() core.Descriptor { return s.Manifest().ToDescriptor() }

// New constructs a function service.
func New(accounts storage.AccountStore, store storage.FunctionStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("functions")
	}
	svc := &Service{base: core.NewBase(accounts), store: store, log: log}
	svc.SetName(svc.Name())
	return svc
}

// Start marks the functions service ready.
func (s *Service) Start(ctx context.Context) error { _ = ctx; s.MarkReady(true); return nil }

// Stop clears readiness.
func (s *Service) Stop(ctx context.Context) error { _ = ctx; s.MarkReady(false); return nil }

// AttachDependencies wires auxiliary services so function workflows can drive
// cross-domain actions (automation, feeds, data streams, datalink, oracle, gas bank, vrf).
func (s *Service) AttachDependencies(
	automation *automationsvc.Service,
	dataFeeds *datafeedsvc.Service,
	dataStreams *datastreamsvc.Service,
	dataLink *datalinksvc.Service,
	oracle *oraclesvc.Service,
	gasBank *gasbanksvc.Service,
	vrf *vrfsvc.Service,
) {
	s.automation = automation
	s.dataFeeds = dataFeeds
	s.dataStreams = dataStreams
	s.dataLink = dataLink
	s.oracle = oracle
	s.gasBank = gasBank
	s.vrf = vrf
}

// AttachExecutor injects a function executor implementation.
func (s *Service) AttachExecutor(exec FunctionExecutor) {
	s.executor = exec
	if aware, ok := exec.(SecretAwareExecutor); ok {
		aware.SetSecretResolver(s.secrets)
	}
}

// AttachSecretResolver wires the secret resolver used for validation and
// execution-time lookup.
func (s *Service) AttachSecretResolver(resolver SecretResolver) {
	s.secrets = resolver
	if aware, ok := s.executor.(SecretAwareExecutor); ok {
		aware.SetSecretResolver(resolver)
	}
}

// Ready reports readiness.
func (s *Service) Ready(ctx context.Context) error {
	return s.ServiceBase.Ready(ctx)
}

// Invoke implements engine.ComputeEngine for the service engine by executing a function.
func (s *Service) Invoke(ctx context.Context, payload any) (any, error) {
	if err := s.Ready(ctx); err != nil {
		return nil, err
	}
	req, ok := payload.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invoke payload must be a map")
	}
	fnID, _ := req["function_id"].(string)
	accountID, _ := req["account_id"].(string)
	rawInput, _ := req["input"]
	input := ""
	switch v := rawInput.(type) {
	case string:
		input = v
	case map[string]any:
		if b, err := json.Marshal(v); err == nil {
			input = string(b)
		}
	}
	if fnID == "" || accountID == "" {
		return nil, fmt.Errorf("account_id and function_id required")
	}
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	if _, err := s.Get(ctx, fnID); err != nil {
		return nil, err
	}
	def, err := s.Get(ctx, fnID)
	if err != nil {
		return nil, err
	}
	result, execErr := s.executor.Execute(ctx, def, map[string]any{"input": input})
	if execErr != nil {
		return nil, execErr
	}
	return result, nil
}

// Create registers a new function definition.
func (s *Service) Create(ctx context.Context, def function.Definition) (function.Definition, error) {
	if def.AccountID == "" {
		return function.Definition{}, core.RequiredError("account_id")
	}
	if def.Name == "" {
		return function.Definition{}, core.RequiredError("name")
	}
	if def.Source == "" {
		return function.Definition{}, core.RequiredError("source")
	}

	if err := s.base.EnsureAccount(ctx, def.AccountID); err != nil {
		return function.Definition{}, fmt.Errorf("account validation failed: %w", err)
	}
	if s.secrets != nil && len(def.Secrets) > 0 {
		if _, err := s.secrets.ResolveSecrets(ctx, def.AccountID, def.Secrets); err != nil {
			return function.Definition{}, fmt.Errorf("secret validation failed: %w", err)
		}
	}

	created, err := s.store.CreateFunction(ctx, def)
	if err != nil {
		return function.Definition{}, err
	}
	s.log.WithField("function_id", created.ID).
		WithField("account_id", created.AccountID).
		Info("function created")
	return created, nil
}

// Update overwrites mutable fields of a function definition.
func (s *Service) Update(ctx context.Context, def function.Definition) (function.Definition, error) {
	existing, err := s.store.GetFunction(ctx, def.ID)
	if err != nil {
		return function.Definition{}, err
	}

	secretsProvided := def.Secrets != nil

	if def.Name == "" {
		def.Name = existing.Name
	}
	if def.Description == "" {
		def.Description = existing.Description
	}
	if def.Source == "" {
		def.Source = existing.Source
	}
	if def.Secrets == nil {
		def.Secrets = existing.Secrets
	}
	def.AccountID = existing.AccountID

	if secretsProvided && s.secrets != nil && len(def.Secrets) > 0 {
		if _, err := s.secrets.ResolveSecrets(ctx, def.AccountID, def.Secrets); err != nil {
			return function.Definition{}, fmt.Errorf("secret validation failed: %w", err)
		}
	}
	if err := s.base.EnsureAccount(ctx, def.AccountID); err != nil {
		return function.Definition{}, fmt.Errorf("account validation failed: %w", err)
	}

	updated, err := s.store.UpdateFunction(ctx, def)
	if err != nil {
		return function.Definition{}, err
	}
	s.log.WithField("function_id", def.ID).
		WithField("account_id", updated.AccountID).
		Info("function updated")
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

// Execute runs the specified function definition with the provided payload and records the run.
func (s *Service) Execute(ctx context.Context, id string, payload map[string]any) (function.Execution, error) {
	if s.executor == nil {
		return function.Execution{}, fmt.Errorf("execute function: %w", errDependencyUnavailable)
	}
	def, err := s.store.GetFunction(ctx, id)
	if err != nil {
		return function.Execution{}, err
	}
	if err := s.base.EnsureAccount(ctx, def.AccountID); err != nil {
		return function.Execution{}, fmt.Errorf("account validation failed: %w", err)
	}

	execPayload := clonePayload(payload)
	inputCopy := clonePayload(payload)
	result, execErr := s.executor.Execute(ctx, def, execPayload)
	if result.FunctionID == "" {
		result.FunctionID = def.ID
	}
	var actionResults []function.ActionResult
	if execErr == nil && len(result.Actions) > 0 {
		actionResults, execErr = s.processActions(ctx, def, result.Actions)
	}
	result.ActionResults = cloneActionResults(actionResults)
	status := result.Status
	if execErr != nil {
		status = function.ExecutionStatusFailed
		result.Error = strings.TrimSpace(execErr.Error())
		if result.StartedAt.IsZero() {
			result.StartedAt = time.Now().UTC()
		}
		if result.CompletedAt.IsZero() {
			result.CompletedAt = time.Now().UTC()
		}
		s.log.WithError(execErr).
			WithField("function_id", def.ID).
			WithField("account_id", def.AccountID).
			Warn("function execution failed")
	} else if status == "" {
		status = function.ExecutionStatusSucceeded
	}
	if result.Duration == 0 && !result.StartedAt.IsZero() && !result.CompletedAt.IsZero() {
		result.Duration = result.CompletedAt.Sub(result.StartedAt)
	}

	record := function.Execution{
		AccountID:   def.AccountID,
		FunctionID:  def.ID,
		Input:       inputCopy,
		Output:      clonePayload(result.Output),
		Logs:        cloneStrings(result.Logs),
		Error:       result.Error,
		Status:      status,
		StartedAt:   result.StartedAt,
		CompletedAt: result.CompletedAt,
		Duration:    result.Duration,
		Actions:     cloneActionResults(actionResults),
	}

	saved, storeErr := s.store.CreateExecution(ctx, record)
	if storeErr != nil {
		var recordErr error
		if execErr != nil {
			s.log.WithError(storeErr).Error("failed to persist execution history for errored run")
			recordErr = fmt.Errorf("record failed execution: %w", storeErr)
			return function.Execution{}, errors.Join(execErr, recordErr)
		}
		s.log.WithError(storeErr).Error("failed to persist execution history")
		recordErr = fmt.Errorf("record execution: %w", storeErr)
		return function.Execution{}, recordErr
	}

	metrics.RecordFunctionExecution(string(status), saved.Duration)

	if execErr != nil {
		return saved, execErr
	}

	if len(actionResults) > 0 {
		saved.Actions = cloneActionResults(actionResults)
	}
	return saved, nil
}

// GetExecution fetches a persisted execution run.
func (s *Service) GetExecution(ctx context.Context, id string) (function.Execution, error) {
	return s.store.GetExecution(ctx, id)
}

// ListExecutions returns execution history for the function in descending order.
func (s *Service) ListExecutions(ctx context.Context, functionID string, limit int) ([]function.Execution, error) {
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListFunctionExecutions(ctx, functionID, clamped)
}

// FunctionExecutor executes function definitions.
type FunctionExecutor interface {
	Execute(ctx context.Context, def function.Definition, payload map[string]any) (function.ExecutionResult, error)
}

// SecretResolver resolves secret values for a given account.
type SecretResolver interface {
	ResolveSecrets(ctx context.Context, accountID string, names []string) (map[string]string, error)
}

// SecretAwareExecutor can accept a secret resolver for runtime lookups.
type SecretAwareExecutor interface {
	SetSecretResolver(resolver SecretResolver)
}
