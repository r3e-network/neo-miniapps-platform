// Package service provides common service infrastructure.
package service

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// Service Metrics
// =============================================================================

// ServiceMetrics collects operational metrics for a service.
type ServiceMetrics struct {
	mu sync.RWMutex

	// Request counters
	requestsTotal   atomic.Int64
	requestsSuccess atomic.Int64
	requestsFailed  atomic.Int64

	// Latency tracking (simple histogram buckets)
	latencyBuckets map[string]*atomic.Int64 // <10ms, <50ms, <100ms, <500ms, <1s, >1s

	// Error tracking
	errorCounts map[string]*atomic.Int64

	// Custom gauges
	gauges map[string]*atomic.Int64

	// Service info
	serviceName string
	startTime   time.Time
}

// NewServiceMetrics creates a new metrics collector.
func NewServiceMetrics(serviceName string) *ServiceMetrics {
	m := &ServiceMetrics{
		serviceName: serviceName,
		startTime:   time.Now(),
		latencyBuckets: map[string]*atomic.Int64{
			"lt_10ms":  {},
			"lt_50ms":  {},
			"lt_100ms": {},
			"lt_500ms": {},
			"lt_1s":    {},
			"gt_1s":    {},
		},
		errorCounts: make(map[string]*atomic.Int64),
		gauges:      make(map[string]*atomic.Int64),
	}
	return m
}

// =============================================================================
// Request Tracking
// =============================================================================

// RecordRequest records a request with its duration and success status.
func (m *ServiceMetrics) RecordRequest(duration time.Duration, success bool) {
	m.requestsTotal.Add(1)
	if success {
		m.requestsSuccess.Add(1)
	} else {
		m.requestsFailed.Add(1)
	}
	m.recordLatency(duration)
}

// recordLatency records latency in the appropriate bucket.
func (m *ServiceMetrics) recordLatency(d time.Duration) {
	var bucket string
	switch {
	case d < 10*time.Millisecond:
		bucket = "lt_10ms"
	case d < 50*time.Millisecond:
		bucket = "lt_50ms"
	case d < 100*time.Millisecond:
		bucket = "lt_100ms"
	case d < 500*time.Millisecond:
		bucket = "lt_500ms"
	case d < time.Second:
		bucket = "lt_1s"
	default:
		bucket = "gt_1s"
	}
	m.latencyBuckets[bucket].Add(1)
}

// =============================================================================
// Error Tracking
// =============================================================================

// RecordError records an error by type.
func (m *ServiceMetrics) RecordError(errorType string) {
	m.mu.Lock()
	if _, ok := m.errorCounts[errorType]; !ok {
		m.errorCounts[errorType] = &atomic.Int64{}
	}
	counter := m.errorCounts[errorType]
	m.mu.Unlock()
	counter.Add(1)
}

// =============================================================================
// Gauge Operations
// =============================================================================

// SetGauge sets a gauge value.
func (m *ServiceMetrics) SetGauge(name string, value int64) {
	m.mu.Lock()
	if _, ok := m.gauges[name]; !ok {
		m.gauges[name] = &atomic.Int64{}
	}
	gauge := m.gauges[name]
	m.mu.Unlock()
	gauge.Store(value)
}

// IncrGauge increments a gauge.
func (m *ServiceMetrics) IncrGauge(name string) {
	m.mu.Lock()
	if _, ok := m.gauges[name]; !ok {
		m.gauges[name] = &atomic.Int64{}
	}
	gauge := m.gauges[name]
	m.mu.Unlock()
	gauge.Add(1)
}

// DecrGauge decrements a gauge.
func (m *ServiceMetrics) DecrGauge(name string) {
	m.mu.Lock()
	if _, ok := m.gauges[name]; !ok {
		m.gauges[name] = &atomic.Int64{}
	}
	gauge := m.gauges[name]
	m.mu.Unlock()
	gauge.Add(-1)
}

// =============================================================================
// Metrics Export
// =============================================================================

// MetricsResponse is the JSON response for metrics endpoint.
type MetricsResponse struct {
	Service   string         `json:"service"`
	Uptime    string         `json:"uptime"`
	Requests  RequestMetrics `json:"requests"`
	Latency   map[string]int64 `json:"latency_buckets"`
	Errors    map[string]int64 `json:"errors,omitempty"`
	Gauges    map[string]int64 `json:"gauges,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// RequestMetrics contains request-related metrics.
type RequestMetrics struct {
	Total      int64   `json:"total"`
	Success    int64   `json:"success"`
	Failed     int64   `json:"failed"`
	SuccessRate float64 `json:"success_rate"`
}

// Export returns all metrics as a structured response.
func (m *ServiceMetrics) Export() *MetricsResponse {
	total := m.requestsTotal.Load()
	success := m.requestsSuccess.Load()

	successRate := float64(0)
	if total > 0 {
		successRate = float64(success) / float64(total) * 100
	}

	// Collect latency buckets
	latency := make(map[string]int64)
	for k, v := range m.latencyBuckets {
		latency[k] = v.Load()
	}

	// Collect errors
	m.mu.RLock()
	errors := make(map[string]int64)
	for k, v := range m.errorCounts {
		errors[k] = v.Load()
	}

	// Collect gauges
	gauges := make(map[string]int64)
	for k, v := range m.gauges {
		gauges[k] = v.Load()
	}
	m.mu.RUnlock()

	return &MetricsResponse{
		Service: m.serviceName,
		Uptime:  time.Since(m.startTime).String(),
		Requests: RequestMetrics{
			Total:       total,
			Success:     success,
			Failed:      m.requestsFailed.Load(),
			SuccessRate: successRate,
		},
		Latency:   latency,
		Errors:    errors,
		Gauges:    gauges,
		Timestamp: time.Now(),
	}
}

// =============================================================================
// HTTP Handler
// =============================================================================

// MetricsHandler returns an HTTP handler for the metrics endpoint.
func (m *ServiceMetrics) MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m.Export())
	}
}

// =============================================================================
// Middleware
// =============================================================================

// MetricsMiddleware wraps an HTTP handler to record request metrics.
func (m *ServiceMetrics) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status
		wrapped := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		success := wrapped.status < 400
		m.RecordRequest(duration, success)

		if !success {
			m.RecordError(http.StatusText(wrapped.status))
		}
	})
}

// statusResponseWriter wraps http.ResponseWriter to capture status code.
type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
