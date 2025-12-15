// Package metrics provides Prometheus metrics collection
package metrics

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	// HTTP metrics
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RequestsInFlight prometheus.Gauge

	// Error metrics
	ErrorsTotal *prometheus.CounterVec

	// Business metrics
	BlockchainTxTotal       *prometheus.CounterVec
	BlockchainTxDuration    *prometheus.HistogramVec
	VRFRequestsTotal        *prometheus.CounterVec
	NeoVaultOperationsTotal *prometheus.CounterVec

	// Database metrics
	DatabaseQueriesTotal    *prometheus.CounterVec
	DatabaseQueryDuration   *prometheus.HistogramVec
	DatabaseConnectionsOpen prometheus.Gauge

	// Service health
	ServiceUptime prometheus.Gauge
	ServiceInfo   *prometheus.GaugeVec
}

// New creates a new Metrics instance with all collectors registered
func New(serviceName string) *Metrics {
	return NewWithRegistry(serviceName, prometheus.DefaultRegisterer)
}

// NewWithRegistry creates a new Metrics instance with a custom registry
func NewWithRegistry(serviceName string, registerer prometheus.Registerer) *Metrics {
	m := &Metrics{
		// HTTP metrics
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"service", "method", "path", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"service", "method", "path"},
		),
		RequestsInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Current number of HTTP requests being processed",
			},
		),

		// Error metrics
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "errors_total",
				Help: "Total number of errors",
			},
			[]string{"service", "type", "operation"},
		),

		// Business metrics
		BlockchainTxTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "blockchain_transactions_total",
				Help: "Total number of blockchain transactions",
			},
			[]string{"service", "chain", "operation", "status"},
		),
		BlockchainTxDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "blockchain_transaction_duration_seconds",
				Help:    "Blockchain transaction duration in seconds",
				Buckets: []float64{.1, .5, 1, 2, 5, 10, 30, 60, 120},
			},
			[]string{"service", "chain", "operation"},
		),
		VRFRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vrf_requests_total",
				Help: "Total number of VRF requests",
			},
			[]string{"service", "status"},
		),
		NeoVaultOperationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "neovault_operations_total",
				Help: "Total number of neovault operations",
			},
			[]string{"service", "operation", "status"},
		),

		// Database metrics
		DatabaseQueriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "database_queries_total",
				Help: "Total number of database queries",
			},
			[]string{"service", "operation", "status"},
		),
		DatabaseQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "database_query_duration_seconds",
				Help:    "Database query duration in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"service", "operation"},
		),
		DatabaseConnectionsOpen: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "database_connections_open",
				Help: "Current number of open database connections",
			},
		),

		// Service health
		ServiceUptime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "service_uptime_seconds",
				Help: "Service uptime in seconds",
			},
		),
		ServiceInfo: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "service_info",
				Help: "Service information",
			},
			[]string{"service", "version", "environment"},
		),
	}

	// Register all collectors
	if registerer != nil {
		registerer.MustRegister(
			m.RequestsTotal,
			m.RequestDuration,
			m.RequestsInFlight,
			m.ErrorsTotal,
			m.BlockchainTxTotal,
			m.BlockchainTxDuration,
			m.VRFRequestsTotal,
			m.NeoVaultOperationsTotal,
			m.DatabaseQueriesTotal,
			m.DatabaseQueryDuration,
			m.DatabaseConnectionsOpen,
			m.ServiceUptime,
			m.ServiceInfo,
		)
	}

	// Set service info
	m.ServiceInfo.WithLabelValues(serviceName, "1.0.0", getEnvironment()).Set(1)

	return m
}

// RecordHTTPRequest records an HTTP request
func (m *Metrics) RecordHTTPRequest(service, method, path, status string, duration time.Duration) {
	m.RequestsTotal.WithLabelValues(service, method, path, status).Inc()
	m.RequestDuration.WithLabelValues(service, method, path).Observe(duration.Seconds())
}

// RecordError records an error
func (m *Metrics) RecordError(service, errorType, operation string) {
	m.ErrorsTotal.WithLabelValues(service, errorType, operation).Inc()
}

// RecordBlockchainTx records a blockchain transaction
func (m *Metrics) RecordBlockchainTx(service, chain, operation, status string, duration time.Duration) {
	m.BlockchainTxTotal.WithLabelValues(service, chain, operation, status).Inc()
	m.BlockchainTxDuration.WithLabelValues(service, chain, operation).Observe(duration.Seconds())
}

// RecordVRFRequest records a VRF request
func (m *Metrics) RecordVRFRequest(service, status string) {
	m.VRFRequestsTotal.WithLabelValues(service, status).Inc()
}

// RecordNeoVaultOperation records a neovault operation
func (m *Metrics) RecordNeoVaultOperation(service, operation, status string) {
	m.NeoVaultOperationsTotal.WithLabelValues(service, operation, status).Inc()
}

// RecordDatabaseQuery records a database query
func (m *Metrics) RecordDatabaseQuery(service, operation, status string, duration time.Duration) {
	m.DatabaseQueriesTotal.WithLabelValues(service, operation, status).Inc()
	m.DatabaseQueryDuration.WithLabelValues(service, operation).Observe(duration.Seconds())
}

// SetDatabaseConnections sets the number of open database connections
func (m *Metrics) SetDatabaseConnections(count int) {
	m.DatabaseConnectionsOpen.Set(float64(count))
}

// UpdateUptime updates the service uptime
func (m *Metrics) UpdateUptime(startTime time.Time) {
	m.ServiceUptime.Set(time.Since(startTime).Seconds())
}

// IncrementInFlight increments the in-flight requests counter
func (m *Metrics) IncrementInFlight() {
	m.RequestsInFlight.Inc()
}

// DecrementInFlight decrements the in-flight requests counter
func (m *Metrics) DecrementInFlight() {
	m.RequestsInFlight.Dec()
}

// Helper functions

func getEnvironment() string {
	env := strings.TrimSpace(os.Getenv("MARBLE_ENV"))
	if env == "" {
		env = strings.TrimSpace(os.Getenv("ENVIRONMENT"))
	}
	if env == "" {
		env = "development"
	}
	return env
}

// Enabled returns whether Prometheus metrics should be exposed.
//
// Defaults:
// - production: disabled unless explicitly enabled via METRICS_ENABLED
// - non-production: enabled unless explicitly disabled via METRICS_ENABLED
func Enabled() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("METRICS_ENABLED")))
	if raw == "" {
		env := strings.ToLower(strings.TrimSpace(os.Getenv("MARBLE_ENV")))
		return env != "production"
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Global metrics instance
var (
	globalMetrics *Metrics
	globalMu      sync.Mutex
)

// Init initializes the global metrics instance
func Init(serviceName string) *Metrics {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalMetrics == nil {
		globalMetrics = New(serviceName)
	}
	return globalMetrics
}

// Global returns the global metrics instance
func Global() *Metrics {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalMetrics == nil {
		globalMetrics = New("unknown")
	}
	return globalMetrics
}
