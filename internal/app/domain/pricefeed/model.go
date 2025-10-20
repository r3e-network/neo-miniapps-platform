package pricefeed

import "time"

// Feed represents a configured price feed definition.
type Feed struct {
	ID               string
	AccountID        string
	BaseAsset        string
	QuoteAsset       string
	Pair             string
	UpdateInterval   string
	DeviationPercent float64
	Heartbeat        string
	Active           bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Snapshot captures a recorded price for a feed.
type Snapshot struct {
	ID          string
	FeedID      string
	Price       float64
	Source      string
	CollectedAt time.Time
	CreatedAt   time.Time
}
