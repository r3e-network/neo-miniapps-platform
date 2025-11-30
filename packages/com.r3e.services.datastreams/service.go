package datastreams

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// Service manages centralized data streams and frames.
type Service struct {
	*framework.ServiceEngine
	store Store
}

// New constructs a data streams service.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	return &Service{
		ServiceEngine: framework.NewServiceEngine(framework.ServiceConfig{
			Name:         "datastreams",
			Description:  "Data stream definitions and frames",
			DependsOn:    []string{"store", "svc-accounts"},
			RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceData},
			Capabilities: []string{"datastreams"},
			Accounts:     accounts,
			Logger:       log,
		}),
		store: store,
	}
}

// Push implements DataEngine for the core engine: publish a frame.
func (s *Service) Push(ctx context.Context, topic string, payload any) error {
	if err := s.Ready(ctx); err != nil {
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
	_, err := s.CreateFrame(ctx, "", streamID, int64(time.Now().UnixNano()), framePayload, 0, FrameStatusOK, nil)
	return err
}

// CreateStream registers a stream for an account.
func (s *Service) CreateStream(ctx context.Context, stream Stream) (Stream, error) {
	accountID, err := s.ValidateAccount(ctx, stream.AccountID)
	if err != nil {
		return Stream{}, err
	}
	stream.AccountID = accountID
	if err := s.normalizeStream(&stream); err != nil {
		return Stream{}, err
	}

	finish := s.StartObservation(ctx, map[string]string{"account_id": stream.AccountID, "stream_id": stream.ID})
	created, err := s.store.CreateStream(ctx, stream)
	finish(err)
	if err != nil {
		return Stream{}, err
	}

	s.LogCreated("stream", created.ID, created.AccountID)
	return created, nil
}

// UpdateStream mutates an existing stream.
func (s *Service) UpdateStream(ctx context.Context, stream Stream) (Stream, error) {
	stored, err := s.store.GetStream(ctx, stream.ID)
	if err != nil {
		return Stream{}, err
	}
	if err := s.ValidateOwnership(stored.AccountID, stream.AccountID, "stream", stream.ID); err != nil {
		return Stream{}, err
	}
	stream.AccountID = stored.AccountID
	if err := s.normalizeStream(&stream); err != nil {
		return Stream{}, err
	}

	finish := s.StartObservation(ctx, map[string]string{"account_id": stream.AccountID, "stream_id": stream.ID})
	updated, err := s.store.UpdateStream(ctx, stream)
	finish(err)
	if err != nil {
		return Stream{}, err
	}

	s.LogUpdated("stream", stream.ID, stream.AccountID)
	return updated, nil
}

// GetStream fetches a stream ensuring ownership.
func (s *Service) GetStream(ctx context.Context, accountID, streamID string) (Stream, error) {
	stream, err := s.store.GetStream(ctx, streamID)
	if err != nil {
		return Stream{}, err
	}
	if err := s.ValidateOwnership(stream.AccountID, accountID, "stream", streamID); err != nil {
		return Stream{}, err
	}
	return stream, nil
}

// ListStreams lists account streams.
func (s *Service) ListStreams(ctx context.Context, accountID string) ([]Stream, error) {
	accountID, err := s.ValidateAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return s.store.ListStreams(ctx, accountID)
}

// CreateFrame ingests a stream frame.
func (s *Service) CreateFrame(ctx context.Context, accountID, streamID string, seq int64, payload map[string]any, latencyMS int, status FrameStatus, metadata map[string]string) (Frame, error) {
	stream, err := s.GetStream(ctx, accountID, streamID)
	if err != nil {
		return Frame{}, err
	}
	if seq <= 0 {
		return Frame{}, fmt.Errorf("sequence must be positive")
	}
	if latencyMS < 0 {
		latencyMS = 0
	}
	if status == "" {
		status = FrameStatusOK
	}

	frame := Frame{
		AccountID: accountID,
		StreamID:  streamID,
		Sequence:  seq,
		Payload:   payload,
		LatencyMS: latencyMS,
		Status:    status,
		Metadata:  s.NormalizeMetadata(metadata),
	}

	finish := s.StartObservation(ctx, map[string]string{"stream_id": stream.ID})
	created, err := s.store.CreateFrame(ctx, frame)
	finish(err)
	if err != nil {
		return Frame{}, err
	}

	s.Logger().WithField("stream_id", stream.ID).WithField("sequence", seq).Info("data stream frame recorded")
	return created, nil
}

// ListFrames lists recent frames.
func (s *Service) ListFrames(ctx context.Context, accountID, streamID string, limit int) ([]Frame, error) {
	if _, err := s.GetStream(ctx, accountID, streamID); err != nil {
		return nil, err
	}
	return s.store.ListFrames(ctx, streamID, s.ClampLimit(limit))
}

// LatestFrame returns the newest frame.
func (s *Service) LatestFrame(ctx context.Context, accountID, streamID string) (Frame, error) {
	if _, err := s.GetStream(ctx, accountID, streamID); err != nil {
		return Frame{}, err
	}
	return s.store.GetLatestFrame(ctx, streamID)
}

func (s *Service) normalizeStream(stream *Stream) error {
	stream.Name = strings.TrimSpace(stream.Name)
	stream.Symbol = strings.ToUpper(strings.TrimSpace(stream.Symbol))
	stream.Description = strings.TrimSpace(stream.Description)
	stream.Frequency = strings.TrimSpace(stream.Frequency)
	stream.Metadata = s.NormalizeMetadata(stream.Metadata)

	if stream.Name == "" {
		return core.RequiredError("name")
	}
	if stream.Symbol == "" {
		return core.RequiredError("symbol")
	}
	if stream.SLAms < 0 {
		stream.SLAms = 0
	}

	status := StreamStatus(strings.ToLower(strings.TrimSpace(string(stream.Status))))
	if status == "" {
		status = StreamStatusInactive
	}
	switch status {
	case StreamStatusInactive, StreamStatusActive, StreamStatusPaused:
		stream.Status = status
	default:
		return fmt.Errorf("invalid status %s", status)
	}
	return nil
}
