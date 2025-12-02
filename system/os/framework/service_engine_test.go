package framework

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/os/core"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

type mockAccountChecker struct {
	accounts map[string]bool
	tenants  map[string]string
}

func (m *mockAccountChecker) AccountExists(ctx context.Context, accountID string) error {
	if m.accounts[accountID] {
		return nil
	}
	return errors.New("account not found")
}

func (m *mockAccountChecker) AccountTenant(ctx context.Context, accountID string) string {
	if m.tenants != nil {
		return m.tenants[accountID]
	}
	return ""
}

type mockWalletChecker struct {
	wallets map[string]map[string]bool
}

func (m *mockWalletChecker) WalletOwnedBy(ctx context.Context, accountID, wallet string) error {
	if wallets, ok := m.wallets[accountID]; ok {
		if wallets[wallet] {
			return nil
		}
	}
	return errors.New("wallet not owned by account")
}

func TestNewServiceEngine(t *testing.T) {
	accounts := &mockAccountChecker{accounts: map[string]bool{"acct-1": true}}

	eng := NewServiceEngine(ServiceConfig{
		Name:        "testservice",
		Description: "Test service",
		Accounts:    accounts,
	})

	if eng.Name() != "testservice" {
		t.Errorf("Name() = %q, want %q", eng.Name(), "testservice")
	}
	if eng.Domain() != "testservice" {
		t.Errorf("Domain() = %q, want %q", eng.Domain(), "testservice")
	}

	manifest := eng.Manifest()
	if manifest.Name != "testservice" {
		t.Errorf("Manifest.Name = %q, want %q", manifest.Name, "testservice")
	}
	if manifest.Description != "Test service" {
		t.Errorf("Manifest.Description = %q, want %q", manifest.Description, "Test service")
	}
}

func TestServiceEngine_CustomDomain(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{
		Name:        "testservice",
		Domain:      "custom-domain",
		Description: "Test service",
	})

	if eng.Domain() != "custom-domain" {
		t.Errorf("Domain() = %q, want %q", eng.Domain(), "custom-domain")
	}
}

func TestServiceEngine_CustomAPIs(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{
		Name:         "testservice",
		Description:  "Test service",
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceData},
	})

	manifest := eng.Manifest()
	if len(manifest.RequiresAPIs) != 2 {
		t.Errorf("RequiresAPIs length = %d, want 2", len(manifest.RequiresAPIs))
	}
}

func TestServiceEngine_ValidateAccount(t *testing.T) {
	accounts := &mockAccountChecker{accounts: map[string]bool{"acct-1": true}}
	eng := NewServiceEngine(ServiceConfig{
		Name:     "testservice",
		Accounts: accounts,
	})

	tests := []struct {
		name      string
		accountID string
		wantID    string
		wantErr   bool
	}{
		{"valid account", "acct-1", "acct-1", false},
		{"valid with spaces", "  acct-1  ", "acct-1", false},
		{"empty", "", "", true},
		{"whitespace only", "   ", "", true},
		{"not found", "acct-2", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := eng.ValidateAccount(context.Background(), tt.accountID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAccount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantID {
				t.Errorf("ValidateAccount() = %v, want %v", got, tt.wantID)
			}
		})
	}
}

func TestServiceEngine_ValidateAccount_NilAccounts(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{
		Name: "testservice",
	})

	// Should still validate empty
	_, err := eng.ValidateAccount(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty account ID")
	}

	// Should pass for non-empty when accounts is nil
	got, err := eng.ValidateAccount(context.Background(), "any-account")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got != "any-account" {
		t.Errorf("expected 'any-account', got %q", got)
	}
}

