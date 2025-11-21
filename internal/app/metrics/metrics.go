package metrics

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Registry holds the application-specific Prometheus collectors.
	Registry = prometheus.NewRegistry()

	httpInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "service_layer",
			Subsystem: "http",
			Name:      "inflight_requests",
			Help:      "Current number of in-flight HTTP requests.",
		},
	)

	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "service_layer",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests handled.",
		},
		[]string{"method", "path", "status"},
	)

	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "service_layer",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Duration of HTTP requests.",
			Buckets:   prometheus.ExponentialBuckets(0.005, 2, 10), // 5ms to ~5s
		},
		[]string{"method", "path"},
	)

	functionExecutions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "service_layer",
			Subsystem: "functions",
			Name:      "executions_total",
			Help:      "Total number of function executions.",
		},
		[]string{"status"},
	)

	functionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "service_layer",
			Subsystem: "functions",
			Name:      "execution_duration_seconds",
			Help:      "Duration of function executions.",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 12), // 1ms to ~4s
		},
		[]string{"status"},
	)

	automationExecutions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "service_layer",
			Subsystem: "automation",
			Name:      "job_runs_total",
			Help:      "Total number of automation job dispatches.",
		},
		[]string{"job_id", "success"},
	)

	automationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "service_layer",
			Subsystem: "automation",
			Name:      "job_run_duration_seconds",
			Help:      "Duration of automation job executions.",
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 10),
		},
		[]string{"job_id"},
	)

	oracleAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "service_layer",
			Subsystem: "oracle",
			Name:      "request_attempts_total",
			Help:      "Total oracle dispatch attempts grouped by account and status.",
		},
		[]string{"account_id", "status"},
	)

	oracleStaleness = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "service_layer",
			Subsystem: "oracle",
			Name:      "oldest_pending_seconds",
			Help:      "Age in seconds of the oldest pending oracle request.",
		},
		[]string{"account_id"},
	)

	datafeedStaleness = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "service_layer",
			Subsystem: "datafeeds",
			Name:      "stale_seconds",
			Help:      "Age in seconds of the latest datafeed update; status label marks healthy|stale|empty.",
		},
		[]string{"feed_id", "status"},
	)

	observationCollectors sync.Map
)

func init() {
	Registry.MustRegister(
		httpInFlight,
		httpRequests,
		httpDuration,
		functionExecutions,
		functionDuration,
		automationExecutions,
		automationDuration,
		oracleAttempts,
		oracleStaleness,
		datafeedStaleness,
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	)
}

// Handler returns an HTTP handler exposing the registered Prometheus metrics.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{})
}

// InstrumentHandler wraps the provided handler with HTTP metrics collection.
func InstrumentHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		httpInFlight.Inc()
		defer httpInFlight.Dec()

		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		path := canonicalPath(r.URL.Path)
		method := strings.ToUpper(r.Method)

		httpRequests.WithLabelValues(method, path, strconv.Itoa(rec.status)).Inc()
		httpDuration.WithLabelValues(method, path).Observe(duration.Seconds())
	})
}

// RecordFunctionExecution records metrics for executed functions.
func RecordFunctionExecution(status string, duration time.Duration) {
	if duration <= 0 {
		duration = time.Millisecond
	}
	functionExecutions.WithLabelValues(status).Inc()
	functionDuration.WithLabelValues(status).Observe(duration.Seconds())
}

// RecordAutomationExecution records metrics for automation job dispatches.
func RecordAutomationExecution(jobID string, duration time.Duration, success bool) {
	if jobID == "" {
		jobID = "unknown"
	}
	if duration <= 0 {
		duration = time.Millisecond
	}
	result := "false"
	if success {
		result = "true"
	}
	automationExecutions.WithLabelValues(jobID, result).Inc()
	automationDuration.WithLabelValues(jobID).Observe(duration.Seconds())
}

// RecordOracleAttempt records oracle dispatch attempts and outcomes.
func RecordOracleAttempt(accountID, status string) {
	if accountID == "" {
		accountID = "unknown"
	}
	if status == "" {
		status = "unknown"
	}
	oracleAttempts.WithLabelValues(accountID, status).Inc()
}

// RecordOracleStaleness tracks the age of the oldest pending oracle request per account.
func RecordOracleStaleness(accountID string, age time.Duration) {
	if accountID == "" {
		accountID = "unknown"
	}
	oracleStaleness.WithLabelValues(accountID).Set(age.Seconds())
}

// RecordDatafeedStaleness tracks feed freshness and status labels.
func RecordDatafeedStaleness(feedID, status string, age time.Duration) {
	if feedID == "" {
		feedID = "unknown"
	}
	if status == "" {
		status = "unknown"
	}
	if age < 0 {
		age = 0
	}
	datafeedStaleness.WithLabelValues(feedID, status).Set(age.Seconds())
}

type observationCollector struct {
	gauge *prometheus.GaugeVec
	hist  *prometheus.HistogramVec
}

