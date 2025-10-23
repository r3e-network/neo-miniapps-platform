package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		prometheus.NewGoCollector(),
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
