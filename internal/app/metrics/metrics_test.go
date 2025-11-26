package metrics

import (
	"fmt"
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

func TestRecordModuleMetrics(t *testing.T) {
	RecordModuleMetrics([]ModuleMetric{
		{Name: "module-a", Domain: "dom-a", Status: "started", Ready: "ready", Waiting: false},
		{Name: "module-b", Domain: "dom-b", Status: "failed", Ready: "not-ready", Waiting: true},
	})
	if !metricGaugeEquals(t, "service_layer_engine_module_ready", map[string]string{"module": "module-a", "domain": "dom-a"}, 1) {
		t.Fatalf("expected module-a ready gauge to be 1")
	}
	if !metricGaugeEquals(t, "service_layer_engine_module_waiting_dependencies", map[string]string{"module": "module-b", "domain": "dom-b"}, 1) {
		t.Fatalf("expected module-b waiting deps gauge to be 1")
	}
	if !metricGaugeEquals(t, "service_layer_engine_module_status", map[string]string{"module": "module-b", "domain": "dom-b", "status": "failed"}, 1) {
		t.Fatalf("expected module-b failed status gauge to be 1")
	}
}

func TestRecordModuleTimings(t *testing.T) {
	RecordModuleTimings([]ModuleTiming{
		{Name: "mod-time", Domain: "dom", StartSeconds: 0.5, StopSeconds: 1.25},
	})
	if !metricGaugeEquals(t, "service_layer_engine_module_start_seconds", map[string]string{"module": "mod-time", "domain": "dom"}, 0.5) {
		t.Fatalf("expected start seconds gauge to be set")
	}
	if !metricGaugeEquals(t, "service_layer_engine_module_stop_seconds", map[string]string{"module": "mod-time", "domain": "dom"}, 1.25) {
		t.Fatalf("expected stop seconds gauge to be set")
	}
}

func TestRecordBusFanout(t *testing.T) {
	RecordBusFanout("unit-kind", nil)
	if !metricCounterGreaterOrEqual(t, "service_layer_engine_bus_fanout_total", map[string]string{"kind": "unit-kind", "result": "ok"}, 1) {
		t.Fatalf("expected bus fan-out ok counter to increase")
	}
	RecordBusFanout("unit-kind", fmt.Errorf("boom"))
	if !metricCounterGreaterOrEqual(t, "service_layer_engine_bus_fanout_total", map[string]string{"kind": "unit-kind", "result": "error"}, 1) {
		t.Fatalf("expected bus fan-out error counter to increase")
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

func metricGaugeEquals(t *testing.T, name string, labels map[string]string, expected float64) bool {
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
			if labelsMatch(metric, labels) && metric.GetGauge() != nil {
				return metric.GetGauge().GetValue() == expected
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

func TestCanonicalPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "/"},
		{"/", "/"},
		{"//", "/"},
		{"/devpack", "/devpack"},
		{"/devpack/test", "/devpack"},
		{"/devpack/test/more", "/devpack"},
		{"/accounts", "/accounts"},
		{"/accounts/", "/accounts"},
		{"/accounts/123", "/accounts/:account"},
		{"/accounts/123/", "/accounts/:account"},
		{"/accounts/abc/xyz", "/accounts/abc"},
		{"/accounts/abc/xyz/more", "/accounts/abc"},
		{"devpack", "/devpack"},
		{"devpack/", "/devpack"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := canonicalPath(tt.input)
			if result != tt.expected {
				t.Errorf("canonicalPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStatusRecorder(t *testing.T) {
	// Test WriteHeader sets status
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, status: http.StatusOK}
	sr.WriteHeader(http.StatusNotFound)
	if sr.status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", sr.status)
	}

	// Test Write sets default status
	rec2 := httptest.NewRecorder()
	sr2 := &statusRecorder{ResponseWriter: rec2, status: 0}
	n, err := sr2.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes written, got %d", n)
	}
	if sr2.status != http.StatusOK {
		t.Errorf("expected default status 200, got %d", sr2.status)
	}

	// Test Write preserves existing status
	rec3 := httptest.NewRecorder()
	sr3 := &statusRecorder{ResponseWriter: rec3, status: http.StatusCreated}
	sr3.Write([]byte("test"))
	if sr3.status != http.StatusCreated {
		t.Errorf("expected status 201 preserved, got %d", sr3.status)
	}
}

func TestRecordOracleAttempt(t *testing.T) {
	// Test with values
	RecordOracleAttempt("acc-123", "success")
	if !metricCounterGreaterOrEqual(t, "service_layer_oracle_request_attempts_total", map[string]string{
		"account_id": "acc-123",
		"status":     "success",
	}, 1) {
		t.Fatal("expected oracle attempt counter to increment")
	}

	// Test with empty values (should use "unknown")
	RecordOracleAttempt("", "")
	if !metricCounterGreaterOrEqual(t, "service_layer_oracle_request_attempts_total", map[string]string{
		"account_id": "unknown",
		"status":     "unknown",
	}, 1) {
		t.Fatal("expected oracle attempt with unknown labels to increment")
	}
}

func TestRecordOracleStaleness(t *testing.T) {
	RecordOracleStaleness("acc-stale", 30*time.Second)
	if !metricGaugeEquals(t, "service_layer_oracle_oldest_pending_seconds", map[string]string{
		"account_id": "acc-stale",
	}, 30) {
		t.Fatal("expected oracle staleness gauge to be set")
	}

	// Empty account ID should use "unknown"
	RecordOracleStaleness("", 60*time.Second)
	if !metricGaugeEquals(t, "service_layer_oracle_oldest_pending_seconds", map[string]string{
		"account_id": "unknown",
	}, 60) {
		t.Fatal("expected oracle staleness with unknown label")
	}
}

func TestRecordDatafeedStaleness(t *testing.T) {
	RecordDatafeedStaleness("feed-1", "healthy", 10*time.Second)
	if !metricGaugeEquals(t, "service_layer_datafeeds_stale_seconds", map[string]string{
		"feed_id": "feed-1",
		"status":  "healthy",
	}, 10) {
		t.Fatal("expected datafeed staleness gauge to be set")
	}

	// Empty values should use "unknown"
	RecordDatafeedStaleness("", "", 0)
	if !metricGaugeEquals(t, "service_layer_datafeeds_stale_seconds", map[string]string{
		"feed_id": "unknown",
		"status":  "unknown",
	}, 0) {
		t.Fatal("expected datafeed staleness with unknown labels")
	}

	// Negative duration should be clamped to 0
	RecordDatafeedStaleness("feed-neg", "stale", -5*time.Second)
	if !metricGaugeEquals(t, "service_layer_datafeeds_stale_seconds", map[string]string{
		"feed_id": "feed-neg",
		"status":  "stale",
	}, 0) {
		t.Fatal("expected negative duration clamped to 0")
	}
}

func TestRecordRPCCall(t *testing.T) {
	RecordRPCCall("neo", "success", 100*time.Millisecond)
	if !metricCounterGreaterOrEqual(t, "service_layer_rpc_requests_total", map[string]string{
		"chain":  "neo",
		"status": "success",
	}, 1) {
		t.Fatal("expected rpc request counter to increment")
	}
	if !metricHistogramCountGreaterOrEqual(t, "service_layer_rpc_request_duration_seconds", map[string]string{
		"chain":  "neo",
		"status": "success",
	}, 1) {
		t.Fatal("expected rpc duration histogram to record")
	}

	// Test normalization (uppercase, whitespace)
	RecordRPCCall("  ETH  ", "  ERROR  ", 50*time.Millisecond)
	if !metricCounterGreaterOrEqual(t, "service_layer_rpc_requests_total", map[string]string{
		"chain":  "eth",
		"status": "error",
	}, 1) {
		t.Fatal("expected normalized labels")
	}

	// Empty values should use "unknown"
	RecordRPCCall("", "", 10*time.Millisecond)
	if !metricCounterGreaterOrEqual(t, "service_layer_rpc_requests_total", map[string]string{
		"chain":  "unknown",
		"status": "unknown",
	}, 1) {
		t.Fatal("expected unknown labels for empty input")
	}
}

func TestRecordFunctionExecution_EdgeCases(t *testing.T) {
	// Test with zero/negative duration (should use 1ms minimum)
	RecordFunctionExecution("edge-test", 0)
	if !metricCounterGreaterOrEqual(t, "service_layer_functions_executions_total", map[string]string{
		"status": "edge-test",
	}, 1) {
		t.Fatal("expected function execution counter with zero duration")
	}

	RecordFunctionExecution("neg-dur", -100*time.Millisecond)
	if !metricCounterGreaterOrEqual(t, "service_layer_functions_executions_total", map[string]string{
		"status": "neg-dur",
	}, 1) {
		t.Fatal("expected function execution counter with negative duration")
	}
}

func TestRecordAutomationExecution_EdgeCases(t *testing.T) {
	// Empty job ID should use "unknown"
	RecordAutomationExecution("", 100*time.Millisecond, false)
	if !metricCounterGreaterOrEqual(t, "service_layer_automation_job_runs_total", map[string]string{
		"job_id":  "unknown",
		"success": "false",
	}, 1) {
		t.Fatal("expected automation counter with unknown job ID")
	}

	// Zero duration should use minimum
	RecordAutomationExecution("zero-dur-job", 0, true)
	if !metricCounterGreaterOrEqual(t, "service_layer_automation_job_runs_total", map[string]string{
		"job_id":  "zero-dur-job",
		"success": "true",
	}, 1) {
		t.Fatal("expected automation counter with zero duration")
	}
}

func TestBusFanoutSnapshot(t *testing.T) {
	// Record some fan-outs
	RecordBusFanout("snapshot-kind", nil)
	RecordBusFanout("snapshot-kind", nil)
	RecordBusFanout("snapshot-kind", fmt.Errorf("error"))

	snapshot := BusFanoutSnapshot()
	if entry, ok := snapshot["snapshot-kind"]; ok {
		if entry.OK < 2 {
			t.Errorf("expected at least 2 OK, got %f", entry.OK)
		}
		if entry.Error < 1 {
			t.Errorf("expected at least 1 error, got %f", entry.Error)
		}
	} else {
		t.Fatal("expected snapshot-kind in snapshot")
	}
}

func TestBusFanoutWindow(t *testing.T) {
	// Record a fan-out
	RecordBusFanout("window-kind", nil)

	// Get window within recent time
	window := BusFanoutWindow(5 * time.Minute)
	if entry, ok := window["window-kind"]; ok {
		if entry.OK < 1 {
			t.Errorf("expected at least 1 OK in window, got %f", entry.OK)
		}
	}

	// Zero/negative window defaults to 5 minutes
	window2 := BusFanoutWindow(0)
	if window2 == nil {
		t.Fatal("expected non-nil window result")
	}

	window3 := BusFanoutWindow(-1 * time.Minute)
	if window3 == nil {
		t.Fatal("expected non-nil window result for negative duration")
	}
}

func TestRecordModuleTimings_EmptyName(t *testing.T) {
	// Empty name should be skipped
	RecordModuleTimings([]ModuleTiming{
		{Name: "", Domain: "dom", StartSeconds: 1.0, StopSeconds: 2.0},
		{Name: "valid-mod", Domain: "dom", StartSeconds: 0.1, StopSeconds: 0.2},
	})
	// Only valid-mod should be recorded
	if !metricGaugeEquals(t, "service_layer_engine_module_start_seconds", map[string]string{
		"module": "valid-mod",
		"domain": "dom",
	}, 0.1) {
		t.Fatal("expected valid module timing to be recorded")
	}
}

func TestMetaLabel(t *testing.T) {
	tests := []struct {
		name     string
		meta     map[string]string
		expected string
	}{
		{
			name:     "nil map",
			meta:     nil,
			expected: "unknown",
		},
		{
			name:     "empty map",
			meta:     map[string]string{},
			expected: "unknown",
		},
		{
			name:     "resource key",
			meta:     map[string]string{"resource": "res-1"},
			expected: "res-1",
		},
		{
			name:     "feed_id key",
			meta:     map[string]string{"feed_id": "feed-1"},
			expected: "feed-1",
		},
		{
			name:     "stream_id key",
			meta:     map[string]string{"stream_id": "stream-1"},
			expected: "stream-1",
		},
		{
			name:     "product_id key",
			meta:     map[string]string{"product_id": "prod-1"},
			expected: "prod-1",
		},
		{
			name:     "order_id key",
			meta:     map[string]string{"order_id": "order-1"},
			expected: "order-1",
		},
		{
			name:     "transaction_id key",
			meta:     map[string]string{"transaction_id": "tx-1"},
			expected: "tx-1",
		},
		{
			name:     "resource takes precedence",
			meta:     map[string]string{"resource": "res-1", "feed_id": "feed-1"},
			expected: "res-1",
		},
		{
			name:     "empty resource falls through",
			meta:     map[string]string{"resource": "", "feed_id": "feed-1"},
			expected: "feed-1",
		},
		{
			name:     "all empty returns unknown",
			meta:     map[string]string{"resource": "", "feed_id": ""},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := metaLabel(tt.meta)
			if result != tt.expected {
				t.Errorf("metaLabel(%v) = %q, want %q", tt.meta, result, tt.expected)
			}
		})
	}
}

func TestHandler(t *testing.T) {
	h := Handler()
	if h == nil {
		t.Fatal("Handler() should return non-nil handler")
	}

	// Test that handler serves metrics
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected non-empty metrics response")
	}
}

func TestInstrumentHandler_MetricsPathPassthrough(t *testing.T) {
	called := false
	handler := InstrumentHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected /metrics path to pass through to handler")
	}
}

