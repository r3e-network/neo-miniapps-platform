package engine

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

// TestEngineCreation tests basic engine instantiation.
func TestEngineCreation(t *testing.T) {
	e := New()
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
	if e.Registry() == nil {
		t.Error("expected non-nil registry")
	}
	if e.Health() == nil {
		t.Error("expected non-nil health monitor")
	}
	if e.Dependencies() == nil {
		t.Error("expected non-nil dependency manager")
	}
	if e.Permissions() == nil {
		t.Error("expected non-nil permission manager")
	}
	if e.Metadata() == nil {
		t.Error("expected non-nil metadata manager")
	}
	if e.Bus() == nil {
		t.Error("expected non-nil bus")
	}
}

// TestEngineRegisterUnregister tests module registration and unregistration.
func TestEngineRegisterUnregister(t *testing.T) {
	e := New()

	mod := &simpleTestModule{name: "test-mod", domain: "test"}
	if err := e.Register(mod); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// Verify registered
	if len(e.Modules()) != 1 {
		t.Errorf("expected 1 module, got %d", len(e.Modules()))
	}
	if e.Lookup("test-mod") == nil {
		t.Error("lookup failed for registered module")
	}

	// Duplicate registration should fail
	if err := e.Register(mod); err == nil {
		t.Error("expected error on duplicate registration")
	}

	// Unregister
	if err := e.Unregister("test-mod"); err != nil {
		t.Fatalf("unregister failed: %v", err)
	}
	if len(e.Modules()) != 0 {
		t.Errorf("expected 0 modules after unregister, got %d", len(e.Modules()))
	}

	// Unregister non-existent should fail
	if err := e.Unregister("non-existent"); err == nil {
		t.Error("expected error on unregister non-existent module")
	}
}

// TestEngineModulesByDomain tests domain-based module lookup.
func TestEngineModulesByDomain(t *testing.T) {
	e := New()

	mod1 := &simpleTestModule{name: "mod1", domain: "data"}
	mod2 := &simpleTestModule{name: "mod2", domain: "compute"}
	mod3 := &simpleTestModule{name: "mod3", domain: "data"}

	e.Register(mod1)
	e.Register(mod2)
	e.Register(mod3)

	dataMods := e.ModulesByDomain("data")
	if len(dataMods) != 2 {
		t.Errorf("expected 2 data modules, got %d", len(dataMods))
	}

	computeMods := e.ModulesByDomain("compute")
	if len(computeMods) != 1 {
		t.Errorf("expected 1 compute module, got %d", len(computeMods))
	}

	unknownMods := e.ModulesByDomain("unknown")
	if len(unknownMods) != 0 {
		t.Errorf("expected 0 unknown modules, got %d", len(unknownMods))
	}
}

// TestEngineLifecycle tests start/stop operations.
func TestEngineLifecycle(t *testing.T) {
	e := New()

	var started, stopped int32
	mod := &simpleTestModule{
		name:   "lifecycle-test",
		domain: "test",
		startFn: func(ctx context.Context) error {
			atomic.AddInt32(&started, 1)
			return nil
		},
		stopFn: func(ctx context.Context) error {
			atomic.AddInt32(&stopped, 1)
			return nil
		},
	}

	e.Register(mod)

	ctx := context.Background()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if atomic.LoadInt32(&started) != 1 {
		t.Error("module start not called")
	}

	if err := e.Stop(ctx); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if atomic.LoadInt32(&stopped) != 1 {
		t.Error("module stop not called")
	}
}

// TestEngineLifecycleError tests error handling during start.
func TestEngineLifecycleError(t *testing.T) {
	e := New()

	expectedErr := errors.New("start failure")
	mod := &simpleTestModule{
		name:   "fail-mod",
		domain: "test",
		startFn: func(ctx context.Context) error {
			return expectedErr
		},
	}

	e.Register(mod)

	ctx := context.Background()
	if err := e.Start(ctx); err == nil {
		t.Error("expected start error")
	}
}

// TestEngineModulesHealth tests health monitoring.
func TestEngineModulesHealth(t *testing.T) {
	e := New()

	mod := &simpleTestModule{name: "health-mod", domain: "test"}
	e.Register(mod)

	health := e.ModulesHealth()
	if len(health) != 1 {
		t.Fatalf("expected 1 health entry, got %d", len(health))
	}
	if health[0].Name != "health-mod" {
		t.Errorf("expected health-mod, got %s", health[0].Name)
	}
	if health[0].Status != StatusRegistered {
		t.Errorf("expected registered status, got %s", health[0].Status)
	}

	// Mark started
	e.MarkStarted("health-mod")
	health = e.ModulesHealth()
	if health[0].Status != StatusStarted {
		t.Errorf("expected started status after MarkStarted, got %s", health[0].Status)
	}

	// Mark stopped
	e.MarkStopped("health-mod")
	health = e.ModulesHealth()
	if health[0].Status != StatusStopped {
		t.Errorf("expected stopped status after MarkStopped, got %s", health[0].Status)
	}
}

