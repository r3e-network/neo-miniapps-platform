// Package datafeeds provides types for the price feed aggregation service.
package datafeedsmarble

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
