package datastreams

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
	"github.com/R3E-Network/service_layer/system/sandbox"
)

// Compile-time check: Service exposes Push for the core engine adapter.
type dataPusher interface {
	Push(context.Context, string, any) error
}

var _ dataPusher = (*Service)(nil)

// Service manages centralized data streams and frames.
type Service struct {
	*framework.SandboxedServiceEngine
	store Store
}

// New constructs a data streams service.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	return &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:         "datastreams",
				Description:  "Data stream definitions and frames",
				DependsOn:    []string{"store", "svc-accounts"},
				RequiresAPIs: []framework.APISurface{framework.APISurfaceStore, framework.APISurfaceData},
				Capabilities: []string{"datastreams"},
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
			StorageQuota: 20 * 1024 * 1024,
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

	ctx, finish := s.StartObservation(ctx, map[string]string{"account_id": stream.AccountID, "stream_id": stream.ID})
	created, err := s.store.CreateStream(ctx, stream)
	finish(err)
	if err != nil {
		return Stream{}, err
	}

	s.LogCreated("stream", created.ID, created.AccountID)
	s.IncrementCounter("datastreams_streams_created_total", map[string]string{"account_id": created.AccountID})
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

	ctx, finish := s.StartObservation(ctx, map[string]string{"account_id": stream.AccountID, "stream_id": stream.ID})
	updated, err := s.store.UpdateStream(ctx, stream)
	finish(err)
	if err != nil {
		return Stream{}, err
	}

	s.LogUpdated("stream", stream.ID, stream.AccountID)
	s.IncrementCounter("datastreams_streams_updated_total", map[string]string{"account_id": stream.AccountID})
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
		Metadata:  core.NormalizeMetadata(metadata),
	}

	ctx, finish := s.StartObservation(ctx, map[string]string{"stream_id": stream.ID})
	created, err := s.store.CreateFrame(ctx, frame)
	finish(err)
	if err != nil {
		return Frame{}, err
	}

	s.Logger().WithField("stream_id", stream.ID).WithField("sequence", seq).Info("data stream frame recorded")
	s.ObserveDuration("datastreams_frame_latency_seconds", map[string]string{"stream_id": stream.ID}, time.Duration(latencyMS)*time.Millisecond)
	s.IncrementCounter("datastreams_frames_created_total", map[string]string{"stream_id": stream.ID})
	dataPayload := map[string]any{
		"stream_id":  stream.ID,
		"sequence":   seq,
		"status":     status,
		"latency_ms": latencyMS,
		"payload":    payload,
	}
	topic := fmt.Sprintf("datastreams/%s", stream.ID)
	if err := s.PushData(ctx, topic, dataPayload); err != nil {
		if errors.Is(err, core.ErrBusUnavailable) {
			s.Logger().WithError(err).Warn("bus unavailable for datastreams push")
		} else {
			return Frame{}, fmt.Errorf("push datastream frame: %w", err)
		}
	}
	return created, nil
}

