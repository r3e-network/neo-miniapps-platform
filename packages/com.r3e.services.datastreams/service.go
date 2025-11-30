package datastreams

import (
	"github.com/R3E-Network/service_layer/domain/account"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/pkg/storage"
	domainds "github.com/R3E-Network/service_layer/domain/datastreams"
	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Compile-time check: Service exposes Push for the core engine adapter.
type dataPusher interface {
	Push(context.Context, string, any) error
}

var _ dataPusher = (*Service)(nil)

// ReadyChecker is satisfied via Ready(ctx) below; the adapter will surface readiness.

// Service manages centralized data streams and frames.
type Service struct {
	framework.ServiceBase
	base  *core.Base
	store storage.DataStreamStore
	log   *logger.Logger
	hooks core.ObservationHooks
}

// Name returns the stable engine module name.
func (s *Service) Name() string { return "datastreams" }

// Domain reports the service domain for engine grouping.
func (s *Service) Domain() string { return "datastreams" }

// Manifest describes the service contract for the engine OS.
func (s *Service) Manifest() *framework.Manifest {
	return &framework.Manifest{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Description:  "Data stream definitions and frames",
		Layer:        "service",
		DependsOn:    []string{"store", "svc-accounts"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceData},
		Capabilities: []string{"datastreams"},
	}
}

// Descriptor advertises the service for system discovery.
func (s *Service) Descriptor() core.Descriptor { return s.Manifest().ToDescriptor() }

// New constructs a data streams service.
func New(accounts storage.AccountStore, store storage.DataStreamStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("datastreams")
	}
	svc := &Service{base: core.NewBaseFromStore[account.Account](accounts), store: store, log: log, hooks: core.NoopObservationHooks}
	svc.SetName(svc.Name())
	return svc
}

// Start/Stop/Ready are inherited from framework.ServiceBase.

// Push implements DataEngine for the core engine: publish a frame.
func (s *Service) Push(ctx context.Context, topic string, payload any) error {
	if err := s.ServiceBase.Ready(ctx); err != nil {
		return err
	}
	streamID := strings.TrimSpace(topic)
	if streamID == "" {
		return fmt.Errorf("stream ID required for push")
	}
	framePayload, ok := payload.(map[string]any)
	if !ok {
		return fmt.Errorf("payload must be a map")
	}
	frame := domainds.Frame{
		StreamID: streamID,
		Sequence: int64(time.Now().UnixNano()),
		Payload:  framePayload,
		Status:   "delivered",
	}
	_, err := s.CreateFrame(ctx, "", streamID, frame.Sequence, framePayload, 0, domainds.FrameStatusOK, nil)
	return err
}

// WithObservationHooks configures callbacks for frame ingestion observability.
func (s *Service) WithObservationHooks(h core.ObservationHooks) {
	if h.OnStart == nil && h.OnComplete == nil {
		s.hooks = core.NoopObservationHooks
		return
	}
	s.hooks = h
}

// CreateStream registers a stream for an account.
func (s *Service) CreateStream(ctx context.Context, stream domainds.Stream) (domainds.Stream, error) {
	if err := s.base.EnsureAccount(ctx, stream.AccountID); err != nil {
		return domainds.Stream{}, err
	}
	if err := s.normalizeStream(&stream); err != nil {
		return domainds.Stream{}, err
	}
	if s.Ready(ctx) == nil {
		start := time.Now()
		s.hooks.OnStart(ctx, map[string]string{"account_id": stream.AccountID, "stream_id": stream.ID})
		defer func() {
			s.hooks.OnComplete(ctx, map[string]string{"account_id": stream.AccountID, "stream_id": stream.ID}, nil, time.Since(start))
		}()
	}
	created, err := s.store.CreateStream(ctx, stream)
	if err != nil {
		return domainds.Stream{}, err
	}
	s.log.WithField("stream_id", created.ID).WithField("account_id", created.AccountID).Info("data stream created")
	return created, nil
}

