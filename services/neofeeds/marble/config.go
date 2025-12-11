// Package neofeeds provides configurable price feed aggregation service.
package neofeeds

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// DataType defines the type of data a feed provides.
type DataType string

const (
	DataTypePrice  DataType = "price"  // Cryptocurrency/forex prices
	DataTypeNumber DataType = "number" // Generic numeric data
	DataTypeString DataType = "string" // Text data
)

// SourceConfig defines a data source configuration.
type SourceConfig struct {
	ID       string            `json:"id" yaml:"id"`
	Name     string            `json:"name" yaml:"name"`
	URL      string            `json:"url" yaml:"url"`             // URL template with {pair}, {base}, {quote} placeholders
	JSONPath string            `json:"json_path" yaml:"json_path"` // JSONPath to extract value
	Weight   int               `json:"weight" yaml:"weight"`       // Weight for aggregation (default: 1)
	Headers  map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Timeout  time.Duration     `json:"timeout,omitempty" yaml:"timeout,omitempty"` // Request timeout (default: 10s)
}

// FeedConfig defines a data feed configuration.
type FeedConfig struct {
	ID             string        `json:"id" yaml:"id"`                                               // Feed identifier (e.g., "BTC/USD")
	Name           string        `json:"name,omitempty" yaml:"name,omitempty"`                       // Human-readable name
	DataType       DataType      `json:"data_type" yaml:"data_type"`                                 // Type of data
	Pair           string        `json:"pair,omitempty" yaml:"pair,omitempty"`                       // Trading pair for price feeds (e.g., "BTCUSDT")
	Base           string        `json:"base,omitempty" yaml:"base,omitempty"`                       // Base asset (e.g., "BTC")
	Quote          string        `json:"quote,omitempty" yaml:"quote,omitempty"`                     // Quote asset (e.g., "USD")
	Decimals       int           `json:"decimals" yaml:"decimals"`                                   // Decimal precision (default: 8)
	Sources        []string      `json:"sources" yaml:"sources"`                                     // Source IDs to use
	UpdateInterval time.Duration `json:"update_interval,omitempty" yaml:"update_interval,omitempty"` // Per-feed update interval
	Enabled        bool          `json:"enabled" yaml:"enabled"`                                     // Whether feed is active
}

// NeoFeedsConfig is the root configuration for the neofeeds service.
type NeoFeedsConfig struct {
	Version        string         `json:"version" yaml:"version"`
	Sources        []SourceConfig `json:"sources" yaml:"sources"`
	Feeds          []FeedConfig   `json:"feeds" yaml:"feeds"`
	DefaultSources []string       `json:"default_sources,omitempty" yaml:"default_sources,omitempty"` // Default sources for feeds that don't specify
	UpdateInterval time.Duration  `json:"update_interval,omitempty" yaml:"update_interval,omitempty"` // Global update interval
}

