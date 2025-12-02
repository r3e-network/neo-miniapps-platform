package vrf

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/os/sandbox"
	
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
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
// Uses SandboxedServiceEngine for capability-based access control.
type Service struct {
	*framework.SandboxedServiceEngine
	store        Store
	dispatcher   Dispatcher
	dispatch     core.DispatchOptions
	customTracer core.Tracer
}

// New constructs a VRF service.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	svc := &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:         "vrf",
				Domain:       "vrf",
				Description:  "VRF key and request management",
				DependsOn:    []string{"store", "svc-accounts"},
				RequiresAPIs: []framework.APISurface{framework.APISurfaceStore, framework.APISurfaceEvent},
				Capabilities: []string{"vrf"},
				Accounts:     accounts,
				Logger:       log,
			},
			SecurityLevel: sandbox.SecurityLevelPrivileged,
			RequestedCapabilities: []sandbox.Capability{
				sandbox.CapStorageRead,
				sandbox.CapStorageWrite,
				sandbox.CapDatabaseRead,
				sandbox.CapDatabaseWrite,
				sandbox.CapBusPublish,
				sandbox.CapServiceCall,
			},
			StorageQuota: 10 * 1024 * 1024, // 10MB
		}),
		store: store,
		dispatcher: DispatcherFunc(func(context.Context, Request, Key) error {
			return nil
		}),
		dispatch: core.NewDispatchOptions(),
	}
	return svc
}

// WithDispatcher overrides the dispatcher implementation.
func (s *Service) WithDispatcher(d Dispatcher) {
	if d != nil {
		s.dispatcher = d
	}
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
	if t == nil {
		s.customTracer = nil
		t = s.Tracer()
	} else {
		s.customTracer = t
	}
	s.dispatch.SetTracer(t)
}

func (s *Service) SetEnvironment(env framework.Environment) {
	s.ServiceEngine.SetEnvironment(env)
	tracer := s.customTracer
	if tracer == nil {
		tracer = s.Tracer()
	}
	s.dispatch.SetTracer(tracer)
}

// Start/Stop/Ready are inherited from framework.ServiceEngine.

