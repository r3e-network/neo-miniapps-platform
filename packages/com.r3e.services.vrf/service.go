package vrf

import (
	"context"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Dispatcher notifies downstream VRF executors when a request is created.
type Dispatcher interface {
	Dispatch(ctx context.Context, req Request, key Key) error
}

// DispatcherFunc allows a function to satisfy Dispatcher.
type DispatcherFunc func(ctx context.Context, req Request, key Key) error

// Dispatch calls the underlying function.
func (f DispatcherFunc) Dispatch(ctx context.Context, req Request, key Key) error {
	return f(ctx, req, key)
}

// Service exposes VRF key + request management.
type Service struct {
	framework.ServiceBase
	accounts   AccountChecker
	wallets    WalletChecker
	store      Store
	dispatcher Dispatcher
	dispatch   core.DispatchOptions
	log        *logger.Logger
}

// New constructs a VRF service.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("vrf")
	}
	svc := &Service{
		accounts: accounts,
		store:    store,
		dispatcher: DispatcherFunc(func(context.Context, Request, Key) error {
			return nil
		}),
		dispatch: core.NewDispatchOptions(),
		log:      log,
	}
	svc.SetName(svc.Name())
	return svc
}

// WithDispatcher overrides the dispatcher implementation.
func (s *Service) WithDispatcher(d Dispatcher) {
	if d != nil {
		s.dispatcher = d
	}
}

// WithWalletChecker injects a wallet checker for ownership validation.
func (s *Service) WithWalletChecker(w WalletChecker) {
	s.wallets = w
}

// WithDispatcherRetry configures retry behavior for dispatcher calls.
func (s *Service) WithDispatcherRetry(policy core.RetryPolicy) {
	s.dispatch.SetRetry(policy)
}

// WithDispatcherHooks configures optional observability hooks.
func (s *Service) WithDispatcherHooks(h core.DispatchHooks) {
	s.dispatch.SetHooks(h)
}

// WithTracer configures a tracer for dispatcher operations.
func (s *Service) WithTracer(t core.Tracer) {
	s.dispatch.SetTracer(t)
}

// Name returns the stable service identifier.
func (s *Service) Name() string { return "vrf" }

// Domain reports the service domain.
func (s *Service) Domain() string { return "vrf" }

// Manifest describes the service contract for the engine OS.
func (s *Service) Manifest() *framework.Manifest {
	return &framework.Manifest{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Description:  "VRF key and request management",
		Layer:        "service",
		DependsOn:    []string{"store", "svc-accounts"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceEvent},
		Capabilities: []string{"vrf"},
	}
}

// Descriptor advertises the service for system discovery.
func (s *Service) Descriptor() core.Descriptor { return s.Manifest().ToDescriptor() }

// Start/Stop/Ready are inherited from framework.ServiceBase.

// CreateKey registers a VRF key for an account.
func (s *Service) CreateKey(ctx context.Context, key Key) (Key, error) {
	if err := s.accounts.AccountExists(ctx, key.AccountID); err != nil {
		return Key{}, err
	}
	if err := s.normalizeKey(&key); err != nil {
		return Key{}, err
	}
	if err := s.ensureWalletOwned(ctx, key.AccountID, key.WalletAddress); err != nil {
		return Key{}, err
	}
	created, err := s.store.CreateKey(ctx, key)
	if err != nil {
		return Key{}, err
	}
	s.log.WithField("key_id", created.ID).WithField("account_id", created.AccountID).Info("vrf key created")
	return created, nil
}

// UpdateKey updates mutable fields on a VRF key.
func (s *Service) UpdateKey(ctx context.Context, accountID string, key Key) (Key, error) {
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return Key{}, err
	}
	stored, err := s.store.GetKey(ctx, key.ID)
	if err != nil {
		return Key{}, err
	}
	if err := core.EnsureOwnership(stored.AccountID, accountID, "key", key.ID); err != nil {
		return Key{}, err
	}
	key.AccountID = stored.AccountID
	if err := s.normalizeKey(&key); err != nil {
		return Key{}, err
	}
	if err := s.ensureWalletOwned(ctx, accountID, key.WalletAddress); err != nil {
		return Key{}, err
	}
	updated, err := s.store.UpdateKey(ctx, key)
	if err != nil {
		return Key{}, err
	}
	s.log.WithField("key_id", key.ID).WithField("account_id", key.AccountID).Info("vrf key updated")
	return updated, nil
}

