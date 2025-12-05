// Package os provides the ServiceOS abstraction layer.
package os

import (
	"sync"
	"time"
)

// metricsAPIImpl implements MetricsAPI.
type metricsAPIImpl struct {
	ctx       *ServiceContext
	serviceID string
	mu        sync.RWMutex

	// Metric storage
	counters   map[string]*counterMetric
	gauges     map[string]*gaugeMetric
	histograms map[string]*histogramMetric
}

type counterMetric struct {
	name       string
	help       string
	labelNames []string
	values     map[string]float64 // label key -> value
}

type gaugeMetric struct {
	name       string
	help       string
	labelNames []string
	values     map[string]float64
}

type histogramMetric struct {
	name       string
	help       string
	buckets    []float64
	labelNames []string
	values     map[string]*histogramData
}

type histogramData struct {
	bucketCounts []uint64
	sum          float64
	count        uint64
}

func newMetricsAPI(ctx *ServiceContext, serviceID string) *metricsAPIImpl {
	return &metricsAPIImpl{
		ctx:        ctx,
		serviceID:  serviceID,
		counters:   make(map[string]*counterMetric),
		gauges:     make(map[string]*gaugeMetric),
		histograms: make(map[string]*histogramMetric),
	}
}

func (m *metricsAPIImpl) Counter(name string, value float64, labels ...string) {
	if err := m.ctx.RequireCapability(CapMetrics); err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	metric, ok := m.counters[name]
	if !ok {
		return
	}

	key := labelsToKey(labels)
	metric.values[key] += value
}

func (m *metricsAPIImpl) Gauge(name string, value float64, labels ...string) {
	if err := m.ctx.RequireCapability(CapMetrics); err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	metric, ok := m.gauges[name]
	if !ok {
		return
	}

	key := labelsToKey(labels)
	metric.values[key] = value
}

func (m *metricsAPIImpl) Histogram(name string, value float64, labels ...string) {
	if err := m.ctx.RequireCapability(CapMetrics); err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	metric, ok := m.histograms[name]
	if !ok {
		return
	}

	key := labelsToKey(labels)
	data, ok := metric.values[key]
	if !ok {
		data = &histogramData{
			bucketCounts: make([]uint64, len(metric.buckets)+1),
		}
		metric.values[key] = data
	}

	// Update histogram
	data.sum += value
	data.count++
	for i, bucket := range metric.buckets {
		if value <= bucket {
			data.bucketCounts[i]++
		}
	}
	data.bucketCounts[len(metric.buckets)]++ // +Inf bucket
}

func (m *metricsAPIImpl) Timer(name string, duration time.Duration, labels ...string) {
	m.Histogram(name, duration.Seconds(), labels...)
}

func (m *metricsAPIImpl) RegisterCounter(name, help string, labelNames ...string) error {
	if err := m.ctx.RequireCapability(CapMetrics); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counters[name] = &counterMetric{
		name:       name,
		help:       help,
		labelNames: labelNames,
		values:     make(map[string]float64),
	}
	return nil
}

func (m *metricsAPIImpl) RegisterGauge(name, help string, labelNames ...string) error {
	if err := m.ctx.RequireCapability(CapMetrics); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.gauges[name] = &gaugeMetric{
		name:       name,
		help:       help,
		labelNames: labelNames,
		values:     make(map[string]float64),
	}
	return nil
}

func (m *metricsAPIImpl) RegisterHistogram(name, help string, buckets []float64, labelNames ...string) error {
	if err := m.ctx.RequireCapability(CapMetrics); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.histograms[name] = &histogramMetric{
		name:       name,
		help:       help,
		buckets:    buckets,
		labelNames: labelNames,
		values:     make(map[string]*histogramData),
	}
	return nil
}

// labelsToKey converts label values to a map key.
func labelsToKey(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	key := labels[0]
	for i := 1; i < len(labels); i++ {
		key += "," + labels[i]
	}
	return key
}
