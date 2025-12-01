package framework

import (
	"context"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// AccountChecker is an alias for core.AccountChecker.
// Services should depend on this interface rather than concrete store types.
type AccountChecker = core.AccountChecker

// WalletChecker is an alias for core.WalletChecker.
type WalletChecker = core.WalletChecker

// ServiceConfig configures a ServiceEngine instance.
type ServiceConfig struct {
	// Required fields
	Name        string
	Description string

	// Optional fields (have sensible defaults)
	Domain       string
	DependsOn    []string
	RequiresAPIs []engine.APISurface
	Capabilities []string
	Quotas       map[string]string

	// Dependencies
	Accounts    AccountChecker
	Wallets     WalletChecker
	Logger      *logger.Logger
	Environment Environment
}

// ServiceEngine provides common functionality for all services. Embed it in
// service structs to reduce boilerplate and access the Android-style
// EngineContext exposed via Context().
//
// Example usage:
//
//	type MyService struct {
//	    *framework.ServiceEngine
//	    store Store
//	}
//
//	func New(accounts AccountChecker, store Store, log *logger.Logger) *MyService {
//	    return &MyService{
//	        ServiceEngine: framework.NewServiceEngine(framework.ServiceConfig{
//	            Name:        "myservice",
//	            Description: "My service description",
//	            Accounts:    accounts,
//	            Logger:      log,
//	        }),
//	        store: store,
//	    }
//	}
type ServiceEngine struct {
	*ServiceBase // Provides: Name(), Domain(), Start(), Stop(), Ready(), state management

	accounts AccountChecker
	wallets  WalletChecker
	log      *logger.Logger
	hooks    core.ObservationHooks

	manifest   *Manifest
	descriptor core.Descriptor
	env        Environment
	context    *serviceContext
}

// NewServiceEngine creates a configured service engine.
func NewServiceEngine(cfg ServiceConfig) *ServiceEngine {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = "unknown"
	}

	domain := strings.TrimSpace(cfg.Domain)
	if domain == "" {
		domain = name
	}

	log := cfg.Logger
	if log == nil {
		log = logger.NewDefault(name)
	}

	manifest := newManifestFromConfig(name, domain, cfg)
	eng := &ServiceEngine{
		ServiceBase: NewServiceBase(name, domain),
		accounts:    cfg.Accounts,
		wallets:     cfg.Wallets,
		log:         log,
		hooks:       core.NoopObservationHooks,
		manifest:    manifest,
		descriptor:  manifest.ToDescriptor(),
	}

	eng.SetEnvironment(cfg.Environment)
	eng.context = newServiceContext(eng)

	return eng
}

// Manifest returns the service manifest.
func (e *ServiceEngine) Manifest() *Manifest {
	if e == nil || e.manifest == nil {
		return nil
	}
	return e.manifest.Clone()
}

// Descriptor returns the service descriptor for engine integration.
func (e *ServiceEngine) Descriptor() core.Descriptor {
	return cloneDescriptor(e.descriptor)
}

// Environment returns the runtime environment (store, bus, rpc, config).
func (e *ServiceEngine) Environment() Environment {
	return e.env
}

// SetEnvironment replaces the runtime environment post-construction.
func (e *ServiceEngine) SetEnvironment(env Environment) {
	if e == nil {
		return
	}
	e.env = normalizeEnvironment(env)
	if e.accounts == nil {
		e.accounts = newStoreAccountChecker(e.env.StoreProvider)
	}
	if e.context != nil {
		e.context.refresh()
	}
}

// Context exposes the Android-inspired EngineContext for this service.
func (e *ServiceEngine) Context() EngineContext {
	if e == nil {
		return nil
	}
	if e.context == nil {
		e.context = newServiceContext(e)
	}
	return e.context
}

// --- Configuration Methods ---

// WithWalletChecker sets the wallet checker for signer validation.
func (e *ServiceEngine) WithWalletChecker(w WalletChecker) {
	e.wallets = w
}

