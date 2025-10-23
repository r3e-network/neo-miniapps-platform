package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	io_prometheus_client "github.com/prometheus/client_model/go"
)

func TestInstrumentHandlerRecordsMetrics(t *testing.T) {
	handler := InstrumentHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest(http.MethodGet, "/devpack/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}

	if !metricCounterGreaterOrEqual(t, "service_layer_http_requests_total", map[string]string{
		"method": "GET",
		"path":   "/devpack",
		"status": "202",
	}, 1) {
		t.Fatalf("expected http request counter to increment")
	}

	if !metricHistogramCountGreaterOrEqual(t, "service_layer_http_request_duration_seconds", map[string]string{
		"method": "GET",
		"path":   "/devpack",
	}, 1) {
		t.Fatalf("expected http duration histogram to record samples")
	}
}

func TestRecordFunctionAndAutomationMetrics(t *testing.T) {
	RecordFunctionExecution("unit-test-status", 250*time.Millisecond)
	if !metricCounterGreaterOrEqual(t, "service_layer_functions_executions_total", map[string]string{
		"status": "unit-test-status",
	}, 1) {
		t.Fatalf("expected function execution counter to increase")
	}

	RecordAutomationExecution("job-unit-test", 150*time.Millisecond, true)
	if !metricCounterGreaterOrEqual(t, "service_layer_automation_job_runs_total", map[string]string{
		"job_id":  "job-unit-test",
		"success": "true",
	}, 1) {
		t.Fatalf("expected automation execution counter to increase")
	}
	if !metricHistogramCountGreaterOrEqual(t, "service_layer_automation_job_run_duration_seconds", map[string]string{
		"job_id": "job-unit-test",
	}, 1) {
		t.Fatalf("expected automation duration histogram to record")
	}
}

func metricCounterGreaterOrEqual(t *testing.T, name string, labels map[string]string, min float64) bool {
	t.Helper()
	families, err := Registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, mf := range families {
		if mf.GetName() != name {
			continue
		}
		for _, metric := range mf.GetMetric() {
			if labelsMatch(metric, labels) && metric.GetCounter() != nil {
				return metric.GetCounter().GetValue() >= min
			}
		}
	}
	return false
}

func metricHistogramCountGreaterOrEqual(t *testing.T, name string, labels map[string]string, min uint64) bool {
	t.Helper()
	families, err := Registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, mf := range families {
		if mf.GetName() != name {
			continue
		}
		for _, metric := range mf.GetMetric() {
			if labelsMatch(metric, labels) && metric.GetHistogram() != nil {
				return metric.GetHistogram().GetSampleCount() >= min
			}
		}
	}
	return false
}

func labelsMatch(metric *io_prometheus_client.Metric, labels map[string]string) bool {
	if len(metric.GetLabel()) < len(labels) {
		return false
	}
	matched := 0
	for _, lbl := range metric.GetLabel() {
		if val, ok := labels[lbl.GetName()]; ok && val == lbl.GetValue() {
			matched++
		}
	}
	return matched == len(labels)
}
