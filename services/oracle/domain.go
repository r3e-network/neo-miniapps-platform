// Package oracle provides the Oracle service.
package oracle

import (
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// DataFeed represents a configured data feed.
type DataFeed struct {
	base.BaseEntity

	// Configuration
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        []byte            `json:"body,omitempty"`

	// Authentication
	SecretName string      `json:"secret_name,omitempty"`
	AuthType   os.AuthType `json:"auth_type,omitempty"`

	// Schedule
	Schedule    string        `json:"schedule,omitempty"` // Cron expression
	Interval    time.Duration `json:"interval,omitempty"` // Polling interval
	LastFetched time.Time     `json:"last_fetched,omitempty"`

	// Status
	Active      bool   `json:"active"`
	LastError   string `json:"last_error,omitempty"`
	FetchCount  int64  `json:"fetch_count"`
	ErrorCount  int64  `json:"error_count"`
}

// NewDataFeed creates a new data feed.
func NewDataFeed(id, name, url, method string) *DataFeed {
	return &DataFeed{
		BaseEntity: base.BaseEntity{ID: id},
		Name:       name,
		URL:        url,
		Method:     method,
		Active:     true,
	}
}

// SetTimestamps updates timestamps.
func (f *DataFeed) SetTimestamps() {
	f.BaseEntity.SetTimestamps()
}

// RecordFetch records a successful fetch.
func (f *DataFeed) RecordFetch() {
	f.LastFetched = time.Now()
	f.FetchCount++
	f.LastError = ""
}

// RecordError records a fetch error.
func (f *DataFeed) RecordError(err error) {
	f.ErrorCount++
	if err != nil {
		f.LastError = err.Error()
	}
}

// =============================================================================
// Data Types
// =============================================================================

// PriceData represents price data from an oracle.
type PriceData struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Volume    float64   `json:"volume,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
}

// WeatherData represents weather data from an oracle.
type WeatherData struct {
	Location    string    `json:"location"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	Conditions  string    `json:"conditions"`
	Timestamp   time.Time `json:"timestamp"`
	Source      string    `json:"source"`
}

// RandomData represents random data from an oracle.
type RandomData struct {
	Value     []byte    `json:"value"`
	Timestamp time.Time `json:"timestamp"`
	Proof     []byte    `json:"proof,omitempty"`
}

// =============================================================================
// Request/Response Types
// =============================================================================

// CreateFeedRequest represents a request to create a data feed.
type CreateFeedRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers,omitempty"`
	SecretName  string            `json:"secret_name,omitempty"`
	AuthType    os.AuthType       `json:"auth_type,omitempty"`
	Schedule    string            `json:"schedule,omitempty"`
	Interval    time.Duration     `json:"interval,omitempty"`
}

// UpdateFeedRequest represents a request to update a data feed.
type UpdateFeedRequest struct {
	Name        *string            `json:"name,omitempty"`
	Description *string            `json:"description,omitempty"`
	URL         *string            `json:"url,omitempty"`
	Method      *string            `json:"method,omitempty"`
	Headers     *map[string]string `json:"headers,omitempty"`
	SecretName  *string            `json:"secret_name,omitempty"`
	AuthType    *os.AuthType       `json:"auth_type,omitempty"`
	Schedule    *string            `json:"schedule,omitempty"`
	Interval    *time.Duration     `json:"interval,omitempty"`
	Active      *bool              `json:"active,omitempty"`
}

// FeedListResponse represents a list of data feeds.
type FeedListResponse struct {
	Feeds []*DataFeed `json:"feeds"`
	Total int         `json:"total"`
}
