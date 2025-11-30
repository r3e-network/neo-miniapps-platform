package ccip

import (
	"context"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Dispatcher notifies downstream deliverers when a CCIP message is ready.
type Dispatcher interface {
	Dispatch(ctx context.Context, msg Message, lane Lane) error
}

// DispatcherFunc adapts a function to the dispatcher interface.
type DispatcherFunc func(ctx context.Context, msg Message, lane Lane) error

// Dispatch calls f(ctx, msg, lane).
func (f DispatcherFunc) Dispatch(ctx context.Context, msg Message, lane Lane) error {
	return f(ctx, msg, lane)
}

// Service orchestrates CCIP lanes and messages.
type Service struct {
	framework.ServiceBase
	accounts   AccountChecker
	wallets    WalletChecker
	store      Store
	dispatcher Dispatcher
	dispatch   core.DispatchOptions
	log        *logger.Logger
}

// New creates a CCIP service instance.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("ccip")
	}
	svc := &Service{
		accounts: accounts,
		store:    store,
		dispatcher: DispatcherFunc(func(context.Context, Message, Lane) error {
			return nil
		}),
		dispatch: core.NewDispatchOptions(),
		log:      log,
	}
	svc.SetName(svc.Name())
	return svc
}

// WithDispatcher overrides the dispatcher used on message creation.
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
func (s *Service) Name() string { return "ccip" }

// Domain reports the service domain.
func (s *Service) Domain() string { return "ccip" }

// Manifest describes the service contract for the engine OS.
func (s *Service) Manifest() *framework.Manifest {
	return &framework.Manifest{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Description:  "CCIP lanes and messages",
		Layer:        "service",
		DependsOn:    []string{"store", "svc-accounts"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceEvent},
		Capabilities: []string{"ccip"},
	}
}

// Start/Stop/Ready are inherited from framework.ServiceBase.

// CreateLane validates and stores a new lane.
func (s *Service) CreateLane(ctx context.Context, lane Lane) (Lane, error) {
	if err := s.accounts.AccountExists(ctx, lane.AccountID); err != nil {
		return Lane{}, err
	}
	if err := s.normalizeLane(&lane); err != nil {
		return Lane{}, err
	}
	if err := s.ensureSignersOwned(ctx, lane.AccountID, lane.SignerSet); err != nil {
		return Lane{}, err
	}
	created, err := s.store.CreateLane(ctx, lane)
	if err != nil {
		return Lane{}, err
	}
	s.log.WithField("lane_id", created.ID).WithField("account_id", created.AccountID).Info("ccip lane created")
	return created, nil
}

// UpdateLane updates a lane if owned by the account.
func (s *Service) UpdateLane(ctx context.Context, lane Lane) (Lane, error) {
	stored, err := s.store.GetLane(ctx, lane.ID)
	if err != nil {
		return Lane{}, err
	}
	if err := core.EnsureOwnership(stored.AccountID, lane.AccountID, "lane", lane.ID); err != nil {
		return Lane{}, err
	}
	lane.AccountID = stored.AccountID
	if err := s.normalizeLane(&lane); err != nil {
		return Lane{}, err
	}
	if err := s.ensureSignersOwned(ctx, lane.AccountID, lane.SignerSet); err != nil {
		return Lane{}, err
	}
	updated, err := s.store.UpdateLane(ctx, lane)
	if err != nil {
		return Lane{}, err
	}
	s.log.WithField("lane_id", lane.ID).WithField("account_id", lane.AccountID).Info("ccip lane updated")
	return updated, nil
}

// GetLane fetches a lane ensuring ownership.
func (s *Service) GetLane(ctx context.Context, accountID, laneID string) (Lane, error) {
	lane, err := s.store.GetLane(ctx, laneID)
	if err != nil {
		return Lane{}, err
	}
	if err := core.EnsureOwnership(lane.AccountID, accountID, "lane", laneID); err != nil {
		return Lane{}, err
	}
	return lane, nil
}

// ListLanes returns account lanes.
func (s *Service) ListLanes(ctx context.Context, accountID string) ([]Lane, error) {
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListLanes(ctx, accountID)
}

// SendMessage creates a message for a lane.
func (s *Service) SendMessage(ctx context.Context, accountID, laneID string, payload map[string]any, tokens []TokenTransfer, metadata map[string]string, tags []string) (Message, error) {
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return Message{}, err
	}
	lane, err := s.store.GetLane(ctx, laneID)
	if err != nil {
		return Message{}, err
	}
	if err := core.EnsureOwnership(lane.AccountID, accountID, "lane", laneID); err != nil {
		return Message{}, err
	}

	msg := Message{
		AccountID:      accountID,
		LaneID:         laneID,
		Status:         MessageStatusPending,
		Payload:        core.CloneAnyMap(payload),
		TokenTransfers: normalizeTransfers(tokens),
		Metadata:       core.NormalizeMetadata(metadata),
		Tags:           core.NormalizeTags(tags),
	}

	created, err := s.store.CreateMessage(ctx, msg)
	if err != nil {
		return Message{}, err
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
func (s *Service) GetMessage(ctx context.Context, accountID, messageID string) (Message, error) {
	msg, err := s.store.GetMessage(ctx, messageID)
	if err != nil {
		return Message{}, err
	}
	if err := core.EnsureOwnership(msg.AccountID, accountID, "message", messageID); err != nil {
		return Message{}, err
	}
	return msg, nil
}

// ListMessages lists messages for an account.
func (s *Service) ListMessages(ctx context.Context, accountID string, limit int) ([]Message, error) {
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListMessages(ctx, accountID, clamped)
}

// Descriptor advertises the service placement and capabilities.
func (s *Service) Descriptor() core.Descriptor { return s.Manifest().ToDescriptor() }

func (s *Service) normalizeLane(lane *Lane) error {
	lane.Name = strings.TrimSpace(lane.Name)
	lane.SourceChain = strings.ToLower(strings.TrimSpace(lane.SourceChain))
	lane.DestChain = strings.ToLower(strings.TrimSpace(lane.DestChain))
	lane.SignerSet = core.NormalizeTags(lane.SignerSet)
	lane.Metadata = core.NormalizeMetadata(lane.Metadata)
	lane.Tags = core.NormalizeTags(lane.Tags)
	lane.AllowedTokens = core.NormalizeTags(lane.AllowedTokens)
	lane.DeliveryPolicy = core.CloneAnyMap(lane.DeliveryPolicy)

	if lane.Name == "" {
		return core.RequiredError("name")
	}
	if lane.SourceChain == "" || lane.DestChain == "" {
		return fmt.Errorf("source_chain and dest_chain are required")
	}
	return nil
}

func normalizeTransfers(transfers []TokenTransfer) []TokenTransfer {
	if len(transfers) == 0 {
		return nil
	}
	result := make([]TokenTransfer, 0, len(transfers))
	for _, tr := range transfers {
		token := strings.ToLower(strings.TrimSpace(tr.Token))
		amount := strings.TrimSpace(tr.Amount)
		recipient := strings.TrimSpace(tr.Recipient)
		if token == "" || amount == "" || recipient == "" {
			continue
		}
		result = append(result, TokenTransfer{Token: token, Amount: amount, Recipient: recipient})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func (s *Service) ensureSignersOwned(ctx context.Context, accountID string, signers []string) error {
	if len(signers) == 0 {
		return nil
	}
	if s.wallets == nil {
		return nil
	}
	for _, signer := range signers {
		if err := s.wallets.WalletOwnedBy(ctx, accountID, signer); err != nil {
			return err
		}
	}
	return nil
}

