// Package neofeeds provides types for the price feed aggregation service.
package neofeeds

import "time"

// =============================================================================
// Request/Response Types
// =============================================================================

// PriceSource defines a price data source (legacy, use SourceConfig instead).
type PriceSource struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	JSONPath string `json:"json_path"`
	Weight   int    `json:"weight"`
}

// PriceResponse represents a price response.
type PriceResponse struct {
	FeedID    string    `json:"feed_id"`
	Pair      string    `json:"pair"`
	Price     int64     `json:"price"`
	Decimals  int       `json:"decimals"`
	Timestamp time.Time `json:"timestamp"`
	Sources   []string  `json:"sources"`
	Signature []byte    `json:"signature,omitempty"`
	PublicKey []byte    `json:"public_key,omitempty"`
}

// FeedSummary represents a feed entry returned by GET /feeds.
type FeedSummary struct {
	ID         string `json:"id"`
	Pair       string `json:"pair"` // Canonical symbol (same as ID for price feeds).
	SourcePair string `json:"source_pair,omitempty"`
	Enabled    bool   `json:"enabled"`
	Decimals   int    `json:"decimals"`
}

// SourceSummary represents a configured source returned by GET /sources.
type SourceSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}
