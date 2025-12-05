// Package oracle provides the Oracle service for fetching external data.
// Architecture: MarbleRun + EGo + Supabase + Neo N3
package oracle

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/ego"
	"github.com/R3E-Network/service_layer/marble/sdk"
	"github.com/R3E-Network/service_layer/services/common"
	"github.com/R3E-Network/service_layer/supabase/client"
)

const (
	ServiceID   = "oracle"
	ServiceName = "Oracle Service"
	Version     = "2.0.0" // MarbleRun architecture
)

// Service implements the Oracle service.
type Service struct {
	*base.BaseService

	mu        sync.RWMutex
	enclave   *Enclave
	store     *Store
	supa      *supabase.Client
	supaTable string
}

// New creates a new Oracle service.
func New(serviceOS os.ServiceOS) (*Service, error) {
	baseService := base.NewBaseService(ServiceID, ServiceName, Version, serviceOS)

	enclave := NewEnclave(serviceOS)
	store := NewStore()

	svc := &Service{
		BaseService: baseService,
		enclave:     enclave,
		store:       store,
	}

	// Register components with BaseService for automatic lifecycle management
	baseService.SetEnclave(enclave)
	baseService.SetStore(store)

	// Set lifecycle hooks for service-specific initialization
	baseService.SetHooks(base.LifecycleHooks{
		OnAfterStart: svc.onAfterStart,
	})

	return svc, nil
}

// Store exposes the Oracle store (used by API handlers).
func (s *Service) Store() *Store {
	return s.store
}

// onAfterStart is called after enclave and store are initialized.
// This handles service-specific initialization like metrics, config hydration, etc.
func (s *Service) onAfterStart(ctx context.Context) error {
	// Register metrics using BaseService helper
	s.RegisterMetrics("oracle", func(metrics os.MetricsAPI) {
		metrics.RegisterCounter("oracle_fetch_total", "Total number of fetch requests")
		metrics.RegisterCounter("oracle_fetch_errors", "Total number of fetch errors")
		metrics.RegisterCounter("oracle_cache_hits", "Total number of cache hits")
		metrics.RegisterCounter("oracle_cache_misses", "Total number of cache misses")
		metrics.RegisterHistogram("oracle_fetch_duration_seconds", "Fetch request duration", []float64{0.1, 0.5, 1, 2, 5, 10})
		metrics.RegisterGauge("oracle_active_feeds", "Number of active data feeds")
	})

	// Hydrate configs from sealed storage (kept enclave-side)
	var cfg Config
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "oracle/config", &cfg); err != nil {
		s.Logger().Warn("oracle storage hydrate failed", "err", err)
	} else if loaded {
		if len(cfg.AllowedHosts) > 0 {
			s.Logger().Info("oracle allowed hosts from sealed config", "hosts", cfg.AllowedHosts)
		}
		// Optional embedded Supabase settings
		if cfg.Supabase.ProjectURL != "" && cfg.Supabase.APIKeySecret != "" {
			if err := s.initSupabase(cfg.Supabase); err != nil {
				s.Logger().Warn("oracle supabase init failed (from oracle/config)", "err", err)
			}
		}
	}

	// Optional: initialize Supabase client from sealed config (oracle/supabase)
	var supaCfg SupabaseConfig
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "oracle/supabase", &supaCfg); err != nil {
		s.Logger().Warn("oracle supabase config load failed", "err", err)
	} else if loaded {
		if err := s.initSupabase(supaCfg); err != nil {
			s.Logger().Warn("oracle supabase client init failed", "err", err)
		}
	}

	// Start scheduled feed updates if scheduler available
	if s.OS().HasCapability(os.CapScheduler) {
		s.startScheduledFeeds(ctx)
	}

	// Subscribe to contract requests using BaseService helper
	s.StartContractListener(ctx, s.handleContractRequest)

	return nil
}

