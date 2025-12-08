// Package datafeeds provides price feed aggregation service.
// This service implements the Push/Auto-Update pattern:
// - TEE periodically fetches prices from multiple sources
// - TEE aggregates and signs the price data
// - TEE pushes updates to the DataFeedsService contract on-chain
// - User contracts read prices directly (no callback needed)
//
// Configuration can be loaded from YAML/JSON file for easy customization
// of data sources and feeds without code changes.
package datafeeds

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

const (
	ServiceID   = "datafeeds"
	ServiceName = "DataFeeds Service"
	Version     = "3.0.0"

	// Service fee per price update request (in GAS smallest unit)
	ServiceFeePerUpdate = 10000 // 0.0001 GAS
)

// Service implements the DataFeeds service.
type Service struct {
	*marble.Service
	httpClient      *http.Client
	signingKey      []byte
	chainlinkClient *ChainlinkClient

	// Configuration
	config  *DataFeedsConfig
	sources map[string]*SourceConfig

	// Chain interaction for push pattern
	chainClient     *chain.Client
	teeFulfiller    *chain.TEEFulfiller
	dataFeedsHash   string
	updateInterval  time.Duration
	stopCh          chan struct{}
	enableChainPush bool
}

// PriceSource defines a price data source (legacy, use SourceConfig instead).
type PriceSource struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	JSONPath string `json:"json_path"`
	Weight   int    `json:"weight"`
}

// Config holds DataFeeds service configuration.
type Config struct {
	Marble      *marble.Marble
	DB          *database.Repository
	ConfigFile  string            // Path to YAML/JSON config file (optional)
	FeedsConfig *DataFeedsConfig  // Direct config (optional, takes precedence over file)
	ArbitrumRPC string            // Arbitrum RPC URL for Chainlink feeds

	// Chain configuration for push pattern
	ChainClient     *chain.Client
	TEEFulfiller    *chain.TEEFulfiller
	DataFeedsHash   string        // Contract hash for DataFeedsService
	UpdateInterval  time.Duration // How often to push prices on-chain (default: from config)
	EnableChainPush bool          // Enable automatic on-chain price updates
}

// New creates a new DataFeeds service.
func New(cfg Config) (*Service, error) {
	base := marble.NewService(marble.ServiceConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	// Load configuration
	var feedsConfig *DataFeedsConfig
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
		Service:         base,
		httpClient:      httpClient,
		config:          feedsConfig,
		sources:         make(map[string]*SourceConfig),
		chainClient:     cfg.ChainClient,
		teeFulfiller:    cfg.TEEFulfiller,
		dataFeedsHash:   cfg.DataFeedsHash,
		updateInterval:  updateInterval,
		stopCh:          make(chan struct{}),
		enableChainPush: cfg.EnableChainPush,
	}

	// Load signing key
	if key, ok := cfg.Marble.Secret("DATAFEEDS_SIGNING_KEY"); ok {
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

	s.registerRoutes()
	return s, nil
}

// GetConfig returns the current configuration.
func (s *Service) GetConfig() *DataFeedsConfig {
	return s.config
}

// GetEnabledFeeds returns all enabled feeds.
func (s *Service) GetEnabledFeeds() []FeedConfig {
	return s.config.GetEnabledFeeds()
}

func (s *Service) registerRoutes() {
	router := s.Router()
	router.HandleFunc("/health", marble.HealthHandler(s.Service)).Methods("GET")
	router.HandleFunc("/info", s.handleInfo).Methods("GET")
	// Use path pattern that matches feed IDs with slashes (e.g., BTC/USD)
	router.PathPrefix("/price/").HandlerFunc(s.handleGetPrice).Methods("GET")
	router.HandleFunc("/prices", s.handleGetPrices).Methods("GET")
	router.HandleFunc("/feeds", s.handleListFeeds).Methods("GET")
	router.HandleFunc("/config", s.handleGetConfig).Methods("GET")
	router.HandleFunc("/sources", s.handleListSources).Methods("GET")
}

func (s *Service) handleInfo(w http.ResponseWriter, r *http.Request) {
	enabledFeeds := s.GetEnabledFeeds()
	feedIDs := make([]string, len(enabledFeeds))
	for i, f := range enabledFeeds {
		feedIDs[i] = f.ID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "active",
		"version":         Version,
		"sources":         len(s.sources),
		"feeds":           feedIDs,
		"update_interval": s.updateInterval.String(),
		"chain_push":      s.enableChainPush,
		"service_fee":     ServiceFeePerUpdate,
	})
}

