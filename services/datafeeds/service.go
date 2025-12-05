// Package datafeeds provides data feed aggregation service.
package datafeeds

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/platform/supabase"
	"github.com/R3E-Network/service_layer/services/base"
)

const (
	ServiceID   = "datafeeds"
	ServiceName = "DataFeeds Service"
	Version     = "1.0.0"
)

// Manifest returns the service manifest.
func Manifest() *os.LegacyManifest {
	return &os.LegacyManifest{
		ServiceID:   ServiceID,
		Version:     Version,
		Description: "Data feed aggregation and price oracle service",
		RequiredCapabilities: []os.Capability{
			os.CapNetwork,
			os.CapStorage,
			os.CapKeys,
		},
		OptionalCapabilities: []os.Capability{
			os.CapSecrets,
			os.CapKeysSign,
			os.CapDatabase,      // Database storage
			os.CapDatabaseWrite, // Database write
			os.CapMetrics,       // Metrics collection
			os.CapCache,         // Price caching
			os.CapScheduler,     // Scheduled updates
			os.CapNeo,           // Neo N3 blockchain
			os.CapNeoSign,       // Neo transaction signing
			os.CapContract,      // Contract interaction
			os.CapContractWrite, // Contract price push
		},
		ResourceLimits: os.ResourceLimits{
			MaxMemory:      128 * 1024 * 1024,
			MaxCPUTime:     30 * time.Second,
			MaxNetworkReqs: 500,
		},
	}
}

// Service implements the DataFeeds service.
type Service struct {
	*base.BaseService
	mu        sync.RWMutex
	enclave   *Enclave
	store     *Store
	supa      *supabase.Client
	supaTable string
}

// Config represents sealed config for DataFeeds.
type Config struct {
	AllowedHosts []string       `json:"allowed_hosts,omitempty"`
	Supabase     SupabaseConfig `json:"supabase,omitempty"`
}

// SupabaseConfig represents sealed Supabase settings for DataFeeds.
type SupabaseConfig struct {
	ProjectURL     string            `json:"project_url"`
	APIKeySecret   string            `json:"api_key_secret"`
	APIKey         string            `json:"api_key,omitempty"`
	AllowedHosts   []string          `json:"allowed_hosts,omitempty"`
	DefaultHeaders map[string]string `json:"default_headers,omitempty"`
	Table          string            `json:"table,omitempty"`
}

// New creates a new DataFeeds service.
func New(serviceOS os.ServiceOS) (*Service, error) {
	baseService := base.NewBaseService(ServiceID, ServiceName, Version, serviceOS)
	enclave, err := NewEnclave(serviceOS)
	if err != nil {
		return nil, fmt.Errorf("create enclave: %w", err)
	}
	store := NewStore()

	s := &Service{
		BaseService: baseService,
		enclave:     enclave,
		store:       store,
	}

	// Register components with BaseService for lifecycle management
	baseService.SetEnclave(enclave)
	baseService.SetStore(store)

	// Set lifecycle hooks
	baseService.SetHooks(base.LifecycleHooks{
		OnAfterStart: s.onAfterStart,
	})

	return s, nil
}

// onAfterStart is called after components are initialized.
func (s *Service) onAfterStart(ctx context.Context) error {
	// Optionally hydrate from sealed storage without exporting plaintext
	var cfg Config
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "datafeeds/config", &cfg); err != nil {
		s.Logger().Warn("datafeeds storage hydrate failed", "err", err)
	} else if loaded {
		if len(cfg.AllowedHosts) > 0 {
			s.Logger().Info("datafeeds allowed hosts from sealed config", "hosts", cfg.AllowedHosts)
		}
		if cfg.Supabase.ProjectURL != "" && cfg.Supabase.APIKeySecret != "" {
			if err := s.initSupabase(cfg.Supabase); err != nil {
				s.Logger().Warn("datafeeds supabase init failed (from datafeeds/config)", "err", err)
			}
		}
	}

	// Optional: initialize Supabase client from sealed config (datafeeds/supabase)
	var supaCfg SupabaseConfig
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "datafeeds/supabase", &supaCfg); err != nil {
		s.Logger().Warn("datafeeds supabase config load failed", "err", err)
	} else if loaded {
		if err := s.initSupabase(supaCfg); err != nil {
			s.Logger().Warn("datafeeds supabase client init failed", "err", err)
		}
	}

	// Register metrics if available
	if s.OS().HasCapability(os.CapMetrics) {
		s.registerMetrics()
	}

	// Start scheduled price updates if scheduler available
	if s.OS().HasCapability(os.CapScheduler) {
		s.startScheduledUpdates(ctx)
	}

	// Start contract price pusher if contract capability available
	if s.OS().HasCapability(os.CapContract) && s.OS().HasCapability(os.CapContractWrite) {
		s.startContractPricePusher(ctx)
	}

	s.Logger().Info("datafeeds service fully initialized")
	return nil
}

