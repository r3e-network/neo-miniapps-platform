package functions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/pkg/metrics"
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
	accounts        AccountChecker
	store           Store
	log             *logger.Logger
	executor        FunctionExecutor
	secrets         SecretResolver
	actionProcessor ActionProcessor
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
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("functions")
	}
	svc := &Service{accounts: accounts, store: store, log: log}
	svc.SetName(svc.Name())
	return svc
}

// Start/Stop are inherited from framework.ServiceBase.

// AttachActionProcessor wires the action processor for handling devpack actions.
// This decouples the Functions service from direct dependencies on other services.
func (s *Service) AttachActionProcessor(processor ActionProcessor) {
	s.actionProcessor = processor
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
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
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
func (s *Service) Create(ctx context.Context, def Definition) (Definition, error) {
	if def.AccountID == "" {
		return Definition{}, core.RequiredError("account_id")
	}
	if def.Name == "" {
		return Definition{}, core.RequiredError("name")
	}
	if def.Source == "" {
		return Definition{}, core.RequiredError("source")
	}

	if err := s.accounts.AccountExists(ctx, def.AccountID); err != nil {
		return Definition{}, fmt.Errorf("account validation failed: %w", err)
	}
	if s.secrets != nil && len(def.Secrets) > 0 {
		if _, err := s.secrets.ResolveSecrets(ctx, def.AccountID, def.Secrets); err != nil {
			return Definition{}, fmt.Errorf("secret validation failed: %w", err)
		}
	}

	created, err := s.store.CreateFunction(ctx, def)
	if err != nil {
		return Definition{}, err
	}
	s.log.WithField("function_id", created.ID).
		WithField("account_id", created.AccountID).
		Info("function created")
	return created, nil
}

// Update overwrites mutable fields of a function definition.
func (s *Service) Update(ctx context.Context, def Definition) (Definition, error) {
	existing, err := s.store.GetFunction(ctx, def.ID)
	if err != nil {
		return Definition{}, err
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
			return Definition{}, fmt.Errorf("secret validation failed: %w", err)
		}
	}
	if err := s.accounts.AccountExists(ctx, def.AccountID); err != nil {
		return Definition{}, fmt.Errorf("account validation failed: %w", err)
	}

	updated, err := s.store.UpdateFunction(ctx, def)
	if err != nil {
		return Definition{}, err
	}
	s.log.WithField("function_id", def.ID).
		WithField("account_id", updated.AccountID).
		Info("function updated")
	return updated, nil
}

// Get retrieves a function by identifier.
func (s *Service) Get(ctx context.Context, id string) (Definition, error) {
	return s.store.GetFunction(ctx, id)
}

// List returns functions belonging to an account.
func (s *Service) List(ctx context.Context, accountID string) ([]Definition, error) {
	return s.store.ListFunctions(ctx, accountID)
}

var errDependencyUnavailable = errors.New("dependent service not configured")

// Execute runs the specified function definition with the provided payload and records the run.
func (s *Service) Execute(ctx context.Context, id string, payload map[string]any) (Execution, error) {
	if s.executor == nil {
		return Execution{}, fmt.Errorf("execute function: %w", errDependencyUnavailable)
	}
	def, err := s.store.GetFunction(ctx, id)
	if err != nil {
		return Execution{}, err
	}
	if err := s.accounts.AccountExists(ctx, def.AccountID); err != nil {
		return Execution{}, fmt.Errorf("account validation failed: %w", err)
	}

	execPayload := clonePayload(payload)
	inputCopy := clonePayload(payload)
	result, execErr := s.executor.Execute(ctx, def, execPayload)
	if result.FunctionID == "" {
		result.FunctionID = def.ID
	}
	var actionResults []ActionResult
	if execErr == nil && len(result.Actions) > 0 {
		actionResults, execErr = s.processActions(ctx, def, result.Actions)
	}
	result.ActionResults = cloneActionResults(actionResults)
	status := result.Status
	if execErr != nil {
		status = ExecutionStatusFailed
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
		status = ExecutionStatusSucceeded
	}
	if result.Duration == 0 && !result.StartedAt.IsZero() && !result.CompletedAt.IsZero() {
		result.Duration = result.CompletedAt.Sub(result.StartedAt)
	}

	record := Execution{
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
			return Execution{}, errors.Join(execErr, recordErr)
		}
		s.log.WithError(storeErr).Error("failed to persist execution history")
		recordErr = fmt.Errorf("record execution: %w", storeErr)
		return Execution{}, recordErr
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
func (s *Service) GetExecution(ctx context.Context, id string) (Execution, error) {
	return s.store.GetExecution(ctx, id)
}

// ListExecutions returns execution history for the function in descending order.
func (s *Service) ListExecutions(ctx context.Context, functionID string, limit int) ([]Execution, error) {
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListFunctionExecutions(ctx, functionID, clamped)
}

// FunctionExecutor executes function definitions.
type FunctionExecutor interface {
	Execute(ctx context.Context, def Definition, payload map[string]any) (ExecutionResult, error)
}

// SecretResolver resolves secret values for a given account.
type SecretResolver interface {
	ResolveSecrets(ctx context.Context, accountID string, names []string) (map[string]string, error)
}

// SecretAwareExecutor can accept a secret resolver for runtime lookups.
type SecretAwareExecutor interface {
	SetSecretResolver(resolver SecretResolver)
}