func TestServiceEngine_ValidateOwnership(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{Name: "testservice"})

	tests := []struct {
		name            string
		resourceAccount string
		requestAccount  string
		wantErr         bool
	}{
		{"same account", "acct-1", "acct-1", false},
		{"different account", "acct-1", "acct-2", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := eng.ValidateOwnership(tt.resourceAccount, tt.requestAccount, "resource", "res-1")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOwnership() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceEngine_ValidateAccountAndOwnership(t *testing.T) {
	accounts := &mockAccountChecker{accounts: map[string]bool{"acct-1": true, "acct-2": true}}
	eng := NewServiceEngine(ServiceConfig{
		Name:     "testservice",
		Accounts: accounts,
	})

	tests := []struct {
		name            string
		resourceAccount string
		requestAccount  string
		wantErr         bool
	}{
		{"valid and owns", "acct-1", "acct-1", false},
		{"valid but doesn't own", "acct-1", "acct-2", true},
		{"invalid requester", "acct-1", "acct-3", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := eng.ValidateAccountAndOwnership(context.Background(), tt.resourceAccount, tt.requestAccount, "resource", "res-1")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAccountAndOwnership() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceEngine_ValidateSigners(t *testing.T) {
	wallets := &mockWalletChecker{
		wallets: map[string]map[string]bool{
			"acct-1": {"wallet-a": true, "wallet-b": true},
		},
	}
	eng := NewServiceEngine(ServiceConfig{Name: "testservice"})
	eng.WithWalletChecker(wallets)

	tests := []struct {
		name      string
		accountID string
		signers   []string
		wantErr   bool
	}{
		{"valid signers", "acct-1", []string{"wallet-a", "wallet-b"}, false},
		{"one invalid", "acct-1", []string{"wallet-a", "wallet-c"}, true},
		{"empty signers", "acct-1", []string{}, false},
		{"nil signers", "acct-1", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := eng.ValidateSigners(context.Background(), tt.accountID, tt.signers)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSigners() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceEngine_ValidateRequired(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{Name: "testservice"})

	tests := []struct {
		name    string
		value   string
		field   string
		want    string
		wantErr bool
	}{
		{"valid", "hello", "name", "hello", false},
		{"with spaces", "  hello  ", "name", "hello", false},
		{"empty", "", "name", "", true},
		{"whitespace", "   ", "name", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := eng.ValidateRequired(tt.value, tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequired() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateRequired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServiceEngine_ClampLimit(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{Name: "testservice"})

	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{"zero uses default", 0, 25}, // DefaultListLimit = 25
		{"negative uses default", -1, 25},
		{"within range", 25, 25},
		{"above max", 1000, 500}, // MaxListLimit = 500
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eng.ClampLimit(tt.limit)
			if got != tt.want {
				t.Errorf("ClampLimit(%d) = %d, want %d", tt.limit, got, tt.want)
			}
		})
	}
}

func TestServiceEngine_Descriptor(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{
		Name:        "testservice",
		Description: "Test service",
	})

	desc := eng.Descriptor()
	if desc.Name != "testservice" {
		t.Errorf("Descriptor.Name = %q, want %q", desc.Name, "testservice")
	}
}

func TestServiceEngine_Logger(t *testing.T) {
	customLog := logger.NewDefault("custom")
	eng := NewServiceEngine(ServiceConfig{
		Name:   "testservice",
		Logger: customLog,
	})

	if eng.Logger() != customLog {
		t.Error("Logger() did not return the custom logger")
	}
}

func TestServiceEngine_DefaultLogger(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{
		Name: "testservice",
	})

	if eng.Logger() == nil {
		t.Error("Logger() should not be nil")
	}
}

func TestServiceEngine_Lifecycle(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{Name: "testservice"})

	if err := eng.Start(context.Background()); err != nil {
		t.Errorf("Start() error = %v", err)
	}
	if err := eng.Ready(context.Background()); err != nil {
		t.Errorf("Ready() error = %v", err)
	}
	if err := eng.Stop(context.Background()); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
	if eng.Ready(context.Background()) == nil {
		t.Error("Ready() should return error after Stop()")
	}
}

func TestServiceEngine_ManifestIsolation(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{
		Name:        "isolation",
		Description: "isolation test",
		DependsOn:   []string{"store"},
	})

	first := eng.Manifest()
	first.Name = "mutated"
	first.DependsOn = append(first.DependsOn, "extra")

	second := eng.Manifest()
	if second.Name == "mutated" {
		t.Fatal("Manifest mutation should not be reflected in subsequent calls")
	}
	for _, dep := range second.DependsOn {
		if dep == "extra" {
			t.Fatal("Manifest should not retain caller mutation")
		}
	}
}

func TestServiceEngine_DescriptorIsolation(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{
		Name:         "descriptor",
		Description:  "descriptor test",
		Capabilities: []string{"one"},
	})

	desc := eng.Descriptor()
	if len(desc.Capabilities) == 0 {
		t.Fatal("expected descriptor to include at least one capability")
	}
	desc.Capabilities[0] = "mutated"

	newDesc := eng.Descriptor()
	if len(newDesc.Capabilities) == 0 {
		t.Fatal("expected descriptor capabilities to persist")
	}
	if newDesc.Capabilities[0] == "mutated" {
		t.Fatal("Descriptor mutation should not leak back into ServiceEngine")
	}
}

type fakeStoreProvider struct{}

func (fakeStoreProvider) Database() any { return "db-conn" }

func (fakeStoreProvider) AccountExists(ctx context.Context, accountID string) error {
	if strings.TrimSpace(accountID) == "acct-1" {
		return nil
	}
	return errors.New("missing")
}

func (fakeStoreProvider) AccountTenant(ctx context.Context, accountID string) string {
	return "tenant-a"
}

type recordingBus struct {
	published       []string
	dataTopics      []string
	computePayloads []any
	computeResults  []ComputeResult
	publishErr      error
	pushErr         error
	computeErr      error
}

func (b *recordingBus) PublishEvent(ctx context.Context, event string, payload any) error {
	b.published = append(b.published, event)
	return b.publishErr
}

func (b *recordingBus) PushData(ctx context.Context, topic string, payload any) error {
	b.dataTopics = append(b.dataTopics, topic)
	return b.pushErr
}

func (b *recordingBus) InvokeCompute(ctx context.Context, payload any) ([]ComputeResult, error) {
	b.computePayloads = append(b.computePayloads, payload)
	if len(b.computeResults) > 0 || b.computeErr != nil {
		return b.computeResults, b.computeErr
	}
	return []ComputeResult{{Module: "test", Result: payload}}, nil
}

func TestServiceEngine_EnvironmentIntegration(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{Name: "env"})

	// Default environment should return bus unavailable
	if err := eng.PublishEvent(context.Background(), "event", nil); !errors.Is(err, core.ErrBusUnavailable) {
		t.Fatalf("expected ErrBusUnavailable, got %v", err)
	}

	bus := &recordingBus{}
	metrics := &fakeMetrics{}
	quota := &fakeQuota{}
	env := Environment{
		StoreProvider: fakeStoreProvider{},
		Bus:           bus,
		Config:        ConfigMap{"foo": "bar"},
		Tracer:        fakeTracer{},
		Metrics:       metrics,
		Quota:         quota,
	}
	eng.SetEnvironment(env)

	if sp := eng.StoreProvider(); sp == nil {
		t.Fatal("expected store provider after environment set")
	}
	if db := eng.Database(); db != "db-conn" {
		t.Fatalf("expected database 'db-conn', got %v", db)
	}

	if _, err := eng.ValidateAccount(context.Background(), "acct-1"); err != nil {
		t.Fatalf("Fallback account checker failed: %v", err)
	}

	if err := eng.PublishEvent(context.Background(), "evt", nil); err != nil {
		t.Fatalf("PublishEvent failed: %v", err)
	}
	if len(bus.published) == 0 || bus.published[0] != "evt" {
		t.Fatalf("expected event recorded, got %v", bus.published)
	}

	if val, ok := eng.Config().Get("foo"); !ok || val != "bar" {
		t.Fatalf("expected config foo=bar, got %q %v", val, ok)
	}

	if err := eng.EnforceQuota("events", 1); err != nil {
		t.Fatalf("EnforceQuota failed: %v", err)
	}
	if len(quota.calls) != 2 {
		t.Fatalf("expected quota enforcer called twice, got %d", len(quota.calls))
	}

	if eng.Tracer() != env.Tracer {
		t.Fatalf("expected tracer instance propagated")
	}

	eng.Metrics().Counter("requests", map[string]string{"status": "ok"}, 1)
	found := false
	for _, c := range metrics.counters {
		if c.name == "requests" {
			found = true
			if c.labels["status"] != "ok" {
				t.Fatalf("expected status label, got %#v", c.labels)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected metrics recorder to capture counter")
	}
}

func TestServiceEngine_MetricHelpers(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{Name: "metrics"})
	fm := &fakeMetrics{}
	eng.SetEnvironment(Environment{
		Metrics: fm,
		Bus:     &recordingBus{},
	})

	eng.IncrementCounter("requests", map[string]string{"status": "ok"})
	eng.AddCounter("requests", nil, 2)
	eng.SetGauge("ready", nil, 1)
	eng.ObserveHistogram("latency", nil, 42)
	eng.ObserveDuration("duration", nil, 50*time.Millisecond)

	if len(fm.counters) != 2 {
		t.Fatalf("expected 2 counter calls, got %d", len(fm.counters))
	}
	if fm.counters[0].labels["status"] != "ok" {
		t.Fatalf("expected status label, got %#v", fm.counters[0].labels)
	}
	if len(fm.gauges) != 1 || fm.gauges[0].value != 1 {
		t.Fatalf("expected gauge recorded")
	}
	if len(fm.histograms) != 2 {
		t.Fatalf("expected histogram samples, got %d", len(fm.histograms))
	}
}

func TestServiceEngine_QuotaHelpers(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{Name: "quota"})
	fq := &fakeQuota{}
	eng.SetEnvironment(Environment{
		Quota: fq,
		Bus:   &recordingBus{},
	})

	_ = eng.EnforceEventQuota(1)
	_ = eng.EnforceDataQuota(2)
	_ = eng.EnforceConcurrencyQuota(3)

	if len(fq.calls) != 3 {
		t.Fatalf("expected quota calls, got %d", len(fq.calls))
	}
	if fq.calls[0].resource != QuotaResourceEvents || fq.calls[1].resource != QuotaResourceDataPush || fq.calls[2].resource != QuotaResourceConcurrency {
		t.Fatalf("unexpected quota resources: %#v", fq.calls)
	}
}

func TestServiceEngine_StartObservation_TracerIntegration(t *testing.T) {
	tracer := &recordingTracer{}
	eng := NewServiceEngine(ServiceConfig{Name: "accounts"})
	eng.SetEnvironment(Environment{Tracer: tracer})
	var startCtx, completeCtx context.Context
	eng.WithObservationHooks(core.ObservationHooks{
		OnStart: func(ctx context.Context, meta map[string]string) {
			startCtx = ctx
			if meta["service"] != "accounts" {
				t.Fatalf("expected service metadata propagated, got %v", meta)
			}
			if meta["domain"] != "accounts" {
				t.Fatalf("expected domain metadata propagated, got %v", meta)
			}
		},
		OnComplete: func(ctx context.Context, meta map[string]string, err error, _ time.Duration) {
			completeCtx = ctx
			if err == nil {
				t.Fatal("expected error to propagate to completion hook")
			}
		},
	})

	baseCtx := context.Background()
	obsCtx, finish := eng.StartObservation(baseCtx, map[string]string{"resource": "account", "operation": "create"})
	if obsCtx == baseCtx {
		t.Fatal("expected StartObservation to derive new context")
	}
	if tracer.startCount != 1 {
		t.Fatalf("expected tracer start invoked once, got %d", tracer.startCount)
	}
	if tracer.lastName != "accounts.create" {
		t.Fatalf("unexpected span name %q", tracer.lastName)
	}
	if tracer.lastAttrs["domain"] != "accounts" {
		t.Fatalf("expected tracer attrs to include domain, got %v", tracer.lastAttrs)
	}
	if obsCtx.Value(testSpanCtxKey) != "span" {
		t.Fatal("expected derived context value from tracer")
	}
	if startCtx == nil || startCtx.Value(testSpanCtxKey) != "span" {
		t.Fatal("expected hooks to see span context")
	}

	finish(errors.New("boom"))
	if tracer.finishCount != 1 {
		t.Fatalf("expected tracer finish invoked once, got %d", tracer.finishCount)
	}
	if completeCtx == nil || completeCtx.Value(testSpanCtxKey) != "span" {
		t.Fatal("expected completion hook to run with span context")
	}
}

func TestServiceEngine_PublishEvent_InstrumentsBus(t *testing.T) {
	bus := &recordingBus{}
	metrics := &fakeMetrics{}
	quota := &fakeQuota{}
	eng := NewServiceEngine(ServiceConfig{Name: "events"})
	eng.SetEnvironment(Environment{
		Bus:     bus,
		Metrics: metrics,
		Quota:   quota,
	})

	if err := eng.PublishEvent(context.Background(), "orders.created", map[string]string{"id": "123"}); err != nil {
		t.Fatalf("PublishEvent returned error: %v", err)
	}
	if len(bus.published) != 1 || bus.published[0] != "orders.created" {
		t.Fatalf("expected event recorded, got %#v", bus.published)
	}
	if len(quota.calls) != 1 || quota.calls[0].resource != QuotaResourceEvents {
		t.Fatalf("expected quota enforcement for events, got %#v", quota.calls)
	}
	assertCounter(t, metrics.counters, "service_bus_events_total", map[string]string{
		"event":     "orders.created",
		"service":   "events",
		"operation": "publish_event",
		"status":    "success",
	})
}

func TestServiceEngine_PushData_InstrumentsBus(t *testing.T) {
	bus := &recordingBus{}
	metrics := &fakeMetrics{}
	quota := &fakeQuota{}
	eng := NewServiceEngine(ServiceConfig{Name: "data"})
	eng.SetEnvironment(Environment{
		Bus:     bus,
		Metrics: metrics,
		Quota:   quota,
	})

	if err := eng.PushData(context.Background(), "frames", []byte("payload")); err != nil {
		t.Fatalf("PushData returned error: %v", err)
	}
	if len(bus.dataTopics) != 1 || bus.dataTopics[0] != "frames" {
		t.Fatalf("expected topic recorded, got %#v", bus.dataTopics)
	}
	if len(quota.calls) != 1 || quota.calls[0].resource != QuotaResourceDataPush {
		t.Fatalf("expected data quota enforcement, got %#v", quota.calls)
	}
	assertCounter(t, metrics.counters, "service_bus_data_total", map[string]string{
		"topic":     "frames",
		"service":   "data",
		"operation": "push_data",
		"status":    "success",
	})
}

func TestServiceEngine_InvokeCompute_Metrics(t *testing.T) {
	bus := &recordingBus{
		computeResults: []ComputeResult{
			{Module: "one", Result: "ok"},
			{Module: "two", Err: errors.New("boom")},
		},
	}
	metrics := &fakeMetrics{}
	quota := &fakeQuota{}
	eng := NewServiceEngine(ServiceConfig{Name: "compute"})
	eng.SetEnvironment(Environment{
		Bus:     bus,
		Metrics: metrics,
		Quota:   quota,
	})

	results, err := eng.InvokeCompute(context.Background(), map[string]string{"job": "123"})
	if err != nil {
		t.Fatalf("InvokeCompute returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected compute results propagated, got %#v", results)
	}
	if len(bus.computePayloads) != 1 {
		t.Fatalf("expected compute payload recorded, got %d", len(bus.computePayloads))
	}
	if len(quota.calls) != 1 || quota.calls[0].resource != QuotaResourceConcurrency {
		t.Fatalf("expected concurrency quota enforcement, got %#v", quota.calls)
	}
	assertCounter(t, metrics.counters, "service_bus_compute_requests_total", map[string]string{
		"service":   "compute",
		"operation": "invoke_compute",
		"status":    "success",
	})
	assertCounter(t, metrics.counters, "service_bus_compute_results_total", map[string]string{
		"service":   "compute",
		"operation": "invoke_compute",
		"status":    "success",
	}, 1)
	assertCounter(t, metrics.counters, "service_bus_compute_results_total", map[string]string{
		"service":   "compute",
		"operation": "invoke_compute",
		"status":    "error",
	}, 1)
}

func TestServiceEngine_ContextExposesRuntime(t *testing.T) {
	bus := &recordingBus{}
	metrics := &fakeMetrics{}
	quota := &fakeQuota{}
	eng := NewServiceEngine(ServiceConfig{
		Name:        "accounts",
		Description: "Accounts test",
	})
	eng.SetEnvironment(Environment{
		StoreProvider: fakeStoreProvider{},
		Bus:           bus,
		Config:        ConfigMap{"foo": "bar"},
		Tracer:        fakeTracer{},
		Metrics:       metrics,
		Quota:         quota,
	})

	ctx, ok := eng.Context().(EngineContext)
	if !ok {
		t.Fatal("ServiceEngine.Context() did not return EngineContext")
	}
	if ctx.Name() != "accounts" {
		t.Fatalf("expected context name 'accounts', got %q", ctx.Name())
	}
	if ctx.Domain() != "accounts" {
		t.Fatalf("expected context domain 'accounts', got %q", ctx.Domain())
	}
	if ctx.Logger() != eng.Logger() {
		t.Fatal("expected logger passthrough")
	}
	if err := ctx.PublishEvent(context.Background(), "ctx.event", nil); err != nil {
		t.Fatalf("PublishEvent via context failed: %v", err)
	}
	if len(bus.published) != 1 || bus.published[0] != "ctx.event" {
		t.Fatalf("expected event published via context, got %#v", bus.published)
	}
	if svc := ctx.SystemService(SystemServiceBus); svc != eng.Bus() {
		t.Fatalf("expected SystemService bus to match engine bus")
	}
	eng.SetEnvironment(Environment{Bus: &recordingBus{}})
	if svc := ctx.SystemService(SystemServiceBus); svc == bus {
		t.Fatal("expected context services to refresh after SetEnvironment")
	}
}

type fakeTracer struct{}

func (fakeTracer) StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, func(error)) {
	return ctx, func(error) {}
}