// TestEngineBusPermissions tests bus permission management.
func TestEngineBusPermissions(t *testing.T) {
	e := New()

	mod := &simpleTestModule{name: "perm-mod", domain: "test"}
	e.Register(mod)

	// Set restricted permissions
	e.SetBusPermissions("perm-mod", BusPermissions{
		AllowEvents:  false,
		AllowData:    true,
		AllowCompute: false,
	})

	perms := e.Permissions().GetPermissions("perm-mod")
	if perms.AllowEvents {
		t.Error("expected events disabled")
	}
	if !perms.AllowData {
		t.Error("expected data enabled")
	}
	if perms.AllowCompute {
		t.Error("expected compute disabled")
	}
}

// TestEngineModuleDeps tests dependency management.
func TestEngineModuleDeps(t *testing.T) {
	e := New()

	mod1 := &simpleTestModule{name: "mod1", domain: "test"}
	mod2 := &simpleTestModule{name: "mod2", domain: "test"}

	e.Register(mod1)
	e.Register(mod2)
	e.SetModuleDeps("mod2", "mod1")

	deps := e.Dependencies().GetDeps("mod2")
	if len(deps) != 1 || deps[0] != "mod1" {
		t.Errorf("expected deps [mod1], got %v", deps)
	}
}

// TestEngineMetadata tests metadata operations.
func TestEngineMetadata(t *testing.T) {
	e := New()

	mod := &simpleTestModule{name: "meta-mod", domain: "test"}
	e.Register(mod)

	e.AddModuleNote("meta-mod", "test note")
	e.SetModuleCapabilities("meta-mod", "cap1", "cap2")
	e.SetModuleQuotas("meta-mod", map[string]string{"rate": "100/s"})
	e.SetModuleLayer("meta-mod", "infra")
	e.SetModuleLabel("meta-mod", "Metadata Module")

	info := e.ModulesInfo()
	if len(info) != 1 {
		t.Fatalf("expected 1 module info, got %d", len(info))
	}
	if info[0].Layer != "infra" {
		t.Errorf("expected layer infra, got %s", info[0].Layer)
	}
	if info[0].Label != "Metadata Module" {
		t.Errorf("expected label Metadata Module, got %s", info[0].Label)
	}
	if len(info[0].Notes) != 1 || info[0].Notes[0] != "test note" {
		t.Errorf("expected note, got %v", info[0].Notes)
	}
}

