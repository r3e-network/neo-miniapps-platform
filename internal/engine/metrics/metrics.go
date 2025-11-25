// Package metrics provides engine-specific metrics collection.
// It wraps Prometheus collectors to provide structured telemetry for
// engine lifecycle, bus operations, recovery actions, and resource usage.
package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Collector provides engine metrics collection.
type Collector struct {
	registry *prometheus.Registry

	// Lifecycle metrics
	moduleStatus       *prometheus.GaugeVec
	moduleReadiness    *prometheus.GaugeVec
	moduleStartLatency *prometheus.HistogramVec
	moduleStopLatency  *prometheus.HistogramVec
	moduleRestarts     *prometheus.CounterVec
	moduleFailures     *prometheus.CounterVec

	// Bus metrics
	busPublishTotal    *prometheus.CounterVec
	busPublishLatency  *prometheus.HistogramVec
	busPushTotal       *prometheus.CounterVec
	busPushLatency     *prometheus.HistogramVec
	busInvokeTotal     *prometheus.CounterVec
	busInvokeLatency   *prometheus.HistogramVec
	busInFlight        *prometheus.GaugeVec
	busQueueDepth      *prometheus.GaugeVec

	// Recovery metrics
	recoveryAttempts  *prometheus.CounterVec
	recoverySuccesses *prometheus.CounterVec
	recoveryFailures  *prometheus.CounterVec
	recoveryLatency   *prometheus.HistogramVec

	// Dependency metrics
	dependencyWait    *prometheus.GaugeVec
	dependencyCycles  prometheus.Counter
	dependencyMissing prometheus.Counter

	// Resource metrics
	goroutines prometheus.Gauge
	uptime     prometheus.Gauge
	startTime  time.Time

	mu sync.RWMutex
}

// NewCollector creates a new engine metrics collector.
func NewCollector(namespace string) *Collector {
	if namespace == "" {
		namespace = "engine"
	}

	c := &Collector{
		registry:  prometheus.NewRegistry(),
		startTime: time.Now(),
	}

	// Lifecycle metrics
	c.moduleStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "module",
			Name:      "status",
			Help:      "Current status of module (0=unknown, 1=registered, 2=starting, 3=running, 4=stopping, 5=stopped, 6=failed, 7=stop_failed)",
		},
		[]string{"module", "domain"},
	)

	c.moduleReadiness = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "module",
			Name:      "readiness",
			Help:      "Current readiness of module (0=unknown, 1=ready, 2=not_ready)",
		},
		[]string{"module", "domain"},
	)

	c.moduleStartLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "module",
			Name:      "start_duration_seconds",
			Help:      "Time taken to start a module",
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 12), // 10ms to ~40s
		},
		[]string{"module", "domain", "result"},
	)

	c.moduleStopLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "module",
			Name:      "stop_duration_seconds",
			Help:      "Time taken to stop a module",
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 10), // 10ms to ~10s
		},
		[]string{"module", "domain", "result"},
	)

	c.moduleRestarts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "module",
			Name:      "restarts_total",
			Help:      "Total number of module restarts",
		},
		[]string{"module", "domain"},
	)

	c.moduleFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "module",
			Name:      "failures_total",
			Help:      "Total number of module failures",
		},
		[]string{"module", "domain", "phase"},
	)

	// Bus metrics
	c.busPublishTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "bus",
			Name:      "publish_total",
			Help:      "Total number of event bus publishes",
		},
		[]string{"topic", "result"},
	)

	c.busPublishLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "bus",
			Name:      "publish_duration_seconds",
			Help:      "Time taken to publish events",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to ~1s
		},
		[]string{"topic"},
	)

	c.busPushTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "bus",
			Name:      "push_total",
			Help:      "Total number of data bus pushes",
		},
		[]string{"topic", "result"},
	)

	c.busPushLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "bus",
			Name:      "push_duration_seconds",
			Help:      "Time taken to push data",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to ~1s
		},
		[]string{"topic"},
	)

	c.busInvokeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "bus",
			Name:      "invoke_total",
			Help:      "Total number of compute bus invocations",
		},
		[]string{"function", "result"},
	)

	c.busInvokeLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "bus",
			Name:      "invoke_duration_seconds",
			Help:      "Time taken for compute bus invocations",
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 12), // 10ms to ~40s
		},
		[]string{"function"},
	)

	c.busInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "bus",
			Name:      "in_flight",
			Help:      "Current number of in-flight bus operations",
		},
		[]string{"kind"},
	)

	c.busQueueDepth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "bus",
			Name:      "queue_depth",
			Help:      "Current depth of bus queues",
		},
		[]string{"kind"},
	)

	// Recovery metrics
	c.recoveryAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "recovery",
			Name:      "attempts_total",
			Help:      "Total number of recovery attempts",
		},
		[]string{"module", "strategy"},
	)

	c.recoverySuccesses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "recovery",
			Name:      "successes_total",
			Help:      "Total number of successful recoveries",
		},
		[]string{"module", "strategy"},
	)

	c.recoveryFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "recovery",
			Name:      "failures_total",
			Help:      "Total number of failed recoveries",
		},
		[]string{"module", "strategy"},
	)

	c.recoveryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "recovery",
			Name:      "duration_seconds",
			Help:      "Time taken for recovery operations",
			Buckets:   prometheus.ExponentialBuckets(0.1, 2, 10), // 100ms to ~100s
		},
		[]string{"module", "strategy", "result"},
	)

	// Dependency metrics
	c.dependencyWait = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "dependency",
			Name:      "wait_seconds",
			Help:      "Time waiting for dependencies",
		},
		[]string{"module", "waiting_for"},
	)

	c.dependencyCycles = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "dependency",
			Name:      "cycles_detected_total",
			Help:      "Total number of dependency cycles detected",
		},
	)

	c.dependencyMissing = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "dependency",
			Name:      "missing_total",
			Help:      "Total number of missing dependencies",
		},
	)

	// Resource metrics
	c.goroutines = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "goroutines",
			Help:      "Current number of engine goroutines",
		},
	)

	c.uptime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "uptime_seconds",
			Help:      "Engine uptime in seconds",
		},
	)

	// Register all collectors
	c.registry.MustRegister(
		c.moduleStatus,
		c.moduleReadiness,
		c.moduleStartLatency,
		c.moduleStopLatency,
		c.moduleRestarts,
		c.moduleFailures,
		c.busPublishTotal,
		c.busPublishLatency,
		c.busPushTotal,
		c.busPushLatency,
		c.busInvokeTotal,
		c.busInvokeLatency,
		c.busInFlight,
		c.busQueueDepth,
		c.recoveryAttempts,
		c.recoverySuccesses,
		c.recoveryFailures,
		c.recoveryLatency,
		c.dependencyWait,
		c.dependencyCycles,
		c.dependencyMissing,
		c.goroutines,
		c.uptime,
	)

	return c
}