// WithObservationHooks sets observability hooks.
func (e *ServiceEngine) WithObservationHooks(h core.ObservationHooks) {
	e.hooks = core.NormalizeHooks(h)
}

// WithEnvironment sets the execution environment and returns the engine for chaining.
func (e *ServiceEngine) WithEnvironment(env Environment) *ServiceEngine {
	e.SetEnvironment(env)
	return e
}

// --- Accessor Methods ---

// Logger returns the service logger.
func (e *ServiceEngine) Logger() *logger.Logger {
	return e.log
}

// Hooks returns the observation hooks.
func (e *ServiceEngine) Hooks() core.ObservationHooks {
	return e.hooks
}

// Tracer returns the configured tracer (defaults to no-op).
func (e *ServiceEngine) Tracer() core.Tracer {
	if e.env.Tracer == nil {
		return core.NoopTracer
	}
	return e.env.Tracer
}

// Metrics returns the metrics recorder for the service environment.
func (e *ServiceEngine) Metrics() Metrics {
	if e.env.Metrics == nil {
		return NoopMetrics()
	}
	return e.env.Metrics
}

// StoreProvider exposes the configured store provider.
func (e *ServiceEngine) StoreProvider() StoreProvider {
	return e.env.StoreProvider
}

// Database returns the raw database handle, if available.
func (e *ServiceEngine) Database() any {
	if e.env.StoreProvider == nil {
		return nil
	}
	return e.env.StoreProvider.Database()
}

// Bus exposes the service bus client, or a noop implementation.
func (e *ServiceEngine) Bus() BusClient {
	return e.env.Bus
}

// PublishEvent fan-outs an event using the configured bus.
func (e *ServiceEngine) PublishEvent(ctx context.Context, event string, payload any) error {
	if err := e.EnforceEventQuota(1); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(event)
	attrs := map[string]string{
		"operation": "publish_event",
		"resource":  "bus_event",
	}
	if trimmed != "" {
		attrs["event"] = trimmed
	}
	err := e.ObserveOperation(ctx, attrs, func(obsCtx context.Context) error {
		return e.env.Bus.PublishEvent(obsCtx, event, payload)
	})
	labels := map[string]string{
		"service":   e.Name(),
		"operation": "publish_event",
		"status":    "success",
	}
	if err != nil {
		labels["status"] = "error"
	}
	if trimmed != "" {
		labels["event"] = trimmed
	}
	e.IncrementCounter("service_bus_events_total", labels)
	return err
}

// PushData fan-outs a data payload using the configured bus.
func (e *ServiceEngine) PushData(ctx context.Context, topic string, payload any) error {
	if err := e.EnforceDataQuota(1); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(topic)
	attrs := map[string]string{
		"operation": "push_data",
		"resource":  "bus_data",
	}
	if trimmed != "" {
		attrs["topic"] = trimmed
	}
	err := e.ObserveOperation(ctx, attrs, func(obsCtx context.Context) error {
		return e.env.Bus.PushData(obsCtx, topic, payload)
	})
	labels := map[string]string{
		"service":   e.Name(),
		"operation": "push_data",
		"status":    "success",
	}
	if err != nil {
		labels["status"] = "error"
	}
	if trimmed != "" {
		labels["topic"] = trimmed
	}
	e.IncrementCounter("service_bus_data_total", labels)
	return err
}

