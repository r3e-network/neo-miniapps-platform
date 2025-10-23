package functions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/automation"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	"github.com/R3E-Network/service_layer/internal/app/metrics"
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
	secrets    SecretResolver
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

	secretsProvided := len(def.Secrets) > 0

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

	if secretsProvided && s.secrets != nil && len(def.Secrets) > 0 {
		if _, err := s.secrets.ResolveSecrets(ctx, def.AccountID, def.Secrets); err != nil {
			return function.Definition{}, fmt.Errorf("secret validation failed: %w", err)
		}
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

// Execute runs the specified function definition with the provided payload and records the run.
func (s *Service) Execute(ctx context.Context, id string, payload map[string]any) (function.Execution, error) {
	if s.executor == nil {
		return function.Execution{}, fmt.Errorf("execute function: %w", errDependencyUnavailable)
	}
	def, err := s.store.GetFunction(ctx, id)
	if err != nil {
		return function.Execution{}, err
	}
	if s.accounts != nil {
		if _, err := s.accounts.GetAccount(ctx, def.AccountID); err != nil {
			return function.Execution{}, fmt.Errorf("account validation failed: %w", err)
		}
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
		if execErr != nil {
			s.log.WithError(storeErr).Error("failed to persist execution history for errored run")
			return function.Execution{}, fmt.Errorf("record failed execution: %w", storeErr)
		}
		s.log.WithError(storeErr).Error("failed to persist execution history")
		return function.Execution{}, fmt.Errorf("record execution: %w", storeErr)
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

func (s *Service) processActions(ctx context.Context, def function.Definition, actions []function.Action) ([]function.ActionResult, error) {
	results := make([]function.ActionResult, 0, len(actions))
	var firstErr error

	for _, action := range actions {
		res := function.ActionResult{
			Action: action,
			Status: function.ActionStatusSucceeded,
		}

		switch action.Type {
		case function.ActionTypeGasBankEnsureAccount:
			accountResult, err := s.handleGasBankEnsure(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = accountResult
			}
		case function.ActionTypeGasBankWithdraw:
			withdrawResult, err := s.handleGasBankWithdraw(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = withdrawResult
			}
		case function.ActionTypeOracleCreateRequest:
			oracleResult, err := s.handleOracleCreateRequest(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = oracleResult
			}
		case function.ActionTypeTriggerRegister:
			triggerResult, err := s.handleTriggerRegister(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = triggerResult
			}
		case function.ActionTypeAutomationSchedule:
			autoResult, err := s.handleAutomationSchedule(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = autoResult
			}
		default:
			res.Status = function.ActionStatusFailed
			res.Error = fmt.Sprintf("unsupported action type %q", action.Type)
		}

		results = append(results, res)
		if res.Status == function.ActionStatusFailed && firstErr == nil {
			firstErr = fmt.Errorf("devpack action %s failed: %s", action.Type, res.Error)
		}
	}

	return results, firstErr
}

func (s *Service) handleGasBankEnsure(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.gasBank == nil {
		return nil, fmt.Errorf("gas bank: %w", errDependencyUnavailable)
	}

	wallet := stringParam(params, "wallet", "")
	acct, err := s.gasBank.EnsureAccount(ctx, def.AccountID, wallet)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"account": structToMap(acct),
	}, nil
}

func (s *Service) handleGasBankWithdraw(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.gasBank == nil {
		return nil, fmt.Errorf("gas bank: %w", errDependencyUnavailable)
	}

	gasAccountID := stringParam(params, "gasAccountId", "")
	wallet := stringParam(params, "wallet", "")
	if gasAccountID == "" && wallet == "" {
		return nil, errors.New("gasbank.withdraw requires gasAccountId or wallet")
	}
	if gasAccountID == "" {
		acct, err := s.gasBank.EnsureAccount(ctx, def.AccountID, wallet)
		if err != nil {
			return nil, fmt.Errorf("ensure account: %w", err)
		}
		gasAccountID = acct.ID
	}

	amount, err := floatParam(params, "amount")
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}
	if amount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}

	toAddress := stringParam(params, "to", "")
	acct, tx, err := s.gasBank.Withdraw(ctx, gasAccountID, amount, toAddress)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"account":     structToMap(acct),
		"transaction": structToMap(tx),
	}, nil
}

func (s *Service) handleOracleCreateRequest(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.oracle == nil {
		return nil, fmt.Errorf("oracle: %w", errDependencyUnavailable)
	}
	dataSourceID := stringParam(params, "dataSourceId", "")
	if dataSourceID == "" {
		return nil, errors.New("oracle.createRequest requires dataSourceId")
	}
	payloadStr, err := stringOrJSON(params["payload"])
	if err != nil {
		return nil, fmt.Errorf("payload: %w", err)
	}
	req, err := s.oracle.CreateRequest(ctx, def.AccountID, dataSourceID, payloadStr)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"request": structToMap(req),
	}, nil
}

