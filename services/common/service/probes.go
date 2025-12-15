// Package service provides common service infrastructure.
package service

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// =============================================================================
// Kubernetes Probes
// =============================================================================

// ProbeStatus represents the status of a probe.
type ProbeStatus struct {
	Ready   bool   `json:"ready"`
	Live    bool   `json:"live"`
	Message string `json:"message,omitempty"`
}

// ProbeManager manages Kubernetes liveness and readiness probes.
type ProbeManager struct {
	ready     atomic.Bool
	live      atomic.Bool
	startTime time.Time

	// Startup grace period before marking unhealthy
	startupGrace time.Duration
}

// NewProbeManager creates a new probe manager.
func NewProbeManager(startupGrace time.Duration) *ProbeManager {
	if startupGrace == 0 {
		startupGrace = 30 * time.Second
	}
	pm := &ProbeManager{
		startTime:    time.Now(),
		startupGrace: startupGrace,
	}
	pm.live.Store(true) // Live by default
	return pm
}

// SetReady marks the service as ready to receive traffic.
func (p *ProbeManager) SetReady(ready bool) {
	p.ready.Store(ready)
}

// SetLive marks the service as alive.
func (p *ProbeManager) SetLive(live bool) {
	p.live.Store(live)
}

// IsReady returns whether the service is ready.
func (p *ProbeManager) IsReady() bool {
	return p.ready.Load()
}

// IsLive returns whether the service is alive.
func (p *ProbeManager) IsLive() bool {
	return p.live.Load()
}

// InStartupGrace returns whether we're still in the startup grace period.
func (p *ProbeManager) InStartupGrace() bool {
	return time.Since(p.startTime) < p.startupGrace
}

// =============================================================================
// HTTP Handlers
// =============================================================================

// LivenessHandler returns an HTTP handler for liveness probes.
// Returns 200 if live, 503 if not.
func (p *ProbeManager) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := ProbeStatus{
			Live:  p.IsLive(),
			Ready: p.IsReady(),
		}

		if !status.Live {
			status.Message = "service not live"
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}

// ReadinessHandler returns an HTTP handler for readiness probes.
// Returns 200 if ready, 503 if not.
func (p *ProbeManager) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := ProbeStatus{
			Live:  p.IsLive(),
			Ready: p.IsReady(),
		}

		if !status.Ready {
			if p.InStartupGrace() {
				status.Message = "starting up"
			} else {
				status.Message = "service not ready"
			}
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}

// StartupHandler returns an HTTP handler for startup probes.
// Returns 200 once startup is complete, 503 during startup.
func (p *ProbeManager) StartupHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Startup is complete when ready
		if p.IsReady() {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"started": true,
				"uptime":  time.Since(p.startTime).String(),
			})
			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{
			"started":         false,
			"startup_time":    time.Since(p.startTime).String(),
			"grace_remaining": (p.startupGrace - time.Since(p.startTime)).String(),
		})
	}
}

// =============================================================================
// Route Registration
// =============================================================================

// RegisterProbeRoutes registers standard Kubernetes probe endpoints.
func (p *ProbeManager) RegisterProbeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", p.LivenessHandler())
	mux.HandleFunc("/readyz", p.ReadinessHandler())
	mux.HandleFunc("/startupz", p.StartupHandler())
}

// =============================================================================
// BaseService Integration
// =============================================================================

// probeManager is the probe manager for the service.
var defaultProbeManager *ProbeManager

// GetProbeManager returns the default probe manager, creating one if needed.
func GetProbeManager() *ProbeManager {
	if defaultProbeManager == nil {
		defaultProbeManager = NewProbeManager(30 * time.Second)
	}
	return defaultProbeManager
}

// MarkReady marks the service as ready (call after successful startup).
func MarkReady() {
	GetProbeManager().SetReady(true)
}

// MarkNotReady marks the service as not ready.
func MarkNotReady() {
	GetProbeManager().SetReady(false)
}

// MarkDead marks the service as not live (triggers restart).
func MarkDead() {
	GetProbeManager().SetLive(false)
}