// InvokeCompute runs compute fan-out using the configured bus.
func (e *ServiceEngine) InvokeCompute(ctx context.Context, payload any) ([]ComputeResult, error) {
	if err := e.EnforceConcurrencyQuota(1); err != nil {
		return nil, err
	}
	attrs := map[string]string{
		"operation": "invoke_compute",
		"resource":  "bus_compute",
	}
	var (
		results []ComputeResult
		callErr error
	)
	err := e.ObserveOperation(ctx, attrs, func(obsCtx context.Context) error {
		results, callErr = e.env.Bus.InvokeCompute(obsCtx, payload)
		return callErr
	})
	labels := map[string]string{
		"service":   e.Name(),
		"operation": "invoke_compute",
		"status":    "success",
	}
	if err != nil {
		labels["status"] = "error"
	}
	e.IncrementCounter("service_bus_compute_requests_total", labels)
	if err == nil && len(results) > 0 {
		var success int
		for _, r := range results {
			if r.Success() {
				success++
			}
		}
		failures := len(results) - success
		if success > 0 {
			e.AddCounter("service_bus_compute_results_total", map[string]string{
				"service":   e.Name(),
				"status":    "success",
				"operation": "invoke_compute",
			}, float64(success))
		}
		if failures > 0 {
			e.AddCounter("service_bus_compute_results_total", map[string]string{
				"service":   e.Name(),
				"status":    "error",
				"operation": "invoke_compute",
			}, float64(failures))
		}
	}
	return results, err
}

// RPCClient returns the RPC client injected by the runtime.
func (e *ServiceEngine) RPCClient() any {
	return e.env.RPCClient
}

// LedgerClient returns the ledger client injected by the runtime.
func (e *ServiceEngine) LedgerClient() any {
	return e.env.LedgerClient
}

// --- Contract Interaction Methods ---

// ContractClient returns the contract client for blockchain interactions.
func (e *ServiceEngine) ContractClient() ContractClient {
	if e.env.ContractClient == nil {
		return NoopContractClient()
	}
	return e.env.ContractClient
}

// InvokeContract calls a smart contract method with state changes.
// This is the primary method for services to interact with on-chain contracts.
func (e *ServiceEngine) InvokeContract(ctx context.Context, req ContractInvokeRequest) (ContractInvokeResult, error) {
	attrs := map[string]string{
		"operation": "invoke_contract",
		"network":   req.Network,
		"method":    req.Method,
	}
	if req.ContractAddress != "" {
		attrs["contract"] = req.ContractAddress
	} else if req.ContractName != "" {
		attrs["contract"] = req.ContractName
	}

	var result ContractInvokeResult
	err := e.ObserveOperation(ctx, attrs, func(obsCtx context.Context) error {
		var invokeErr error
		result, invokeErr = e.env.ContractClient.InvokeContract(obsCtx, req)
		return invokeErr
	})

	labels := map[string]string{
		"service":  e.Name(),
		"network":  req.Network,
		"method":   req.Method,
		"status":   "success",
	}
	if err != nil {
		labels["status"] = "error"
	}
	e.IncrementCounter("service_contract_invocations_total", labels)

	return result, err
}

// QueryContract calls a smart contract method without state changes (read-only).
// Use this for queries that don't require gas.
func (e *ServiceEngine) QueryContract(ctx context.Context, req ContractInvokeRequest) (ContractInvokeResult, error) {
	attrs := map[string]string{
		"operation": "query_contract",
		"network":   req.Network,
		"method":    req.Method,
	}
	if req.ContractAddress != "" {
		attrs["contract"] = req.ContractAddress
	} else if req.ContractName != "" {
		attrs["contract"] = req.ContractName
	}

	var result ContractInvokeResult
	err := e.ObserveOperation(ctx, attrs, func(obsCtx context.Context) error {
		var queryErr error
		result, queryErr = e.env.ContractClient.InvokeContractReadOnly(obsCtx, req)
		return queryErr
	})

	labels := map[string]string{
		"service": e.Name(),
		"network": req.Network,
		"method":  req.Method,
		"status":  "success",
	}
	if err != nil {
		labels["status"] = "error"
	}
	e.IncrementCounter("service_contract_queries_total", labels)

	return result, err
}

// GetEngineContract returns the address of a registered engine contract.
// Engine contracts are core infrastructure contracts like AccountManager, GasBank, etc.
func (e *ServiceEngine) GetEngineContract(ctx context.Context, network, contractName string) (string, error) {
	return e.env.ContractClient.GetContractAddress(ctx, network, contractName)
}

