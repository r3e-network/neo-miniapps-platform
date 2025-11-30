// Package contracts provides the Service Layer contract management service.
// It orchestrates smart contract deployments, invocations, and lifecycle
// management across multiple blockchain networks.
//
// The service supports three contract categories:
//   - Engine contracts: Core Service Layer infrastructure (AccountManager, GasBank, etc.)
//   - Service contracts: Per-service contracts (Oracle, VRF, DataFeeds)
//   - User contracts: Custom contracts deployed by users through SDK
//
// This design follows the Android OS pattern where system provides core contracts
// and apps (services) can register their own contracts that integrate with common
// account and gas management systems.
package contracts

import (
	"context"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Deployer handles contract deployment to blockchain networks.
type Deployer interface {
	Deploy(ctx context.Context, deployment Deployment) (Deployment, error)
}

// DeployerFunc adapts a function to the Deployer interface.
type DeployerFunc func(ctx context.Context, deployment Deployment) (Deployment, error)

// Deploy calls f(ctx, deployment).
func (f DeployerFunc) Deploy(ctx context.Context, deployment Deployment) (Deployment, error) {
	return f(ctx, deployment)
}

// Invoker handles contract method invocations.
type Invoker interface {
	Invoke(ctx context.Context, invocation Invocation) (Invocation, error)
}

// InvokerFunc adapts a function to the Invoker interface.
type InvokerFunc func(ctx context.Context, invocation Invocation) (Invocation, error)

// Invoke calls f(ctx, invocation).
func (f InvokerFunc) Invoke(ctx context.Context, invocation Invocation) (Invocation, error) {
	return f(ctx, invocation)
}

// Service manages smart contracts, deployments, and invocations.
type Service struct {
	framework.ServiceBase
	base     *core.Base
	store    Store
	deployer Deployer
	invoker  Invoker
	dispatch core.DispatchOptions
	log      *logger.Logger
	hooks    core.ObservationHooks
}

// Name returns the stable service identifier.
func (s *Service) Name() string { return "contracts" }

// Domain reports the service domain.
func (s *Service) Domain() string { return "contracts" }

// Manifest describes the service contract for the engine OS.
func (s *Service) Manifest() *framework.Manifest {
	return &framework.Manifest{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Description:  "Smart contract deployment, invocation, and lifecycle management",
		Layer:        "service",
		DependsOn:    []string{"store", "svc-accounts", "svc-gasbank"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceContracts, engine.APISurfaceGasBank},
		Capabilities: []string{"contracts", "deploy", "invoke"},
	}
}

// Descriptor advertises the service for system discovery.
func (s *Service) Descriptor() core.Descriptor { return s.Manifest().ToDescriptor() }

// Start/Stop/Ready are inherited from framework.ServiceBase.

// New constructs a contracts service.
func New(accounts core.AccountChecker, store Store, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("contracts")
	}
	svc := &Service{
		base:     core.NewBase(accounts),
		store:    store,
		log:      log,
		hooks:    core.NoopObservationHooks,
		dispatch: core.NewDispatchOptions(),
		deployer: DeployerFunc(func(_ context.Context, d Deployment) (Deployment, error) {
			return d, nil // No-op default
		}),
		invoker: InvokerFunc(func(_ context.Context, inv Invocation) (Invocation, error) {
			return inv, nil // No-op default
		}),
	}
	svc.SetName(svc.Name())
	return svc
}

// WithDeployer injects a deployer for contract deployments.
func (s *Service) WithDeployer(d Deployer) {
	if d != nil {
		s.deployer = d
	}
}

// WithInvoker injects an invoker for contract calls.
func (s *Service) WithInvoker(inv Invoker) {
	if inv != nil {
		s.invoker = inv
	}
}

// WithDispatcherRetry configures retry behavior.
func (s *Service) WithDispatcherRetry(policy core.RetryPolicy) {
	s.dispatch.SetRetry(policy)
}

// WithDispatcherHooks configures observability hooks.
func (s *Service) WithDispatcherHooks(h core.DispatchHooks) {
	s.dispatch.SetHooks(h)
}

// WithTracer configures a tracer for operations.
func (s *Service) WithTracer(t core.Tracer) {
	s.dispatch.SetTracer(t)
}

// WithObservationHooks configures callbacks for observability.
func (s *Service) WithObservationHooks(h core.ObservationHooks) {
	s.hooks = core.NormalizeHooks(h)
}

// CreateContract registers a new contract.
func (s *Service) CreateContract(ctx context.Context, c Contract) (Contract, error) {
	if err := s.base.EnsureAccount(ctx, c.AccountID); err != nil {
		return Contract{}, err
	}
	if err := s.normalizeContract(&c); err != nil {
		return Contract{}, err
	}
	created, err := s.store.CreateContract(ctx, c)
	if err != nil {
		return Contract{}, err
	}
	s.log.WithField("contract_id", created.ID).WithField("account_id", created.AccountID).Info("contract registered")
	return created, nil
}

