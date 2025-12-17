// Package neofeeds provides chain push logic for the price feed aggregation service.
package neofeeds

import (
	"context"
	"fmt"
	"math/big"
	"time"
)

// =============================================================================
// Chain Push Logic (Push/Auto-Update Pattern)
// =============================================================================

// pushPricesToChain fetches all configured prices and pushes them on-chain.
func (s *Service) pushPricesToChain(ctx context.Context) {
	if s == nil {
		return
	}

	// Anchor to the platform PriceFeed contract.
	if s.priceFeed == nil || s.priceFeedHash == "" || s.txProxy == nil {
		return
	}

	s.pushPricesToPriceFeed(ctx)
}

// PushSinglePrice pushes a single price update on-chain.
func (s *Service) PushSinglePrice(ctx context.Context, feedID string) error {
	if s.priceFeed == nil || s.priceFeedHash == "" || s.txProxy == nil {
		return fmt.Errorf("chain push not configured")
	}

	latest, err := s.GetPrice(ctx, feedID)
	if err != nil {
		return fmt.Errorf("get price: %w", err)
	}
	if latest == nil {
		return fmt.Errorf("price unavailable for %s", feedID)
	}

	timestampSecs := latest.Timestamp.Unix()
	if timestampSecs < 0 {
		return fmt.Errorf("invalid timestamp for feed %s", feedID)
	}

	sourceSetID := sourceSetIDFromSources(latest.Sources)

	s.publishMu.Lock()
	state := s.publishState[feedID]
	if state == nil {
		state = &pricePublishState{}
		s.publishState[feedID] = state
	}
	next := state.lastRoundID + 1
	if next <= 0 {
		next = 1
	}
	s.publishMu.Unlock()

	if _, err := s.invokePriceFeedUpdate(ctx, feedID, big.NewInt(next), big.NewInt(latest.Price), uint64(timestampSecs), sourceSetID, true); err != nil {
		return err
	}

	s.publishMu.Lock()
	state = s.publishState[feedID]
	if state == nil {
		state = &pricePublishState{}
		s.publishState[feedID] = state
	}
	state.lastRoundID = next
	state.lastPublishedPrice = latest.Price
	state.lastPublishedAt = time.Now()
	state.publishTimes = append(state.publishTimes, state.lastPublishedAt)
	s.publishMu.Unlock()

	return nil
}
