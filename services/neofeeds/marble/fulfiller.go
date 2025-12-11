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

// DefaultFeeds defines the default price feeds (for backward compatibility).
var DefaultFeeds = []string{
	"BTC/USD",
	"ETH/USD",
	"NEO/USD",
	"GAS/USD",
	"NEO/GAS",
}

// runChainPushLoop periodically fetches prices and pushes them on-chain.
func (s *Service) runChainPushLoop(ctx context.Context) {
	ticker := time.NewTicker(s.updateInterval)
	defer ticker.Stop()

	// Initial push
	s.pushPricesToChain(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.StopChan():
			return
		case <-ticker.C:
			s.pushPricesToChain(ctx)
		}
	}
}

// pushPricesToChain fetches all configured prices and pushes them on-chain.
func (s *Service) pushPricesToChain(ctx context.Context) {
	enabledFeeds := s.GetEnabledFeeds()
	if len(enabledFeeds) == 0 {
		return
	}

	feedIDs := make([]string, 0, len(enabledFeeds))
	prices := make([]*big.Int, 0, len(enabledFeeds))
	timestamps := make([]uint64, 0, len(enabledFeeds))

	for _, feed := range enabledFeeds {
		pair := feed.Pair
		if pair == "" {
			pair = feedIDToPair(feed.ID)
		}

		price, err := s.GetPrice(ctx, pair)
		if err != nil {
			continue
		}

		feedIDs = append(feedIDs, feed.ID)
		prices = append(prices, big.NewInt(price.Price))
		timestamps = append(timestamps, uint64(price.Timestamp.UnixMilli()))
	}

	if len(feedIDs) == 0 {
		return
	}

	_, _ = s.teeFulfiller.UpdatePrices(ctx, s.neoFeedsHash, feedIDs, prices, timestamps)
}

// PushSinglePrice pushes a single price update on-chain.
func (s *Service) PushSinglePrice(ctx context.Context, feedID string) error {
	if s.teeFulfiller == nil || s.neoFeedsHash == "" {
		return fmt.Errorf("chain push not configured")
	}

	pair := feedIDToPair(feedID)
	price, err := s.GetPrice(ctx, pair)
	if err != nil {
		return fmt.Errorf("get price: %w", err)
	}

	_, err = s.teeFulfiller.UpdatePrice(
		ctx,
		s.neoFeedsHash,
		feedID,
		big.NewInt(price.Price),
		uint64(price.Timestamp.UnixMilli()),
	)
	return err
}

// feedIDToPair converts a feed ID to a trading pair format.
// e.g., "BTC/USD" -> "BTCUSD"
func feedIDToPair(feedID string) string {
	pair := ""
	for _, c := range feedID {
		if c != '/' {
			pair += string(c)
		}
	}
	return pair
}
