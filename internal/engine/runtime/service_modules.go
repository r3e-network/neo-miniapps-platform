package runtime

import (
	"context"
	"fmt"
	"strings"

	app "github.com/R3E-Network/service_layer/internal/app"
	engine "github.com/R3E-Network/service_layer/internal/engine"
	"github.com/R3E-Network/service_layer/internal/framework"
)

// wrapServices registers domain services as ServiceModules for lifecycle discipline.
func wrapServices(a *app.Application, eng *engine.Engine) error {
	if a == nil || eng == nil {
		return nil
	}

	if err := registerModule(eng, "svc-accounts", "accounts", a.Accounts, true); err != nil {
		return fmt.Errorf("register svc-accounts: %w", err)
	}
	if err := registerModule(eng, "svc-functions", "functions", a.Functions, true); err != nil {
		return fmt.Errorf("register svc-functions: %w", err)
	}
	if err := registerModule(eng, "svc-triggers", "triggers", a.Triggers, true); err != nil {
		return fmt.Errorf("register svc-triggers: %w", err)
	}
	if err := registerModule(eng, "svc-gasbank", "gasbank", a.GasBank, true); err != nil {
		return fmt.Errorf("register svc-gasbank: %w", err)
	}
	if err := registerModule(eng, "svc-automation", "automation", a.Automation, true); err != nil {
		return fmt.Errorf("register svc-automation: %w", err)
	}
	if err := registerModule(eng, "svc-pricefeed", "pricefeed", a.PriceFeeds, true); err != nil {
		return fmt.Errorf("register svc-pricefeed: %w", err)
	}
	if err := registerModule(eng, "svc-datafeeds", "datafeeds", a.DataFeeds, true); err != nil {
		return fmt.Errorf("register svc-datafeeds: %w", err)
	}
	if err := registerModule(eng, "svc-datastreams", "datastreams", a.DataStreams, true); err != nil {
		return fmt.Errorf("register svc-datastreams: %w", err)
	}
	if err := registerModule(eng, "svc-datalink", "datalink", a.DataLink, true); err != nil {
		return fmt.Errorf("register svc-datalink: %w", err)
	}
	if err := registerModule(eng, "svc-dta", "dta", a.DTA, true); err != nil {
		return fmt.Errorf("register svc-dta: %w", err)
	}
	if err := registerModule(eng, "svc-confidential", "confidential", a.Confidential, true); err != nil {
		return fmt.Errorf("register svc-confidential: %w", err)
	}
	if err := registerModule(eng, "svc-cre", "cre", a.CRE, true); err != nil {
		return fmt.Errorf("register svc-cre: %w", err)
	}
	if err := registerModule(eng, "svc-ccip", "ccip", a.CCIP, true); err != nil {
		return fmt.Errorf("register svc-ccip: %w", err)
	}
	if err := registerModule(eng, "svc-vrf", "vrf", a.VRF, true); err != nil {
		return fmt.Errorf("register svc-vrf: %w", err)
	}
	if err := registerModule(eng, "svc-secrets", "secrets", a.Secrets, true); err != nil {
		return fmt.Errorf("register svc-secrets: %w", err)
	}
	if err := registerModule(eng, "svc-random", "random", a.Random, true); err != nil {
		return fmt.Errorf("register svc-random: %w", err)
	}
	if err := registerModule(eng, "svc-oracle", "oracle", a.Oracle, true); err != nil {
		return fmt.Errorf("register svc-oracle: %w", err)
	}
	if err := registerModuleOptional(eng, "runner-automation", "automation", a.AutomationRunner, true); err != nil {
		return fmt.Errorf("register runner-automation: %w", err)
	}
	if err := registerModuleOptional(eng, "runner-pricefeed", "pricefeed", a.PriceFeedRunner, true); err != nil {
		return fmt.Errorf("register runner-pricefeed: %w", err)
	}
	if err := registerModuleOptional(eng, "runner-oracle", "oracle", a.OracleRunner, true); err != nil {
		return fmt.Errorf("register runner-oracle: %w", err)
	}
	if err := registerModuleOptional(eng, "runner-gasbank", "gasbank", a.GasBankSettlement, true); err != nil {
		return fmt.Errorf("register runner-gasbank: %w", err)
	}
	return nil
}

