package functions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	"github.com/R3E-Network/service_layer/internal/app/domain/automation"
	domaindatastreams "github.com/R3E-Network/service_layer/internal/app/domain/datastreams"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	"github.com/R3E-Network/service_layer/internal/app/metrics"
	automationsvc "github.com/R3E-Network/service_layer/internal/app/services/automation"
	datafeedsvc "github.com/R3E-Network/service_layer/internal/app/services/datafeeds"
	datalinksvc "github.com/R3E-Network/service_layer/internal/app/services/datalink"
	datastreamsvc "github.com/R3E-Network/service_layer/internal/app/services/datastreams"
	gasbanksvc "github.com/R3E-Network/service_layer/internal/app/services/gasbank"
	oraclesvc "github.com/R3E-Network/service_layer/internal/app/services/oracle"
	pricefeedsvc "github.com/R3E-Network/service_layer/internal/app/services/pricefeed"
	randomsvc "github.com/R3E-Network/service_layer/internal/app/services/random"
	"github.com/R3E-Network/service_layer/internal/app/services/triggers"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service manages function definitions.
type Service struct {
	base        *core.Base
	store       storage.FunctionStore
	log         *logger.Logger
	triggers    *triggers.Service
	automation  *automationsvc.Service
	priceFeeds  *pricefeedsvc.Service
	dataFeeds   *datafeedsvc.Service
	dataStreams *datastreamsvc.Service
	dataLink    *datalinksvc.Service
	oracle      *oraclesvc.Service
	gasBank     *gasbanksvc.Service
	random      *randomsvc.Service
	executor    FunctionExecutor
	secrets     SecretResolver
}

// New constructs a function service.
func New(accounts storage.AccountStore, store storage.FunctionStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("functions")
	}
	return &Service{base: core.NewBase(accounts), store: store, log: log}
}

// AttachDependencies wires auxiliary services so function workflows can drive
// cross-domain actions (triggers, automation, feeds, data streams, datalink,
// oracle, gas bank, randomness).
func (s *Service) AttachDependencies(
	triggers *triggers.Service,
	automation *automationsvc.Service,
	priceFeeds *pricefeedsvc.Service,
	dataFeeds *datafeedsvc.Service,
	dataStreams *datastreamsvc.Service,
	dataLink *datalinksvc.Service,
	oracle *oraclesvc.Service,
	gasBank *gasbanksvc.Service,
	random *randomsvc.Service,
) {
	s.triggers = triggers
	s.automation = automation
	s.priceFeeds = priceFeeds
	s.dataFeeds = dataFeeds
	s.dataStreams = dataStreams
	s.dataLink = dataLink
	s.oracle = oracle
	s.gasBank = gasBank
	s.random = random
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
		case function.ActionTypeGasBankBalance:
			balanceResult, err := s.handleGasBankBalance(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = balanceResult
			}
		case function.ActionTypeGasBankListTx:
			listResult, err := s.handleGasBankListTransactions(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = listResult
			}
		case function.ActionTypeOracleCreateRequest:
			oracleResult, err := s.handleOracleCreateRequest(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = oracleResult
			}
		case function.ActionTypePriceFeedSnapshot:
			priceResult, err := s.handlePriceFeedSnapshot(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = priceResult
			}
		case function.ActionTypeDataFeedSubmit:
			dfResult, err := s.handleDataFeedSubmit(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = dfResult
			}
		case function.ActionTypeDatastreamPublish:
			dsResult, err := s.handleDatastreamPublish(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = dsResult
			}
		case function.ActionTypeDatalinkDeliver:
			dlResult, err := s.handleDatalinkDelivery(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = dlResult
			}
		case function.ActionTypeRandomGenerate:
			randomResult, err := s.handleRandomGenerate(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = randomResult
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
		ensured, err := s.gasBank.EnsureAccount(ctx, def.AccountID, wallet)
		if err != nil {
			return nil, fmt.Errorf("ensure account: %w", err)
		}
		gasAccountID = ensured.ID
	} else {
		existing, err := s.gasBank.GetAccount(ctx, gasAccountID)
		if err != nil {
			return nil, err
		}
		if existing.AccountID != def.AccountID {
			return nil, fmt.Errorf("gas account %s does not belong to account %s", gasAccountID, def.AccountID)
		}
	}

	amount, err := floatParam(params, "amount")
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}
	if amount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}

	toAddress := stringParam(params, "to", "")
	updated, tx, err := s.gasBank.Withdraw(ctx, def.AccountID, gasAccountID, amount, toAddress)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"account":     structToMap(updated),
		"transaction": structToMap(tx),
	}, nil
}

