package neofeedschain

import (
	"fmt"

	"github.com/R3E-Network/service_layer/internal/chain"
)

// =============================================================================
// NeoFeeds Service Events (Push/Auto-Update Pattern)
// =============================================================================
// Note: NeoFeeds uses push pattern - TEE periodically updates prices on-chain.
// No user request events - TEE proactively pushes price updates.

// NeoFeedsPriceUpdatedEvent represents a PriceUpdated event from NeoFeedsService.
// Event: PriceUpdated(feedId, price, decimals, timestamp)
type NeoFeedsPriceUpdatedEvent struct {
	FeedID    string
	Price     uint64
	Decimals  uint64
	Timestamp uint64
}

// ParseNeoFeedsPriceUpdatedEvent parses a PriceUpdated event.
func ParseNeoFeedsPriceUpdatedEvent(event *chain.ContractEvent) (*NeoFeedsPriceUpdatedEvent, error) {
	if event.EventName != "PriceUpdated" {
		return nil, fmt.Errorf("not a PriceUpdated event")
	}
	if len(event.State) < 4 {
		return nil, fmt.Errorf("invalid event state: expected 4 items, got %d", len(event.State))
	}

	feedID, err := chain.ParseStringFromItem(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse feedId: %w", err)
	}

	price, err := chain.ParseInteger(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse price: %w", err)
	}

	decimals, err := chain.ParseInteger(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse decimals: %w", err)
	}

	timestamp, err := chain.ParseInteger(event.State[3])
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}

	return &NeoFeedsPriceUpdatedEvent{
		FeedID:    feedID,
		Price:     price.Uint64(),
		Decimals:  decimals.Uint64(),
		Timestamp: timestamp.Uint64(),
	}, nil
}

// NeoFeedsFeedRegisteredEvent represents a FeedRegistered event.
// Event: FeedRegistered(feedId, description, decimals)
type NeoFeedsFeedRegisteredEvent struct {
	FeedID      string
	Description string
	Decimals    uint64
}

// ParseNeoFeedsFeedRegisteredEvent parses a FeedRegistered event.
func ParseNeoFeedsFeedRegisteredEvent(event *chain.ContractEvent) (*NeoFeedsFeedRegisteredEvent, error) {
	if event.EventName != "FeedRegistered" {
		return nil, fmt.Errorf("not a FeedRegistered event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state: expected 3 items, got %d", len(event.State))
	}

	feedID, err := chain.ParseStringFromItem(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse feedId: %w", err)
	}

	description, err := chain.ParseStringFromItem(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse description: %w", err)
	}

	decimals, err := chain.ParseInteger(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse decimals: %w", err)
	}

	return &NeoFeedsFeedRegisteredEvent{
		FeedID:      feedID,
		Description: description,
		Decimals:    decimals.Uint64(),
	}, nil
}

// NeoFeedsFeedDeactivatedEvent represents a FeedDeactivated event.
// Event: FeedDeactivated(feedId)
type NeoFeedsFeedDeactivatedEvent struct {
	FeedID string
}

// ParseNeoFeedsFeedDeactivatedEvent parses a FeedDeactivated event.
func ParseNeoFeedsFeedDeactivatedEvent(event *chain.ContractEvent) (*NeoFeedsFeedDeactivatedEvent, error) {
	if event.EventName != "FeedDeactivated" {
		return nil, fmt.Errorf("not a FeedDeactivated event")
	}
	if len(event.State) < 1 {
		return nil, fmt.Errorf("invalid event state: expected 1 item, got %d", len(event.State))
	}

	feedID, err := chain.ParseStringFromItem(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse feedId: %w", err)
	}

	return &NeoFeedsFeedDeactivatedEvent{
		FeedID: feedID,
	}, nil
}
