package datalink

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
	
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
	"github.com/R3E-Network/service_layer/system/sandbox"
)

// Compile-time check: Service exposes Publish for the core engine adapter.
var _ core.EventPublisher = (*Service)(nil)

// Dispatcher handles delivery attempts.
type Dispatcher interface {
	Dispatch(ctx context.Context, delivery Delivery, channel Channel) error
}

// DispatcherFunc converts a function to a Dispatcher.
type DispatcherFunc func(ctx context.Context, delivery Delivery, channel Channel) error

// Dispatch calls f(ctx, delivery, channel).
func (f DispatcherFunc) Dispatch(ctx context.Context, delivery Delivery, channel Channel) error {
	return f(ctx, delivery, channel)
}

// Service manages datalink channels and deliveries.
type Service struct {
	*framework.SandboxedServiceEngine
	wallets      WalletChecker
	store        Store
	dispatcher   Dispatcher
	dispatch     core.DispatchOptions
	customTracer core.Tracer
}

// New constructs a service.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	return &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:         "datalink",
				Domain:       "datalink",
				Description:  "DataLink channels and deliveries",
				DependsOn:    []string{"store", "svc-accounts"},
				RequiresAPIs: []framework.APISurface{framework.APISurfaceStore, framework.APISurfaceData, framework.APISurfaceEvent},
				Capabilities: []string{"datalink"},
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
				sandbox.CapNetworkOutbound,
			},
			StorageQuota: 10 * 1024 * 1024,
		}),
		store: store,
		dispatcher: DispatcherFunc(func(context.Context, Delivery, Channel) error {
			return nil
		}),
		dispatch: core.NewDispatchOptions(),
	}
}

// Start/Stop/Ready are inherited from framework.ServiceEngine.

// Publish implements EventEngine for the core engine by enqueuing a delivery.
func (s *Service) Publish(ctx context.Context, event string, payload any) error {
	if !strings.EqualFold(event, "delivery") {
		return fmt.Errorf("unsupported event: %s", event)
	}
	body, ok := payload.(map[string]any)
	if !ok {
		return fmt.Errorf("payload must be a map")
	}
	accountID, _ := body["account_id"].(string)
	channelID, _ := body["channel_id"].(string)
	meta, _ := body["metadata"].(map[string]string)
	payloadMap, _ := body["payload"].(map[string]any)
	_, err := s.CreateDelivery(ctx, accountID, channelID, payloadMap, meta)
	return err
}

// Subscribe is not implemented for datalink; use Publish via the engine bus.
func (s *Service) Subscribe(ctx context.Context, event string, handler func(context.Context, any) error) error {
	if !strings.EqualFold(event, "delivery") {
		return fmt.Errorf("unsupported event: %s", event)
	}
	if handler == nil {
		return core.RequiredError("handler")
	}
	return fmt.Errorf("subscribe not supported for datalink; publish delivery events instead")
}

// WithDispatcher overrides dispatch logic.
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

// WithWallets injects wallet validation for channels.
func (s *Service) WithWallets(wallets WalletChecker) {
	s.wallets = wallets
	s.SandboxedServiceEngine.WithWalletChecker(wallets)
}

// WithWalletChecker is an alias for WithWallets for API consistency.
func (s *Service) WithWalletChecker(w WalletChecker) {
	s.wallets = w
	s.SandboxedServiceEngine.WithWalletChecker(w)
}

// WithObservationHooks configures optional observability hooks.
func (s *Service) WithObservationHooks(h core.ObservationHooks) {
	s.SandboxedServiceEngine.WithObservationHooks(h)
}

// SetEnvironment ensures dispatcher dependencies follow the runtime environment.
func (s *Service) SetEnvironment(env framework.Environment) {
	s.SandboxedServiceEngine.SetEnvironment(env)
	tracer := s.customTracer
	if tracer == nil {
		tracer = s.Tracer()
	}
	s.dispatch.SetTracer(tracer)
}

// CreateChannel registers a channel.
func (s *Service) CreateChannel(ctx context.Context, ch Channel) (Channel, error) {
	if err := s.ValidateAccountExists(ctx, ch.AccountID); err != nil {
		return Channel{}, err
	}
	if err := s.normalizeChannel(&ch); err != nil {
		return Channel{}, err
	}
	if s.wallets != nil {
		for _, signer := range ch.SignerSet {
			if err := s.wallets.WalletOwnedBy(ctx, ch.AccountID, signer); err != nil {
				return Channel{}, fmt.Errorf("signer %s not owned by account: %w", signer, err)
			}
		}
	}
	created, err := s.store.CreateChannel(ctx, ch)
	if err != nil {
		return Channel{}, err
	}
	s.Logger().WithField("channel_id", created.ID).WithField("account_id", created.AccountID).Info("datalink channel created")
	s.IncrementCounter("datalink_channels_created_total", map[string]string{"account_id": created.AccountID})
	return created, nil
}

