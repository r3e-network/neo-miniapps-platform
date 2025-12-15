// Package chain provides NEO N3 blockchain interaction utilities.
package chain

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// RPC Pool Types
// =============================================================================

// RPCEndpoint represents a NEO N3 RPC endpoint with health tracking.
type RPCEndpoint struct {
	URL              string        `json:"url"`
	Priority         int           `json:"priority"`
	Healthy          bool          `json:"healthy"`
	ConsecutiveFails int           `json:"consecutive_fails"`
	LastCheck        time.Time     `json:"last_check"`
	LastLatency      time.Duration `json:"last_latency"`
	AvgLatency       time.Duration `json:"avg_latency"`
}

// RPCPoolConfig holds configuration for the RPC pool.
type RPCPoolConfig struct {
	// Endpoints is a comma-separated list of RPC URLs or a slice.
	Endpoints []string

	// HealthCheckInterval is how often to check endpoint health.
	HealthCheckInterval time.Duration

	// HealthCheckTimeout is the timeout for health check requests.
	HealthCheckTimeout time.Duration

	// MaxConsecutiveFails marks an endpoint unhealthy after this many failures.
	MaxConsecutiveFails int

	// HTTPClient is the HTTP client to use (optional, for TEE external client).
	HTTPClient *http.Client
}

// DefaultRPCPoolConfig returns sensible defaults.
func DefaultRPCPoolConfig() *RPCPoolConfig {
	return &RPCPoolConfig{
		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		MaxConsecutiveFails: 3,
	}
}

// =============================================================================
// RPC Pool Implementation
// =============================================================================

// RPCPool manages multiple RPC endpoints with health checking and failover.
type RPCPool struct {
	mu        sync.RWMutex
	endpoints []*RPCEndpoint
	current   int
	config    *RPCPoolConfig
	client    *http.Client
	stopCh    chan struct{}
	stopOnce  sync.Once
}

// NewRPCPool creates a new RPC pool from configuration.
func NewRPCPool(cfg *RPCPoolConfig) (*RPCPool, error) {
	if cfg == nil {
		cfg = DefaultRPCPoolConfig()
	}

	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("rpcpool: at least one endpoint required")
	}

	endpoints := make([]*RPCEndpoint, len(cfg.Endpoints))
	for i, url := range cfg.Endpoints {
		endpoints[i] = &RPCEndpoint{
			URL:      strings.TrimSpace(url),
			Priority: i,
			Healthy:  true, // Assume healthy until proven otherwise
		}
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: cfg.HealthCheckTimeout,
		}
	}

	return &RPCPool{
		endpoints: endpoints,
		config:    cfg,
		client:    client,
		stopCh:    make(chan struct{}),
	}, nil
}