// startScheduledFeeds starts scheduled feed updates.
func (s *Service) startScheduledFeeds(ctx context.Context) {
	scheduler := s.OS().Scheduler()

	// Schedule periodic feed refresh (every 5 minutes)
	_, err := scheduler.ScheduleInterval(ctx, 5*time.Minute, &os.ScheduledTask{
		Name:       "refresh_feeds",
		Handler:    "refresh_feeds",
		MaxRetries: 3,
		Timeout:    2 * time.Minute,
	})
	if err != nil {
		s.Logger().Warn("failed to schedule feed refresh", "error", err)
	}

	s.Logger().Info("scheduled feed updates started")
}

// NOTE: Start(), Stop(), and Health() are now handled by BaseService
// with automatic component lifecycle management.

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
	s.Logger().Info("oracle supabase client initialized", "table", cfg.Table)
	return nil
}

// PublishPriceSupabase writes price data to Supabase if configured.
func (s *Service) PublishPriceSupabase(ctx context.Context, data *PriceData) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.supa == nil || s.supaTable == "" {
		return fmt.Errorf("supabase not configured")
	}
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal price: %w", err)
	}
	resp, err := s.supa.Insert(ctx, s.supaTable, body)
	if err != nil {
		return fmt.Errorf("supabase insert: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("supabase insert failed: status %d", resp.StatusCode)
	}
	return nil
}

// =============================================================================
// Oracle Operations
// =============================================================================