// TestEngineBusOperations tests basic bus operations.
func TestEngineBusOperations(t *testing.T) {
	e := New()

	var received int32
	ctx := context.Background()

	// Subscribe
	err := e.SubscribeEvent(ctx, "test-event", func(ctx context.Context, payload any) error {
		atomic.AddInt32(&received, 1)
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Publish
	err = e.PublishEvent(ctx, "test-event", "payload")
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	if atomic.LoadInt32(&received) != 1 {
		t.Error("event handler not called")
	}
}

// TestEngineLogger tests logger accessor.
func TestEngineLogger(t *testing.T) {
	e := New()
	if e.Logger() == nil {
		t.Error("expected non-nil logger")
	}

	var nilEngine *Engine
	if nilEngine.Logger() != nil {
		t.Error("expected nil logger for nil engine")
	}
}

// TestEngineTypedEngines tests typed engine accessors.
func TestEngineTypedEngines(t *testing.T) {
	e := New()

	// Register modules implementing various engine types
	accountMod := &accountModule{simpleTestModule: simpleTestModule{name: "acc", domain: "account"}}
	storeMod := &storeModule{simpleTestModule: simpleTestModule{name: "store", domain: "storage"}}
	computeMod := &computeModule{simpleTestModule: simpleTestModule{name: "compute", domain: "compute"}}
	dataMod := &dataModule{simpleTestModule: simpleTestModule{name: "data", domain: "data"}}
	ledgerMod := &ledgerModule{simpleTestModule: simpleTestModule{name: "ledger", domain: "ledger"}}
	rpcMod := &rpcModule{simpleTestModule: simpleTestModule{name: "rpc", domain: "rpc"}}

	e.Register(accountMod)
	e.Register(storeMod)
	e.Register(computeMod)
	e.Register(dataMod)
	e.Register(ledgerMod)
	e.Register(rpcMod)

	if len(e.AccountEngines()) != 1 {
		t.Errorf("expected 1 account engine, got %d", len(e.AccountEngines()))
	}
	if len(e.StoreEngines()) != 1 {
		t.Errorf("expected 1 store engine, got %d", len(e.StoreEngines()))
	}
	if len(e.ComputeEngines()) != 1 {
		t.Errorf("expected 1 compute engine, got %d", len(e.ComputeEngines()))
	}
	if len(e.DataEngines()) != 1 {
		t.Errorf("expected 1 data engine, got %d", len(e.DataEngines()))
	}
	if len(e.LedgerEngines()) != 1 {
		t.Errorf("expected 1 ledger engine, got %d", len(e.LedgerEngines()))
	}
	if len(e.RPCEngines()) != 1 {
		t.Errorf("expected 1 rpc engine, got %d", len(e.RPCEngines()))
	}
	// These should be empty
	if len(e.IndexerEngines()) != 0 {
		t.Errorf("expected 0 indexer engines, got %d", len(e.IndexerEngines()))
	}
	if len(e.DataSourceEngines()) != 0 {
		t.Errorf("expected 0 data source engines, got %d", len(e.DataSourceEngines()))
	}
	if len(e.ContractsEngines()) != 0 {
		t.Errorf("expected 0 contracts engines, got %d", len(e.ContractsEngines()))
	}
	if len(e.ServiceBankEngines()) != 0 {
		t.Errorf("expected 0 service bank engines, got %d", len(e.ServiceBankEngines()))
	}
	if len(e.CryptoEngines()) != 0 {
		t.Errorf("expected 0 crypto engines, got %d", len(e.CryptoEngines()))
	}
}

// TestEngineBusPushData tests data push operations.
func TestEngineBusPushData(t *testing.T) {
	e := New()

	dataMod := &dataModule{simpleTestModule: simpleTestModule{name: "data", domain: "data"}}
	e.Register(dataMod)

	ctx := context.Background()
	err := e.PushData(ctx, "test-topic", "test-payload")
	if err != nil {
		t.Fatalf("push data failed: %v", err)
	}

	if dataMod.lastTopic != "test-topic" {
		t.Errorf("expected topic test-topic, got %s", dataMod.lastTopic)
	}
}

// TestEngineBusInvokeCompute tests compute invocation.
func TestEngineBusInvokeCompute(t *testing.T) {
	e := New()

	computeMod := &computeModule{
		simpleTestModule: simpleTestModule{name: "compute", domain: "compute"},
		result:           "computed-result",
	}
	e.Register(computeMod)

	ctx := context.Background()
	results, err := e.InvokeComputeAll(ctx, "input")
	if err != nil {
		t.Fatalf("invoke compute failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Result != "computed-result" {
		t.Errorf("expected computed-result, got %v", results[0].Result)
	}
}

// TestBusLocalSubscribers tests local subscriber listing.
func TestBusLocalSubscribers(t *testing.T) {
	e := New()
	ctx := context.Background()

	e.SubscribeEvent(ctx, "event1", func(ctx context.Context, payload any) error { return nil })
	e.SubscribeEvent(ctx, "event2", func(ctx context.Context, payload any) error { return nil })

	subs := e.Bus().LocalSubscribers("event1")
	if subs != 1 {
		t.Errorf("expected 1 local subscriber for event1, got %d", subs)
	}

	events := e.Bus().LocalEvents()
	if len(events) != 2 {
		t.Errorf("expected 2 local events, got %d", len(events))
	}

	allEvents := e.Bus().AllEvents()
	if len(allEvents) != 2 {
		t.Errorf("expected 2 all events, got %d", len(allEvents))
	}

	e.Bus().ClearSubscribers()
	subs = e.Bus().LocalSubscribers("event1")
	if subs != 0 {
		t.Errorf("expected 0 local subscribers after clear, got %d", subs)
	}
}

// TestBusWithConfig tests bus with custom config.
func TestBusWithConfig(t *testing.T) {
	e := New()
	bus := NewBusWithConfig(e.Registry(), e.Permissions(), BusConfig{
		Timeout: 5000,
	})
	if bus == nil {
		t.Fatal("expected non-nil bus with config")
	}
	if bus.GetTimeout() != 5000 {
		t.Errorf("expected timeout 5000, got %d", bus.GetTimeout())
	}
}

// TestBusPermissionManagerAllAndClear tests bus permission manager all/clear.
func TestBusPermissionManagerAllAndClear(t *testing.T) {
	pm := NewBusPermissionManager()
	pm.SetPermissions("mod1", BusPermissions{AllowEvents: true})
	pm.SetPermissions("mod2", BusPermissions{AllowData: true})

	all := pm.AllPermissions()
	if len(all) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(all))
	}

	if !pm.HasPermission("mod1", "events") {
		t.Error("expected mod1 to have events permission")
	}
	if pm.HasPermission("mod1", "data") {
		t.Error("expected mod1 to not have data permission")
	}

	pm.Clear()
	all = pm.AllPermissions()
	if len(all) != 0 {
		t.Errorf("expected 0 permissions after clear, got %d", len(all))
	}
}

func TestMissingRequiredAPIs(t *testing.T) {
	e := New()
	// stub module exposing data surface
	dataMod := serviceModuleStub{name: "data", surface: APISurfaceData}
	// module requiring rpc which is missing
	reqMod := serviceModuleStub{name: "req", requires: []APISurface{APISurfaceRPC}}
	if err := e.Register(dataMod); err != nil {
		t.Fatalf("register data: %v", err)
	}
	if err := e.Register(reqMod); err != nil {
		t.Fatalf("register req: %v", err)
	}
	e.SetModuleRequiredAPIs("req", reqMod.requires...)

	missing := e.MissingRequiredAPIs()
	if len(missing) != 1 || len(missing["req"]) != 1 || missing["req"][0] != "rpc" {
		t.Fatalf("expected req to miss rpc, got %v", missing)
	}
}

func TestModuleInfoLayerDefaultsAndOverrides(t *testing.T) {
	e := New()
	noLayer := serviceModuleStub{name: "svc-no-layer"}
	withLayer := serviceModuleStub{name: "svc-infra"}

	if err := e.Register(noLayer); err != nil {
		t.Fatalf("register no layer: %v", err)
	}
	if err := e.Register(withLayer); err != nil {
		t.Fatalf("register with layer: %v", err)
	}
	e.SetModuleLayer("svc-infra", "infra")

	info := e.ModulesInfo()
	layerByName := map[string]string{}
	for _, i := range info {
		layerByName[i.Name] = i.Layer
	}
	if got := layerByName["svc-no-layer"]; got != "service" {
		t.Fatalf("expected default service layer, got %q", got)
	}
	if got := layerByName["svc-infra"]; got != "infra" {
		t.Fatalf("expected explicit layer infra, got %q", got)
	}
}

// serviceModuleStub helps validate MissingRequiredAPIs without pulling real services.
type serviceModuleStub struct {
	name     string
	surface  APISurface
	requires []APISurface
}

func (s serviceModuleStub) Name() string                  { return s.name }
func (serviceModuleStub) Domain() string                  { return "stub" }
func (serviceModuleStub) Start(ctx context.Context) error { return nil }
func (serviceModuleStub) Stop(ctx context.Context) error  { return nil }
func (s serviceModuleStub) APIs() []APIDescriptor {
	if s.surface == "" {
		return nil
	}
	return []APIDescriptor{{Surface: s.surface}}
}

// simpleTestModule is a flexible test module with configurable behavior.
type simpleTestModule struct {
	name    string
	domain  string
	startFn func(ctx context.Context) error
	stopFn  func(ctx context.Context) error
}

func (m *simpleTestModule) Name() string   { return m.name }
func (m *simpleTestModule) Domain() string { return m.domain }
func (m *simpleTestModule) Start(ctx context.Context) error {
	if m.startFn != nil {
		return m.startFn(ctx)
	}
	return nil
}
func (m *simpleTestModule) Stop(ctx context.Context) error {
	if m.stopFn != nil {
		return m.stopFn(ctx)
	}
	return nil
}

// Typed engine test modules

type accountModule struct {
	simpleTestModule
}

func (m *accountModule) CreateAccount(ctx context.Context, owner string, metadata map[string]string) (string, error) {
	return "acc-1", nil
}
func (m *accountModule) ListAccounts(ctx context.Context) ([]any, error) { return nil, nil }

type storeModule struct {
	simpleTestModule
}

func (m *storeModule) Ping(ctx context.Context) error { return nil }

type computeModule struct {
	simpleTestModule
	result any
}

func (m *computeModule) Invoke(ctx context.Context, payload any) (any, error) {
	return m.result, nil
}

type dataModule struct {
	simpleTestModule
	lastTopic   string
	lastPayload any
}

func (m *dataModule) Push(ctx context.Context, topic string, payload any) error {
	m.lastTopic = topic
	m.lastPayload = payload
	return nil
}

type ledgerModule struct {
	simpleTestModule
}

func (m *ledgerModule) LedgerInfo() string { return "test-ledger" }

type rpcModule struct {
	simpleTestModule
}

func (m *rpcModule) RPCInfo() string { return "test-rpc" }
func (m *rpcModule) RPCEndpoints() map[string]string {
	return map[string]string{"neo": "http://localhost:10332"}
}
