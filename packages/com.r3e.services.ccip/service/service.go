package ccip

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
	
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
	"github.com/R3E-Network/service_layer/system/os/sandbox"
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
	*framework.SandboxedServiceEngine
	wallets      WalletChecker
	store        Store
	dispatcher   Dispatcher
	dispatch     core.DispatchOptions
	customTracer core.Tracer
}

// New creates a CCIP service instance.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	svc := &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:         "ccip",
				Description:  "CCIP lanes and messages",
				Domain:       "ccip",
				DependsOn:    []string{"store", "svc-accounts"},
				RequiresAPIs: []framework.APISurface{framework.APISurfaceStore, framework.APISurfaceEvent},
				Capabilities: []string{"ccip"},
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
		dispatcher: DispatcherFunc(func(context.Context, Message, Lane) error {
			return nil
		}),
		dispatch: core.NewDispatchOptions(),
	}
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
	if t == nil {
		s.customTracer = nil
		t = s.Tracer()
	} else {
		s.customTracer = t
	}
	s.dispatch.SetTracer(t)
}

func (s *Service) SetEnvironment(env framework.Environment) {
	s.SandboxedServiceEngine.SetEnvironment(env)
	tracer := s.customTracer
	if tracer == nil {
		tracer = s.Tracer()
	}
	s.dispatch.SetTracer(tracer)
}

// Start/Stop/Ready/Name/Domain/Manifest/Descriptor are inherited from framework.ServiceEngine.

