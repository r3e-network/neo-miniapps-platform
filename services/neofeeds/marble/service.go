// Package neofeeds provides price feed aggregation service.
// This service implements the Push/Auto-Update pattern:
// - TEE periodically fetches prices from multiple sources
// - TEE aggregates and signs the price data
// - TEE pushes updates to the NeoFeedsService contract on-chain
// - User contracts read prices directly (no callback needed)
//
// Configuration can be loaded from YAML/JSON file for easy customization
// of data sources and feeds without code changes.
package neofeeds

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	commonservice "github.com/R3E-Network/service_layer/services/common/service"
)

const (
	ServiceID   = "neofeeds"
	ServiceName = "NeoFeeds Service"
	Version     = "3.0.0"

	// Service fee per price update request (in GAS smallest unit)
	ServiceFeePerUpdate = 10000 // 0.0001 GAS
)

// Service implements the NeoFeeds service.
type Service struct {
	*commonservice.BaseService
	httpClient      *http.Client
	signingKey      []byte
	chainlinkClient *ChainlinkClient

	// Configuration
	config  *NeoFeedsConfig
	sources map[string]*SourceConfig

	// Chain interaction for push pattern
	chainClient     *chain.Client
	teeFulfiller    *chain.TEEFulfiller
	neoFeedsHash    string
	updateInterval  time.Duration
	enableChainPush bool
}

// Config holds NeoFeeds service configuration.
type Config struct {
	Marble      *marble.Marble
	DB          database.RepositoryInterface
	ConfigFile  string          // Path to YAML/JSON config file (optional)
	FeedsConfig *NeoFeedsConfig // Direct config (optional, takes precedence over file)
	ArbitrumRPC string          // Arbitrum RPC URL for Chainlink feeds

	// Chain configuration for push pattern
	ChainClient     *chain.Client
	TEEFulfiller    *chain.TEEFulfiller
	NeoFeedsHash    string        // Contract hash for NeoFeedsService
	UpdateInterval  time.Duration // How often to push prices on-chain (default: from config)
	EnableChainPush bool          // Enable automatic on-chain price updates
}

// New creates a new NeoFeeds service.
func New(cfg Config) (*Service, error) {
	base := commonservice.NewBase(commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	// Load configuration
	var feedsConfig *NeoFeedsConfig
	var err error

	if cfg.FeedsConfig != nil {
		feedsConfig = cfg.FeedsConfig
	} else if cfg.ConfigFile != "" {
		feedsConfig, err = LoadConfigFromFile(cfg.ConfigFile)
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
	} else {
		feedsConfig = DefaultConfig()
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}

	// Use config-specified interval, then service config, then default
	updateInterval := feedsConfig.UpdateInterval
	if cfg.UpdateInterval > 0 {
		updateInterval = cfg.UpdateInterval
	}

	s := &Service{
		BaseService:     base,
		httpClient:      httpClient,
		config:          feedsConfig,
		sources:         make(map[string]*SourceConfig),
		chainClient:     cfg.ChainClient,
		teeFulfiller:    cfg.TEEFulfiller,
		neoFeedsHash:    cfg.NeoFeedsHash,
		updateInterval:  updateInterval,
		enableChainPush: cfg.EnableChainPush,
	}

	// Load signing key
	if key, ok := cfg.Marble.Secret("NEOFEEDS_SIGNING_KEY"); ok {
		s.signingKey = key
	}

	// Initialize Chainlink client for Arbitrum
	chainlinkClient, err := NewChainlinkClient(cfg.ArbitrumRPC)
	if err != nil {
		// Log warning but don't fail - will fall back to HTTP sources
		fmt.Printf("Warning: Chainlink client init failed: %v\n", err)
	} else {
		s.chainlinkClient = chainlinkClient
	}

	// Index sources by ID
	for i := range feedsConfig.Sources {
		src := &feedsConfig.Sources[i]
		s.sources[src.ID] = src
	}

	// Register chain push worker if enabled
	if s.enableChainPush && s.teeFulfiller != nil && s.neoFeedsHash != "" {
		base.AddWorker(s.runChainPushLoop)
	}

	// Register statistics provider for /info endpoint
	base.WithStats(s.statistics)

	// Register standard routes (/health, /info) plus service-specific routes
	base.RegisterStandardRoutes()
	s.registerRoutes()

	return s, nil
}

// statistics returns runtime statistics for the /info endpoint.
func (s *Service) statistics() map[string]any {
	enabledFeeds := s.GetEnabledFeeds()
	feedIDs := make([]string, len(enabledFeeds))
	for i, f := range enabledFeeds {
		feedIDs[i] = f.ID
	}

	return map[string]any{
		"sources":         len(s.sources),
		"feeds":           feedIDs,
		"update_interval": s.updateInterval.String(),
		"chain_push":      s.enableChainPush,
		"service_fee":     ServiceFeePerUpdate,
	}
}

// GetConfig returns the current configuration.
func (s *Service) GetConfig() *NeoFeedsConfig {
	return s.config
}

// GetEnabledFeeds returns all enabled feeds.
func (s *Service) GetEnabledFeeds() []FeedConfig {
	return s.config.GetEnabledFeeds()
}
