package neorand

import (
	"context"
	"encoding/hex"
	"log"
	"strconv"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	vrfchain "github.com/R3E-Network/service_layer/services/neorand/chain"
	"github.com/google/uuid"
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
	listener.On("NeoRandRequest", func(event *chain.ContractEvent) error {
		parsed, err := vrfchain.ParseVRFRequestEvent(event)
		if err != nil {
			return err
		}
		s.handleVRFRequestEvent(ctx, parsed)
		return nil
	})

	go listener.Start(ctx)
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
	request := &NeoRandRequest{
		ID:               uuid.New().String(),
		RequestID:        reqID,
		UserID:           "", // not provided by event; can be mapped via gateway if needed
		RequesterAddress: event.UserContract,
		Seed:             hex.EncodeToString(event.Seed),
		NumWords:         int(event.NumWords),
		CallbackGasLimit: 100000,
		Status:           StatusPending,
		CreatedAt:        time.Now(),
	}

	if s.repo != nil {
		if err := s.repo.Create(ctx, neorandRecordFromReq(request)); err != nil {
			log.Printf("[neorand] failed to persist VRF request %s: %v", request.RequestID, err)
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
