package datastreams

import (
	"context"
	"fmt"
	"strings"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	domainds "github.com/R3E-Network/service_layer/internal/app/domain/datastreams"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service manages centralized data streams and frames.
type Service struct {
	base  *core.Base
	store storage.DataStreamStore
	log   *logger.Logger
	hooks core.ObservationHooks
}

// New constructs a data streams service.
func New(accounts storage.AccountStore, store storage.DataStreamStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("datastreams")
	}
	return &Service{base: core.NewBase(accounts), store: store, log: log, hooks: core.NoopObservationHooks}
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
	if stored.AccountID != stream.AccountID {
		return domainds.Stream{}, fmt.Errorf("stream %s does not belong to account %s", stream.ID, stream.AccountID)
	}
	stream.AccountID = stored.AccountID
	if err := s.normalizeStream(&stream); err != nil {
		return domainds.Stream{}, err
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
	if stream.AccountID != accountID {
		return domainds.Stream{}, fmt.Errorf("stream %s does not belong to account %s", streamID, accountID)
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

// Descriptor advertises the service placement and capabilities.
func (s *Service) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "datastreams",
		Domain:       "datastreams",
		Layer:        core.LayerPlatform,
		Capabilities: []string{"streams", "frames", "ingest"},
	}
}

func (s *Service) normalizeStream(stream *domainds.Stream) error {
	stream.Name = strings.TrimSpace(stream.Name)
	stream.Symbol = strings.ToUpper(strings.TrimSpace(stream.Symbol))
	stream.Description = strings.TrimSpace(stream.Description)
	stream.Frequency = strings.TrimSpace(stream.Frequency)
	stream.Metadata = core.NormalizeMetadata(stream.Metadata)
	if stream.Name == "" {
		return fmt.Errorf("name is required")
	}
	if stream.Symbol == "" {
		return fmt.Errorf("symbol is required")
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