// Registry returns the Prometheus registry.
func (c *Collector) Registry() *prometheus.Registry {
	return c.registry
}

// RecordModuleStatus records the current status of a module.
func (c *Collector) RecordModuleStatus(module, domain string, status int) {
	c.moduleStatus.WithLabelValues(module, domain).Set(float64(status))
}

// RecordModuleReadiness records the current readiness of a module.
func (c *Collector) RecordModuleReadiness(module, domain string, readiness int) {
	c.moduleReadiness.WithLabelValues(module, domain).Set(float64(readiness))
}

// RecordModuleStart records module start latency.
func (c *Collector) RecordModuleStart(module, domain string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	c.moduleStartLatency.WithLabelValues(module, domain, result).Observe(duration.Seconds())
	if err != nil {
		c.moduleFailures.WithLabelValues(module, domain, "start").Inc()
	}
}

// RecordModuleStop records module stop latency.
func (c *Collector) RecordModuleStop(module, domain string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	c.moduleStopLatency.WithLabelValues(module, domain, result).Observe(duration.Seconds())
	if err != nil {
		c.moduleFailures.WithLabelValues(module, domain, "stop").Inc()
	}
}

// RecordModuleRestart increments the restart counter.
func (c *Collector) RecordModuleRestart(module, domain string) {
	c.moduleRestarts.WithLabelValues(module, domain).Inc()
}

// RecordModuleFailure records a module failure.
func (c *Collector) RecordModuleFailure(module, domain, phase string) {
	c.moduleFailures.WithLabelValues(module, domain, phase).Inc()
}

// RecordBusPublish records event bus publish metrics.
func (c *Collector) RecordBusPublish(topic string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	c.busPublishTotal.WithLabelValues(topic, result).Inc()
	c.busPublishLatency.WithLabelValues(topic).Observe(duration.Seconds())
}

// RecordBusPush records data bus push metrics.
func (c *Collector) RecordBusPush(topic string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	c.busPushTotal.WithLabelValues(topic, result).Inc()
	c.busPushLatency.WithLabelValues(topic).Observe(duration.Seconds())
}

// RecordBusInvoke records compute bus invocation metrics.
func (c *Collector) RecordBusInvoke(function string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	c.busInvokeTotal.WithLabelValues(function, result).Inc()
	c.busInvokeLatency.WithLabelValues(function).Observe(duration.Seconds())
}

// RecordBusInFlight records current in-flight operations.
func (c *Collector) RecordBusInFlight(kind string, count int) {
	c.busInFlight.WithLabelValues(kind).Set(float64(count))
}

// RecordBusQueueDepth records current queue depth.
func (c *Collector) RecordBusQueueDepth(kind string, depth int) {
	c.busQueueDepth.WithLabelValues(kind).Set(float64(depth))
}

