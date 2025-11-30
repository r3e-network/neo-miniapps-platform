package datalink

import (
	"context"
	"fmt"
	"strings"

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
	framework.ServiceBase
	accounts   AccountChecker
	wallets    WalletChecker
	store      Store
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
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("datalink")
	}
	svc := &Service{
		accounts: accounts,
		store:    store,
		dispatcher: DispatcherFunc(func(context.Context, Delivery, Channel) error {
			return nil
		}),
		dispatch: core.NewDispatchOptions(),
		log:      log,
	}
	svc.SetName(svc.Name())
	return svc
}

// Start/Stop/Ready are inherited from framework.ServiceBase.

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
	s.dispatch.SetTracer(t)
}

// WithWallets injects wallet validation for channels.
func (s *Service) WithWallets(wallets WalletChecker) {
	s.wallets = wallets
}

// WithWalletChecker is an alias for WithWallets for API consistency.
func (s *Service) WithWalletChecker(w WalletChecker) {
	s.wallets = w
}

// CreateChannel registers a channel.
func (s *Service) CreateChannel(ctx context.Context, ch Channel) (Channel, error) {
	if err := s.accounts.AccountExists(ctx, ch.AccountID); err != nil {
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
	s.log.WithField("channel_id", created.ID).WithField("account_id", created.AccountID).Info("datalink channel created")
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
	s.log.WithField("channel_id", ch.ID).WithField("account_id", ch.AccountID).Info("datalink channel updated")
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
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
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
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
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
