package framework

import (
	"context"
	"strings"

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
	Accounts AccountChecker
	Wallets  WalletChecker
	Logger   *logger.Logger
}

// ServiceEngine provides common functionality for all services.
// Embed this in service structs to reduce boilerplate.
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
	ServiceBase // Provides: Name(), Domain(), Start(), Stop(), Ready(), state management

	accounts AccountChecker
	wallets  WalletChecker
	log      *logger.Logger
	hooks    core.ObservationHooks

	manifestBuilder *ManifestBuilder
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

	if cfg.Logger == nil {
		cfg.Logger = logger.NewDefault(name)
	}

	// Build manifest
	builder := NewManifestBuilder(name, cfg.Description).Domain(domain)

	if len(cfg.DependsOn) > 0 {
		builder.DependsOn(cfg.DependsOn...)
	}
	if len(cfg.RequiresAPIs) > 0 {
		builder.RequiresAPIs(cfg.RequiresAPIs...)
	}
	if len(cfg.Capabilities) > 0 {
		builder.Capabilities(cfg.Capabilities...)
	}
	for k, v := range cfg.Quotas {
		builder.Quota(k, v)
	}

	eng := &ServiceEngine{
		accounts:        cfg.Accounts,
		wallets:         cfg.Wallets,
		log:             cfg.Logger,
		hooks:           core.NoopObservationHooks,
		manifestBuilder: builder,
	}

	// Set name/domain on embedded ServiceBase
	eng.SetName(name)
	eng.SetDomain(domain)

	return eng
}

// Manifest returns the service manifest.
func (e *ServiceEngine) Manifest() *Manifest {
	return e.manifestBuilder.Build()
}

// Descriptor returns the service descriptor for engine integration.
func (e *ServiceEngine) Descriptor() core.Descriptor {
	return e.Manifest().ToDescriptor()
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

// --- Accessor Methods ---

// Logger returns the service logger.
func (e *ServiceEngine) Logger() *logger.Logger {
	return e.log
}

// Hooks returns the observation hooks.
func (e *ServiceEngine) Hooks() core.ObservationHooks {
	return e.hooks
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
	e.log.WithField(resourceType+"_id", resourceID).
		WithField("account_id", accountID).
		Info(resourceType + " created")
}

// LogUpdated logs a resource update event.
func (e *ServiceEngine) LogUpdated(resourceType, resourceID, accountID string) {
	e.log.WithField(resourceType+"_id", resourceID).
		WithField("account_id", accountID).
		Info(resourceType + " updated")
}

// LogDeleted logs a resource deletion event.
func (e *ServiceEngine) LogDeleted(resourceType, resourceID, accountID string) {
	e.log.WithField(resourceType+"_id", resourceID).
		WithField("account_id", accountID).
		Info(resourceType + " deleted")
}

// LogAction logs a custom action with resource context.
func (e *ServiceEngine) LogAction(action, resourceType, resourceID, accountID string) {
	e.log.WithField(resourceType+"_id", resourceID).
		WithField("account_id", accountID).
		Info(resourceType + " " + action)
}

// LogWithFields returns a logger entry with the given fields.
func (e *ServiceEngine) LogWithFields(fields map[string]interface{}) *logrus.Entry {
	entry := e.log.WithFields(nil)
	for k, v := range fields {
		entry = entry.WithField(k, v)
	}
	return entry
}

// --- Observation Methods ---

// StartObservation begins an observed operation and returns a finish function.
// Call the returned function with the error (or nil) when the operation completes.
func (e *ServiceEngine) StartObservation(ctx context.Context, attrs map[string]string) func(error) {
	return core.StartObservation(ctx, e.hooks, attrs)
}

// ObserveOperation wraps an operation with observation hooks.
func (e *ServiceEngine) ObserveOperation(ctx context.Context, attrs map[string]string, op func() error) error {
	finish := e.StartObservation(ctx, attrs)
	err := op()
	finish(err)
	return err
}

// --- Utility Methods ---

// ClampLimit clamps a limit value to safe bounds.
func (e *ServiceEngine) ClampLimit(limit int) int {
	return core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
}

// NormalizeMetadata normalizes a metadata map.
func (e *ServiceEngine) NormalizeMetadata(m map[string]string) map[string]string {
	return core.NormalizeMetadata(m)
}

// NormalizeTags normalizes and deduplicates a tag slice.
func (e *ServiceEngine) NormalizeTags(tags []string) []string {
	return core.NormalizeTags(tags)
}