// LoadConfigFromFile loads configuration from a JSON or YAML file.
func LoadConfigFromFile(path string) (*NeoFeedsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg NeoFeedsConfig

	// Try YAML first (also handles JSON)
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// Try JSON explicitly
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	// Validate and set defaults
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate validates the configuration and sets defaults.
func (c *NeoFeedsConfig) Validate() error {
	if len(c.Sources) == 0 {
		return fmt.Errorf("at least one source required")
	}

	sourceMap := make(map[string]bool)
	for i := range c.Sources {
		src := &c.Sources[i]
		if src.ID == "" {
			return fmt.Errorf("source[%d]: id required", i)
		}
		if src.URL == "" {
			return fmt.Errorf("source[%d]: url required", i)
		}
		if src.JSONPath == "" {
			return fmt.Errorf("source[%d]: json_path required", i)
		}
		if src.Weight <= 0 {
			src.Weight = 1
		}
		if src.Timeout <= 0 {
			src.Timeout = 10 * time.Second
		}
		sourceMap[src.ID] = true
	}

	for i := range c.Feeds {
		feed := &c.Feeds[i]
		if feed.ID == "" {
			return fmt.Errorf("feed[%d]: id required", i)
		}
		if feed.DataType == "" {
			feed.DataType = DataTypePrice
		}
		if feed.Decimals <= 0 {
			feed.Decimals = 8
		}
		if len(feed.Sources) == 0 {
			feed.Sources = c.DefaultSources
		}
		// Validate source references
		for _, srcID := range feed.Sources {
			if !sourceMap[srcID] {
				return fmt.Errorf("feed[%d]: unknown source %q", i, srcID)
			}
		}
	}

	if c.UpdateInterval <= 0 {
		c.UpdateInterval = 60 * time.Second
	}

	return nil
}

// GetSource returns a source by ID.
func (c *NeoFeedsConfig) GetSource(id string) *SourceConfig {
	for i := range c.Sources {
		if c.Sources[i].ID == id {
			return &c.Sources[i]
		}
	}
	return nil
}

// GetFeed returns a feed by ID.
func (c *NeoFeedsConfig) GetFeed(id string) *FeedConfig {
	for i := range c.Feeds {
		if c.Feeds[i].ID == id {
			return &c.Feeds[i]
		}
	}
	return nil
}

// GetEnabledFeeds returns all enabled feeds.
func (c *NeoFeedsConfig) GetEnabledFeeds() []FeedConfig {
	var feeds []FeedConfig
	for _, f := range c.Feeds {
		if f.Enabled {
			feeds = append(feeds, f)
		}
	}
	return feeds
}

// DefaultConfig returns the default configuration.
// Chainlink feeds on Arbitrum: BTC, ETH, LINK, SOL, BNB, DOGE, ADA, AVAX, LTC, UNI, XRP
// Binance-only feeds: NEO, GAS, TRX, HYPE, XMR, ZEC, SUI, BCH, ASTR
func DefaultConfig() *NeoFeedsConfig {
	return &NeoFeedsConfig{
		Version: "1.0",
		Sources: []SourceConfig{
			{
				ID:       "binance",
				Name:     "Binance",
				URL:      "https://api.binance.com/api/v3/ticker/price?symbol={pair}",
				JSONPath: "price",
				Weight:   3,
				Timeout:  10 * time.Second,
				Headers:  map[string]string{"X-MBX-APIKEY": "${BINANCE_API_KEY}"},
			},
		},
		Feeds: []FeedConfig{
			// Chainlink supported feeds (will use Chainlink first, Binance as fallback)
			{ID: "BTC/USD", Name: "Bitcoin", DataType: DataTypePrice, Pair: "BTCUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "ETH/USD", Name: "Ethereum", DataType: DataTypePrice, Pair: "ETHUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "XRP/USD", Name: "Ripple", DataType: DataTypePrice, Pair: "XRPUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "BNB/USD", Name: "BNB", DataType: DataTypePrice, Pair: "BNBUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "SOL/USD", Name: "Solana", DataType: DataTypePrice, Pair: "SOLUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "DOGE/USD", Name: "Dogecoin", DataType: DataTypePrice, Pair: "DOGEUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "ADA/USD", Name: "Cardano", DataType: DataTypePrice, Pair: "ADAUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "LINK/USD", Name: "Chainlink", DataType: DataTypePrice, Pair: "LINKUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "LTC/USD", Name: "Litecoin", DataType: DataTypePrice, Pair: "LTCUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "AVAX/USD", Name: "Avalanche", DataType: DataTypePrice, Pair: "AVAXUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "UNI/USD", Name: "Uniswap", DataType: DataTypePrice, Pair: "UNIUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			// Binance-only feeds (Chainlink doesn't support these on Arbitrum)
			{ID: "NEO/USD", Name: "Neo", DataType: DataTypePrice, Pair: "NEOUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "GAS/USD", Name: "Gas", DataType: DataTypePrice, Pair: "GASUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "TRX/USD", Name: "Tron", DataType: DataTypePrice, Pair: "TRXUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "HYPE/USD", Name: "Hyperliquid", DataType: DataTypePrice, Pair: "HYPEUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "XMR/USD", Name: "Monero", DataType: DataTypePrice, Pair: "XMRUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "ZEC/USD", Name: "Zcash", DataType: DataTypePrice, Pair: "ZECUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "SUI/USD", Name: "Sui", DataType: DataTypePrice, Pair: "SUIUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "BCH/USD", Name: "Bitcoin Cash", DataType: DataTypePrice, Pair: "BCHUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
			{ID: "ASTR/USD", Name: "Astar", DataType: DataTypePrice, Pair: "ASTRUSDT", Decimals: 8, Sources: []string{"binance"}, Enabled: true},
		},
		DefaultSources: []string{"binance"},
		UpdateInterval: 60 * time.Second,
	}
}

// ToJSON serializes config to JSON.
func (c *NeoFeedsConfig) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// ToYAML serializes config to YAML.
func (c *NeoFeedsConfig) ToYAML() ([]byte, error) {
	return yaml.Marshal(c)
}