// UpdateContract updates contract metadata.
func (s *Service) UpdateContract(ctx context.Context, c Contract) (Contract, error) {
	stored, err := s.store.GetContract(ctx, c.ID)
	if err != nil {
		return Contract{}, err
	}
	if err := core.EnsureOwnership(stored.AccountID, c.AccountID, "contract", c.ID); err != nil {
		return Contract{}, err
	}
	c.AccountID = stored.AccountID
	if err := s.normalizeContract(&c); err != nil {
		return Contract{}, err
	}
	updated, err := s.store.UpdateContract(ctx, c)
	if err != nil {
		return Contract{}, err
	}
	s.log.WithField("contract_id", c.ID).WithField("account_id", c.AccountID).Info("contract updated")
	return updated, nil
}

// GetContract fetches a contract ensuring ownership.
func (s *Service) GetContract(ctx context.Context, accountID, contractID string) (Contract, error) {
	c, err := s.store.GetContract(ctx, contractID)
	if err != nil {
		return Contract{}, err
	}
	// Engine contracts are accessible to all accounts
	if c.Type == ContractTypeEngine {
		return c, nil
	}
	if err := core.EnsureOwnership(c.AccountID, accountID, "contract", contractID); err != nil {
		return Contract{}, err
	}
	return c, nil
}

// GetContractByAddress fetches a contract by network and address.
func (s *Service) GetContractByAddress(ctx context.Context, network Network, address string) (Contract, error) {
	return s.store.GetContractByAddress(ctx, network, address)
}

// ListContracts lists account contracts.
func (s *Service) ListContracts(ctx context.Context, accountID string) ([]Contract, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListContracts(ctx, accountID)
}

// ListEngineContracts lists all engine-level contracts.
func (s *Service) ListEngineContracts(ctx context.Context) ([]Contract, error) {
	return s.store.ListContractsByService(ctx, "")
}

// ListContractsByService lists contracts for a specific service.
func (s *Service) ListContractsByService(ctx context.Context, serviceID string) ([]Contract, error) {
	return s.store.ListContractsByService(ctx, serviceID)
}

// ListContractsByNetwork lists contracts deployed on a network.
func (s *Service) ListContractsByNetwork(ctx context.Context, network Network) ([]Contract, error) {
	return s.store.ListContractsByNetwork(ctx, network)
}

// Deploy initiates a contract deployment.
func (s *Service) Deploy(ctx context.Context, accountID, contractID string, constructorArgs map[string]any, gasLimit int64, metadata map[string]string) (Deployment, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return Deployment{}, err
	}
	c, err := s.GetContract(ctx, accountID, contractID)
	if err != nil {
		return Deployment{}, err
	}
	if c.Bytecode == "" {
		return Deployment{}, fmt.Errorf("contract %s has no bytecode", contractID)
	}

	deployment := Deployment{
		AccountID:       accountID,
		ContractID:      contractID,
		Network:         c.Network,
		Bytecode:        c.Bytecode,
		ConstructorArgs: constructorArgs,
		GasLimit:        gasLimit,
		Status:          DeploymentStatusPending,
		Metadata:        core.NormalizeMetadata(metadata),
	}

	created, err := s.store.CreateDeployment(ctx, deployment)
	if err != nil {
		return Deployment{}, err
	}

	attrs := map[string]string{"deployment_id": created.ID, "contract_id": contractID}
	if err := s.dispatch.Run(ctx, "contracts.deploy", attrs, func(spanCtx context.Context) error {
		result, err := s.deployer.Deploy(spanCtx, created)
		if err != nil {
			s.log.WithError(err).WithField("deployment_id", created.ID).Warn("contract deployment failed")
			return err
		}
		// Update with result
		if result.Address != "" {
			created = result
		}
		return nil
	}); err != nil {
		return created, err
	}

	s.log.WithField("deployment_id", created.ID).WithField("contract_id", contractID).Info("contract deployment initiated")
	return created, nil
}