func (s *Service) handleTriggerRegister(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.triggers == nil {
		return nil, fmt.Errorf("triggers: %w", errDependencyUnavailable)
	}

	triggerType := stringParam(params, "type", "")
	if triggerType == "" {
		return nil, errors.New("triggers.register requires type")
	}
	rule := stringParam(params, "rule", "")
	configMap, err := mapStringStringParam(params, "config")
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	enabled := boolParam(params, "enabled", true)

	trg := trigger.Trigger{
		AccountID:  def.AccountID,
		FunctionID: def.ID,
		Type:       trigger.Type(triggerType),
		Rule:       rule,
		Config:     configMap,
		Enabled:    enabled,
	}
	created, err := s.RegisterTrigger(ctx, trg)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"trigger": structToMap(created),
	}, nil
}

func (s *Service) handleAutomationSchedule(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.automation == nil {
		return nil, fmt.Errorf("automation: %w", errDependencyUnavailable)
	}

	name := stringParam(params, "name", "")
	if name == "" {
		return nil, errors.New("automation.schedule requires name")
	}
	schedule := stringParam(params, "schedule", "")
	if schedule == "" {
		return nil, errors.New("automation.schedule requires schedule")
	}
	description := stringParam(params, "description", "")
	job, err := s.ScheduleAutomationJob(ctx, def.AccountID, def.ID, name, schedule, description)
	if err != nil {
		return nil, err
	}
	if _, exists := params["enabled"]; exists {
		if enabled := boolParam(params, "enabled", true); enabled != job.Enabled {
			if _, err := s.SetAutomationEnabled(ctx, job.ID, enabled); err != nil {
				return nil, fmt.Errorf("set automation enabled: %w", err)
			}
			job.Enabled = enabled
		}
	}
	return map[string]any{
		"job": structToMap(job),
	}, nil
}

// GetExecution fetches a persisted execution run.
func (s *Service) GetExecution(ctx context.Context, id string) (function.Execution, error) {
	return s.store.GetExecution(ctx, id)
}

// ListExecutions returns execution history for the function in descending order.
func (s *Service) ListExecutions(ctx context.Context, functionID string, limit int) ([]function.Execution, error) {
	return s.store.ListFunctionExecutions(ctx, functionID, limit)
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

func clonePayload(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		if payload == nil {
			return nil
		}
		return map[string]any{}
	}
	dup := make(map[string]any, len(payload))
	for k, v := range payload {
		dup[k] = cloneValue(v)
	}
	return dup
}

func cloneValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return clonePayload(v)
	case []any:
		if len(v) == 0 {
			return []any{}
		}
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = cloneValue(item)
		}
		return out
	case []byte:
		if v == nil {
			return []byte(nil)
		}
		out := make([]byte, len(v))
		copy(out, v)
		return out
	default:
		return v
	}
}

func cloneStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	dup := make([]string, len(items))
	copy(dup, items)
	return dup
}

func cloneActionResults(actions []function.ActionResult) []function.ActionResult {
	if len(actions) == 0 {
		return nil
	}
	copied := make([]function.ActionResult, len(actions))
	for i, a := range actions {
		copied[i] = function.ActionResult{
			Action: function.Action{
				ID:     a.ID,
				Type:   a.Type,
				Params: clonePayload(a.Params),
			},
			Status: a.Status,
			Result: clonePayload(a.Result),
			Error:  a.Error,
			Meta:   clonePayload(a.Meta),
		}
	}
	return copied
}

func stringParam(params map[string]any, key, fallback string) string {
	if params == nil {
		return fallback
	}
	if value, ok := params[key]; ok {
		switch v := value.(type) {
		case string:
			return v
		case fmt.Stringer:
			return v.String()
		default:
			return fmt.Sprint(v)
		}
	}
	return fallback
}

func floatParam(params map[string]any, key string) (float64, error) {
	if params == nil {
		return 0, fmt.Errorf("missing %s", key)
	}
	value, ok := params[key]
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case json.Number:
		return v.Float64()
	case string:
		return strconv.ParseFloat(strings.TrimSpace(v), 64)
	default:
		return 0, fmt.Errorf("unsupported number type %T", v)
	}
}

func boolParam(params map[string]any, key string, fallback bool) bool {
	if params == nil {
		return fallback
	}
	value, ok := params[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	case int:
		return v != 0
	default:
		return fallback
	}
}

func mapStringStringParam(params map[string]any, key string) (map[string]string, error) {
	if params == nil {
		return nil, nil
	}
	value, ok := params[key]
	if !ok || value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case map[string]string:
		copyMap := make(map[string]string, len(v))
		for k, val := range v {
			copyMap[k] = val
		}
		return copyMap, nil
	case map[string]any:
		copyMap := make(map[string]string, len(v))
		for k, val := range v {
			copyMap[k] = fmt.Sprint(val)
		}
		return copyMap, nil
	default:
		return nil, fmt.Errorf("expected map for %s, got %T", key, value)
	}
}

func stringOrJSON(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	switch v := value.(type) {
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
}

func structToMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]any{
			"error": fmt.Sprintf("marshal: %v", err),
		}
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return map[string]any{
			"error": fmt.Sprintf("unmarshal: %v", err),
		}
	}
	return result
}
