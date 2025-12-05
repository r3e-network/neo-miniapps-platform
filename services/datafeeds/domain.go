// Package datafeeds provides data feed aggregation service.
package datafeeds

import (
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// PriceData represents price data from a feed.
type PriceData struct {
	base.BaseEntity
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Volume    float64   `json:"volume,omitempty"`
	High24h   float64   `json:"high_24h,omitempty"`
	Low24h    float64   `json:"low_24h,omitempty"`
	Change24h float64   `json:"change_24h,omitempty"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
	Signature []byte    `json:"signature,omitempty"`
}

// FeedStatus represents feed status.
type FeedStatus string

const (
	FeedStatusActive   FeedStatus = "active"
	FeedStatusPaused   FeedStatus = "paused"
	FeedStatusDisabled FeedStatus = "disabled"
)

// Feed represents a data feed configuration.
type Feed struct {
	base.BaseEntity
	Name        string        `json:"name"`
	Symbol      string        `json:"symbol"`
	Sources     []string      `json:"sources"`
	Interval    time.Duration `json:"interval"`
	Status      FeedStatus    `json:"status"`
	LastFetched time.Time     `json:"last_fetched"`
	ErrorCount  int           `json:"error_count"`
}

// AggregatedPrice represents aggregated price from multiple sources.
type AggregatedPrice struct {
	Symbol     string      `json:"symbol"`
	Price      float64     `json:"price"`
	Sources    []string    `json:"sources"`
	Confidence float64     `json:"confidence"`
	Timestamp  time.Time   `json:"timestamp"`
	Signature  []byte      `json:"signature,omitempty"`
}