// Invoke calls a contract method.
func (s *Service) Invoke(ctx context.Context, accountID, contractID, methodName string, args map[string]any, gasLimit int64, value string, metadata map[string]string) (Invocation, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return Invocation{}, err
	}
	c, err := s.GetContract(ctx, accountID, contractID)
	if err != nil {
		return Invocation{}, err
	}
	if c.Status != ContractStatusActive {
		return Invocation{}, fmt.Errorf("contract %s is not active (status: %s)", contractID, c.Status)
	}
	if methodName = strings.TrimSpace(methodName); methodName == "" {
		return Invocation{}, core.RequiredError("method_name")
	}

	invocation := Invocation{
		AccountID:  accountID,
		ContractID: contractID,
		MethodName: methodName,
		Args:       args,
		GasLimit:   gasLimit,
		Value:      strings.TrimSpace(value),
		Status:     InvocationStatusPending,
		Metadata:   core.NormalizeMetadata(metadata),
	}

	created, err := s.store.CreateInvocation(ctx, invocation)
	if err != nil {
		return Invocation{}, err
	}

	attrs := map[string]string{"invocation_id": created.ID, "contract_id": contractID, "method": methodName}
	if err := s.dispatch.Run(ctx, "contracts.invoke", attrs, func(spanCtx context.Context) error {
		result, err := s.invoker.Invoke(spanCtx, created)
		if err != nil {
			s.log.WithError(err).WithField("invocation_id", created.ID).Warn("contract invocation failed")
			return err
		}
		created = result
		return nil
	}); err != nil {
		return created, err
	}

	s.log.WithField("invocation_id", created.ID).WithField("contract_id", contractID).WithField("method", methodName).Info("contract invocation submitted")
	return created, nil
}

// GetInvocation fetches an invocation.
func (s *Service) GetInvocation(ctx context.Context, accountID, invocationID string) (Invocation, error) {
	inv, err := s.store.GetInvocation(ctx, invocationID)
	if err != nil {
		return Invocation{}, err
	}
	if err := core.EnsureOwnership(inv.AccountID, accountID, "invocation", invocationID); err != nil {
		return Invocation{}, err
	}
	return inv, nil
}

// ListInvocations lists invocations for a contract.
func (s *Service) ListInvocations(ctx context.Context, accountID, contractID string, limit int) ([]Invocation, error) {
	if _, err := s.GetContract(ctx, accountID, contractID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListInvocations(ctx, contractID, clamped)
}

// ListAccountInvocations lists all invocations for an account.
func (s *Service) ListAccountInvocations(ctx context.Context, accountID string, limit int) ([]Invocation, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListAccountInvocations(ctx, accountID, clamped)
}

// GetDeployment fetches a deployment.
func (s *Service) GetDeployment(ctx context.Context, accountID, deploymentID string) (Deployment, error) {
	d, err := s.store.GetDeployment(ctx, deploymentID)
	if err != nil {
		return Deployment{}, err
	}
	if err := core.EnsureOwnership(d.AccountID, accountID, "deployment", deploymentID); err != nil {
		return Deployment{}, err
	}
	return d, nil
}

// ListDeployments lists deployments for a contract.
func (s *Service) ListDeployments(ctx context.Context, accountID, contractID string, limit int) ([]Deployment, error) {
	if _, err := s.GetContract(ctx, accountID, contractID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListDeployments(ctx, contractID, clamped)
}

// GetNetworkConfig returns configuration for a network.
func (s *Service) GetNetworkConfig(ctx context.Context, network Network) (NetworkConfig, error) {
	return s.store.GetNetworkConfig(ctx, network)
}

// ListNetworkConfigs lists all configured networks.
func (s *Service) ListNetworkConfigs(ctx context.Context) ([]NetworkConfig, error) {
	return s.store.ListNetworkConfigs(ctx)
}

func (s *Service) normalizeContract(c *Contract) error {
	c.Name = strings.TrimSpace(c.Name)
	c.Symbol = strings.ToUpper(strings.TrimSpace(c.Symbol))
	c.Description = strings.TrimSpace(c.Description)
	c.Address = strings.TrimSpace(c.Address)
	c.Version = strings.TrimSpace(c.Version)
	c.Metadata = core.NormalizeMetadata(c.Metadata)
	c.Tags = core.NormalizeTags(c.Tags)

	if c.Name == "" {
		return core.RequiredError("name")
	}
	if c.Network == "" {
		return core.RequiredError("network")
	}
	if c.Version == "" {
		c.Version = "1.0.0"
	}

	contractType := ContractType(strings.ToLower(strings.TrimSpace(string(c.Type))))
	if contractType == "" {
		contractType = ContractTypeUser
	}
	switch contractType {
	case ContractTypeEngine, ContractTypeService, ContractTypeUser:
		c.Type = contractType
	default:
		return fmt.Errorf("invalid contract type %s", contractType)
	}

	status := ContractStatus(strings.ToLower(strings.TrimSpace(string(c.Status))))
	if status == "" {
		status = ContractStatusDraft
	}
	switch status {
	case ContractStatusDraft, ContractStatusDeploying,
		ContractStatusActive, ContractStatusPaused,
		ContractStatusUpgrading, ContractStatusDeprecated,
		ContractStatusRevoked:
		c.Status = status
	default:
		return fmt.Errorf("invalid status %s", status)
	}

	return nil
}