// WaitForTransaction waits for a transaction to be confirmed on-chain.
func (e *ServiceEngine) WaitForTransaction(ctx context.Context, network, txHash string, timeout time.Duration) (TransactionStatus, error) {
	return e.env.ContractClient.WaitForConfirmation(ctx, network, txHash, timeout)
}

// GetTransactionStatus checks the current status of a transaction.
func (e *ServiceEngine) GetTransactionStatus(ctx context.Context, network, txHash string) (TransactionStatus, error) {
	return e.env.ContractClient.GetTransactionStatus(ctx, network, txHash)
}

// Config provides access to the runtime/service configuration.
func (e *ServiceEngine) Config() Config {
	return e.env.Config
}

// Quota returns the quota enforcer for the environment.
func (e *ServiceEngine) Quota() QuotaEnforcer {
	if e.env.Quota == nil {
		return NoopQuota()
	}
	return e.env.Quota
}

// EnforceQuota applies quota checks for the provided resource.
func (e *ServiceEngine) EnforceQuota(resource string, amount int64) error {
	return e.Quota().Enforce(resource, amount)
}

// --- Metrics Helpers ---

// IncrementCounter increments a counter metric by 1.
func (e *ServiceEngine) IncrementCounter(name string, labels map[string]string) {
	e.Metrics().Counter(name, labels, 1)
}

// AddCounter increments a counter metric by delta.
func (e *ServiceEngine) AddCounter(name string, labels map[string]string, delta float64) {
	if delta <= 0 {
		return
	}
	e.Metrics().Counter(name, labels, delta)
}

// SetGauge sets a gauge metric to value.
func (e *ServiceEngine) SetGauge(name string, labels map[string]string, value float64) {
	e.Metrics().Gauge(name, labels, value)
}

// ObserveHistogram records a numeric observation.
func (e *ServiceEngine) ObserveHistogram(name string, labels map[string]string, value float64) {
	e.Metrics().Histogram(name, labels, value)
}

// ObserveDuration records a histogram sample in seconds.
func (e *ServiceEngine) ObserveDuration(name string, labels map[string]string, d time.Duration) {
	if d < 0 {
		d = 0
	}
	e.Metrics().Histogram(name, labels, d.Seconds())
}

// --- Quota Helpers ---

const (
	QuotaResourceEvents      = "events"
	QuotaResourceDataPush    = "data_push"
	QuotaResourceConcurrency = "concurrent_requests"
)

// EnforceEventQuota enforces the events quota resource.
func (e *ServiceEngine) EnforceEventQuota(units int64) error {
	return e.EnforceQuota(QuotaResourceEvents, units)
}

// EnforceDataQuota enforces the data push quota resource.
func (e *ServiceEngine) EnforceDataQuota(units int64) error {
	return e.EnforceQuota(QuotaResourceDataPush, units)
}

// EnforceConcurrencyQuota enforces the concurrent request quota.
func (e *ServiceEngine) EnforceConcurrencyQuota(units int64) error {
	return e.EnforceQuota(QuotaResourceConcurrency, units)
}

// --- Validation Methods ---

// ValidateAccount trims and validates an account ID.
// Returns the trimmed account ID or an error if validation fails.
// This is the primary validation method - use it at the start of service methods.
func (e *ServiceEngine) ValidateAccount(ctx context.Context, accountID string) (string, error) {
	trimmed := strings.TrimSpace(accountID)
	if trimmed == "" {
		return "", core.RequiredError("account_id")
	}
	if e.accounts != nil {
		if err := e.accounts.AccountExists(ctx, trimmed); err != nil {
			return "", err
		}
	}
	return trimmed, nil
}

// ValidateAccountExists checks if an account exists without returning the trimmed ID.
func (e *ServiceEngine) ValidateAccountExists(ctx context.Context, accountID string) error {
	_, err := e.ValidateAccount(ctx, accountID)
	return err
}