func (s *Service) handleGasBankBalance(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.gasBank == nil {
		return nil, fmt.Errorf("gas bank: %w", errDependencyUnavailable)
	}
	gasAccountID := stringParam(params, "gasAccountId", "")
	wallet := stringParam(params, "wallet", "")
	if gasAccountID == "" && wallet == "" {
		return nil, errors.New("gasbank.balance requires gasAccountId or wallet")
	}
	var acct gasbank.Account
	var err error
	if gasAccountID != "" {
		acct, err = s.gasBank.GetAccount(ctx, gasAccountID)
		if err != nil {
			return nil, err
		}
		if acct.AccountID != def.AccountID {
			return nil, fmt.Errorf("gas account %s does not belong to account %s", gasAccountID, def.AccountID)
		}
	} else {
		acct, err = s.gasBank.EnsureAccount(ctx, def.AccountID, wallet)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"account": structToMap(acct),
	}, nil
}

func (s *Service) handleGasBankListTransactions(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.gasBank == nil {
		return nil, fmt.Errorf("gas bank: %w", errDependencyUnavailable)
	}
	gasAccountID := stringParam(params, "gasAccountId", "")
	wallet := stringParam(params, "wallet", "")
	if gasAccountID == "" && wallet == "" {
		return nil, errors.New("gasbank.listTransactions requires gasAccountId or wallet")
	}
	var acct gasbank.Account
	var err error
	if gasAccountID != "" {
		acct, err = s.gasBank.GetAccount(ctx, gasAccountID)
		if err != nil {
			return nil, err
		}
		if acct.AccountID != def.AccountID {
			return nil, fmt.Errorf("gas account %s does not belong to account %s", gasAccountID, def.AccountID)
		}
	} else {
		acct, err = s.gasBank.EnsureAccount(ctx, def.AccountID, wallet)
		if err != nil {
			return nil, err
		}
	}
	status := stringParam(params, "status", "")
	txType := stringParam(params, "type", "")
	limit := intParam(params, "limit", core.DefaultListLimit)
	if limit <= 0 {
		limit = core.DefaultListLimit
	}
	txs, err := s.gasBank.ListTransactionsFiltered(ctx, acct.ID, txType, status, limit)
	if err != nil {
		return nil, err
	}
	serialized := make([]map[string]any, len(txs))
	for i, tx := range txs {
		serialized[i] = structToMap(tx)
	}
	return map[string]any{
		"transactions": serialized,
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
	alternateSources := stringSliceParam(params, "alternateSourceIds")
	if len(alternateSources) > 0 {
		var payloadObj map[string]any
		if trimmed := strings.TrimSpace(payloadStr); trimmed != "" {
			_ = json.Unmarshal([]byte(trimmed), &payloadObj)
		}
		if payloadObj == nil {
			payloadObj = make(map[string]any)
		}
		payloadObj["alternate_source_ids"] = alternateSources
		if updated, err := json.Marshal(payloadObj); err == nil {
			payloadStr = string(updated)
		}
	}
	req, err := s.oracle.CreateRequest(ctx, def.AccountID, dataSourceID, payloadStr)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"request": structToMap(req),
	}, nil
}

func (s *Service) handlePriceFeedSnapshot(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.priceFeeds == nil {
		return nil, fmt.Errorf("pricefeed: %w", errDependencyUnavailable)
	}
	feedID := stringParam(params, "feedId", "")
	if feedID == "" {
		feedID = stringParam(params, "feed_id", "")
	}
	if feedID == "" {
		return nil, errors.New("pricefeed.recordSnapshot requires feedId")
	}
	price, err := floatParam(params, "price")
	if err != nil {
		return nil, fmt.Errorf("price: %w", err)
	}
	if price <= 0 {
		return nil, errors.New("price must be positive")
	}
	source := stringParam(params, "source", "")
	collectedAt := time.Now().UTC()
	if ts := stringParam(params, "collectedAt", stringParam(params, "collected_at", "")); ts != "" {
		parsed, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			return nil, fmt.Errorf("collected_at: %w", err)
		}
		collectedAt = parsed
	}

	snap, err := s.priceFeeds.RecordSnapshot(ctx, feedID, price, source, collectedAt)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"snapshot": structToMap(snap),
	}, nil
}