// registerMetrics registers service metrics.
func (s *Service) registerMetrics() {
	metrics := s.OS().Metrics()
	metrics.RegisterCounter("datafeeds_updates_total", "Total number of price updates")
	metrics.RegisterCounter("datafeeds_updates_failed", "Total number of failed updates")
	metrics.RegisterCounter("datafeeds_cache_hits", "Total number of cache hits")
	metrics.RegisterGauge("datafeeds_active_feeds", "Number of active data feeds")
	metrics.RegisterHistogram("datafeeds_update_duration_seconds", "Price update duration", []float64{0.1, 0.5, 1, 2, 5})
}

// startScheduledUpdates starts scheduled price updates.
func (s *Service) startScheduledUpdates(ctx context.Context) {
	scheduler := s.OS().Scheduler()

	// Schedule periodic price updates (every minute)
	_, err := scheduler.ScheduleInterval(ctx, 1*time.Minute, &os.ScheduledTask{
		Name:       "update_prices",
		Handler:    "update_prices",
		MaxRetries: 3,
		Timeout:    30 * time.Second,
	})
	if err != nil {
		s.Logger().Warn("failed to schedule price updates", "error", err)
	}

	s.Logger().Info("scheduled price updates started")
}

// GetPrice fetches the latest price for a symbol.
func (s *Service) GetPrice(ctx context.Context, symbol string) (*PriceData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}
	return s.store.GetLatestPrice(ctx, symbol)
}

// UpdatePrice updates price data with TEE signature.
func (s *Service) UpdatePrice(ctx context.Context, data *PriceData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}
	// Sign in enclave
	signature, err := s.enclave.SignPriceData(ctx, data)
	if err != nil {
		return fmt.Errorf("sign price: %w", err)
	}
	data.Signature = signature
	data.SetTimestamps()

	// Optional: publish to Supabase if configured
	if s.supa != nil && s.supaTable != "" {
		_ = s.publishSupabase(ctx, data) // best-effort; errors logged in helper
	}

	return s.store.SavePrice(ctx, data)
}

func (s *Service) publishSupabase(ctx context.Context, data *PriceData) error {
	body, err := json.Marshal(data)
	if err != nil {
		s.Logger().Warn("datafeeds supabase marshal failed", "err", err)
		return err
	}
	resp, err := s.supa.Insert(ctx, s.supaTable, body)
	if err != nil {
		s.Logger().Warn("datafeeds supabase insert failed", "err", err)
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.Logger().Warn("datafeeds supabase insert status", "status", resp.StatusCode)
		return fmt.Errorf("supabase insert status %d", resp.StatusCode)
	}
	return nil
}

func (s *Service) initSupabase(cfg SupabaseConfig) error {
	client, err := supabase.New(s.OS(), supabase.Config{
		ProjectURL:     cfg.ProjectURL,
		APIKeySecret:   cfg.APIKeySecret,
		DefaultHeaders: cfg.DefaultHeaders,
		AllowedHosts:   cfg.AllowedHosts,
	})
	if err != nil {
		return err
	}
	s.supa = client
	s.supaTable = cfg.Table
	s.Logger().Info("datafeeds supabase client initialized", "table", cfg.Table)
	return nil
}

// =============================================================================
// Contract Integration (Push Model)
// =============================================================================

// startContractPricePusher starts the background price pusher for contracts.
// DataFeeds uses a push model - prices are periodically fetched and pushed to contracts.
func (s *Service) startContractPricePusher(ctx context.Context) {
	// Load configured feeds from sealed storage
	var feedConfigs []FeedPushConfig
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "datafeeds/push_feeds", &feedConfigs); err != nil {
		s.Logger().Warn("failed to load push feed configs", "error", err)
	} else if loaded && len(feedConfigs) > 0 {
		for _, cfg := range feedConfigs {
			go s.runFeedPusher(ctx, cfg)
		}
		s.Logger().Info("contract price pushers started", "count", len(feedConfigs))
	}
}