type ctxKey string

const testSpanCtxKey ctxKey = "span"

type recordingTracer struct {
	startCount  int
	finishCount int
	lastName    string
	lastAttrs   map[string]string
}

func (t *recordingTracer) StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, func(error)) {
	t.startCount++
	t.lastName = name
	t.lastAttrs = attrs
	spanCtx := context.WithValue(ctx, testSpanCtxKey, "span")
	return spanCtx, func(error) {
		t.finishCount++
	}
}

type quotaCall struct {
	resource string
	amount   int64
}

type fakeQuota struct {
	calls []quotaCall
}

func (f *fakeQuota) Enforce(resource string, amount int64) error {
	f.calls = append(f.calls, quotaCall{resource: resource, amount: amount})
	return nil
}

type metricCall struct {
	name   string
	labels map[string]string
	value  float64
}

type fakeMetrics struct {
	counters   []metricCall
	gauges     []metricCall
	histograms []metricCall
}

func (f *fakeMetrics) Counter(name string, labels map[string]string, delta float64) {
	f.counters = append(f.counters, metricCall{name: name, labels: labels, value: delta})
}

func (f *fakeMetrics) Gauge(name string, labels map[string]string, value float64) {
	f.gauges = append(f.gauges, metricCall{name: name, labels: labels, value: value})
}

func (f *fakeMetrics) Histogram(name string, labels map[string]string, value float64) {
	f.histograms = append(f.histograms, metricCall{name: name, labels: labels, value: value})
}

func assertCounter(t *testing.T, counters []metricCall, name string, labels map[string]string, expected ...float64) {
	t.Helper()
	wanted := 1.0
	if len(expected) > 0 {
		wanted = expected[0]
	}
	for _, c := range counters {
		if c.name != name {
			continue
		}
		match := true
		for k, v := range labels {
			if c.labels == nil || c.labels[k] != v {
				match = false
				break
			}
		}
		if match {
			if c.value != wanted {
				t.Fatalf("counter %s value = %f, want %f", name, c.value, wanted)
			}
			return
		}
	}
	t.Fatalf("counter %s with labels %v not found. counters: %#v", name, labels, counters)
}
