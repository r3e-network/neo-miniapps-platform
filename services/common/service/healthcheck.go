// Package service provides common service infrastructure.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// =============================================================================
// Deep Health Check Types
// =============================================================================

// ComponentHealth represents the health of a single component.
type ComponentHealth struct {
	Name      string         `json:"name"`
	Status    string         `json:"status"` // healthy, degraded, unhealthy
	Latency   string         `json:"latency,omitempty"`
	Message   string         `json:"message,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	CheckedAt time.Time      `json:"checked_at"`
}

// DeepHealthResponse is the response for deep health checks.
type DeepHealthResponse struct {
	Status     string             `json:"status"` // healthy, degraded, unhealthy
	Service    string             `json:"service"`
	Version    string             `json:"version"`
	Enclave    bool               `json:"enclave"`
	Uptime     string             `json:"uptime"`
	Components []*ComponentHealth `json:"components"`
	CheckedAt  time.Time          `json:"checked_at"`
}

// HealthCheckFunc is a function that checks a component's health.
type HealthCheckFunc func(ctx context.Context) *ComponentHealth

// =============================================================================
// Deep Health Checker
// =============================================================================

// DeepHealthChecker manages multiple component health checks.
type DeepHealthChecker struct {
	mu         sync.RWMutex
	checks     map[string]HealthCheckFunc
	timeout    time.Duration
	lastResult *DeepHealthResponse
}

// NewDeepHealthChecker creates a new deep health checker.
func NewDeepHealthChecker(timeout time.Duration) *DeepHealthChecker {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &DeepHealthChecker{
		checks:  make(map[string]HealthCheckFunc),
		timeout: timeout,
	}
}

// Register adds a health check for a component.
func (d *DeepHealthChecker) Register(name string, check HealthCheckFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.checks[name] = check
}

// Check runs all registered health checks and returns aggregated result.
func (d *DeepHealthChecker) Check(ctx context.Context, service, version string, enclave bool, uptime time.Duration) *DeepHealthResponse {
	d.mu.RLock()
	checks := make(map[string]HealthCheckFunc, len(d.checks))
	for k, v := range d.checks {
		checks[k] = v
	}
	d.mu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	// Run checks in parallel
	var wg sync.WaitGroup
	results := make(chan *ComponentHealth, len(checks))

	for name, check := range checks {
		wg.Add(1)
		go func(n string, c HealthCheckFunc) {
			defer wg.Done()
			start := time.Now()
			result := c(ctx)
			if result == nil {
				result = &ComponentHealth{
					Name:   n,
					Status: "unknown",
				}
			}
			result.Name = n
			result.Latency = time.Since(start).String()
			result.CheckedAt = time.Now()
			results <- result
		}(name, check)
	}

	wg.Wait()
	close(results)

	// Aggregate results
	components := make([]*ComponentHealth, 0, len(checks))
	overallStatus := "healthy"

	for result := range results {
		components = append(components, result)
		switch result.Status {
		case "unhealthy":
			overallStatus = "unhealthy"
		case "degraded":
			if overallStatus != "unhealthy" {
				overallStatus = "degraded"
			}
		}
	}

	resp := &DeepHealthResponse{
		Status:     overallStatus,
		Service:    service,
		Version:    version,
		Enclave:    enclave,
		Uptime:     uptime.String(),
		Components: components,
		CheckedAt:  time.Now(),
	}

	d.mu.Lock()
	d.lastResult = resp
	d.mu.Unlock()

	return resp
}

// LastResult returns the most recent health check result.
func (d *DeepHealthChecker) LastResult() *DeepHealthResponse {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastResult
}

// =============================================================================
// Standard Health Check Functions
// =============================================================================

// HTTPHealthCheck creates a health check for an HTTP endpoint.
func HTTPHealthCheck(name, url string, timeout time.Duration) HealthCheckFunc {
	client := &http.Client{Timeout: timeout}
	return func(ctx context.Context) *ComponentHealth {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return &ComponentHealth{
				Name:    name,
				Status:  "unhealthy",
				Message: fmt.Sprintf("create request: %v", err),
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			return &ComponentHealth{
				Name:    name,
				Status:  "unhealthy",
				Message: fmt.Sprintf("request failed: %v", err),
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			return &ComponentHealth{
				Name:    name,
				Status:  "unhealthy",
				Message: fmt.Sprintf("status %d", resp.StatusCode),
			}
		}

		if resp.StatusCode >= 400 {
			return &ComponentHealth{
				Name:    name,
				Status:  "degraded",
				Message: fmt.Sprintf("status %d", resp.StatusCode),
			}
		}

		return &ComponentHealth{
			Name:   name,
			Status: "healthy",
		}
	}
}

// DatabaseHealthCheck creates a health check for a database connection.
func DatabaseHealthCheck(name string, pingFunc func(context.Context) error) HealthCheckFunc {
	return func(ctx context.Context) *ComponentHealth {
		if err := pingFunc(ctx); err != nil {
			return &ComponentHealth{
				Name:    name,
				Status:  "unhealthy",
				Message: err.Error(),
			}
		}
		return &ComponentHealth{
			Name:   name,
			Status: "healthy",
		}
	}
}

// =============================================================================
// HTTP Handler
// =============================================================================

// DeepHealthHandler returns an HTTP handler for deep health checks.
func DeepHealthHandler(checker *DeepHealthChecker, service, version string, enclave bool, uptimeFunc func() time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uptime := time.Duration(0)
		if uptimeFunc != nil {
			uptime = uptimeFunc()
		}

		result := checker.Check(r.Context(), service, version, enclave, uptime)

		status := http.StatusOK
		if result.Status == "unhealthy" {
			status = http.StatusServiceUnavailable
		} else if result.Status == "degraded" {
			status = http.StatusOK // Still return 200 for degraded
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(result)
	}
}