func (s *Service) handleDataFeedSubmit(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.dataFeeds == nil {
		return nil, fmt.Errorf("datafeeds: %w", errDependencyUnavailable)
	}
	feedID := stringParam(params, "feedId", stringParam(params, "feed_id", ""))
	if feedID == "" {
		return nil, errors.New("datafeeds.submitUpdate requires feedId")
	}
	roundID := int64(intParam(params, "roundId", intParam(params, "round_id", 0)))
	if roundID <= 0 {
		return nil, errors.New("roundId must be positive")
	}
	price := stringParam(params, "price", "")
	if price == "" {
		return nil, errors.New("price is required")
	}
	ts := time.Now().UTC()
	if tsStr := stringParam(params, "timestamp", ""); tsStr != "" {
		parsed, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			return nil, fmt.Errorf("timestamp: %w", err)
		}
		ts = parsed
	}
	signer := stringParam(params, "signer", "")
	signature := stringParam(params, "signature", "")
	meta, _ := mapStringStringParam(params, "metadata")

	update, err := s.dataFeeds.SubmitUpdate(ctx, def.AccountID, feedID, roundID, price, ts, signer, signature, meta)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"update": structToMap(update),
	}, nil
}

func (s *Service) handleRandomGenerate(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.random == nil {
		return nil, fmt.Errorf("random: %w", errDependencyUnavailable)
	}
	length := intParam(params, "length", 32)
	if length <= 0 {
		length = 32
	}
	requestID := stringParam(params, "requestId", stringParam(params, "request_id", ""))

	res, err := s.random.Generate(ctx, def.AccountID, length, requestID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"length":     res.Length,
		"value":      randomsvc.EncodeResult(res),
		"created_at": res.CreatedAt,
		"request_id": res.RequestID,
		"counter":    res.Counter,
		"signature":  base64.StdEncoding.EncodeToString(res.Signature),
		"public_key": base64.StdEncoding.EncodeToString(res.PublicKey),
	}, nil
}

func (s *Service) handleDatastreamPublish(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.dataStreams == nil {
		return nil, fmt.Errorf("datastreams: %w", errDependencyUnavailable)
	}
	streamID := stringParam(params, "streamId", stringParam(params, "stream_id", ""))
	if streamID == "" {
		return nil, errors.New("datastreams.publishFrame requires streamId")
	}
	seq := int64(intParam(params, "sequence", intParam(params, "seq", 0)))
	if seq <= 0 {
		return nil, errors.New("sequence must be positive")
	}
	payload := mapFromAny(params["payload"])
	latency := intParam(params, "latencyMs", 0)
	status := domaindatastreams.FrameStatus(stringParam(params, "status", ""))
	metadata, _ := mapStringStringParam(params, "metadata")

	frame, err := s.dataStreams.CreateFrame(ctx, def.AccountID, streamID, seq, payload, latency, status, metadata)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"frame": structToMap(frame),
	}, nil
}

func (s *Service) handleDatalinkDelivery(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.dataLink == nil {
		return nil, fmt.Errorf("datalink: %w", errDependencyUnavailable)
	}
	channelID := stringParam(params, "channelId", stringParam(params, "channel_id", ""))
	if channelID == "" {
		return nil, errors.New("datalink.createDelivery requires channelId")
	}
	payload := mapFromAny(params["payload"])
	metadata, _ := mapStringStringParam(params, "metadata")

	delivery, err := s.dataLink.CreateDelivery(ctx, def.AccountID, channelID, payload, metadata)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"delivery": structToMap(delivery),
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
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListFunctionExecutions(ctx, functionID, clamped)
}

// Descriptor advertises the service placement and capabilities.
func (s *Service) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "functions",
		Domain:       "functions",
		Layer:        core.LayerPlatform,
		Capabilities: []string{"registry", "devpack", "execution"},
	}
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

func stringSliceParam(params map[string]any, key string) []string {
	if params == nil {
		return nil
	}
	value, ok := params[key]
	if !ok {
		return nil
	}
	var list []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v != "" {
			list = append(list, v)
		}
	}
	switch v := value.(type) {
	case []string:
		for _, item := range v {
			add(item)
		}
	case []any:
		for _, item := range v {
			add(fmt.Sprint(item))
		}
	case string:
		parts := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == ';'
		})
		for _, item := range parts {
			add(item)
		}
	default:
		add(fmt.Sprint(v))
	}
	if len(list) == 0 {
		return nil
	}
	return list
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

func intParam(params map[string]any, key string, fallback int) int {
	if params == nil {
		return fallback
	}
	value, ok := params[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed)
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return fallback
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