// RecordRecoveryAttempt records a recovery attempt.
func (c *Collector) RecordRecoveryAttempt(module, strategy string) {
	c.recoveryAttempts.WithLabelValues(module, strategy).Inc()
}

// RecordRecoveryResult records the result of a recovery attempt.
func (c *Collector) RecordRecoveryResult(module, strategy string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "failure"
		c.recoveryFailures.WithLabelValues(module, strategy).Inc()
	} else {
		c.recoverySuccesses.WithLabelValues(module, strategy).Inc()
	}
	c.recoveryLatency.WithLabelValues(module, strategy, result).Observe(duration.Seconds())
}

// RecordDependencyWait records time waiting for a dependency.
func (c *Collector) RecordDependencyWait(module, waitingFor string, duration time.Duration) {
	c.dependencyWait.WithLabelValues(module, waitingFor).Set(duration.Seconds())
}

// RecordDependencyCycle records a detected dependency cycle.
func (c *Collector) RecordDependencyCycle() {
	c.dependencyCycles.Inc()
}

// RecordDependencyMissing records a missing dependency.
func (c *Collector) RecordDependencyMissing() {
	c.dependencyMissing.Inc()
}

// RecordGoroutines records the current goroutine count.
func (c *Collector) RecordGoroutines(count int) {
	c.goroutines.Set(float64(count))
}

// UpdateUptime updates the uptime metric.
func (c *Collector) UpdateUptime() {
	c.uptime.Set(time.Since(c.startTime).Seconds())
}

// Reset resets all metrics.
func (c *Collector) Reset() {
	c.moduleStatus.Reset()
	c.moduleReadiness.Reset()
	c.busInFlight.Reset()
	c.busQueueDepth.Reset()
	c.dependencyWait.Reset()
	c.startTime = time.Now()
}

// NoOpCollector is a metrics collector that discards all metrics.
type NoOpCollector struct{}

// NewNoOpCollector creates a no-op metrics collector.
func NewNoOpCollector() *NoOpCollector {
	return &NoOpCollector{}
}

func (*NoOpCollector) RecordModuleStatus(module, domain string, status int)               {}
func (*NoOpCollector) RecordModuleReadiness(module, domain string, readiness int)         {}
func (*NoOpCollector) RecordModuleStart(module, domain string, d time.Duration, err error) {}
func (*NoOpCollector) RecordModuleStop(module, domain string, d time.Duration, err error)  {}
func (*NoOpCollector) RecordModuleRestart(module, domain string)                           {}
func (*NoOpCollector) RecordModuleFailure(module, domain, phase string)                    {}
func (*NoOpCollector) RecordBusPublish(topic string, d time.Duration, err error)           {}
func (*NoOpCollector) RecordBusPush(topic string, d time.Duration, err error)              {}
func (*NoOpCollector) RecordBusInvoke(function string, d time.Duration, err error)         {}
func (*NoOpCollector) RecordBusInFlight(kind string, count int)                            {}
func (*NoOpCollector) RecordBusQueueDepth(kind string, depth int)                          {}
func (*NoOpCollector) RecordRecoveryAttempt(module, strategy string)                       {}
func (*NoOpCollector) RecordRecoveryResult(module, strategy string, d time.Duration, err error) {
}
func (*NoOpCollector) RecordDependencyWait(module, waitingFor string, d time.Duration) {}
func (*NoOpCollector) RecordDependencyCycle()                                          {}
func (*NoOpCollector) RecordDependencyMissing()                                        {}
func (*NoOpCollector) RecordGoroutines(count int)                                      {}
func (*NoOpCollector) UpdateUptime()                                                   {}
func (*NoOpCollector) Reset()                                                          {}

// MetricsCollector is the interface for metrics collection.
type MetricsCollector interface {
	RecordModuleStatus(module, domain string, status int)
	RecordModuleReadiness(module, domain string, readiness int)
	RecordModuleStart(module, domain string, duration time.Duration, err error)
	RecordModuleStop(module, domain string, duration time.Duration, err error)
	RecordModuleRestart(module, domain string)
	RecordModuleFailure(module, domain, phase string)
	RecordBusPublish(topic string, duration time.Duration, err error)
	RecordBusPush(topic string, duration time.Duration, err error)
	RecordBusInvoke(function string, duration time.Duration, err error)
	RecordBusInFlight(kind string, count int)
	RecordBusQueueDepth(kind string, depth int)
	RecordRecoveryAttempt(module, strategy string)
	RecordRecoveryResult(module, strategy string, duration time.Duration, err error)
	RecordDependencyWait(module, waitingFor string, duration time.Duration)
	RecordDependencyCycle()
	RecordDependencyMissing()
	RecordGoroutines(count int)
	UpdateUptime()
	Reset()
}

// Verify interface compliance
var (
	_ MetricsCollector = (*Collector)(nil)
	_ MetricsCollector = (*NoOpCollector)(nil)
)
