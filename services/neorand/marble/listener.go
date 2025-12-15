package neorand

import (
	"context"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/internal/chain"
	vrfchain "github.com/R3E-Network/service_layer/services/neorand/chain"
)

// =============================================================================
// Event Listener - Monitors chain for VRF requests
// =============================================================================

// runEventListener registers for NeoRandRequest events via the chain event listener.
func (s *Service) runEventListener(ctx context.Context) {
	if s.eventListener == nil {
		return
	}

	listener := s.eventListener
	listener.On("VRFRequest", func(event *chain.ContractEvent) error {
		parsed, err := vrfchain.ParseVRFRequestEvent(event)
		if err != nil {
			return err
		}
		s.handleVRFRequestEvent(ctx, parsed)
		return nil
	})

	if err := listener.Start(ctx); err != nil {
		s.Logger().WithContext(ctx).WithError(err).Warn("failed to start event listener")
	}
}

// handleVRFRequestEvent processes a single VRF request event.
func (s *Service) handleVRFRequestEvent(ctx context.Context, event *vrfchain.VRFRequestEvent) {
	s.mu.Lock()
	if _, exists := s.requests[strconv.FormatUint(event.RequestID, 10)]; exists {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	reqID := strconv.FormatUint(event.RequestID, 10)

	numWordsUint64 := event.NumWords
	if numWordsUint64 == 0 {
		numWordsUint64 = 1
	}
	if numWordsUint64 > uint64(MaxNumWords) {
		s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
			"request_id": reqID,
			"num_words":  numWordsUint64,
			"max_words":  MaxNumWords,
		}).Warn("ignoring VRF request: numWords exceeds max")
		return
	}
	numWords := int(numWordsUint64)

	request := &Request{
		ID:               uuid.New().String(),
		RequestID:        reqID,
		UserID:           "", // not provided by event; can be mapped via gateway if needed
		RequesterAddress: event.UserContract,
		Seed:             hex.EncodeToString(event.Seed),
		NumWords:         numWords,
		CallbackGasLimit: 100000,
		Status:           StatusPending,
		CreatedAt:        time.Now(),
	}

	if s.repo != nil {
		if err := s.repo.Create(ctx, neorandRecordFromReq(request)); err != nil {
			s.Logger().WithContext(ctx).WithError(err).WithField("request_id", request.RequestID).Warn("failed to persist VRF request")
		}
	}

	s.mu.Lock()
	s.requests[request.RequestID] = request
	s.mu.Unlock()

	select {
	case s.pendingRequests <- request:
	default:
	}
}