// CreateLane validates and stores a new lane.
func (s *Service) CreateLane(ctx context.Context, lane Lane) (Lane, error) {
	if err := s.ValidateAccountExists(ctx, lane.AccountID); err != nil {
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
	s.Logger().WithField("lane_id", created.ID).WithField("account_id", created.AccountID).Info("ccip lane created")
	s.IncrementCounter("ccip_lanes_created_total", map[string]string{"account_id": created.AccountID})
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
	s.Logger().WithField("lane_id", lane.ID).WithField("account_id", lane.AccountID).Info("ccip lane updated")
	s.IncrementCounter("ccip_lanes_updated_total", map[string]string{"account_id": lane.AccountID})
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
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListLanes(ctx, accountID)
}

// SendMessage creates a message for a lane.
func (s *Service) SendMessage(ctx context.Context, accountID, laneID string, payload map[string]any, tokens []TokenTransfer, metadata map[string]string, tags []string) (Message, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
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
	s.IncrementCounter("ccip_messages_created_total", map[string]string{"lane_id": lane.ID})
	eventPayload := map[string]any{
		"message_id": created.ID,
		"account_id": accountID,
		"lane_id":    lane.ID,
	}
	if err := s.PublishEvent(ctx, "ccip.message.created", eventPayload); err != nil {
		if errors.Is(err, core.ErrBusUnavailable) {
			s.Logger().WithError(err).Warn("bus unavailable for ccip message event")
		} else {
			return Message{}, fmt.Errorf("publish message event: %w", err)
		}
	}
	attrs := map[string]string{"message_id": created.ID, "lane_id": lane.ID}
	if err := s.dispatch.Run(ctx, "ccip.dispatch", attrs, func(spanCtx context.Context) error {
		if err := s.dispatcher.Dispatch(spanCtx, created, lane); err != nil {
			s.Logger().WithError(err).WithField("message_id", created.ID).Warn("ccip dispatcher error")
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
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListMessages(ctx, accountID, clamped)
}

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

// HTTP API Methods
// These methods follow the HTTP{Method}{Path} naming convention for automatic route discovery.

// HTTPGetLanes handles GET /lanes - list all lanes for an account.
func (s *Service) HTTPGetLanes(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListLanes(ctx, req.AccountID)
}

// HTTPPostLanes handles POST /lanes - create a new lane.
func (s *Service) HTTPPostLanes(ctx context.Context, req core.APIRequest) (any, error) {
	name, _ := req.Body["name"].(string)
	sourceChain, _ := req.Body["source_chain"].(string)
	destChain, _ := req.Body["dest_chain"].(string)

	var signerSet, allowedTokens, tags []string
	if rawSigners, ok := req.Body["signer_set"].([]any); ok {
		for _, s := range rawSigners {
			if str, ok := s.(string); ok {
				signerSet = append(signerSet, str)
			}
		}
	}
	if rawTokens, ok := req.Body["allowed_tokens"].([]any); ok {
		for _, t := range rawTokens {
			if str, ok := t.(string); ok {
				allowedTokens = append(allowedTokens, str)
			}
		}
	}
	if rawTags, ok := req.Body["tags"].([]any); ok {
		for _, t := range rawTags {
			if str, ok := t.(string); ok {
				tags = append(tags, str)
			}
		}
	}

	metadata := core.ExtractMetadataRaw(req.Body, "")

	var deliveryPolicy map[string]any
	if dp, ok := req.Body["delivery_policy"].(map[string]any); ok {
		deliveryPolicy = dp
	}

	lane := Lane{
		AccountID:      req.AccountID,
		Name:           name,
		SourceChain:    sourceChain,
		DestChain:      destChain,
		SignerSet:      signerSet,
		AllowedTokens:  allowedTokens,
		DeliveryPolicy: deliveryPolicy,
		Tags:           tags,
		Metadata:       metadata,
	}

	return s.CreateLane(ctx, lane)
}

// HTTPGetLanesById handles GET /lanes/{id} - get a specific lane.
func (s *Service) HTTPGetLanesById(ctx context.Context, req core.APIRequest) (any, error) {
	laneID := req.PathParams["id"]
	return s.GetLane(ctx, req.AccountID, laneID)
}

// HTTPPatchLanesById handles PATCH /lanes/{id} - update a lane.
func (s *Service) HTTPPatchLanesById(ctx context.Context, req core.APIRequest) (any, error) {
	laneID := req.PathParams["id"]

	// Get existing lane first
	existing, err := s.GetLane(ctx, req.AccountID, laneID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if name, ok := req.Body["name"].(string); ok {
		existing.Name = name
	}
	if sourceChain, ok := req.Body["source_chain"].(string); ok {
		existing.SourceChain = sourceChain
	}
	if destChain, ok := req.Body["dest_chain"].(string); ok {
		existing.DestChain = destChain
	}

	existing.AccountID = req.AccountID
	return s.UpdateLane(ctx, existing)
}

// HTTPGetMessages handles GET /messages - list all messages for an account.
func (s *Service) HTTPGetMessages(ctx context.Context, req core.APIRequest) (any, error) {
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListMessages(ctx, req.AccountID, limit)
}

// HTTPPostMessages handles POST /messages - send a new message.
func (s *Service) HTTPPostMessages(ctx context.Context, req core.APIRequest) (any, error) {
	laneID, _ := req.Body["lane_id"].(string)

	var payload map[string]any
	if p, ok := req.Body["payload"].(map[string]any); ok {
		payload = p
	}

	var tokens []TokenTransfer
	if rawTokens, ok := req.Body["token_transfers"].([]any); ok {
		for _, rt := range rawTokens {
			if t, ok := rt.(map[string]any); ok {
				transfer := TokenTransfer{
					Token:     core.GetString(t, "token"),
					Amount:    core.GetString(t, "amount"),
					Recipient: core.GetString(t, "recipient"),
				}
				tokens = append(tokens, transfer)
			}
		}
	}

	var tags []string
	if rawTags, ok := req.Body["tags"].([]any); ok {
		for _, t := range rawTags {
			if str, ok := t.(string); ok {
				tags = append(tags, str)
			}
		}
	}

	metadata := core.ExtractMetadataRaw(req.Body, "")

	return s.SendMessage(ctx, req.AccountID, laneID, payload, tokens, metadata, tags)
}

// HTTPGetMessagesById handles GET /messages/{id} - get a specific message.
func (s *Service) HTTPGetMessagesById(ctx context.Context, req core.APIRequest) (any, error) {
	messageID := req.PathParams["id"]
	return s.GetMessage(ctx, req.AccountID, messageID)
}
