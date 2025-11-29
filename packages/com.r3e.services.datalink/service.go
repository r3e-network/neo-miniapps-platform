package datalink

import (
	"context"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/applications/storage"
	domainlink "github.com/R3E-Network/service_layer/domain/datalink"
	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Compile-time check: Service exposes Publish for the core engine adapter.
type eventPublisher interface {
	Publish(context.Context, string, any) error
}

var _ eventPublisher = (*Service)(nil)

// Dispatcher handles delivery attempts.
type Dispatcher interface {
	Dispatch(ctx context.Context, delivery domainlink.Delivery, channel domainlink.Channel) error
}

// DispatcherFunc converts a function to a Dispatcher.
type DispatcherFunc func(ctx context.Context, delivery domainlink.Delivery, channel domainlink.Channel) error

// Dispatch calls f(ctx, delivery, channel).
func (f DispatcherFunc) Dispatch(ctx context.Context, delivery domainlink.Delivery, channel domainlink.Channel) error {
	return f(ctx, delivery, channel)
}

// Service manages datalink channels and deliveries.
type Service struct {
	framework.ServiceBase
	base       *core.Base
	store      storage.DataLinkStore
	dispatcher Dispatcher
	dispatch   core.DispatchOptions
	log        *logger.Logger
}

// Name returns the stable engine module name.
func (s *Service) Name() string { return "datalink" }

// Domain reports the service domain for engine grouping.
func (s *Service) Domain() string { return "datalink" }

// Manifest describes the service contract for the engine OS.
func (s *Service) Manifest() *framework.Manifest {
	return &framework.Manifest{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Description:  "DataLink channels and deliveries",
		Layer:        "service",
		DependsOn:    []string{"store", "svc-accounts"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceData, engine.APISurfaceEvent},
		Capabilities: []string{"datalink"},
	}
}

// Descriptor advertises the service for system discovery.
func (s *Service) Descriptor() core.Descriptor { return s.Manifest().ToDescriptor() }

// New constructs a service.
func New(accounts storage.AccountStore, store storage.DataLinkStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("datalink")
	}
	svc := &Service{
		base:  core.NewBase(accounts),
		store: store,
		dispatcher: DispatcherFunc(func(context.Context, domainlink.Delivery, domainlink.Channel) error {
			return nil
		}),
		dispatch: core.NewDispatchOptions(),
		log:      log,
	}
	svc.SetName(svc.Name())
	return svc
}

// Start marks the service ready for dispatch hooks.
func (s *Service) Start(ctx context.Context) error { _ = ctx; s.MarkReady(true); return nil }

// Stop clears readiness flag.
func (s *Service) Stop(ctx context.Context) error { _ = ctx; s.MarkReady(false); return nil }

// Ready reports readiness.
func (s *Service) Ready(ctx context.Context) error {
	return s.ServiceBase.Ready(ctx)
}

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
		return fmt.Errorf("handler is required")
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
	s.dispatch.SetTracer(t)
}

// WithWorkspaceWallets injects wallet validation for channels.
func (s *Service) WithWorkspaceWallets(store storage.WorkspaceWalletStore) {
	s.base.SetWallets(store)
}

// CreateChannel registers a channel.
func (s *Service) CreateChannel(ctx context.Context, ch domainlink.Channel) (domainlink.Channel, error) {
	if err := s.base.EnsureAccount(ctx, ch.AccountID); err != nil {
		return domainlink.Channel{}, err
	}
	if err := s.normalizeChannel(&ch); err != nil {
		return domainlink.Channel{}, err
	}
	if err := s.base.EnsureSignersOwned(ctx, ch.AccountID, ch.SignerSet); err != nil {
		return domainlink.Channel{}, err
	}
	created, err := s.store.CreateChannel(ctx, ch)
	if err != nil {
		return domainlink.Channel{}, err
	}
	s.log.WithField("channel_id", created.ID).WithField("account_id", created.AccountID).Info("datalink channel created")
	return created, nil
}