// ValidateOwnership checks if a resource belongs to the requesting account.
func (e *ServiceEngine) ValidateOwnership(resourceAccountID, requestAccountID, resourceType, resourceID string) error {
	return core.EnsureOwnership(resourceAccountID, requestAccountID, resourceType, resourceID)
}

// ValidateAccountAndOwnership validates account and checks ownership in one call.
// This is a common pattern: validate the requester, then check they own the resource.
func (e *ServiceEngine) ValidateAccountAndOwnership(ctx context.Context, resourceAccountID, requestAccountID, resourceType, resourceID string) error {
	if _, err := e.ValidateAccount(ctx, requestAccountID); err != nil {
		return err
	}
	return e.ValidateOwnership(resourceAccountID, requestAccountID, resourceType, resourceID)
}

// ValidateSigners checks that all signers belong to the account's workspace.
func (e *ServiceEngine) ValidateSigners(ctx context.Context, accountID string, signers []string) error {
	if len(signers) == 0 || e.wallets == nil {
		return nil
	}
	for _, signer := range signers {
		signer = strings.TrimSpace(signer)
		if signer == "" {
			continue
		}
		if err := e.wallets.WalletOwnedBy(ctx, accountID, signer); err != nil {
			return err
		}
	}
	return nil
}

// ValidateRequired checks that a string field is not empty after trimming.
func (e *ServiceEngine) ValidateRequired(value, fieldName string) (string, error) {
	return core.TrimAndValidate(value, fieldName)
}

// --- Logging Methods ---

// LogCreated logs a resource creation event.
func (e *ServiceEngine) LogCreated(resourceType, resourceID, accountID string) {
	e.logResource("created", resourceType, resourceID, accountID)
}

// LogUpdated logs a resource update event.
func (e *ServiceEngine) LogUpdated(resourceType, resourceID, accountID string) {
	e.logResource("updated", resourceType, resourceID, accountID)
}

// LogDeleted logs a resource deletion event.
func (e *ServiceEngine) LogDeleted(resourceType, resourceID, accountID string) {
	e.logResource("deleted", resourceType, resourceID, accountID)
}

// LogAction logs a custom action with resource context.
func (e *ServiceEngine) LogAction(action, resourceType, resourceID, accountID string) {
	e.logResource(action, resourceType, resourceID, accountID)
}

// --- Observation Methods ---

// StartObservation begins an observed operation, returning the derived context
// and a completion callback. Always call the returned function with the final
// error (or nil) when the operation completes.
func (e *ServiceEngine) StartObservation(ctx context.Context, attrs map[string]string) (context.Context, func(error)) {
	if e == nil {
		finish := core.StartObservation(ctx, core.NoopObservationHooks, attrs)
		return ctx, func(err error) { finish(err) }
	}
	meta := e.decorateObservationMeta(attrs)
	spanName := e.observationSpanName(meta)
	spanCtx, finishSpan := e.Tracer().StartSpan(ctx, spanName, meta)
	finishHooks := core.StartObservation(spanCtx, e.hooks, meta)
	return spanCtx, func(err error) {
		finishHooks(err)
		finishSpan(err)
	}
}

// ObserveOperation wraps an operation with observation hooks and tracing.
func (e *ServiceEngine) ObserveOperation(ctx context.Context, attrs map[string]string, op func(context.Context) error) error {
	obsCtx, finish := e.StartObservation(ctx, attrs)
	err := op(obsCtx)
	finish(err)
	return err
}

func (e *ServiceEngine) observationSpanName(attrs map[string]string) string {
	name := strings.TrimSpace(e.Name())
	if name == "" {
		name = "service"
	}
	if len(attrs) == 0 {
		return name
	}
	if op := strings.TrimSpace(attrs["operation"]); op != "" {
		return name + "." + op
	}
	if op := strings.TrimSpace(attrs["op"]); op != "" {
		return name + "." + op
	}
	if action := strings.TrimSpace(attrs["action"]); action != "" {
		return name + "." + action
	}
	if resource := strings.TrimSpace(attrs["resource"]); resource != "" {
		return name + "." + resource
	}
	return name
}

