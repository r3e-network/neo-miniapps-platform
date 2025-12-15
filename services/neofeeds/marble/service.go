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
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/R3E-Network/service_layer/internal/runtime"
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

	// TxSubmitter integration (replaces direct teeFulfiller usage)
	txSubmitterAdapter *TxSubmitterAdapter
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
func New(cfg *Config) (*Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if cfg.Marble == nil {
		return nil, fmt.Errorf("marble is required")
	}

	strict := runtime.StrictIdentityMode() || cfg.Marble.IsEnclave()

	requiredSecrets := []string(nil)
	if strict {
		requiredSecrets = []string{"NEOFEEDS_SIGNING_KEY"}
	}

	base := commonservice.NewBase(&commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
		// Signing key must be stable in production/enclave mode for verification.
		RequiredSecrets: requiredSecrets,
	})

	// Load configuration
	var feedsConfig *NeoFeedsConfig
	var err error

	switch {
	case cfg.FeedsConfig != nil:
		feedsConfig = cfg.FeedsConfig
	case cfg.ConfigFile != "":
		feedsConfig, err = LoadConfigFromFile(cfg.ConfigFile)
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
	default:
		feedsConfig = DefaultConfig()
	}

	// In production/SGX mode, enforce TLS for all outbound price sources.
	if strict {
		for _, src := range feedsConfig.Sources {
			raw := strings.TrimSpace(src.URL)
			if !strings.HasPrefix(strings.ToLower(raw), "https://") {
				return nil, fmt.Errorf("neofeeds: source %q url must use https in strict identity mode", src.ID)
			}
		}
		if rpc := strings.TrimSpace(cfg.ArbitrumRPC); rpc != "" && !strings.HasPrefix(strings.ToLower(rpc), "https://") {
			return nil, fmt.Errorf("neofeeds: ArbitrumRPC must use https in strict identity mode")
		}
	}

	// Use the Marble-provided external client (system roots, no mTLS). Apply
	// per-source timeouts via request contexts to avoid creating clients per call.
	httpClient := cfg.Marble.ExternalHTTPClient()
	httpClient.Timeout = 0

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
	if key, ok := cfg.Marble.Secret("NEOFEEDS_SIGNING_KEY"); ok && len(key) >= 32 {
		s.signingKey = key
	} else if strict {
		return nil, fmt.Errorf("neofeeds: NEOFEEDS_SIGNING_KEY is required and must be at least 32 bytes")
	} else {
		s.Logger().WithFields(nil).Warn("NEOFEEDS_SIGNING_KEY not configured; price responses will be unsigned (development/testing only)")
	}

	// Initialize Chainlink client for Arbitrum
	chainlinkClient, err := NewChainlinkClient(cfg.ArbitrumRPC)
	if err != nil {
		// Log warning but don't fail - will fall back to HTTP sources
		s.Logger().WithError(err).Warn("chainlink client init failed")
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
		base.AddTickerWorker(s.updateInterval, func(ctx context.Context) error {
			s.pushPricesToChain(ctx)
			return nil
		}, commonservice.WithTickerWorkerName("chain-push"), commonservice.WithTickerWorkerImmediate())
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
	for i := range enabledFeeds {
		feedIDs[i] = enabledFeeds[i].ID
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