// UpdateChannel mutates channel fields.
func (s *Service) UpdateChannel(ctx context.Context, ch Channel) (Channel, error) {
	stored, err := s.store.GetChannel(ctx, ch.ID)
	if err != nil {
		return Channel{}, err
	}
	if err := core.EnsureOwnership(stored.AccountID, ch.AccountID, "channel", ch.ID); err != nil {
		return Channel{}, err
	}
	ch.AccountID = stored.AccountID
	if err := s.normalizeChannel(&ch); err != nil {
		return Channel{}, err
	}
	if s.wallets != nil {
		for _, signer := range ch.SignerSet {
			if err := s.wallets.WalletOwnedBy(ctx, ch.AccountID, signer); err != nil {
				return Channel{}, fmt.Errorf("signer %s not owned by account: %w", signer, err)
			}
		}
	}
	updated, err := s.store.UpdateChannel(ctx, ch)
	if err != nil {
		return Channel{}, err
	}
	s.Logger().WithField("channel_id", ch.ID).WithField("account_id", ch.AccountID).Info("datalink channel updated")
	s.IncrementCounter("datalink_channels_updated_total", map[string]string{"account_id": ch.AccountID})
	return updated, nil
}

// GetChannel fetches a channel ensuring ownership.
func (s *Service) GetChannel(ctx context.Context, accountID, channelID string) (Channel, error) {
	ch, err := s.store.GetChannel(ctx, channelID)
	if err != nil {
		return Channel{}, err
	}
	if err := core.EnsureOwnership(ch.AccountID, accountID, "channel", channelID); err != nil {
		return Channel{}, err
	}
	return ch, nil
}

// ListChannels lists account channels.
func (s *Service) ListChannels(ctx context.Context, accountID string) ([]Channel, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListChannels(ctx, accountID)
}

// CreateDelivery enqueues a delivery.
func (s *Service) CreateDelivery(ctx context.Context, accountID, channelID string, payload map[string]any, metadata map[string]string) (Delivery, error) {
	ch, err := s.GetChannel(ctx, accountID, channelID)
	if err != nil {
		return Delivery{}, err
	}
	del := Delivery{
		AccountID: accountID,
		ChannelID: channelID,
		Payload:   payload,
		Metadata:  core.NormalizeMetadata(metadata),
		Status:    DeliveryStatusPending,
	}
	created, err := s.store.CreateDelivery(ctx, del)
	if err != nil {
		return Delivery{}, err
	}
	attrs := map[string]string{"delivery_id": created.ID, "channel_id": ch.ID}
	s.IncrementCounter("datalink_deliveries_created_total", map[string]string{"channel_id": ch.ID})
	eventPayload := map[string]any{
		"delivery_id": created.ID,
		"account_id":  accountID,
		"channel_id":  ch.ID,
	}
	if err := s.PublishEvent(ctx, "datalink.delivery.created", eventPayload); err != nil {
		if errors.Is(err, core.ErrBusUnavailable) {
			s.Logger().WithError(err).Warn("bus unavailable for datalink delivery event")
		} else {
			return Delivery{}, fmt.Errorf("publish delivery event: %w", err)
		}
	}
	if err := s.dispatch.Run(ctx, "datalink.dispatch", attrs, func(spanCtx context.Context) error {
		if err := s.dispatcher.Dispatch(spanCtx, created, ch); err != nil {
			s.Logger().WithError(err).WithField("delivery_id", created.ID).Warn("datalink dispatcher error")
			return err
		}
		return nil
	}); err != nil {
		return created, err
	}
	return created, nil
}

// GetDelivery fetches a delivery.
func (s *Service) GetDelivery(ctx context.Context, accountID, deliveryID string) (Delivery, error) {
	del, err := s.store.GetDelivery(ctx, deliveryID)
	if err != nil {
		return Delivery{}, err
	}
	if err := core.EnsureOwnership(del.AccountID, accountID, "delivery", deliveryID); err != nil {
		return Delivery{}, err
	}
	return del, nil
}

// ListDeliveries lists account deliveries.
func (s *Service) ListDeliveries(ctx context.Context, accountID string, limit int) ([]Delivery, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListDeliveries(ctx, accountID, clamped)
}

