package datalink

import (
	"context"
	"fmt"
	"strings"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	domainlink "github.com/R3E-Network/service_layer/internal/app/domain/datalink"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

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
	base       *core.Base
	store      storage.DataLinkStore
	dispatcher Dispatcher
	dispatch   core.DispatchOptions
	log        *logger.Logger
}

// New constructs a service.
func New(accounts storage.AccountStore, store storage.DataLinkStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("datalink")
	}
	return &Service{
		base:  core.NewBase(accounts),
		store: store,
		dispatcher: DispatcherFunc(func(context.Context, domainlink.Delivery, domainlink.Channel) error {
			return nil
		}),
		dispatch: core.NewDispatchOptions(),
		log:      log,
	}
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
	if stored.AccountID != ch.AccountID {
		return domainlink.Channel{}, fmt.Errorf("channel %s does not belong to account %s", ch.ID, ch.AccountID)
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
	if ch.AccountID != accountID {
		return domainlink.Channel{}, fmt.Errorf("channel %s does not belong to account %s", channelID, accountID)
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
	if del.AccountID != accountID {
		return domainlink.Delivery{}, fmt.Errorf("delivery %s does not belong to account %s", deliveryID, accountID)
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

// Descriptor advertises the service placement and capabilities.
func (s *Service) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "datalink",
		Domain:       "datalink",
		Layer:        core.LayerPlatform,
		Capabilities: []string{"channels", "deliveries", "dispatch"},
	}
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
