package vrf

import (
	"context"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/applications/storage"
	domainvrf "github.com/R3E-Network/service_layer/domain/vrf"
	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Dispatcher notifies downstream VRF executors when a request is created.
type Dispatcher interface {
	Dispatch(ctx context.Context, req domainvrf.Request, key domainvrf.Key) error
}

// DispatcherFunc allows a function to satisfy Dispatcher.
type DispatcherFunc func(ctx context.Context, req domainvrf.Request, key domainvrf.Key) error

// Dispatch calls the underlying function.
func (f DispatcherFunc) Dispatch(ctx context.Context, req domainvrf.Request, key domainvrf.Key) error {
	return f(ctx, req, key)
}

// Service exposes VRF key + request management.
type Service struct {
	framework.ServiceBase
	base       *core.Base
	store      storage.VRFStore
	dispatcher Dispatcher
	dispatch   core.DispatchOptions
	log        *logger.Logger
}

// New constructs a VRF service.
func New(accounts storage.AccountStore, store storage.VRFStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("vrf")
	}
	svc := &Service{
		base:  core.NewBase(accounts),
		store: store,
		dispatcher: DispatcherFunc(func(context.Context, domainvrf.Request, domainvrf.Key) error {
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

// WithWorkspaceWallets injects a wallet store for ownership validation.
func (s *Service) WithWorkspaceWallets(store storage.WorkspaceWalletStore) {
	s.base.SetWallets(store)
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

// Start is a no-op lifecycle hook to satisfy the system.Service contract.
func (s *Service) Start(ctx context.Context) error { _ = ctx; s.MarkReady(true); return nil }

// Stop is a no-op lifecycle hook to satisfy the system.Service contract.
func (s *Service) Stop(ctx context.Context) error { _ = ctx; s.MarkReady(false); return nil }

// Ready reports readiness for engine probes.
func (s *Service) Ready(ctx context.Context) error {
	return s.ServiceBase.Ready(ctx)
}

// CreateKey registers a VRF key for an account.
func (s *Service) CreateKey(ctx context.Context, key domainvrf.Key) (domainvrf.Key, error) {
	if err := s.base.EnsureAccount(ctx, key.AccountID); err != nil {
		return domainvrf.Key{}, err
	}
	if err := s.normalizeKey(&key); err != nil {
		return domainvrf.Key{}, err
	}
	if err := s.ensureWalletOwned(ctx, key.AccountID, key.WalletAddress); err != nil {
		return domainvrf.Key{}, err
	}
	created, err := s.store.CreateVRFKey(ctx, key)
	if err != nil {
		return domainvrf.Key{}, err
	}
	s.log.WithField("key_id", created.ID).WithField("account_id", created.AccountID).Info("vrf key created")
	return created, nil
}

// UpdateKey updates mutable fields on a VRF key.
func (s *Service) UpdateKey(ctx context.Context, accountID string, key domainvrf.Key) (domainvrf.Key, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return domainvrf.Key{}, err
	}
	stored, err := s.store.GetVRFKey(ctx, key.ID)
	if err != nil {
		return domainvrf.Key{}, err
	}
	if err := core.EnsureOwnership(stored.AccountID, accountID, "key", key.ID); err != nil {
		return domainvrf.Key{}, err
	}
	key.AccountID = stored.AccountID
	if err := s.normalizeKey(&key); err != nil {
		return domainvrf.Key{}, err
	}
	if err := s.ensureWalletOwned(ctx, accountID, key.WalletAddress); err != nil {
		return domainvrf.Key{}, err
	}
	updated, err := s.store.UpdateVRFKey(ctx, key)
	if err != nil {
		return domainvrf.Key{}, err
	}
	s.log.WithField("key_id", key.ID).WithField("account_id", key.AccountID).Info("vrf key updated")
	return updated, nil
}

// GetKey fetches a key ensuring ownership.
func (s *Service) GetKey(ctx context.Context, accountID, keyID string) (domainvrf.Key, error) {
	key, err := s.store.GetVRFKey(ctx, keyID)
	if err != nil {
		return domainvrf.Key{}, err
	}
	if err := core.EnsureOwnership(key.AccountID, accountID, "key", keyID); err != nil {
		return domainvrf.Key{}, err
	}
	return key, nil
}

// ListKeys lists keys for an account.
func (s *Service) ListKeys(ctx context.Context, accountID string) ([]domainvrf.Key, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListVRFKeys(ctx, accountID)
}

// CreateRequest enqueues a randomness request.
func (s *Service) CreateRequest(ctx context.Context, accountID, keyID, consumer, seed string, metadata map[string]string) (domainvrf.Request, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return domainvrf.Request{}, err
	}
	key, err := s.store.GetVRFKey(ctx, keyID)
	if err != nil {
		return domainvrf.Request{}, err
	}
	if err := core.EnsureOwnership(key.AccountID, accountID, "key", keyID); err != nil {
		return domainvrf.Request{}, err
	}
	consumer = strings.TrimSpace(consumer)
	seed = strings.TrimSpace(seed)
	if consumer == "" {
		return domainvrf.Request{}, fmt.Errorf("consumer is required")
	}
	if seed == "" {
		return domainvrf.Request{}, fmt.Errorf("seed is required")
	}
	req := domainvrf.Request{
		AccountID: accountID,
		KeyID:     keyID,
		Consumer:  consumer,
		Seed:      seed,
		Status:    domainvrf.RequestStatusPending,
		Metadata:  core.NormalizeMetadata(metadata),
	}
	created, err := s.store.CreateVRFRequest(ctx, req)
	if err != nil {
		return domainvrf.Request{}, err
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
func (s *Service) GetRequest(ctx context.Context, accountID, requestID string) (domainvrf.Request, error) {
	req, err := s.store.GetVRFRequest(ctx, requestID)
	if err != nil {
		return domainvrf.Request{}, err
	}
	if err := core.EnsureOwnership(req.AccountID, accountID, "request", requestID); err != nil {
		return domainvrf.Request{}, err
	}
	return req, nil
}

// ListRequests lists requests for an account.
func (s *Service) ListRequests(ctx context.Context, accountID string, limit int) ([]domainvrf.Request, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListVRFRequests(ctx, accountID, clamped)
}

func (s *Service) normalizeKey(key *domainvrf.Key) error {
	key.PublicKey = strings.TrimSpace(key.PublicKey)
	key.Label = strings.TrimSpace(key.Label)
	key.WalletAddress = strings.ToLower(strings.TrimSpace(key.WalletAddress))
	key.Attestation = strings.TrimSpace(key.Attestation)
	key.Metadata = core.NormalizeMetadata(key.Metadata)
	if key.PublicKey == "" {
		return fmt.Errorf("public_key is required")
	}
	if key.WalletAddress == "" {
		return fmt.Errorf("wallet_address is required")
	}
	status := domainvrf.KeyStatus(strings.ToLower(strings.TrimSpace(string(key.Status))))
	if status == "" {
		status = domainvrf.KeyStatusInactive
	}
	switch status {
	case domainvrf.KeyStatusInactive, domainvrf.KeyStatusPendingApproval, domainvrf.KeyStatusActive, domainvrf.KeyStatusRevoked:
		key.Status = status
	default:
		return fmt.Errorf("invalid status %s", status)
	}
	return nil
}

func (s *Service) ensureWalletOwned(ctx context.Context, accountID, wallet string) error {
	if strings.TrimSpace(wallet) == "" {
		return fmt.Errorf("wallet_address is required")
	}
	return s.base.EnsureSignersOwned(ctx, accountID, []string{wallet})
}