func (s *Service) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.config)
}

func (s *Service) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources := make([]map[string]interface{}, 0, len(s.sources))
	for id, src := range s.sources {
		sources = append(sources, map[string]interface{}{
			"id":     id,
			"name":   src.Name,
			"weight": src.Weight,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sources)
}

// =============================================================================
// Request/Response Types
// =============================================================================

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

// =============================================================================
// Handlers
// =============================================================================

func (s *Service) handleGetPrice(w http.ResponseWriter, r *http.Request) {
	// Extract pair from URL (e.g., /price/BTCUSDT)
	pair := r.URL.Path[len("/price/"):]
	if pair == "" {
		http.Error(w, "pair required", http.StatusBadRequest)
		return
	}

	price, err := s.GetPrice(r.Context(), pair)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(price)
}

func (s *Service) handleGetPrices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.DB() == nil {
		json.NewEncoder(w).Encode([]PriceResponse{})
		return
	}

	var responses []PriceResponse
	for _, feedID := range DefaultFeeds {
		if latest, err := s.DB().GetLatestPrice(r.Context(), feedID); err == nil {
			responses = append(responses, PriceResponse{
				FeedID:    latest.FeedID,
				Pair:      latest.Pair,
				Price:     latest.Price,
				Decimals:  latest.Decimals,
				Timestamp: latest.Timestamp,
				Sources:   latest.Sources,
				Signature: latest.Signature,
			})
		}
	}
	json.NewEncoder(w).Encode(responses)
}