func TestObservationHooks(t *testing.T) {
	hooks := ObservationHooks("test_ns", "test_sub", "test_op")

	if hooks.OnStart == nil {
		t.Fatal("OnStart should not be nil")
	}
	if hooks.OnComplete == nil {
		t.Fatal("OnComplete should not be nil")
	}

	// Call OnStart
	hooks.OnStart(nil, map[string]string{"resource": "test-res"})

	// Call OnComplete with success
	hooks.OnComplete(nil, map[string]string{"resource": "test-res"}, nil, 100*time.Millisecond)

	// Call OnComplete with error
	hooks.OnComplete(nil, map[string]string{"resource": "test-res"}, fmt.Errorf("test error"), 50*time.Millisecond)

	// Verify hooks can be called multiple times (reuse from cache)
	hooks2 := ObservationHooks("test_ns", "test_sub", "test_op")
	if hooks2.OnStart == nil || hooks2.OnComplete == nil {
		t.Fatal("cached hooks should be valid")
	}
}

func TestDispatcherHooks(t *testing.T) {
	hooks := DispatcherHooks("dispatch_ns", "dispatch_sub", "dispatch_op")
	if hooks.OnStart == nil || hooks.OnComplete == nil {
		t.Fatal("DispatcherHooks should return valid hooks")
	}
}