// ParseEndpoints parses a comma-separated list of RPC URLs.
func ParseEndpoints(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// Start begins the health check loop.
func (p *RPCPool) Start(ctx context.Context) {
	go p.healthCheckLoop(ctx)
}

// Stop stops the health check loop.
func (p *RPCPool) Stop() {
	p.stopOnce.Do(func() {
		close(p.stopCh)
	})
}

// GetBestEndpoint returns the best healthy endpoint.
func (p *RPCPool) GetBestEndpoint() (*RPCEndpoint, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Sort by: healthy first, then by avg latency, then by priority
	healthy := make([]*RPCEndpoint, 0, len(p.endpoints))
	for _, ep := range p.endpoints {
		if ep.Healthy {
			healthy = append(healthy, ep)
		}
	}

	if len(healthy) == 0 {
		// Fallback: return first endpoint even if unhealthy
		if len(p.endpoints) > 0 {
			return p.endpoints[0], fmt.Errorf("no healthy endpoints, using fallback")
		}
		return nil, fmt.Errorf("no endpoints available")
	}

	sort.Slice(healthy, func(i, j int) bool {
		if healthy[i].AvgLatency != healthy[j].AvgLatency {
			return healthy[i].AvgLatency < healthy[j].AvgLatency
		}
		return healthy[i].Priority < healthy[j].Priority
	})

	return healthy[0], nil
}

// GetNextEndpoint returns the next endpoint in round-robin fashion (for failover).
func (p *RPCPool) GetNextEndpoint() *RPCEndpoint {
	p.mu.Lock()
	defer p.mu.Unlock()

	startIdx := p.current
	for i := 0; i < len(p.endpoints); i++ {
		idx := (startIdx + i + 1) % len(p.endpoints)
		if p.endpoints[idx].Healthy {
			p.current = idx
			return p.endpoints[idx]
		}
	}

	// No healthy endpoint, return next anyway
	p.current = (p.current + 1) % len(p.endpoints)
	return p.endpoints[p.current]
}

// MarkUnhealthy marks an endpoint as unhealthy after a failure.
func (p *RPCPool) MarkUnhealthy(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, ep := range p.endpoints {
		if ep.URL == url {
			ep.ConsecutiveFails++
			if ep.ConsecutiveFails >= p.config.MaxConsecutiveFails {
				ep.Healthy = false
			}
			return
		}
	}
}

// MarkHealthy marks an endpoint as healthy after a successful request.
func (p *RPCPool) MarkHealthy(url string, latency time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, ep := range p.endpoints {
		if ep.URL == url {
			ep.Healthy = true
			ep.ConsecutiveFails = 0
			ep.LastLatency = latency
			// Exponential moving average for latency
			if ep.AvgLatency == 0 {
				ep.AvgLatency = latency
			} else {
				ep.AvgLatency = (ep.AvgLatency*7 + latency*3) / 10
			}
			return
		}
	}
}

// GetEndpoints returns a copy of all endpoints with their status.
func (p *RPCPool) GetEndpoints() []RPCEndpoint {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]RPCEndpoint, len(p.endpoints))
	for i, ep := range p.endpoints {
		result[i] = *ep
	}
	return result
}

// HealthyCount returns the number of healthy endpoints.
func (p *RPCPool) HealthyCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := 0
	for _, ep := range p.endpoints {
		if ep.Healthy {
			count++
		}
	}
	return count
}

// =============================================================================
// Health Check Loop
// =============================================================================

func (p *RPCPool) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	// Initial health check
	p.checkAllEndpoints(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.checkAllEndpoints(ctx)
		}
	}
}

func (p *RPCPool) checkAllEndpoints(ctx context.Context) {
	var wg sync.WaitGroup
	for _, ep := range p.endpoints {
		wg.Add(1)
		go func(endpoint *RPCEndpoint) {
			defer wg.Done()
			p.checkEndpoint(ctx, endpoint)
		}(ep)
	}
	wg.Wait()
}

func (p *RPCPool) checkEndpoint(ctx context.Context, ep *RPCEndpoint) {
	start := time.Now()

	// Use getblockcount as a cheap health check
	reqBody := `{"jsonrpc":"2.0","method":"getblockcount","params":[],"id":1}`

	ctx, cancel := context.WithTimeout(ctx, p.config.HealthCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", ep.URL, strings.NewReader(reqBody))
	if err != nil {
		p.MarkUnhealthy(ep.URL)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		p.MarkUnhealthy(ep.URL)
		return
	}
	defer resp.Body.Close()

	latency := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		p.MarkUnhealthy(ep.URL)
		return
	}

	p.MarkHealthy(ep.URL, latency)

	p.mu.Lock()
	ep.LastCheck = time.Now()
	p.mu.Unlock()
}

// =============================================================================
// Execute with Failover
// =============================================================================

// ExecuteWithFailover executes a function with automatic failover on failure.
// The function receives the endpoint URL and should return an error if failover is needed.
func (p *RPCPool) ExecuteWithFailover(ctx context.Context, maxRetries int, fn func(url string) error) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var ep *RPCEndpoint
		var err error

		if attempt == 0 {
			ep, err = p.GetBestEndpoint()
		} else {
			ep = p.GetNextEndpoint()
		}

		if ep == nil {
			return fmt.Errorf("no endpoints available")
		}

		start := time.Now()
		err = fn(ep.URL)
		latency := time.Since(start)

		if err == nil {
			p.MarkHealthy(ep.URL, latency)
			return nil
		}

		lastErr = err
		p.MarkUnhealthy(ep.URL)

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return fmt.Errorf("all retries exhausted: %w", lastErr)
}