// serviceModule is a small adapter to fit services into core engine.
type serviceModule struct {
	NameValue        string
	DomainValue      string
	StartFunc        func(context.Context) error
	StopFunc         func(context.Context) error
	ReadyFunc        func(context.Context) error
	ReadySetFunc     func(string, string)
	DataFunc         func(context.Context, string, any) error
	EventFunc        func(context.Context, string, any) error
	SubscribeFunc    func(context.Context, string, func(context.Context, any) error) error
	ComputeFunc      func(context.Context, any) (any, error)
	AccountFunc      func(context.Context, string, map[string]string) (string, error)
	ListAccountsFunc func(context.Context) ([]any, error)
}

func (s serviceModule) Name() string   { return s.NameValue }
func (s serviceModule) Domain() string { return s.DomainValue }
func (s serviceModule) Start(ctx context.Context) error {
	if s.StartFunc == nil {
		return nil
	}
	return s.StartFunc(ctx)
}
func (s serviceModule) Stop(ctx context.Context) error {
	if s.StopFunc == nil {
		return nil
	}
	return s.StopFunc(ctx)
}
func (s serviceModule) Ready(ctx context.Context) error {
	if s.ReadyFunc == nil {
		return nil
	}
	return s.ReadyFunc(ctx)
}
func (s serviceModule) SetReady(status, errMsg string) {
	if s.ReadySetFunc != nil {
		s.ReadySetFunc(status, errMsg)
	}
}
func (s serviceModule) HasAccount() bool { return s.AccountFunc != nil && s.ListAccountsFunc != nil }
func (s serviceModule) HasCompute() bool { return s.ComputeFunc != nil }
func (s serviceModule) HasData() bool    { return s.DataFunc != nil }
func (s serviceModule) HasEvent() bool   { return s.EventFunc != nil }

// Push implements engine.DataEngine when provided.
func (s serviceModule) Push(ctx context.Context, topic string, payload any) error {
	if s.DataFunc == nil {
		return fmt.Errorf("data push not supported")
	}
	return s.DataFunc(ctx, topic, payload)
}

// Publish implements engine.EventEngine when provided.
func (s serviceModule) Publish(ctx context.Context, event string, payload any) error {
	if s.EventFunc == nil {
		return fmt.Errorf("event publish not supported")
	}
	return s.EventFunc(ctx, event, payload)
}

// Subscribe implements engine.EventEngine subscribe when supported; no-op here for simplicity.
func (s serviceModule) Subscribe(ctx context.Context, event string, handler func(context.Context, any) error) error {
	if s.SubscribeFunc == nil {
		return fmt.Errorf("subscribe not supported")
	}
	return s.SubscribeFunc(ctx, event, handler)
}

// Invoke implements engine.ComputeEngine when provided.
func (s serviceModule) Invoke(ctx context.Context, payload any) (any, error) {
	if s.ComputeFunc == nil {
		return nil, fmt.Errorf("compute invoke not supported")
	}
	return s.ComputeFunc(ctx, payload)
}

// CreateAccount implements engine.AccountEngine when provided.
func (s serviceModule) CreateAccount(ctx context.Context, owner string, metadata map[string]string) (string, error) {
	if s.AccountFunc == nil {
		return "", fmt.Errorf("account creation not supported")
	}
	return s.AccountFunc(ctx, owner, metadata)
}

// ListAccounts implements engine.AccountEngine when provided.
func (s serviceModule) ListAccounts(ctx context.Context) ([]any, error) {
	if s.ListAccountsFunc == nil {
		return nil, fmt.Errorf("list accounts not supported")
	}
	return s.ListAccountsFunc(ctx)
}

// wrapStart/Stop allow services that expose optional Start/Stop to be plugged into the engine.
func wrapStart(s any) func(context.Context) error {
	type starter interface{ Start(context.Context) error }
	if v, ok := s.(starter); ok {
		return v.Start
	}
	return nil
}

