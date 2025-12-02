package metrics

import (
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/R3E-Network/service_layer/system/os/framework"
)

const (
	defaultNamespace = "service_layer"
	defaultSubsystem = "services"
)

// Ensure Recorder implements the framework metrics interface.
var _ framework.Metrics = (*Recorder)(nil)

// Recorder adapts Prometheus collectors to the framework.Metrics interface.
// It lazily registers metrics per unique name and label set.
type Recorder struct {
	registry *prometheus.Registry

	mu         sync.Mutex
	counters   map[string]metricCounter
	gauges     map[string]metricGauge
	histograms map[string]metricHistogram
}

type metricCounter struct {
	labels []string
	vec    *prometheus.CounterVec
}

type metricGauge struct {
	labels []string
	vec    *prometheus.GaugeVec
}

type metricHistogram struct {
	labels []string
	vec    *prometheus.HistogramVec
}

// NewRecorder creates a recorder backed by the provided registry.
// If reg is nil, the global metrics Registry is used.
func NewRecorder(reg *prometheus.Registry) *Recorder {
	if reg == nil {
		reg = Registry
	}
	return &Recorder{
		registry:   reg,
		counters:   make(map[string]metricCounter),
		gauges:     make(map[string]metricGauge),
		histograms: make(map[string]metricHistogram),
	}
}

// Counter increments a counter metric by delta.
func (r *Recorder) Counter(name string, labels map[string]string, delta float64) {
	if r == nil || delta <= 0 {
		return
	}
	labelNames, labelValues := normalizeLabels(labels)
	vec := r.getCounterVec(name, labelNames)
	if vec == nil {
		return
	}
	vec.WithLabelValues(labelValues...).Add(delta)
}

// Gauge sets a gauge metric to value.
func (r *Recorder) Gauge(name string, labels map[string]string, value float64) {
	if r == nil {
		return
	}
	labelNames, labelValues := normalizeLabels(labels)
	vec := r.getGaugeVec(name, labelNames)
	if vec == nil {
		return
	}
	vec.WithLabelValues(labelValues...).Set(value)
}

// Histogram observes a value sample.
func (r *Recorder) Histogram(name string, labels map[string]string, value float64) {
	if r == nil {
		return
	}
	labelNames, labelValues := normalizeLabels(labels)
	vec := r.getHistogramVec(name, labelNames)
	if vec == nil {
		return
	}
	vec.WithLabelValues(labelValues...).Observe(value)
}

func (r *Recorder) getCounterVec(name string, labelNames []string) *prometheus.CounterVec {
	sanitized := sanitizeMetricName(name)
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.counters[sanitized]; ok {
		if sameLabelNames(existing.labels, labelNames) {
			return existing.vec
		}
		// Label mismatch - reuse existing vector to avoid Prometheus conflicts.
		return existing.vec
	}

	vec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: defaultNamespace,
		Subsystem: defaultSubsystem,
		Name:      sanitized,
		Help:      "Service-level counter: " + name,
	}, labelNames)

	if err := r.registry.Register(vec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if counter, ok := are.ExistingCollector.(*prometheus.CounterVec); ok {
				r.counters[sanitized] = metricCounter{labels: labelNames, vec: counter}
				return counter
			}
		}
		return nil
	}

	r.counters[sanitized] = metricCounter{labels: labelNames, vec: vec}
	return vec
}

func (r *Recorder) getGaugeVec(name string, labelNames []string) *prometheus.GaugeVec {
	sanitized := sanitizeMetricName(name)
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.gauges[sanitized]; ok {
		if sameLabelNames(existing.labels, labelNames) {
			return existing.vec
		}
		return existing.vec
	}

	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: defaultNamespace,
		Subsystem: defaultSubsystem,
		Name:      sanitized,
		Help:      "Service-level gauge: " + name,
	}, labelNames)

	if err := r.registry.Register(vec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if gauge, ok := are.ExistingCollector.(*prometheus.GaugeVec); ok {
				r.gauges[sanitized] = metricGauge{labels: labelNames, vec: gauge}
				return gauge
			}
		}
		return nil
	}

	r.gauges[sanitized] = metricGauge{labels: labelNames, vec: vec}
	return vec
}

func (r *Recorder) getHistogramVec(name string, labelNames []string) *prometheus.HistogramVec {
	sanitized := sanitizeMetricName(name)
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.histograms[sanitized]; ok {
		if sameLabelNames(existing.labels, labelNames) {
			return existing.vec
		}
		return existing.vec
	}

	vec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: defaultNamespace,
		Subsystem: defaultSubsystem,
		Name:      sanitized,
		Help:      "Service-level histogram: " + name,
		Buckets:   prometheus.ExponentialBuckets(0.001, 2, 15),
	}, labelNames)

	if err := r.registry.Register(vec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if hist, ok := are.ExistingCollector.(*prometheus.HistogramVec); ok {
				r.histograms[sanitized] = metricHistogram{labels: labelNames, vec: hist}
				return hist
			}
		}
		return nil
	}

	r.histograms[sanitized] = metricHistogram{labels: labelNames, vec: vec}
	return vec
}

func sameLabelNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func normalizeLabels(labels map[string]string) ([]string, []string) {
	if len(labels) == 0 {
		return nil, nil
	}
	clean := make(map[string]string, len(labels))
	for k, v := range labels {
		name := sanitizeLabelName(k)
		if name == "" {
			continue
		}
		clean[name] = v
	}
	if len(clean) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(clean))
	for k := range clean {
		names = append(names, k)
	}
	sort.Strings(names)
	values := make([]string, len(names))
	for i, k := range names {
		values[i] = clean[k]
	}
	return names, values
}

func sanitizeMetricName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "custom_metric"
	}
	var b strings.Builder
	b.Grow(len(name) + len("svc_"))
	b.WriteString("svc_")
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == '_' || r == ':' {
			b.WriteRune('_')
		} else {
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return "svc_metric"
	}
	if out[0] >= '0' && out[0] <= '9' {
		return "svc_" + out
	}
	return out
}

func sanitizeLabelName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_':
			b.WriteRune('_')
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return ""
	}
	if unicode.IsDigit(rune(out[0])) {
		out = "_" + out
	}
	return out
}