// FeedPushConfig configures a price feed for contract pushing.
type FeedPushConfig struct {
	FeedID       string        `json:"feed_id"`
	Symbol       string        `json:"symbol"`
	Sources      []string      `json:"sources"`
	Interval     time.Duration `json:"interval"`
	DeviationBps uint32        `json:"deviation_bps"` // Deviation threshold in basis points
}

// runFeedPusher runs a price pusher for a single feed.
func (s *Service) runFeedPusher(ctx context.Context, cfg FeedPushConfig) {
	interval := cfg.Interval
	if interval == 0 {
		interval = 1 * time.Minute // Default 1 minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastPrice float64

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			price, err := s.fetchAndPushPrice(ctx, cfg, lastPrice)
			if err != nil {
				s.Logger().Warn("price push failed", "feed_id", cfg.FeedID, "error", err)
				continue
			}
			lastPrice = price
		}
	}
}

// fetchAndPushPrice fetches the latest price and pushes it to the contract.
func (s *Service) fetchAndPushPrice(ctx context.Context, cfg FeedPushConfig, lastPrice float64) (float64, error) {
	// Get latest price from store
	priceData, err := s.store.GetLatestPrice(ctx, cfg.Symbol)
	if err != nil {
		return lastPrice, fmt.Errorf("get price: %w", err)
	}

	// Check deviation threshold
	if lastPrice > 0 && cfg.DeviationBps > 0 {
		deviation := absFloat(priceData.Price-lastPrice) * 10000 / lastPrice
		if deviation < float64(cfg.DeviationBps) {
			// Price hasn't changed enough, skip push
			return lastPrice, nil
		}
	}

	// Sign the price data
	signature, err := s.enclave.SignPriceData(ctx, priceData)
	if err != nil {
		return lastPrice, fmt.Errorf("sign price: %w", err)
	}

	// Convert float64 price to int64 (scaled by 10^8 for 8 decimals)
	const decimals uint8 = 8
	scaledPrice := int64(priceData.Price * 1e8)

	// Push to contract
	update := &os.PriceUpdate{
		FeedID:    cfg.FeedID,
		Price:     scaledPrice,
		Decimals:  decimals,
		Timestamp: priceData.Timestamp,
		Signature: signature,
	}

	if err := s.OS().Contract().UpdatePrice(ctx, update); err != nil {
		return lastPrice, fmt.Errorf("push price: %w", err)
	}

	s.Logger().Info("price pushed to contract",
		"feed_id", cfg.FeedID,
		"price", priceData.Price,
		"scaled_price", scaledPrice,
		"decimals", decimals,
	)

	// Record metrics
	if s.OS().HasCapability(os.CapMetrics) {
		s.OS().Metrics().Counter("datafeeds_contract_pushes_total", 1, "feed_id", cfg.FeedID)
	}

	return priceData.Price, nil
}

// abs returns the absolute value of x.
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// absFloat returns the absolute value of x.
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// PushPriceToContract manually pushes a price update to the contract.
func (s *Service) PushPriceToContract(ctx context.Context, feedID string, priceData *PriceData) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}

	if !s.OS().HasCapability(os.CapContractWrite) {
		return fmt.Errorf("contract write capability not available")
	}

	// Sign the price data
	signature, err := s.enclave.SignPriceData(ctx, priceData)
	if err != nil {
		return fmt.Errorf("sign price: %w", err)
	}

	// Convert float64 price to int64 (scaled by 10^8 for 8 decimals)
	const decimals uint8 = 8
	scaledPrice := int64(priceData.Price * 1e8)

	// Push to contract
	update := &os.PriceUpdate{
		FeedID:    feedID,
		Price:     scaledPrice,
		Decimals:  decimals,
		Timestamp: priceData.Timestamp,
		Signature: signature,
	}

	return s.OS().Contract().UpdatePrice(ctx, update)
}

// Store exposes the DataFeeds store.
func (s *Service) Store() *Store {
	return s.store
}