func wrapStop(s any) func(context.Context) error {
	type stopper interface{ Stop(context.Context) error }
	if v, ok := s.(stopper); ok {
		return v.Stop
	}
	return nil
}

func wrapReady(s any) func(context.Context) error {
	type ready interface{ Ready(context.Context) error }
	if v, ok := s.(ready); ok {
		return v.Ready
	}
	return nil
}

func wrapReadySetter(s any) func(string, string) {
	type readySetter interface{ SetReady(string, string) }
	if v, ok := s.(readySetter); ok {
		return v.SetReady
	}
	return nil
}

func wrapData(s any) func(context.Context, string, any) error {
	type data interface {
		Push(context.Context, string, any) error
	}
	if v, ok := s.(data); ok {
		return v.Push
	}
	return nil
}

func wrapEvent(s any) func(context.Context, string, any) error {
	type event interface {
		Publish(context.Context, string, any) error
	}
	if v, ok := s.(event); ok {
		return v.Publish
	}
	return nil
}

func wrapSubscribe(s any) func(context.Context, string, func(context.Context, any) error) error {
	type subscriber interface {
		Subscribe(context.Context, string, func(context.Context, any) error) error
	}
	if v, ok := s.(subscriber); ok {
		return v.Subscribe
	}
	return nil
}

func wrapCompute(s any) func(context.Context, any) (any, error) {
	type compute interface {
		Invoke(context.Context, any) (any, error)
	}
	if v, ok := s.(compute); ok {
		return v.Invoke
	}
	return nil
}

func wrapAccounts(s any) (func(context.Context, string, map[string]string) (string, error), func(context.Context) ([]any, error)) {
	type account interface {
		CreateAccount(context.Context, string, map[string]string) (string, error)
		ListAccounts(context.Context) ([]any, error)
	}
	if v, ok := s.(account); ok {
		return v.CreateAccount, v.ListAccounts
	}
	return nil, nil
}

func moduleName(mod any, fallback string) string {
	if fallback = strings.TrimSpace(fallback); fallback != "" {
		return fallback
	}
	type named interface{ Name() string }
	if v, ok := mod.(named); ok {
		if n := strings.TrimSpace(v.Name()); n != "" {
			return n
		}
	}
	return ""
}

func moduleDomain(mod any, fallback string) string {
	if fallback = strings.TrimSpace(fallback); fallback != "" {
		return fallback
	}
	type domained interface{ Domain() string }
	if v, ok := mod.(domained); ok {
		if d := strings.TrimSpace(v.Domain()); d != "" {
			return d
		}
	}
	return ""
}

// Merge intended descriptors into module metadata so /system/status can present normalized names/domains.
func normalizeDescriptorNameDomain(name, domain string) (string, string) {
	return strings.TrimSpace(name), strings.TrimSpace(domain)
}

// ensureUniqueModuleName appends a deterministic suffix when a name collision occurs.
func ensureUniqueModuleName(eng *engine.Engine, base, domain string) (string, bool) {
	if eng == nil {
		return base, false
	}
	candidate := base
	suffix := 1
	collided := false
	for {
		collision := false
		for _, existing := range eng.Modules() {
			if existing == candidate {
				collision = true
				break
			}
		}
		if !collision {
			return candidate, collided
		}
		collided = true
		candidate = fmt.Sprintf("%s-%s-%d", base, domain, suffix)
		suffix++
	}
}