// ObservationHooks creates core observation hooks backed by Prometheus metrics.
func ObservationHooks(namespace, subsystem, name string) core.ObservationHooks {
	key := namespace + ":" + subsystem + ":" + name
	var collector observationCollector
	if entry, ok := observationCollectors.Load(key); ok {
		collector = entry.(observationCollector)
	} else {
		collector = createObservationCollector(namespace, subsystem, name)
		observationCollectors.Store(key, collector)
	}
	return core.ObservationHooks{
		OnStart: func(ctx context.Context, meta map[string]string) {
			label := metaLabel(meta)
			collector.gauge.WithLabelValues(label).Inc()
		},
		OnComplete: func(ctx context.Context, meta map[string]string, err error, duration time.Duration) {
			label := metaLabel(meta)
			collector.gauge.WithLabelValues(label).Dec()
			status := "success"
			if err != nil {
				status = "error"
			}
			collector.hist.WithLabelValues(label, status).Observe(duration.Seconds())
		},
	}
}

func createObservationCollector(namespace, subsystem, name string) observationCollector {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      name + "_in_flight",
			Help:      "Current operations in flight for " + subsystem,
		},
		[]string{"resource"},
	)
	hist := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      name + "_duration_seconds",
			Help:      "Duration of operations for " + subsystem,
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 10),
		},
		[]string{"resource", "status"},
	)
	Registry.MustRegister(gauge, hist)
	return observationCollector{gauge: gauge, hist: hist}
}

func metaLabel(meta map[string]string) string {
	if meta == nil {
		return "unknown"
	}
	if id, ok := meta["resource"]; ok && id != "" {
		return id
	}
	if id, ok := meta["feed_id"]; ok && id != "" {
		return id
	}
	if id, ok := meta["stream_id"]; ok && id != "" {
		return id
	}
	if id, ok := meta["product_id"]; ok && id != "" {
		return id
	}
	if id, ok := meta["order_id"]; ok && id != "" {
		return id
	}
	if id, ok := meta["transaction_id"]; ok && id != "" {
		return id
	}
	return "unknown"
}

// PriceFeedSubmissionHooks captures per-feed price submissions.
func PriceFeedSubmissionHooks() core.ObservationHooks {
	return ObservationHooks("service_layer", "pricefeed", "submissions")
}

// DispatcherHooks wraps ObservationHooks for dispatcher instrumentation.
func DispatcherHooks(namespace, subsystem, name string) core.DispatchHooks {
	return ObservationHooks(namespace, subsystem, name)
}

// CCIPDispatchHooks captures CCIP dispatcher attempts.
func CCIPDispatchHooks() core.DispatchHooks {
	return DispatcherHooks("service_layer", "ccip", "dispatch")
}

// DataLinkDispatchHooks captures datalink dispatcher attempts.
func DataLinkDispatchHooks() core.DispatchHooks {
	return DispatcherHooks("service_layer", "datalink", "dispatch")
}

// VRFDispatchHooks captures VRF dispatcher attempts.
func VRFDispatchHooks() core.DispatchHooks {
	return DispatcherHooks("service_layer", "vrf", "dispatch")
}

// PriceFeedRefreshHooks captures refresher fetch attempts.
func PriceFeedRefreshHooks() core.ObservationHooks {
	return ObservationHooks("service_layer", "pricefeed", "refresh")
}

// GasBankSettlementHooks captures withdrawal settlement attempts.
func GasBankSettlementHooks() core.ObservationHooks {
	return ObservationHooks("service_layer", "gasbank", "settlement")
}

// DataFeedUpdateHooks captures data feed update submissions.
func DataFeedUpdateHooks() core.ObservationHooks {
	return ObservationHooks("service_layer", "datafeeds", "updates")
}

// DTAOrderHooks captures DTA order creation events.
func DTAOrderHooks() core.ObservationHooks {
	return ObservationHooks("service_layer", "dta", "orders")
}

// DatastreamFrameHooks captures frame ingestion operations.
func DatastreamFrameHooks() core.ObservationHooks {
	return ObservationHooks("service_layer", "datastreams", "frames")
}

// ConfidentialSealedKeyHooks captures sealed key storage events.
func ConfidentialSealedKeyHooks() core.ObservationHooks {
	return ObservationHooks("service_layer", "confidential", "sealed_keys")
}

// ConfidentialAttestationHooks captures attestation storage events.
func ConfidentialAttestationHooks() core.ObservationHooks {
	return ObservationHooks("service_layer", "confidential", "attestations")
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func canonicalPath(raw string) string {
	if raw == "" || raw == "/" {
		return "/"
	}
	trimmed := strings.Trim(raw, "/")
	if trimmed == "" {
		return "/"
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 {
		return "/"
	}
	if parts[0] != "accounts" {
		return "/" + parts[0]
	}
	if len(parts) == 1 {
		return "/accounts"
	}
	if len(parts) == 2 {
		return "/accounts/:account"
	}
	resource := parts[1]
	return "/accounts/" + resource
}
