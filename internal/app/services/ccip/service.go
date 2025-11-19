package ccip

import (
	"context"
	"fmt"
	"strings"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	domainccip "github.com/R3E-Network/service_layer/internal/app/domain/ccip"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Dispatcher notifies downstream deliverers when a CCIP message is ready.
type Dispatcher interface {
	Dispatch(ctx context.Context, msg domainccip.Message, lane domainccip.Lane) error
}

// DispatcherFunc adapts a function to the dispatcher interface.
type DispatcherFunc func(ctx context.Context, msg domainccip.Message, lane domainccip.Lane) error

// Dispatch calls f(ctx, msg, lane).
func (f DispatcherFunc) Dispatch(ctx context.Context, msg domainccip.Message, lane domainccip.Lane) error {
	return f(ctx, msg, lane)
}

// Service orchestrates CCIP lanes and messages.
type Service struct {
	base       *core.Base
	store      storage.CCIPStore
	dispatcher Dispatcher
	dispatch   core.DispatchOptions
	log        *logger.Logger
}

// New creates a CCIP service instance.
func New(accounts storage.AccountStore, store storage.CCIPStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("ccip")
	}
	return &Service{
		base:  core.NewBase(accounts),
		store: store,
		dispatcher: DispatcherFunc(func(context.Context, domainccip.Message, domainccip.Lane) error {
			return nil
		}),
		dispatch: core.NewDispatchOptions(),
		log:      log,
	}
}

// WithDispatcher overrides the dispatcher used on message creation.
func (s *Service) WithDispatcher(d Dispatcher) {
	if d != nil {
		s.dispatcher = d
	}
}

// WithWorkspaceWallets injects wallet validation for signer sets.
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

// CreateLane validates and stores a new lane.
func (s *Service) CreateLane(ctx context.Context, lane domainccip.Lane) (domainccip.Lane, error) {
	if err := s.base.EnsureAccount(ctx, lane.AccountID); err != nil {
		return domainccip.Lane{}, err
	}
	if err := s.normalizeLane(&lane); err != nil {
		return domainccip.Lane{}, err
	}
	if err := s.base.EnsureSignersOwned(ctx, lane.AccountID, lane.SignerSet); err != nil {
		return domainccip.Lane{}, err
	}
	created, err := s.store.CreateLane(ctx, lane)
	if err != nil {
		return domainccip.Lane{}, err
	}
	s.log.WithField("lane_id", created.ID).WithField("account_id", created.AccountID).Info("ccip lane created")
	return created, nil
}

// UpdateLane updates a lane if owned by the account.
func (s *Service) UpdateLane(ctx context.Context, lane domainccip.Lane) (domainccip.Lane, error) {
	stored, err := s.store.GetLane(ctx, lane.ID)
	if err != nil {
		return domainccip.Lane{}, err
	}
	if stored.AccountID != lane.AccountID {
		return domainccip.Lane{}, fmt.Errorf("lane %s does not belong to account %s", lane.ID, lane.AccountID)
	}
	lane.AccountID = stored.AccountID
	if err := s.normalizeLane(&lane); err != nil {
		return domainccip.Lane{}, err
	}
	if err := s.base.EnsureSignersOwned(ctx, lane.AccountID, lane.SignerSet); err != nil {
		return domainccip.Lane{}, err
	}
	updated, err := s.store.UpdateLane(ctx, lane)
	if err != nil {
		return domainccip.Lane{}, err
	}
	s.log.WithField("lane_id", lane.ID).WithField("account_id", lane.AccountID).Info("ccip lane updated")
	return updated, nil
}

// GetLane fetches a lane ensuring ownership.
func (s *Service) GetLane(ctx context.Context, accountID, laneID string) (domainccip.Lane, error) {
	lane, err := s.store.GetLane(ctx, laneID)
	if err != nil {
		return domainccip.Lane{}, err
	}
	if lane.AccountID != accountID {
		return domainccip.Lane{}, fmt.Errorf("lane %s does not belong to account %s", laneID, accountID)
	}
	return lane, nil
}

// ListLanes returns account lanes.
func (s *Service) ListLanes(ctx context.Context, accountID string) ([]domainccip.Lane, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListLanes(ctx, accountID)
}