// UpdateStream mutates an existing stream.
func (s *Service) UpdateStream(ctx context.Context, stream domainds.Stream) (domainds.Stream, error) {
	stored, err := s.store.GetStream(ctx, stream.ID)
	if err != nil {
		return domainds.Stream{}, err
	}
	if err := core.EnsureOwnership(stored.AccountID, stream.AccountID, "stream", stream.ID); err != nil {
		return domainds.Stream{}, err
	}
	stream.AccountID = stored.AccountID
	if err := s.normalizeStream(&stream); err != nil {
		return domainds.Stream{}, err
	}
	var start time.Time
	if s.Ready(ctx) == nil {
		start = time.Now()
		s.hooks.OnStart(ctx, map[string]string{"account_id": stream.AccountID, "stream_id": stream.ID})
		defer func() {
			s.hooks.OnComplete(ctx, map[string]string{"account_id": stream.AccountID, "stream_id": stream.ID}, nil, time.Since(start))
		}()
	}
	updated, err := s.store.UpdateStream(ctx, stream)
	if err != nil {
		return domainds.Stream{}, err
	}
	s.log.WithField("stream_id", stream.ID).WithField("account_id", stream.AccountID).Info("data stream updated")
	return updated, nil
}

// GetStream fetches a stream ensuring ownership.
func (s *Service) GetStream(ctx context.Context, accountID, streamID string) (domainds.Stream, error) {
	stream, err := s.store.GetStream(ctx, streamID)
	if err != nil {
		return domainds.Stream{}, err
	}
	if err := core.EnsureOwnership(stream.AccountID, accountID, "stream", streamID); err != nil {
		return domainds.Stream{}, err
	}
	return stream, nil
}

// ListStreams lists account streams.
func (s *Service) ListStreams(ctx context.Context, accountID string) ([]domainds.Stream, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListStreams(ctx, accountID)
}

// CreateFrame ingests a stream frame.
func (s *Service) CreateFrame(ctx context.Context, accountID, streamID string, seq int64, payload map[string]any, latencyMS int, status domainds.FrameStatus, metadata map[string]string) (domainds.Frame, error) {
	stream, err := s.GetStream(ctx, accountID, streamID)
	if err != nil {
		return domainds.Frame{}, err
	}
	if seq <= 0 {
		return domainds.Frame{}, fmt.Errorf("sequence must be positive")
	}
	if latencyMS < 0 {
		latencyMS = 0
	}
	if status == "" {
		status = domainds.FrameStatusOK
	}
	frame := domainds.Frame{
		AccountID: accountID,
		StreamID:  streamID,
		Sequence:  seq,
		Payload:   payload,
		LatencyMS: latencyMS,
		Status:    status,
		Metadata:  core.NormalizeMetadata(metadata),
	}
	attrs := map[string]string{"stream_id": stream.ID}
	finish := core.StartObservation(ctx, s.hooks, attrs)
	created, err := s.store.CreateFrame(ctx, frame)
	if err != nil {
		finish(err)
		return domainds.Frame{}, err
	}
	finish(nil)
	s.log.WithField("stream_id", stream.ID).WithField("sequence", seq).Info("data stream frame recorded")
	return created, nil
}

// ListFrames lists recent frames.
func (s *Service) ListFrames(ctx context.Context, accountID, streamID string, limit int) ([]domainds.Frame, error) {
	if _, err := s.GetStream(ctx, accountID, streamID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListFrames(ctx, streamID, clamped)
}

// LatestFrame returns the newest frame.
func (s *Service) LatestFrame(ctx context.Context, accountID, streamID string) (domainds.Frame, error) {
	if _, err := s.GetStream(ctx, accountID, streamID); err != nil {
		return domainds.Frame{}, err
	}
	return s.store.GetLatestFrame(ctx, streamID)
}

func (s *Service) normalizeStream(stream *domainds.Stream) error {
	stream.Name = strings.TrimSpace(stream.Name)
	stream.Symbol = strings.ToUpper(strings.TrimSpace(stream.Symbol))
	stream.Description = strings.TrimSpace(stream.Description)
	stream.Frequency = strings.TrimSpace(stream.Frequency)
	stream.Metadata = core.NormalizeMetadata(stream.Metadata)
	if stream.Name == "" {
		return core.RequiredError("name")
	}
	if stream.Symbol == "" {
		return core.RequiredError("symbol")
	}
	if stream.SLAms < 0 {
		stream.SLAms = 0
	}
	status := domainds.StreamStatus(strings.ToLower(strings.TrimSpace(string(stream.Status))))
	if status == "" {
		status = domainds.StreamStatusInactive
	}
	switch status {
	case domainds.StreamStatusInactive, domainds.StreamStatusActive, domainds.StreamStatusPaused:
		stream.Status = status
	default:
		return fmt.Errorf("invalid status %s", status)
	}
	return nil
}
