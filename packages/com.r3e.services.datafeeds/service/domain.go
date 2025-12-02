// Package datafeeds provides the DATAFEEDS Service as a ServicePackage.
package datafeeds

import "time"

// DataType defines the type of data a feed provides.
type DataType string

const (
	DataTypePrice  DataType = "price"  // Numeric price with decimals
	DataTypeNumber DataType = "number" // Generic numeric value
	DataTypeString DataType = "string" // String value
	DataTypeJSON   DataType = "json"   // JSON object
	DataTypeBool   DataType = "bool"   // Boolean value
)

// FeedStatus represents the operational status of a feed.
type FeedStatus string

const (
	FeedStatusActive   FeedStatus = "active"   // Feed is operational
	FeedStatusPaused   FeedStatus = "paused"   // Feed is temporarily paused
	FeedStatusError    FeedStatus = "error"    // Feed has errors
	FeedStatusDisabled FeedStatus = "disabled" // Feed is disabled
)

// SourceConfig defines how to fetch data from an external source.
type SourceConfig struct {
	URL            string            `json:"url"`                        // Source URL
	Method         string            `json:"method,omitempty"`           // HTTP method (GET, POST)
	Headers        map[string]string `json:"headers,omitempty"`          // Custom headers
	Body           string            `json:"body,omitempty"`             // Request body for POST
	AuthType       string            `json:"auth_type,omitempty"`        // none, basic, bearer, api_key
	AuthCredential string            `json:"auth_credential,omitempty"`  // Encrypted credential reference
	JSONPath       string            `json:"json_path,omitempty"`        // JSONPath to extract value
	Timeout        time.Duration     `json:"timeout,omitempty"`          // Request timeout
	RetryCount     int               `json:"retry_count,omitempty"`      // Number of retries
	RetryDelay     time.Duration     `json:"retry_delay,omitempty"`      // Delay between retries
	ValidateSSL    bool              `json:"validate_ssl,omitempty"`     // Validate SSL certificates
	CacheDuration  time.Duration     `json:"cache_duration,omitempty"`   // How long to cache results
}

// Feed describes a data feed configuration with configurable sources.
type Feed struct {
	ID           string            `json:"id"`
	AccountID    string            `json:"account_id"`
	Name         string            `json:"name"`                    // Human-readable name
	Pair         string            `json:"pair,omitempty"`          // Trading pair (for price feeds)
	Description  string            `json:"description"`
	DataType     DataType          `json:"data_type"`               // Type of data
	Decimals     int               `json:"decimals"`                // Decimal places for numeric types
	Heartbeat    time.Duration     `json:"heartbeat"`               // Max time between updates
	ThresholdPPM int               `json:"threshold_ppm"`           // Deviation threshold in PPM
	SignerSet    []string          `json:"signer_set"`              // Authorized signers
	Threshold    int               `json:"threshold"`               // Min signers required
	Aggregation  string            `json:"aggregation,omitempty"`   // median, mean, min, max
	Sources      []SourceConfig    `json:"sources,omitempty"`       // Data sources for enclave fetching
	FetchInterval time.Duration   `json:"fetch_interval,omitempty"` // Auto-fetch interval
	Status       FeedStatus        `json:"status"`                  // Operational status
	LastFetchAt  time.Time         `json:"last_fetch_at,omitempty"` // Last successful fetch
	LastError    string            `json:"last_error,omitempty"`    // Last error message
	ErrorCount   int               `json:"error_count,omitempty"`   // Consecutive error count
	Metadata     map[string]string `json:"metadata,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// UpdateStatus enumerates update lifecycle states.
type UpdateStatus string

const (
	UpdateStatusPending  UpdateStatus = "pending"
	UpdateStatusAccepted UpdateStatus = "accepted"
	UpdateStatusRejected UpdateStatus = "rejected"
	UpdateStatusFetched  UpdateStatus = "fetched" // Auto-fetched by enclave
)

// Update captures a submitted data observation/round.
type Update struct {
	ID          string            `json:"id"`
	AccountID   string            `json:"account_id"`
	FeedID      string            `json:"feed_id"`
	RoundID     int64             `json:"round_id"`
	Value       string            `json:"value"`                   // Generic value (was Price)
	NumericValue *float64         `json:"numeric_value,omitempty"` // Parsed numeric for sorting/comparison
	Signer      string            `json:"signer"`
	Timestamp   time.Time         `json:"timestamp"`
	Signature   string            `json:"signature"`
	Status      UpdateStatus      `json:"status"`
	Source      string            `json:"source,omitempty"`        // Which source provided this update
	FetchedInEnclave bool         `json:"fetched_in_enclave,omitempty"` // Was this fetched in TEE
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// FeedValue represents the current value of a feed for frontend display.
type FeedValue struct {
	FeedID       string     `json:"feed_id"`
	FeedName     string     `json:"feed_name"`
	Pair         string     `json:"pair,omitempty"`
	DataType     DataType   `json:"data_type"`
	Value        string     `json:"value"`
	NumericValue *float64   `json:"numeric_value,omitempty"`
	Decimals     int        `json:"decimals"`
	RoundID      int64      `json:"round_id"`
	Timestamp    time.Time  `json:"timestamp"`
	Status       FeedStatus `json:"status"`
	LastUpdated  time.Time  `json:"last_updated"`
	Confidence   float64    `json:"confidence,omitempty"` // Aggregation confidence (0-1)
	SourceCount  int        `json:"source_count"`         // Number of sources that contributed
}

// FeedListItem is a summary view for listing feeds.
type FeedListItem struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Pair        string     `json:"pair,omitempty"`
	DataType    DataType   `json:"data_type"`
	Status      FeedStatus `json:"status"`
	Value       string     `json:"value,omitempty"`
	LastUpdated time.Time  `json:"last_updated,omitempty"`
	SourceCount int        `json:"source_count"`
	Tags        []string   `json:"tags,omitempty"`
}

// FeedStats provides statistics for a feed.
type FeedStats struct {
	FeedID           string        `json:"feed_id"`
	TotalUpdates     int64         `json:"total_updates"`
	UpdatesLast24h   int64         `json:"updates_last_24h"`
	AverageLatency   time.Duration `json:"average_latency"`
	ErrorRate        float64       `json:"error_rate"`
	UptimePercent    float64       `json:"uptime_percent"`
	LastHeartbeat    time.Time     `json:"last_heartbeat"`
	SourcesHealthy   int           `json:"sources_healthy"`
	SourcesUnhealthy int           `json:"sources_unhealthy"`
}
