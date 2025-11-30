package datastreams

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/pkg/testutil"
	"github.com/google/uuid"
)

// Re-export centralized mock for convenience.
type MockAccountChecker = testutil.MockAccountChecker

// NewMockAccountChecker creates a new mock account checker.
var NewMockAccountChecker = testutil.NewMockAccountChecker

// MemoryStore is an in-memory implementation of Store for testing.
type MemoryStore struct {
	mu      sync.RWMutex
	streams map[string]Stream
	frames  map[string]Frame
}

// NewMemoryStore creates a new in-memory store for testing.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		streams: make(map[string]Stream),
		frames:  make(map[string]Frame),
	}
}

// CreateStream creates a new stream.
func (s *MemoryStore) CreateStream(_ context.Context, stream Stream) (Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if stream.ID == "" {
		stream.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	stream.CreatedAt = now
	stream.UpdatedAt = now

	s.streams[stream.ID] = stream
	return stream, nil
}

// UpdateStream updates an existing stream.
func (s *MemoryStore) UpdateStream(_ context.Context, stream Stream) (Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.streams[stream.ID]
	if !ok {
		return Stream{}, fmt.Errorf("stream not found: %s", stream.ID)
	}

	stream.CreatedAt = existing.CreatedAt
	stream.UpdatedAt = time.Now().UTC()
	s.streams[stream.ID] = stream
	return stream, nil
}

// GetStream retrieves a stream by ID.
func (s *MemoryStore) GetStream(_ context.Context, id string) (Stream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stream, ok := s.streams[id]
	if !ok {
		return Stream{}, fmt.Errorf("stream not found: %s", id)
	}
	return stream, nil
}

// ListStreams returns all streams for an account.
func (s *MemoryStore) ListStreams(_ context.Context, accountID string) ([]Stream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Stream
	for _, stream := range s.streams {
		if stream.AccountID == accountID {
			result = append(result, stream)
		}
	}
	return result, nil
}

// CreateFrame creates a new frame.
func (s *MemoryStore) CreateFrame(_ context.Context, frame Frame) (Frame, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if frame.ID == "" {
		frame.ID = uuid.NewString()
	}
	frame.CreatedAt = time.Now().UTC()

	s.frames[frame.ID] = frame
	return frame, nil
}

// ListFrames returns frames for a stream.
func (s *MemoryStore) ListFrames(_ context.Context, streamID string, limit int) ([]Frame, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Frame
	for _, frame := range s.frames {
		if frame.StreamID == streamID {
			result = append(result, frame)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// GetLatestFrame returns the latest frame for a stream.
func (s *MemoryStore) GetLatestFrame(_ context.Context, streamID string) (Frame, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latest Frame
	for _, frame := range s.frames {
		if frame.StreamID == streamID {
			if latest.ID == "" || frame.Sequence > latest.Sequence {
				latest = frame
			}
		}
	}
	if latest.ID == "" {
		return Frame{}, fmt.Errorf("no frames found for stream: %s", streamID)
	}
	return latest, nil
}

// Compile-time checks
var _ Store = (*MemoryStore)(nil)
var _ AccountChecker = (*MockAccountChecker)(nil)