func (s *Service) normalizeChannel(ch *Channel) error {
	ch.Name = strings.TrimSpace(ch.Name)
	ch.Endpoint = strings.TrimSpace(ch.Endpoint)
	ch.AuthToken = strings.TrimSpace(ch.AuthToken)
	ch.Metadata = core.NormalizeMetadata(ch.Metadata)
	ch.SignerSet = core.NormalizeTags(ch.SignerSet)
	if ch.Name == "" {
		return core.RequiredError("name")
	}
	if ch.Endpoint == "" {
		return core.RequiredError("endpoint")
	}
	if len(ch.SignerSet) == 0 {
		return core.RequiredError("signer_set")
	}
	status := ChannelStatus(strings.ToLower(strings.TrimSpace(string(ch.Status))))
	if status == "" {
		status = ChannelStatusInactive
	}
	switch status {
	case ChannelStatusInactive, ChannelStatusActive, ChannelStatusSuspended:
		ch.Status = status
	default:
		return fmt.Errorf("invalid status %s", status)
	}
	return nil
}

// HTTP API Methods
// These methods follow the HTTP{Method}{Path} naming convention for automatic route discovery.

// HTTPGetChannels handles GET /channels - list all channels for an account.
func (s *Service) HTTPGetChannels(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListChannels(ctx, req.AccountID)
}

// HTTPPostChannels handles POST /channels - create a new channel.
func (s *Service) HTTPPostChannels(ctx context.Context, req core.APIRequest) (any, error) {
	name, _ := req.Body["name"].(string)
	endpoint, _ := req.Body["endpoint"].(string)
	authToken, _ := req.Body["auth_token"].(string)
	status, _ := req.Body["status"].(string)

	var signerSet []string
	if rawSigners, ok := req.Body["signer_set"].([]any); ok {
		for _, s := range rawSigners {
			if str, ok := s.(string); ok {
				signerSet = append(signerSet, str)
			}
		}
	}

	metadata := core.ExtractMetadataRaw(req.Body, "")

	ch := Channel{
		AccountID: req.AccountID,
		Name:      name,
		Endpoint:  endpoint,
		AuthToken: authToken,
		Status:    ChannelStatus(status),
		SignerSet: signerSet,
		Metadata:  metadata,
	}

	return s.CreateChannel(ctx, ch)
}

// HTTPGetChannelsById handles GET /channels/{id} - get a specific channel.
func (s *Service) HTTPGetChannelsById(ctx context.Context, req core.APIRequest) (any, error) {
	channelID := req.PathParams["id"]
	return s.GetChannel(ctx, req.AccountID, channelID)
}

// HTTPPatchChannelsById handles PATCH /channels/{id} - update a channel.
func (s *Service) HTTPPatchChannelsById(ctx context.Context, req core.APIRequest) (any, error) {
	channelID := req.PathParams["id"]

	// Get existing channel first
	existing, err := s.GetChannel(ctx, req.AccountID, channelID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if name, ok := req.Body["name"].(string); ok {
		existing.Name = name
	}
	if endpoint, ok := req.Body["endpoint"].(string); ok {
		existing.Endpoint = endpoint
	}
	if authToken, ok := req.Body["auth_token"].(string); ok {
		existing.AuthToken = authToken
	}
	if status, ok := req.Body["status"].(string); ok {
		existing.Status = ChannelStatus(status)
	}
	if rawSigners, ok := req.Body["signer_set"].([]any); ok {
		var signerSet []string
		for _, s := range rawSigners {
			if str, ok := s.(string); ok {
				signerSet = append(signerSet, str)
			}
		}
		existing.SignerSet = signerSet
	}

	existing.AccountID = req.AccountID
	return s.UpdateChannel(ctx, existing)
}

// HTTPGetDeliveries handles GET /deliveries - list all deliveries for an account.
func (s *Service) HTTPGetDeliveries(ctx context.Context, req core.APIRequest) (any, error) {
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListDeliveries(ctx, req.AccountID, limit)
}

// HTTPPostDeliveries handles POST /deliveries - create a new delivery.
func (s *Service) HTTPPostDeliveries(ctx context.Context, req core.APIRequest) (any, error) {
	channelID, _ := req.Body["channel_id"].(string)

	var payload map[string]any
	if p, ok := req.Body["payload"].(map[string]any); ok {
		payload = p
	}

	metadata := core.ExtractMetadataRaw(req.Body, "")

	return s.CreateDelivery(ctx, req.AccountID, channelID, payload, metadata)
}

// HTTPGetDeliveriesById handles GET /deliveries/{id} - get a specific delivery.
func (s *Service) HTTPGetDeliveriesById(ctx context.Context, req core.APIRequest) (any, error) {
	deliveryID := req.PathParams["id"]
	return s.GetDelivery(ctx, req.AccountID, deliveryID)
}