// registerModule adapts a runtime service to a core engine module with optional lifecycle hooks.
func registerModule(eng *engine.Engine, name, domain string, mod any, manageLifecycle bool) error {
	if eng == nil {
		return nil
	}
	if mod == nil {
		return fmt.Errorf("module %q (domain=%s) is nil", name, domain)
	}

	// Prefer descriptor-provided names/domains for stability; fall back to service hints when descriptors are empty.
	name, domain = normalizeDescriptorNameDomain(moduleName(mod, name), moduleDomain(mod, domain))
	if name == "" {
		return fmt.Errorf("module name required")
	}
	if domain == "" {
		return fmt.Errorf("module domain required")
	}
	name, collided := ensureUniqueModuleName(eng, name, domain)
	if collided {
		note := fmt.Sprintf("name collision, registered as %q", name)
		if l := eng.Logger(); l != nil {
			l.Printf("runtime: module name collision detected, %s (domain=%s)", name, domain)
		}
		eng.AddModuleNote(name, note)
	}

	sm := serviceModule{
		NameValue:    name,
		DomainValue:  domain,
		ReadyFunc:    wrapReady(mod),
		ReadySetFunc: wrapReadySetter(mod),
	}
	// Set explicit bus permissions based on implemented interfaces.
	perms := engine.BusPermissions{}
	if _, ok := mod.(engine.EventEngine); ok {
		perms.AllowEvents = true
		if cap, ok := mod.(interface{ HasEvent() bool }); ok && !cap.HasEvent() {
			perms.AllowEvents = false
		}
	}
	if _, ok := mod.(engine.DataEngine); ok {
		perms.AllowData = true
		if cap, ok := mod.(interface{ HasData() bool }); ok && !cap.HasData() {
			perms.AllowData = false
		}
	}
	if _, ok := mod.(engine.ComputeEngine); ok {
		perms.AllowCompute = true
		if cap, ok := mod.(interface{ HasCompute() bool }); ok && !cap.HasCompute() {
			perms.AllowCompute = false
		}
	}
	if perms.AllowEvents || perms.AllowData || perms.AllowCompute {
		eng.SetBusPermissions(name, perms)
	}
	if manageLifecycle {
		sm.StartFunc = wrapStart(mod)
		sm.StopFunc = wrapStop(mod)
	}
	if asData := wrapData(mod); asData != nil {
		sm.DataFunc = asData
	}
	if asEvent := wrapEvent(mod); asEvent != nil {
		sm.EventFunc = asEvent
	}
	if asSub := wrapSubscribe(mod); asSub != nil {
		sm.SubscribeFunc = asSub
	}
	if asCompute := wrapCompute(mod); asCompute != nil {
		sm.ComputeFunc = asCompute
	}
	if asAccountCreate, asAccountList := wrapAccounts(mod); asAccountCreate != nil && asAccountList != nil {
		sm.AccountFunc = asAccountCreate
		sm.ListAccountsFunc = asAccountList
	}
	if mp, ok := mod.(interface{ Manifest() *framework.Manifest }); ok {
		if manifest := mp.Manifest(); manifest != nil {
			manifest.Normalize()
			if err := manifest.Validate(); err == nil {
				if manifest.Layer != "" {
					eng.AddModuleNote(name, "layer: "+manifest.Layer)
					eng.SetModuleLayer(name, manifest.Layer)
				}
				if len(manifest.DependsOn) > 0 {
					var deps []string
					for _, dep := range manifest.DependsOn {
						if eng.Lookup(dep) != nil {
							deps = append(deps, dep)
						} else {
							eng.AddModuleNote(name, fmt.Sprintf("declares dependency %q (not registered)", dep))
						}
					}
					if len(deps) > 0 {
						eng.SetModuleDeps(name, deps...)
					}
				}
				if len(manifest.Capabilities) > 0 {
					eng.SetModuleCapabilities(name, manifest.Capabilities...)
				}
				if len(manifest.Quotas) > 0 {
					eng.SetModuleQuotas(name, manifest.Quotas)
				}
				if len(manifest.RequiresAPIs) > 0 {
					eng.SetModuleRequiredAPIs(name, manifest.RequiresAPIs...)
				}
				for _, cap := range manifest.Capabilities {
					eng.AddModuleNote(name, "capability: "+cap)
				}
				for k, v := range manifest.Quotas {
					eng.AddModuleNote(name, fmt.Sprintf("quota %s=%s", k, v))
				}
			}
		}
	}
	return eng.Register(sm)
}

// registerModuleOptional registers when a module is present; skips nil modules quietly.
func registerModuleOptional(eng *engine.Engine, name, domain string, mod any, manageLifecycle bool) error {
	if mod == nil {
		return nil
	}
	return registerModule(eng, name, domain, mod, manageLifecycle)
}