// GetKey fetches a key ensuring ownership.
func (s *Service) GetKey(ctx context.Context, accountID, keyID string) (Key, error) {
	key, err := s.store.GetKey(ctx, keyID)
	if err != nil {
		return Key{}, err
	}
	if err := core.EnsureOwnership(key.AccountID, accountID, "key", keyID); err != nil {
		return Key{}, err
	}
	return key, nil
}

// ListKeys lists keys for an account.
func (s *Service) ListKeys(ctx context.Context, accountID string) ([]Key, error) {
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListKeys(ctx, accountID)
}

// CreateRequest enqueues a randomness request.
func (s *Service) CreateRequest(ctx context.Context, accountID, keyID, consumer, seed string, metadata map[string]string) (Request, error) {
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return Request{}, err
	}
	key, err := s.store.GetKey(ctx, keyID)
	if err != nil {
		return Request{}, err
	}
	if err := core.EnsureOwnership(key.AccountID, accountID, "key", keyID); err != nil {
		return Request{}, err
	}
	consumer = strings.TrimSpace(consumer)
	seed = strings.TrimSpace(seed)
	if consumer == "" {
		return Request{}, core.RequiredError("consumer")
	}
	if seed == "" {
		return Request{}, core.RequiredError("seed")
	}
	req := Request{
		AccountID: accountID,
		KeyID:     keyID,
		Consumer:  consumer,
		Seed:      seed,
		Status:    RequestStatusPending,
		Metadata:  core.NormalizeMetadata(metadata),
	}
	created, err := s.store.CreateRequest(ctx, req)
	if err != nil {
		return Request{}, err
	}
	attrs := map[string]string{"request_id": created.ID, "key_id": key.ID}
	if err := s.dispatch.Run(ctx, "vrf.dispatch", attrs, func(spanCtx context.Context) error {
		if err := s.dispatcher.Dispatch(spanCtx, created, key); err != nil {
			s.log.WithError(err).WithField("request_id", created.ID).Warn("vrf dispatcher error")
			return err
		}
		return nil
	}); err != nil {
		return created, err
	}
	return created, nil
}

// GetRequest fetches a request ensuring ownership.
func (s *Service) GetRequest(ctx context.Context, accountID, requestID string) (Request, error) {
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return Request{}, err
	}
	if err := core.EnsureOwnership(req.AccountID, accountID, "request", requestID); err != nil {
		return Request{}, err
	}
	return req, nil
}

// ListRequests lists requests for an account.
func (s *Service) ListRequests(ctx context.Context, accountID string, limit int) ([]Request, error) {
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListRequests(ctx, accountID, clamped)
}

func (s *Service) normalizeKey(key *Key) error {
	key.PublicKey = strings.TrimSpace(key.PublicKey)
	key.Label = strings.TrimSpace(key.Label)
	key.WalletAddress = strings.ToLower(strings.TrimSpace(key.WalletAddress))
	key.Attestation = strings.TrimSpace(key.Attestation)
	key.Metadata = core.NormalizeMetadata(key.Metadata)
	if key.PublicKey == "" {
		return core.RequiredError("public_key")
	}
	if key.WalletAddress == "" {
		return core.RequiredError("wallet_address")
	}
	status := KeyStatus(strings.ToLower(strings.TrimSpace(string(key.Status))))
	if status == "" {
		status = KeyStatusInactive
	}
	switch status {
	case KeyStatusInactive, KeyStatusPendingApproval, KeyStatusActive, KeyStatusRevoked:
		key.Status = status
	default:
		return fmt.Errorf("invalid status %s", status)
	}
	return nil
}

func (s *Service) ensureWalletOwned(ctx context.Context, accountID, wallet string) error {
	if strings.TrimSpace(wallet) == "" {
		return core.RequiredError("wallet_address")
	}
	if s.wallets == nil {
		// No wallet checker configured, skip validation
		return nil
	}
	return s.wallets.WalletOwnedBy(ctx, accountID, wallet)
}
