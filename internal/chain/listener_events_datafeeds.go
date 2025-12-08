package chain

import "fmt"

// =============================================================================
// DataFeeds Service Events (Push/Auto-Update Pattern)
// =============================================================================
// Note: DataFeeds uses push pattern - TEE periodically updates prices on-chain.
// No user request events - TEE proactively pushes price updates.

// DataFeedsPriceUpdatedEvent represents a PriceUpdated event from DataFeedsService.
// Event: PriceUpdated(feedId, price, decimals, timestamp)
type DataFeedsPriceUpdatedEvent struct {
	FeedID    string
	Price     uint64
	Decimals  uint64
	Timestamp uint64
}

// ParseDataFeedsPriceUpdatedEvent parses a PriceUpdated event.
func ParseDataFeedsPriceUpdatedEvent(event *ContractEvent) (*DataFeedsPriceUpdatedEvent, error) {
	if event.EventName != "PriceUpdated" {
		return nil, fmt.Errorf("not a PriceUpdated event")
	}
	if len(event.State) < 4 {
		return nil, fmt.Errorf("invalid event state: expected 4 items, got %d", len(event.State))
	}

	feedID, err := parseStringFromItem(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse feedId: %w", err)
	}

	price, err := parseInteger(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse price: %w", err)
	}

	decimals, err := parseInteger(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse decimals: %w", err)
	}

	timestamp, err := parseInteger(event.State[3])
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}

	return &DataFeedsPriceUpdatedEvent{
		FeedID:    feedID,
		Price:     price.Uint64(),
		Decimals:  decimals.Uint64(),
		Timestamp: timestamp.Uint64(),
	}, nil
}

// DataFeedsFeedRegisteredEvent represents a FeedRegistered event.
// Event: FeedRegistered(feedId, description, decimals)
type DataFeedsFeedRegisteredEvent struct {
	FeedID      string
	Description string
	Decimals    uint64
}

// ParseDataFeedsFeedRegisteredEvent parses a FeedRegistered event.
func ParseDataFeedsFeedRegisteredEvent(event *ContractEvent) (*DataFeedsFeedRegisteredEvent, error) {
	if event.EventName != "FeedRegistered" {
		return nil, fmt.Errorf("not a FeedRegistered event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state: expected 3 items, got %d", len(event.State))
	}

	feedID, err := parseStringFromItem(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse feedId: %w", err)
	}

	description, err := parseStringFromItem(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse description: %w", err)
	}

	decimals, err := parseInteger(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse decimals: %w", err)
	}

	return &DataFeedsFeedRegisteredEvent{
		FeedID:      feedID,
		Description: description,
		Decimals:    decimals.Uint64(),
	}, nil
}

// DataFeedsFeedDeactivatedEvent represents a FeedDeactivated event.
// Event: FeedDeactivated(feedId)
type DataFeedsFeedDeactivatedEvent struct {
	FeedID string
}

// ParseDataFeedsFeedDeactivatedEvent parses a FeedDeactivated event.
func ParseDataFeedsFeedDeactivatedEvent(event *ContractEvent) (*DataFeedsFeedDeactivatedEvent, error) {
	if event.EventName != "FeedDeactivated" {
		return nil, fmt.Errorf("not a FeedDeactivated event")
	}
	if len(event.State) < 1 {
		return nil, fmt.Errorf("invalid event state: expected 1 item, got %d", len(event.State))
	}

	feedID, err := parseStringFromItem(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse feedId: %w", err)
	}

	return &DataFeedsFeedDeactivatedEvent{
		FeedID: feedID,
	}, nil
}