// CreateKey registers a VRF key for an account.
func (s *Service) CreateKey(ctx context.Context, key Key) (Key, error) {
	if err := s.ValidateAccountExists(ctx, key.AccountID); err != nil {
		return Key{}, err
	}
	if err := s.normalizeKey(&key); err != nil {
		return Key{}, err
	}
	if err := s.ensureWalletOwned(ctx, key.AccountID, key.WalletAddress); err != nil {
		return Key{}, err
	}
	attrs := map[string]string{"account_id": key.AccountID, "resource": "vrf_key"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	created, err := s.store.CreateKey(ctx, key)
	if err != nil {
		return Key{}, err
	}
	s.Logger().WithField("key_id", created.ID).WithField("account_id", created.AccountID).Info("vrf key created")
	s.IncrementCounter("vrf_keys_created_total", map[string]string{"account_id": created.AccountID})
	return created, nil
}

// UpdateKey updates mutable fields on a VRF key.
func (s *Service) UpdateKey(ctx context.Context, accountID string, key Key) (Key, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
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
	attrs := map[string]string{"account_id": key.AccountID, "key_id": key.ID, "resource": "vrf_key"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	updated, err := s.store.UpdateKey(ctx, key)
	if err != nil {
		return Key{}, err
	}
	s.Logger().WithField("key_id", key.ID).WithField("account_id", key.AccountID).Info("vrf key updated")
	s.IncrementCounter("vrf_keys_updated_total", map[string]string{"account_id": key.AccountID})
	return updated, nil
}

// GetKey fetches a key ensuring ownership.
func (s *Service) GetKey(ctx context.Context, accountID, keyID string) (Key, error) {
	attrs := map[string]string{"account_id": accountID, "key_id": keyID, "resource": "vrf_get_key"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
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
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	attrs := map[string]string{"account_id": accountID, "resource": "vrf_list_keys"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	return s.store.ListKeys(ctx, accountID)
}

// CreateRequest enqueues a randomness request.
func (s *Service) CreateRequest(ctx context.Context, accountID, keyID, consumer, seed string, metadata map[string]string) (Request, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
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
	attrs := map[string]string{"account_id": accountID, "key_id": key.ID, "resource": "vrf_request"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	created, err := s.store.CreateRequest(ctx, req)
	if err != nil {
		return Request{}, err
	}
	s.IncrementCounter("vrf_requests_created_total", map[string]string{"key_id": key.ID})
	eventPayload := map[string]any{
		"request_id": created.ID,
		"account_id": accountID,
		"key_id":     key.ID,
	}
	if err := s.PublishEvent(ctx, "vrf.request.created", eventPayload); err != nil {
		if errors.Is(err, core.ErrBusUnavailable) {
			s.Logger().WithError(err).Warn("bus unavailable for vrf request event")
		} else {
			return Request{}, fmt.Errorf("publish vrf request event: %w", err)
		}
	}
	dispatchAttrs := map[string]string{"request_id": created.ID, "key_id": key.ID}
	if err := s.dispatch.Run(ctx, "vrf.dispatch", dispatchAttrs, func(spanCtx context.Context) error {
		if err := s.dispatcher.Dispatch(spanCtx, created, key); err != nil {
			s.Logger().WithError(err).WithField("request_id", created.ID).Warn("vrf dispatcher error")
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
	attrs := map[string]string{"account_id": accountID, "request_id": requestID, "resource": "vrf_request_get"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
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
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	attrs := map[string]string{"account_id": accountID, "resource": "vrf_list_requests"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
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
	// Use ServiceEngine's ValidateSigners to check wallet ownership
	return s.ValidateSigners(ctx, accountID, []string{wallet})
}

// HTTP API Methods
// These methods follow the HTTP{Method}{Path} naming convention for automatic route discovery.

// HTTPGetKeys handles GET /keys - list all VRF keys for an account.
func (s *Service) HTTPGetKeys(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListKeys(ctx, req.AccountID)
}

// HTTPPostKeys handles POST /keys - create a new VRF key.
func (s *Service) HTTPPostKeys(ctx context.Context, req core.APIRequest) (any, error) {
	publicKey, _ := req.Body["public_key"].(string)
	walletAddress, _ := req.Body["wallet_address"].(string)
	label, _ := req.Body["label"].(string)
	attestation, _ := req.Body["attestation"].(string)

	metadata := core.ExtractMetadataRaw(req.Body, "")

	key := Key{
		AccountID:     req.AccountID,
		PublicKey:     publicKey,
		WalletAddress: walletAddress,
		Label:         label,
		Attestation:   attestation,
		Metadata:      metadata,
	}
	return s.CreateKey(ctx, key)
}

// HTTPGetKeysById handles GET /keys/{id} - get a specific VRF key.
func (s *Service) HTTPGetKeysById(ctx context.Context, req core.APIRequest) (any, error) {
	keyID := req.PathParams["id"]
	return s.GetKey(ctx, req.AccountID, keyID)
}

// HTTPPatchKeysById handles PATCH /keys/{id} - update a VRF key.
func (s *Service) HTTPPatchKeysById(ctx context.Context, req core.APIRequest) (any, error) {
	keyID := req.PathParams["id"]

	// Get existing key first
	existing, err := s.GetKey(ctx, req.AccountID, keyID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if label, ok := req.Body["label"].(string); ok {
		existing.Label = label
	}
	if status, ok := req.Body["status"].(string); ok {
		existing.Status = KeyStatus(status)
	}
	if _, ok := req.Body["metadata"].(map[string]any); ok {
		existing.Metadata = core.ExtractMetadataRaw(req.Body, "")
	}

	return s.UpdateKey(ctx, req.AccountID, existing)
}

// HTTPGetRequests handles GET /requests - list all VRF requests for an account.
func (s *Service) HTTPGetRequests(ctx context.Context, req core.APIRequest) (any, error) {
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListRequests(ctx, req.AccountID, limit)
}

// HTTPPostRequests handles POST /requests - create a new VRF request.
func (s *Service) HTTPPostRequests(ctx context.Context, req core.APIRequest) (any, error) {
	keyID, _ := req.Body["key_id"].(string)
	consumer, _ := req.Body["consumer"].(string)
	seed, _ := req.Body["seed"].(string)

	metadata := core.ExtractMetadataRaw(req.Body, "")

	return s.CreateRequest(ctx, req.AccountID, keyID, consumer, seed, metadata)
}

// HTTPGetRequestsById handles GET /requests/{id} - get a specific VRF request.
func (s *Service) HTTPGetRequestsById(ctx context.Context, req core.APIRequest) (any, error) {
	requestID := req.PathParams["id"]
	return s.GetRequest(ctx, req.AccountID, requestID)
}