func (e *ServiceEngine) decorateObservationMeta(attrs map[string]string) map[string]string {
	meta := make(map[string]string, len(attrs)+2)
	for k, v := range attrs {
		if key := strings.TrimSpace(k); key != "" {
			meta[key] = v
		}
	}
	if e != nil {
		if _, ok := meta["service"]; !ok {
			meta["service"] = e.Name()
		}
		if _, ok := meta["domain"]; !ok {
			meta["domain"] = e.Domain()
		}
	}
	return meta
}

// --- High-Order Operation Methods ---
// These methods reduce boilerplate in service implementations by combining
// common patterns: validation, observation, logging, and metrics.

// OperationContext holds context for an observed operation.
type OperationContext struct {
	Ctx    context.Context
	Finish func(error)
	Attrs  map[string]string
}

// BeginOperation starts an observed operation with account validation.
// This is the primary entry point for service methods that require account validation.
// Returns an OperationContext that must be finished with ctx.Finish(err).
//
// Example usage:
//
//	func (s *Service) CreateResource(ctx context.Context, accountID string, ...) (Resource, error) {
//	    opCtx, err := s.BeginOperation(ctx, accountID, "resource", "create")
//	    if err != nil {
//	        return Resource{}, err
//	    }
//	    defer func() { opCtx.Finish(err) }()
//	    // ... business logic ...
//	}
func (e *ServiceEngine) BeginOperation(ctx context.Context, accountID, resource, operation string) (*OperationContext, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, core.RequiredError("account_id")
	}

	attrs := map[string]string{
		"account_id": accountID,
		"resource":   resource,
		"operation":  operation,
	}

	if err := e.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}

	obsCtx, finish := e.StartObservation(ctx, attrs)
	return &OperationContext{
		Ctx:    obsCtx,
		Finish: finish,
		Attrs:  attrs,
	}, nil
}

// BeginSimpleOperation starts an observed operation without account validation.
// Use this for operations that don't require account validation (e.g., internal operations).
func (e *ServiceEngine) BeginSimpleOperation(ctx context.Context, resource, operation string) *OperationContext {
	attrs := map[string]string{
		"resource":  resource,
		"operation": operation,
	}
	obsCtx, finish := e.StartObservation(ctx, attrs)
	return &OperationContext{
		Ctx:    obsCtx,
		Finish: finish,
		Attrs:  attrs,
	}
}

// RunOperation executes an operation with full observation lifecycle.
// This is a convenience wrapper for simple operations.
func (e *ServiceEngine) RunOperation(ctx context.Context, accountID, resource, operation string, op func(context.Context) error) error {
	opCtx, err := e.BeginOperation(ctx, accountID, resource, operation)
	if err != nil {
		return err
	}
	err = op(opCtx.Ctx)
	opCtx.Finish(err)
	return err
}

// RecordCreate logs creation and increments counter for a resource.
// Call this after successfully creating a resource.
func (e *ServiceEngine) RecordCreate(resourceType, resourceID, accountID string) {
	e.LogCreated(resourceType, resourceID, accountID)
	e.IncrementCounter(e.Name()+"_"+resourceType+"_created_total", map[string]string{
		"account_id": accountID,
	})
}

// RecordUpdate logs update and increments counter for a resource.
func (e *ServiceEngine) RecordUpdate(resourceType, resourceID, accountID string) {
	e.LogUpdated(resourceType, resourceID, accountID)
	e.IncrementCounter(e.Name()+"_"+resourceType+"_updated_total", map[string]string{
		"account_id": accountID,
	})
}

// RecordDelete logs deletion and increments counter for a resource.
func (e *ServiceEngine) RecordDelete(resourceType, resourceID, accountID string) {
	e.LogDeleted(resourceType, resourceID, accountID)
	e.IncrementCounter(e.Name()+"_"+resourceType+"_deleted_total", map[string]string{
		"account_id": accountID,
	})
}