// SendMessage creates a message for a lane.
func (s *Service) SendMessage(ctx context.Context, accountID, laneID string, payload map[string]any, tokens []domainccip.TokenTransfer, metadata map[string]string, tags []string) (domainccip.Message, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return domainccip.Message{}, err
	}
	lane, err := s.store.GetLane(ctx, laneID)
	if err != nil {
		return domainccip.Message{}, err
	}
	if lane.AccountID != accountID {
		return domainccip.Message{}, fmt.Errorf("lane %s does not belong to account %s", laneID, accountID)
	}

	msg := domainccip.Message{
		AccountID:      accountID,
		LaneID:         laneID,
		Status:         domainccip.MessageStatusPending,
		Payload:        cloneAnyMap(payload),
		TokenTransfers: normalizeTransfers(tokens),
		Metadata:       core.NormalizeMetadata(metadata),
		Tags:           core.NormalizeTags(tags),
	}

	created, err := s.store.CreateMessage(ctx, msg)
	if err != nil {
		return domainccip.Message{}, err
	}
	attrs := map[string]string{"message_id": created.ID, "lane_id": lane.ID}
	if err := s.dispatch.Run(ctx, "ccip.dispatch", attrs, func(spanCtx context.Context) error {
		if err := s.dispatcher.Dispatch(spanCtx, created, lane); err != nil {
			s.log.WithError(err).WithField("message_id", created.ID).Warn("ccip dispatcher error")
			return err
		}
		return nil
	}); err != nil {
		return created, err
	}
	return created, nil
}

// GetMessage fetches a message for the account.
func (s *Service) GetMessage(ctx context.Context, accountID, messageID string) (domainccip.Message, error) {
	msg, err := s.store.GetMessage(ctx, messageID)
	if err != nil {
		return domainccip.Message{}, err
	}
	if msg.AccountID != accountID {
		return domainccip.Message{}, fmt.Errorf("message %s does not belong to account %s", messageID, accountID)
	}
	return msg, nil
}

// ListMessages lists messages for an account.
func (s *Service) ListMessages(ctx context.Context, accountID string, limit int) ([]domainccip.Message, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListMessages(ctx, accountID, clamped)
}

// Descriptor advertises the service placement and capabilities.
func (s *Service) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "ccip",
		Domain:       "ccip",
		Layer:        core.LayerPlatform,
		Capabilities: []string{"lanes", "dispatch", "messages"},
	}
}

func (s *Service) normalizeLane(lane *domainccip.Lane) error {
	lane.Name = strings.TrimSpace(lane.Name)
	lane.SourceChain = strings.ToLower(strings.TrimSpace(lane.SourceChain))
	lane.DestChain = strings.ToLower(strings.TrimSpace(lane.DestChain))
	lane.SignerSet = core.NormalizeTags(lane.SignerSet)
	lane.Metadata = core.NormalizeMetadata(lane.Metadata)
	lane.Tags = core.NormalizeTags(lane.Tags)
	lane.AllowedTokens = normalizeStringList(lane.AllowedTokens)
	lane.DeliveryPolicy = cloneAnyMap(lane.DeliveryPolicy)

	if lane.Name == "" {
		return fmt.Errorf("name is required")
	}
	if lane.SourceChain == "" || lane.DestChain == "" {
		return fmt.Errorf("source_chain and dest_chain are required")
	}
	return nil
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, val := range values {
		trimmed := strings.ToLower(strings.TrimSpace(val))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func normalizeTransfers(transfers []domainccip.TokenTransfer) []domainccip.TokenTransfer {
	if len(transfers) == 0 {
		return nil
	}
	result := make([]domainccip.TokenTransfer, 0, len(transfers))
	for _, tr := range transfers {
		token := strings.ToLower(strings.TrimSpace(tr.Token))
		amount := strings.TrimSpace(tr.Amount)
		recipient := strings.TrimSpace(tr.Recipient)
		if token == "" || amount == "" || recipient == "" {
			continue
		}
		result = append(result, domainccip.TokenTransfer{Token: token, Amount: amount, Recipient: recipient})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