func (s *Service) handleListFeeds(w http.ResponseWriter, r *http.Request) {
	feeds := make([]map[string]string, 0, len(s.sources))
	for id, src := range s.sources {
		feeds = append(feeds, map[string]string{
			"id":   id,
			"name": src.Name,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(feeds)
}

// =============================================================================
// Core Logic
// =============================================================================

// GetPrice fetches and aggregates price from multiple sources.
// Priority: Chainlink (free, on-chain) -> HTTP sources (Binance)
func (s *Service) GetPrice(ctx context.Context, pair string) (*PriceResponse, error) {
	// Try to find feed config for this pair
	feed := s.findFeedByPair(pair)

	feedID := pair
	if feed != nil {
		feedID = feed.ID
	}

	var prices []float64
	var sources []string
	decimals := 8
	if feed != nil && feed.Decimals > 0 {
		decimals = feed.Decimals
	}

	// Try Chainlink first (free, on-chain data)
	if s.chainlinkClient != nil && s.chainlinkClient.HasFeed(feedID) {
		price, _, err := s.chainlinkClient.GetPrice(ctx, feedID)
		if err == nil && price > 0 {
			prices = append(prices, price, price, price) // Weight 3 for Chainlink
			sources = append(sources, "chainlink")
		}
	}

	// Fall back to HTTP sources (Binance) if Chainlink not available or failed
	if len(prices) == 0 {
		var wg sync.WaitGroup
		var mu sync.Mutex

		sourcesToUse := s.getSourcesForFeed(feed)

		for _, srcConfig := range sourcesToUse {
			wg.Add(1)
			go func(src *SourceConfig) {
				defer wg.Done()

				price, err := s.fetchPriceFromSource(ctx, pair, feed, src)
				if err != nil {
					return
				}

				mu.Lock()
				for i := 0; i < src.Weight; i++ {
					prices = append(prices, price)
				}
				sources = append(sources, src.ID)
				mu.Unlock()
			}(srcConfig)
		}

		wg.Wait()
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no prices available for %s", pair)
	}

	medianPrice := s.calculateMedian(prices)
	priceInt := int64(medianPrice * float64(pow10(decimals)))

	response := &PriceResponse{
		FeedID:    feedID,
		Pair:      pair,
		Price:     priceInt,
		Decimals:  decimals,
		Timestamp: time.Now(),
		Sources:   sources,
	}

	if len(s.signingKey) > 0 {
		sig, pub, _ := s.signPrice(response)
		response.Signature = append([]byte{}, sig...)
		response.PublicKey = append([]byte{}, pub...)
	}

	if s.DB() != nil {
		_ = s.DB().CreatePriceFeed(ctx, &database.PriceFeed{
			ID:        uuid.New().String(),
			FeedID:    feedID,
			Pair:      pair,
			Price:     priceInt,
			Decimals:  response.Decimals,
			Timestamp: response.Timestamp,
			Sources:   response.Sources,
			Signature: response.Signature,
		})
	}

	return response, nil
}

// findFeedByPair finds a feed config by pair or feed ID.
func (s *Service) findFeedByPair(pair string) *FeedConfig {
	for i := range s.config.Feeds {
		f := &s.config.Feeds[i]
		if f.Pair == pair || f.ID == pair {
			return f
		}
	}
	return nil
}

// getSourcesForFeed returns sources to use for a feed.
func (s *Service) getSourcesForFeed(feed *FeedConfig) []*SourceConfig {
	if feed != nil && len(feed.Sources) > 0 {
		sources := make([]*SourceConfig, 0, len(feed.Sources))
		for _, srcID := range feed.Sources {
			if src := s.sources[srcID]; src != nil {
				sources = append(sources, src)
			}
		}
		return sources
	}
	// Return all sources if no feed config
	sources := make([]*SourceConfig, 0, len(s.sources))
	for _, src := range s.sources {
		sources = append(sources, src)
	}
	return sources
}

// fetchPriceFromSource fetches price from a single source.
func (s *Service) fetchPriceFromSource(ctx context.Context, pair string, feed *FeedConfig, src *SourceConfig) (float64, error) {
	// Use feed.Pair (e.g., NEOUSDT) for URL template if available, otherwise use raw pair (e.g., NEO/USD)
	pairForURL := pair
	if feed != nil && feed.Pair != "" {
		pairForURL = feed.Pair
	}
	url := formatSourceURLNew(src.URL, pairForURL, feed)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	for k, v := range src.Headers {
		req.Header.Set(k, resolveEnvVar(v))
	}

	client := s.httpClient
	if src.Timeout > 0 {
		client = &http.Client{
			Timeout: src.Timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	jsonPath := formatJSONPath(src.JSONPath, feed)
	result := gjson.GetBytes(body, jsonPath)
	if !result.Exists() {
		return 0, fmt.Errorf("price not found in response")
	}

	return result.Float(), nil
}

func (s *Service) fetchPrice(ctx context.Context, pair string, source PriceSource) (float64, error) {
	url := formatSourceURL(source.URL, pair)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	result := gjson.GetBytes(body, source.JSONPath)
	if !result.Exists() {
		return 0, fmt.Errorf("price not found in response")
	}

	return result.Float(), nil
}

func (s *Service) calculateMedian(prices []float64) float64 {
	sort.Float64s(prices)
	n := len(prices)
	if n%2 == 0 {
		return (prices[n/2-1] + prices[n/2]) / 2
	}
	return prices[n/2]
}

func (s *Service) signPrice(price *PriceResponse) ([]byte, []byte, error) {
	data, _ := json.Marshal(map[string]interface{}{
		"pair":      price.Pair,
		"price":     price.Price,
		"decimals":  price.Decimals,
		"timestamp": price.Timestamp.Unix(),
	})

	seed, err := crypto.DeriveKey(s.signingKey, nil, "price-signing", 32)
	if err != nil {
		return nil, nil, err
	}
	defer crypto.ZeroBytes(seed)

	curve := elliptic.P256()
	d := new(big.Int).SetBytes(seed)
	n := new(big.Int).Sub(curve.Params().N, big.NewInt(1))
	d.Mod(d, n)
	d.Add(d, big.NewInt(1))
	priv := &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve}, D: d}
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	sig, err := crypto.Sign(priv, data)
	if err != nil {
		return nil, nil, err
	}
	pubBytes := crypto.PublicKeyToBytes(&priv.PublicKey)
	return sig, pubBytes, nil
}

func formatSourceURL(tmpl, pair string) string {
	if strings.Contains(tmpl, "%sPAIR%s") {
		return fmt.Sprintf(tmpl, "", pair, "")
	}
	return strings.ReplaceAll(tmpl, "{pair}", pair)
}

// formatSourceURLNew formats URL template with feed-specific placeholders.
func formatSourceURLNew(tmpl, pair string, feed *FeedConfig) string {
	url := tmpl
	url = strings.ReplaceAll(url, "{pair}", pair)

	if feed != nil {
		url = strings.ReplaceAll(url, "{base}", feed.Base)
		url = strings.ReplaceAll(url, "{quote}", feed.Quote)
	} else {
		// Parse base/quote from pair (e.g., BTCUSDT -> BTC, USDT)
		if len(pair) >= 6 {
			base := strings.ToLower(pair[:3])
			quote := strings.ToLower(pair[3:])
			url = strings.ReplaceAll(url, "{base}", base)
			url = strings.ReplaceAll(url, "{quote}", quote)
		}
	}

	// Legacy format support
	if strings.Contains(url, "%sPAIR%s") {
		url = fmt.Sprintf(url, "", pair, "")
	}

	return url
}

// formatJSONPath formats JSON path with feed-specific placeholders.
func formatJSONPath(tmpl string, feed *FeedConfig) string {
	if feed == nil {
		return tmpl
	}
	path := tmpl
	path = strings.ReplaceAll(path, "{base}", feed.Base)
	path = strings.ReplaceAll(path, "{quote}", feed.Quote)
	return path
}

// pow10 returns 10^n.
func pow10(n int) int64 {
	result := int64(1)
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

// =============================================================================
// Chain Push Logic (Push/Auto-Update Pattern)
// =============================================================================

// DefaultFeeds defines the default price feeds (for backward compatibility).
var DefaultFeeds = []string{
	"BTC/USD",
	"ETH/USD",
	"NEO/USD",
	"GAS/USD",
	"NEO/GAS",
}

// Start starts the DataFeeds service including the chain push loop.
func (s *Service) Start(ctx context.Context) error {
	if err := s.Service.Start(ctx); err != nil {
		return err
	}

	if s.enableChainPush && s.teeFulfiller != nil && s.dataFeedsHash != "" {
		go s.runChainPushLoop(ctx)
	}

	return nil
}

// Stop stops the DataFeeds service.
func (s *Service) Stop() error {
	close(s.stopCh)
	return s.Service.Stop()
}

// runChainPushLoop periodically fetches prices and pushes them on-chain.
func (s *Service) runChainPushLoop(ctx context.Context) {
	ticker := time.NewTicker(s.updateInterval)
	defer ticker.Stop()

	s.pushPricesToChain(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.pushPricesToChain(ctx)
		}
	}
}

// pushPricesToChain fetches all configured prices and pushes them on-chain.
func (s *Service) pushPricesToChain(ctx context.Context) {
	enabledFeeds := s.GetEnabledFeeds()
	if len(enabledFeeds) == 0 {
		return
	}

	feedIDs := make([]string, 0, len(enabledFeeds))
	prices := make([]*big.Int, 0, len(enabledFeeds))
	timestamps := make([]uint64, 0, len(enabledFeeds))

	for _, feed := range enabledFeeds {
		pair := feed.Pair
		if pair == "" {
			pair = feedIDToPair(feed.ID)
		}

		price, err := s.GetPrice(ctx, pair)
		if err != nil {
			continue
		}

		feedIDs = append(feedIDs, feed.ID)
		prices = append(prices, big.NewInt(price.Price))
		timestamps = append(timestamps, uint64(price.Timestamp.UnixMilli()))
	}

	if len(feedIDs) == 0 {
		return
	}

	_, _ = s.teeFulfiller.UpdatePrices(ctx, s.dataFeedsHash, feedIDs, prices, timestamps)
}

// PushSinglePrice pushes a single price update on-chain.
func (s *Service) PushSinglePrice(ctx context.Context, feedID string) error {
	if s.teeFulfiller == nil || s.dataFeedsHash == "" {
		return fmt.Errorf("chain push not configured")
	}

	pair := feedIDToPair(feedID)
	price, err := s.GetPrice(ctx, pair)
	if err != nil {
		return fmt.Errorf("get price: %w", err)
	}

	_, err = s.teeFulfiller.UpdatePrice(
		ctx,
		s.dataFeedsHash,
		feedID,
		big.NewInt(price.Price),
		uint64(price.Timestamp.UnixMilli()),
	)
	return err
}

// feedIDToPair converts a feed ID to a trading pair format.
// e.g., "BTC/USD" -> "BTCUSD"
func feedIDToPair(feedID string) string {
	pair := ""
	for _, c := range feedID {
		if c != '/' {
			pair += string(c)
		}
	}
	return pair
}

// resolveEnvVar resolves ${VAR_NAME} placeholders with environment values.
func resolveEnvVar(value string) string {
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		envKey := value[2 : len(value)-1]
		if envVal := os.Getenv(envKey); envVal != "" {
			return envVal
		}
	}
	return value
}