// RecordAction logs a custom action and increments counter.
func (e *ServiceEngine) RecordAction(action, resourceType, resourceID, accountID string) {
	e.LogAction(action, resourceType, resourceID, accountID)
	e.IncrementCounter(e.Name()+"_"+resourceType+"_"+action+"_total", map[string]string{
		"account_id": accountID,
	})
}

// --- Utility Methods ---

// ClampLimit clamps a limit value to safe bounds.
func (e *ServiceEngine) ClampLimit(limit int) int {
	return core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
}

func (e *ServiceEngine) logResource(action, resourceType, resourceID, accountID string) {
	if e.log == nil {
		return
	}

	action = strings.TrimSpace(action)
	resourceType = strings.TrimSpace(resourceType)
	accountID = strings.TrimSpace(accountID)

	parts := make([]string, 0, 2)
	if resourceType != "" {
		parts = append(parts, resourceType)
	}
	if action != "" {
		parts = append(parts, action)
	}
	message := strings.TrimSpace(strings.Join(parts, " "))
	if message == "" {
		message = "resource event"
	}

	var fields logrus.Fields
	if resourceType != "" && resourceID != "" {
		fields = logrus.Fields{resourceType + "_id": strings.TrimSpace(resourceID)}
	}
	if accountID != "" {
		if fields == nil {
			fields = logrus.Fields{}
		}
		fields["account_id"] = accountID
	}

	if len(fields) == 0 {
		e.log.Info(message)
		return
	}
	e.log.WithFields(fields).Info(message)
}

func newManifestFromConfig(name, domain string, cfg ServiceConfig) *Manifest {
	manifest := &Manifest{
		Name:         name,
		Domain:       domain,
		Description:  strings.TrimSpace(cfg.Description),
		Layer:        "service",
		DependsOn:    []string{"store", "svc-accounts"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore},
		Capabilities: []string{name},
	}

	if len(cfg.DependsOn) > 0 {
		manifest.DependsOn = cloneStringSlice(cfg.DependsOn)
	}
	if len(cfg.RequiresAPIs) > 0 {
		manifest.RequiresAPIs = cloneAPISlice(cfg.RequiresAPIs)
	}
	if len(cfg.Capabilities) > 0 {
		manifest.Capabilities = cloneStringSlice(cfg.Capabilities)
	}

	if len(cfg.Quotas) > 0 {
		manifest.Quotas = make(map[string]string, len(cfg.Quotas))
		for k, v := range cfg.Quotas {
			key := strings.TrimSpace(k)
			val := strings.TrimSpace(v)
			if key == "" || val == "" {
				continue
			}
			manifest.Quotas[key] = val
		}
	}

	manifest.Normalize()
	return manifest
}

func cloneDescriptor(d core.Descriptor) core.Descriptor {
	clone := core.Descriptor{
		Name:   d.Name,
		Domain: d.Domain,
		Layer:  d.Layer,
	}

	if len(d.Capabilities) > 0 {
		clone.Capabilities = make([]string, len(d.Capabilities))
		copy(clone.Capabilities, d.Capabilities)
	}
	if len(d.RequiresAPIs) > 0 {
		clone.RequiresAPIs = make([]string, len(d.RequiresAPIs))
		copy(clone.RequiresAPIs, d.RequiresAPIs)
	}
	if len(d.DependsOn) > 0 {
		clone.DependsOn = make([]string, len(d.DependsOn))
		copy(clone.DependsOn, d.DependsOn)
	}

	return clone
}

func cloneStringSlice(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	out := make([]string, len(src))
	copy(out, src)
	return out
}

func cloneAPISlice(src []engine.APISurface) []engine.APISurface {
	if len(src) == 0 {
		return nil
	}
	out := make([]engine.APISurface, len(src))
	copy(out, src)
	return out
}