func TestSpecificHookFactories(t *testing.T) {
	// Test all the specific hook factory functions
	tests := []struct {
		name  string
		hooks func() interface{}
	}{
		{"PriceFeedSubmissionHooks", func() interface{} { return PriceFeedSubmissionHooks() }},
		{"CCIPDispatchHooks", func() interface{} { return CCIPDispatchHooks() }},
		{"DataLinkDispatchHooks", func() interface{} { return DataLinkDispatchHooks() }},
		{"VRFDispatchHooks", func() interface{} { return VRFDispatchHooks() }},
		{"PriceFeedRefreshHooks", func() interface{} { return PriceFeedRefreshHooks() }},
		{"GasBankSettlementHooks", func() interface{} { return GasBankSettlementHooks() }},
		{"DataFeedUpdateHooks", func() interface{} { return DataFeedUpdateHooks() }},
		{"DTAOrderHooks", func() interface{} { return DTAOrderHooks() }},
		{"DatastreamFrameHooks", func() interface{} { return DatastreamFrameHooks() }},
		{"ConfidentialSealedKeyHooks", func() interface{} { return ConfidentialSealedKeyHooks() }},
		{"ConfidentialAttestationHooks", func() interface{} { return ConfidentialAttestationHooks() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.hooks()
			if result == nil {
				t.Errorf("%s() returned nil", tt.name)
			}
		})
	}
}