// ListFrames lists recent frames.
func (s *Service) ListFrames(ctx context.Context, accountID, streamID string, limit int) ([]Frame, error) {
	if _, err := s.GetStream(ctx, accountID, streamID); err != nil {
		return nil, err
	}
	attrs := map[string]string{"account_id": accountID, "stream_id": streamID, "resource": "datastreams_frames"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	return s.store.ListFrames(ctx, streamID, s.ClampLimit(limit))
}

// LatestFrame returns the newest frame.
func (s *Service) LatestFrame(ctx context.Context, accountID, streamID string) (Frame, error) {
	if _, err := s.GetStream(ctx, accountID, streamID); err != nil {
		return Frame{}, err
	}
	attrs := map[string]string{"account_id": accountID, "stream_id": streamID, "resource": "datastreams_latest_frame"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	return s.store.GetLatestFrame(ctx, streamID)
}

func (s *Service) normalizeStream(stream *Stream) error {
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

// HTTP API Methods
// These methods follow the HTTP{Method}{Path} naming convention for automatic route discovery.

// HTTPGetStreams handles GET /streams - list all streams for an account.
func (s *Service) HTTPGetStreams(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListStreams(ctx, req.AccountID)
}

// HTTPPostStreams handles POST /streams - create a new stream.
func (s *Service) HTTPPostStreams(ctx context.Context, req core.APIRequest) (any, error) {
	name, _ := req.Body["name"].(string)
	symbol, _ := req.Body["symbol"].(string)
	description, _ := req.Body["description"].(string)
	frequency, _ := req.Body["frequency"].(string)
	status, _ := req.Body["status"].(string)
	slaMS := 0
	if s, ok := req.Body["sla_ms"].(float64); ok {
		slaMS = int(s)
	}

	metadata := core.ExtractMetadataRaw(req.Body, "")

	stream := Stream{
		AccountID:   req.AccountID,
		Name:        name,
		Symbol:      symbol,
		Description: description,
		Frequency:   frequency,
		Status:      StreamStatus(status),
		SLAms:       slaMS,
		Metadata:    metadata,
	}

	return s.CreateStream(ctx, stream)
}

// HTTPGetStreamsById handles GET /streams/{id} - get a specific stream.
func (s *Service) HTTPGetStreamsById(ctx context.Context, req core.APIRequest) (any, error) {
	streamID := req.PathParams["id"]
	return s.GetStream(ctx, req.AccountID, streamID)
}

// HTTPPatchStreamsById handles PATCH /streams/{id} - update a stream.
func (s *Service) HTTPPatchStreamsById(ctx context.Context, req core.APIRequest) (any, error) {
	streamID := req.PathParams["id"]

	// Get existing stream first
	existing, err := s.GetStream(ctx, req.AccountID, streamID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if name, ok := req.Body["name"].(string); ok {
		existing.Name = name
	}
	if symbol, ok := req.Body["symbol"].(string); ok {
		existing.Symbol = symbol
	}
	if description, ok := req.Body["description"].(string); ok {
		existing.Description = description
	}
	if frequency, ok := req.Body["frequency"].(string); ok {
		existing.Frequency = frequency
	}
	if status, ok := req.Body["status"].(string); ok {
		existing.Status = StreamStatus(status)
	}
	if slaMS, ok := req.Body["sla_ms"].(float64); ok {
		existing.SLAms = int(slaMS)
	}

	existing.AccountID = req.AccountID
	return s.UpdateStream(ctx, existing)
}

// HTTPGetStreamsIdFrames handles GET /streams/{id}/frames - list frames for a stream.
func (s *Service) HTTPGetStreamsIdFrames(ctx context.Context, req core.APIRequest) (any, error) {
	streamID := req.PathParams["id"]
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListFrames(ctx, req.AccountID, streamID, limit)
}

// HTTPPostStreamsIdFrames handles POST /streams/{id}/frames - create a new frame.
func (s *Service) HTTPPostStreamsIdFrames(ctx context.Context, req core.APIRequest) (any, error) {
	streamID := req.PathParams["id"]
	seq := int64(time.Now().UnixNano())
	if s, ok := req.Body["sequence"].(float64); ok {
		seq = int64(s)
	}
	latencyMS := 0
	if l, ok := req.Body["latency_ms"].(float64); ok {
		latencyMS = int(l)
	}
	status := FrameStatusOK
	if st, ok := req.Body["status"].(string); ok {
		status = FrameStatus(st)
	}

	var payload map[string]any
	if p, ok := req.Body["payload"].(map[string]any); ok {
		payload = p
	}

	metadata := core.ExtractMetadataRaw(req.Body, "")

	return s.CreateFrame(ctx, req.AccountID, streamID, seq, payload, latencyMS, status, metadata)
}

// HTTPGetStreamsIdLatest handles GET /streams/{id}/latest - get latest frame.
func (s *Service) HTTPGetStreamsIdLatest(ctx context.Context, req core.APIRequest) (any, error) {
	streamID := req.PathParams["id"]
	return s.LatestFrame(ctx, req.AccountID, streamID)
}