// FetchRequest represents a data fetch request.
type FetchRequest struct {
	URL        string            `json:"url"`
	Method     string            `json:"method"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
	SecretName string            `json:"secret_name,omitempty"` // Optional API key secret
	AuthType   os.AuthType       `json:"auth_type,omitempty"`
}

// FetchResponse represents a data fetch response.
type FetchResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
	Signature  []byte            `json:"signature,omitempty"` // TEE signature
	Timestamp  time.Time         `json:"timestamp"`
}

// Config represents sealed config for Oracle service.
type Config struct {
	AllowedHosts []string       `json:"allowed_hosts,omitempty"`
	Supabase     SupabaseConfig `json:"supabase,omitempty"`
}

// SupabaseConfig represents sealed Supabase settings for Oracle.
type SupabaseConfig struct {
	ProjectURL     string            `json:"project_url"`
	APIKeySecret   string            `json:"api_key_secret"`
	APIKey         string            `json:"api_key,omitempty"`
	AllowedHosts   []string          `json:"allowed_hosts,omitempty"`
	DefaultHeaders map[string]string `json:"default_headers,omitempty"`
	Table          string            `json:"table,omitempty"`
}

// Fetch fetches data from an external source with TEE protection.
func (s *Service) Fetch(ctx context.Context, req *FetchRequest) (*FetchResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	startTime := time.Now()

	// Record metrics
	if s.OS().HasCapability(os.CapMetrics) {
		s.OS().Metrics().Counter("oracle_fetch_total", 1)
	}

	// Try cache first (for GET requests)
	if req.Method == "GET" && s.OS().HasCapability(os.CapCache) {
		cacheKey := "fetch:" + req.URL
		data, err := s.OS().Cache().Get(ctx, cacheKey)
		if err == nil && data != nil {
			if s.OS().HasCapability(os.CapMetrics) {
				s.OS().Metrics().Counter("oracle_cache_hits", 1)
			}
			// Deserialize cached response
			var resp FetchResponse
			if err := json.Unmarshal(data, &resp); err == nil {
				return &resp, nil
			}
		}
		if s.OS().HasCapability(os.CapMetrics) {
			s.OS().Metrics().Counter("oracle_cache_misses", 1)
		}
	}

	// Delegate to enclave for secure fetch
	resp, err := s.enclave.SecureFetch(ctx, req)

	// Record duration
	if s.OS().HasCapability(os.CapMetrics) {
		duration := time.Since(startTime).Seconds()
		s.OS().Metrics().Histogram("oracle_fetch_duration_seconds", duration)
		if err != nil {
			s.OS().Metrics().Counter("oracle_fetch_errors", 1)
		}
	}

	if err != nil {
		return nil, err
	}

	// Cache successful GET responses
	if req.Method == "GET" && s.OS().HasCapability(os.CapCache) && resp.StatusCode == 200 {
		cacheKey := "fetch:" + req.URL
		if data, err := json.Marshal(resp); err == nil {
			s.OS().Cache().Set(ctx, cacheKey, data, 5*time.Minute)
		}
	}

	return resp, nil
}

// CreateDataFeed creates a new data feed.
func (s *Service) CreateDataFeed(ctx context.Context, feed *DataFeed) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}

	feed.SetTimestamps()
	return s.store.CreateFeed(ctx, feed)
}

// GetDataFeed retrieves a data feed by ID.
func (s *Service) GetDataFeed(ctx context.Context, id string) (*DataFeed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.GetFeed(ctx, id)
}

// ListDataFeeds lists all data feeds.
func (s *Service) ListDataFeeds(ctx context.Context) ([]*DataFeed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.ListFeeds(ctx)
}

// UpdateDataFeed updates a data feed.
func (s *Service) UpdateDataFeed(ctx context.Context, feed *DataFeed) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	feed.SetTimestamps()
	return s.store.UpdateFeed(ctx, feed)
}

// DeleteDataFeed deletes a data feed.
func (s *Service) DeleteDataFeed(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.store.DeleteFeed(ctx, id)
}

// ExecuteFeed executes a data feed and returns the result.
func (s *Service) ExecuteFeed(ctx context.Context, feedID string) (*FetchResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	// Get feed configuration
	feed, err := s.store.GetFeed(ctx, feedID)
	if err != nil {
		return nil, fmt.Errorf("get feed: %w", err)
	}

	// Build fetch request
	req := &FetchRequest{
		URL:        feed.URL,
		Method:     feed.Method,
		Headers:    feed.Headers,
		SecretName: feed.SecretName,
		AuthType:   feed.AuthType,
	}

	// Execute fetch
	return s.enclave.SecureFetch(ctx, req)
}

// =============================================================================
// Contract Integration
// =============================================================================

// handleContractRequest handles incoming service requests from contracts.
func (s *Service) handleContractRequest(ctx context.Context, req *os.ServiceRequest) error {
	s.Logger().Info("received contract request",
		"request_id", req.RequestID,
		"requester", req.Requester,
	)

	// Parse the oracle request payload
	var oracleReq OracleContractRequest
	if err := json.Unmarshal(req.Payload, &oracleReq); err != nil {
		return s.SendContractError(ctx, req.RequestID, "invalid payload: "+err.Error())
	}

	// Execute the fetch request
	fetchReq := &FetchRequest{
		URL:      oracleReq.URL,
		Method:   oracleReq.Method,
		Headers:  oracleReq.Headers,
		Body:     oracleReq.Body,
		AuthType: os.AuthType(oracleReq.AuthType),
	}

	resp, err := s.Fetch(ctx, fetchReq)
	if err != nil {
		return s.SendContractError(ctx, req.RequestID, "fetch failed: "+err.Error())
	}

	// Apply JSON path extraction if specified
	var result []byte
	if oracleReq.JSONPath != "" {
		extracted, err := s.enclave.ExtractJSONPath(ctx, resp.Body, oracleReq.JSONPath)
		if err != nil {
			return s.SendContractError(ctx, req.RequestID, "json path extraction failed: "+err.Error())
		}
		result = extracted
	} else {
		result = resp.Body
	}

	// Sign the result
	signature, err := s.enclave.SignData(ctx, result)
	if err != nil {
		return s.SendContractError(ctx, req.RequestID, "signing failed: "+err.Error())
	}

	// Send callback with result using BaseService helper
	return s.SendContractCallback(ctx, req.RequestID, result, signature)
}

// OracleContractRequest represents an oracle request from a contract.
type OracleContractRequest struct {
	URL      string            `json:"url"`
	Method   string            `json:"method"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     []byte            `json:"body,omitempty"`
	JSONPath string            `json:"json_path,omitempty"`
	AuthType string            `json:"auth_type,omitempty"`
}

// NOTE: sendContractCallback and sendContractError are now provided by BaseService
// as SendContractCallback and SendContractError methods.