// UpdateChannel mutates channel fields.
func (s *Service) UpdateChannel(ctx context.Context, ch domainlink.Channel) (domainlink.Channel, error) {
	stored, err := s.store.GetChannel(ctx, ch.ID)
	if err != nil {
		return domainlink.Channel{}, err
	}
	if err := core.EnsureOwnership(stored.AccountID, ch.AccountID, "channel", ch.ID); err != nil {
		return domainlink.Channel{}, err
	}
	ch.AccountID = stored.AccountID
	if err := s.normalizeChannel(&ch); err != nil {
		return domainlink.Channel{}, err
	}
	if err := s.base.EnsureSignersOwned(ctx, ch.AccountID, ch.SignerSet); err != nil {
		return domainlink.Channel{}, err
	}
	updated, err := s.store.UpdateChannel(ctx, ch)
	if err != nil {
		return domainlink.Channel{}, err
	}
	s.log.WithField("channel_id", ch.ID).WithField("account_id", ch.AccountID).Info("datalink channel updated")
	return updated, nil
}

// GetChannel fetches a channel ensuring ownership.
func (s *Service) GetChannel(ctx context.Context, accountID, channelID string) (domainlink.Channel, error) {
	ch, err := s.store.GetChannel(ctx, channelID)
	if err != nil {
		return domainlink.Channel{}, err
	}
	if err := core.EnsureOwnership(ch.AccountID, accountID, "channel", channelID); err != nil {
		return domainlink.Channel{}, err
	}
	return ch, nil
}

// ListChannels lists account channels.
func (s *Service) ListChannels(ctx context.Context, accountID string) ([]domainlink.Channel, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListChannels(ctx, accountID)
}

// CreateDelivery enqueues a delivery.
func (s *Service) CreateDelivery(ctx context.Context, accountID, channelID string, payload map[string]any, metadata map[string]string) (domainlink.Delivery, error) {
	ch, err := s.GetChannel(ctx, accountID, channelID)
	if err != nil {
		return domainlink.Delivery{}, err
	}
	del := domainlink.Delivery{
		AccountID: accountID,
		ChannelID: channelID,
		Payload:   payload,
		Metadata:  core.NormalizeMetadata(metadata),
		Status:    domainlink.DeliveryStatusPending,
	}
	created, err := s.store.CreateDelivery(ctx, del)
	if err != nil {
		return domainlink.Delivery{}, err
	}
	attrs := map[string]string{"delivery_id": created.ID, "channel_id": ch.ID}
	if err := s.dispatch.Run(ctx, "datalink.dispatch", attrs, func(spanCtx context.Context) error {
		if err := s.dispatcher.Dispatch(spanCtx, created, ch); err != nil {
			s.log.WithError(err).WithField("delivery_id", created.ID).Warn("datalink dispatcher error")
			return err
		}
		return nil
	}); err != nil {
		return created, err
	}
	return created, nil
}

// GetDelivery fetches a delivery.
func (s *Service) GetDelivery(ctx context.Context, accountID, deliveryID string) (domainlink.Delivery, error) {
	del, err := s.store.GetDelivery(ctx, deliveryID)
	if err != nil {
		return domainlink.Delivery{}, err
	}
	if err := core.EnsureOwnership(del.AccountID, accountID, "delivery", deliveryID); err != nil {
		return domainlink.Delivery{}, err
	}
	return del, nil
}

// ListDeliveries lists account deliveries.
func (s *Service) ListDeliveries(ctx context.Context, accountID string, limit int) ([]domainlink.Delivery, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListDeliveries(ctx, accountID, clamped)
}

func (s *Service) normalizeChannel(ch *domainlink.Channel) error {
	ch.Name = strings.TrimSpace(ch.Name)
	ch.Endpoint = strings.TrimSpace(ch.Endpoint)
	ch.AuthToken = strings.TrimSpace(ch.AuthToken)
	ch.Metadata = core.NormalizeMetadata(ch.Metadata)
	ch.SignerSet = core.NormalizeTags(ch.SignerSet)
	if ch.Name == "" {
		return fmt.Errorf("name is required")
	}
	if ch.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if len(ch.SignerSet) == 0 {
		return fmt.Errorf("signer_set is required")
	}
	status := domainlink.ChannelStatus(strings.ToLower(strings.TrimSpace(string(ch.Status))))
	if status == "" {
		status = domainlink.ChannelStatusInactive
	}
	switch status {
	case domainlink.ChannelStatusInactive, domainlink.ChannelStatusActive, domainlink.ChannelStatusSuspended:
		ch.Status = status
	default:
		return fmt.Errorf("invalid status %s", status)
	}
	return nil
}
