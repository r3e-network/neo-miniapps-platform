// Package pkg provides sandbox-aware runtime that integrates the sandbox system
// with the existing PackageRuntime interface.
package pkg

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	engine "github.com/R3E-Network/service_layer/system/os/core"
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
	"github.com/R3E-Network/service_layer/system/os/sandbox"
)

// =============================================================================
// Sandboxed Package Runtime
// =============================================================================

// SandboxedRuntime implements PackageRuntime with full sandbox isolation.
// This wraps a ServiceSandbox and provides the standard PackageRuntime interface.
type SandboxedRuntime struct {
	// Sandbox components
	sandbox *sandbox.ServiceSandbox
	manager *sandbox.Manager

	// Legacy runtime components (for backward compatibility)
	packageID     string
	manifest      PackageManifest
	engine        *engine.Engine
	config        PackageConfig
	storeProvider StoreProvider
	tracer        core.Tracer
	metrics       framework.Metrics

	// Quota enforcer (wraps sandbox capabilities)
	quotaEnforcer *sandboxQuotaEnforcer
}

// SandboxedRuntimeConfig contains configuration for creating a sandboxed runtime.
type SandboxedRuntimeConfig struct {
	PackageID     string
	Manifest      PackageManifest
	Engine        *engine.Engine
	Config        PackageConfig
	StoreProvider StoreProvider
	Tracer        core.Tracer
	Metrics       framework.Metrics
	DB            *sql.DB
}

// NewSandboxedRuntime creates a new sandboxed runtime for a package.
func NewSandboxedRuntime(
	ctx context.Context,
	manager *sandbox.Manager,
	cfg SandboxedRuntimeConfig,
) (*SandboxedRuntime, error) {
	// Map manifest permissions to sandbox capabilities
	caps := mapPermissionsToCapabilities(cfg.Manifest.Permissions)

	// Determine security level based on package
	secLevel := determineSecurityLevel(cfg.PackageID, cfg.Manifest)

	// Create sandbox request
	req := sandbox.CreateSandboxRequest{
		ServiceID:             cfg.PackageID,
		PackageID:             cfg.PackageID,
		SecurityLevel:         secLevel,
		RequestedCapabilities: caps,
		StorageQuota:          cfg.Manifest.Resources.MaxStorageBytes,
		AllowedTables:         extractAllowedTables(cfg.PackageID),
	}

	// Create the sandbox
	sb, err := manager.CreateSandbox(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}

	return &SandboxedRuntime{
		sandbox:       sb,
		manager:       manager,
		packageID:     cfg.PackageID,
		manifest:      cfg.Manifest,
		engine:        cfg.Engine,
		config:        cfg.Config,
		storeProvider: cfg.StoreProvider,
		tracer:        cfg.Tracer,
		metrics:       cfg.Metrics,
		quotaEnforcer: newSandboxQuotaEnforcer(sb, cfg.Manifest.Resources),
	}, nil
}

// =============================================================================
// PackageRuntime Interface Implementation
// =============================================================================

func (r *SandboxedRuntime) Logger() any {
	if r.engine == nil {
		return nil
	}
	return r.engine.Logger()
}

func (r *SandboxedRuntime) Config() PackageConfig {
	return r.config
}

func (r *SandboxedRuntime) Storage() (PackageStorage, error) {
	// Check capability through sandbox
	if err := r.sandbox.Context.CheckCapability(context.Background(), sandbox.CapStorageRead); err != nil {
		return nil, fmt.Errorf("permission denied: storage access requires storage.read capability")
	}

	// Return sandbox-isolated storage wrapped as PackageStorage
	return &sandboxStorageAdapter{
		storage: r.sandbox.Storage,
	}, nil
}

func (r *SandboxedRuntime) StoreProvider() StoreProvider {
	if r.storeProvider == nil {
		return NilStoreProvider()
	}
	return r.storeProvider
}

func (r *SandboxedRuntime) Bus() (framework.BusClient, error) {
	// Check capability through sandbox
	hasPub := r.sandbox.Caps.Has(sandbox.CapBusPublish)
	hasSub := r.sandbox.Caps.Has(sandbox.CapBusSubscribe)

	if !hasPub && !hasSub {
		return nil, fmt.Errorf("permission denied: bus access requires bus.publish or bus.subscribe capability")
	}

	// Return sandbox-secured bus client
	return &sandboxBusAdapter{
		sandbox: r.sandbox,
		bus:     r.engine.Bus(),
		manager: r.manager,
	}, nil
}

func (r *SandboxedRuntime) RPCClient() (any, error) {
	// Check capability
	if !r.sandbox.Caps.Has(sandbox.CapNetworkOutbound) {
		return nil, fmt.Errorf("permission denied: RPC access requires network.outbound capability")
	}

	rpcEngines := r.engine.RPCEngines()
	if len(rpcEngines) == 0 {
		return nil, fmt.Errorf("no RPC engines available")
	}
	return rpcEngines[0], nil
}

func (r *SandboxedRuntime) LedgerClient() (any, error) {
	// Check capability
	if !r.sandbox.Caps.Has(sandbox.CapNetworkOutbound) {
		return nil, fmt.Errorf("permission denied: ledger access requires network.outbound capability")
	}

	ledgerEngines := r.engine.LedgerEngines()
	if len(ledgerEngines) == 0 {
		return nil, fmt.Errorf("no ledger engines available")
	}
	return ledgerEngines[0], nil
}

func (r *SandboxedRuntime) EnforceQuota(resource string, amount int64) error {
	return r.quotaEnforcer.Enforce(resource, amount)
}

func (r *SandboxedRuntime) Quota() framework.QuotaEnforcer {
	return r.quotaEnforcer
}

func (r *SandboxedRuntime) Metrics() framework.Metrics {
	if r.metrics == nil {
		return framework.NoopMetrics()
	}
	return r.metrics
}

func (r *SandboxedRuntime) Tracer() core.Tracer {
	if r.tracer == nil {
		return core.NoopTracer
	}
	return r.tracer
}

// =============================================================================
// Sandbox-Specific Methods
// =============================================================================

// Sandbox returns the underlying ServiceSandbox.
func (r *SandboxedRuntime) Sandbox() *sandbox.ServiceSandbox {
	return r.sandbox
}

// IPC returns the IPC proxy for inter-service communication.
func (r *SandboxedRuntime) IPC() *sandbox.IPCProxy {
	return r.sandbox.IPC
}

// CheckCapability checks if the service has a specific capability.
func (r *SandboxedRuntime) CheckCapability(ctx context.Context, cap sandbox.Capability) error {
	return r.sandbox.Context.CheckCapability(ctx, cap)
}

// Identity returns the service's identity.
func (r *SandboxedRuntime) Identity() *sandbox.ServiceIdentity {
	return r.sandbox.Identity
}

// =============================================================================
// Adapters
// =============================================================================

// sandboxStorageAdapter adapts IsolatedStorage to PackageStorage interface.
type sandboxStorageAdapter struct {
	storage *sandbox.IsolatedStorage
}

func (a *sandboxStorageAdapter) Set(ctx context.Context, key string, value []byte) error {
	return a.storage.Set(ctx, key, value)
}

func (a *sandboxStorageAdapter) Get(ctx context.Context, key string) ([]byte, error) {
	return a.storage.Get(ctx, key)
}

func (a *sandboxStorageAdapter) Delete(ctx context.Context, key string) error {
	return a.storage.Delete(ctx, key)
}

func (a *sandboxStorageAdapter) List(ctx context.Context, prefix string) ([]string, error) {
	return a.storage.List(ctx, prefix)
}

func (a *sandboxStorageAdapter) UsedBytes() int64 {
	return a.storage.Quota().UsedBytes
}

func (a *sandboxStorageAdapter) AvailableBytes() int64 {
	q := a.storage.Quota()
	if q.MaxBytes <= 0 {
		return -1
	}
	return q.MaxBytes - q.UsedBytes
}

// sandboxBusAdapter adapts the engine Bus with sandbox security.
type sandboxBusAdapter struct {
	sandbox *sandbox.ServiceSandbox
	bus     *engine.Bus
	manager *sandbox.Manager
}

func (a *sandboxBusAdapter) PublishEvent(ctx context.Context, event string, payload any) error {
	// Verify capability
	if !a.sandbox.Caps.Has(sandbox.CapBusPublish) {
		return fmt.Errorf("permission denied: bus.publish capability required")
	}

	// Wrap payload with caller identity
	securePayload := map[string]any{
		"_caller":    a.sandbox.Identity.ServiceID,
		"_timestamp": a.sandbox.Identity.CreatedAt,
		"payload":    payload,
	}

	return a.bus.PublishEvent(ctx, event, securePayload)
}

func (a *sandboxBusAdapter) PushData(ctx context.Context, topic string, payload any) error {
	// Verify capability
	if !a.sandbox.Caps.Has(sandbox.CapBusPublish) {
		return fmt.Errorf("permission denied: bus.publish capability required")
	}

	return a.bus.PushData(ctx, topic, payload)
}

func (a *sandboxBusAdapter) InvokeCompute(ctx context.Context, payload any) ([]framework.ComputeResult, error) {
	// Verify capability
	if !a.sandbox.Caps.Has(sandbox.CapBusInvoke) {
		return nil, fmt.Errorf("permission denied: bus.invoke capability required")
	}

	results, err := a.bus.InvokeComputeAll(ctx, payload)
	if err != nil {
		return nil, err
	}

	fwResults := make([]framework.ComputeResult, len(results))
	for i, r := range results {
		fwResults[i] = framework.ComputeResult{
			Module: r.Module,
			Result: r.Result,
			Err:    r.Err,
		}
	}

	return fwResults, nil
}

// sandboxQuotaEnforcer wraps sandbox capabilities with quota enforcement.
type sandboxQuotaEnforcer struct {
	sandbox *sandbox.ServiceSandbox
	quotas  ResourceQuotas

	// Quota tracking state
	mu           sync.Mutex
	storageUsed  int64
	databaseUsed int64
	networkUsed  int64
}

func newSandboxQuotaEnforcer(sb *sandbox.ServiceSandbox, quotas ResourceQuotas) *sandboxQuotaEnforcer {
	return &sandboxQuotaEnforcer{
		sandbox: sb,
		quotas:  quotas,
	}
}

func (e *sandboxQuotaEnforcer) Enforce(resource string, amount int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Map resource to capability check and quota enforcement
	switch resource {
	case "storage":
		if !e.sandbox.Caps.Has(sandbox.CapStorageWrite) {
			return fmt.Errorf("storage write not permitted")
		}
		// Check storage quota
		if e.quotas.MaxStorageBytes > 0 {
			if e.storageUsed+amount > e.quotas.MaxStorageBytes {
				return &QuotaExceededError{
					Resource: "storage",
					Used:     e.storageUsed,
					Limit:    e.quotas.MaxStorageBytes,
					Requested: amount,
				}
			}
			e.storageUsed += amount
		}

	case "database":
		if !e.sandbox.Caps.Has(sandbox.CapDatabaseWrite) {
			return fmt.Errorf("database write not permitted")
		}
		// Check database quota (using storage bytes as proxy)
		if e.quotas.MaxStorageBytes > 0 {
			if e.databaseUsed+amount > e.quotas.MaxStorageBytes {
				return &QuotaExceededError{
					Resource: "database",
					Used:     e.databaseUsed,
					Limit:    e.quotas.MaxStorageBytes,
					Requested: amount,
				}
			}
			e.databaseUsed += amount
		}

	case "network":
		if !e.sandbox.Caps.Has(sandbox.CapNetworkOutbound) {
			return fmt.Errorf("network access not permitted")
		}
		// Track network usage (no hard limit, but track for monitoring)
		e.networkUsed += amount
	}

	return nil
}

// Release releases previously allocated quota (e.g., when data is deleted).
func (e *sandboxQuotaEnforcer) Release(resource string, amount int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch resource {
	case "storage":
		e.storageUsed -= amount
		if e.storageUsed < 0 {
			e.storageUsed = 0
		}
	case "database":
		e.databaseUsed -= amount
		if e.databaseUsed < 0 {
			e.databaseUsed = 0
		}
	}
}

// Usage returns current quota usage for a resource.
func (e *sandboxQuotaEnforcer) Usage(resource string) (used, limit int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch resource {
	case "storage":
		return e.storageUsed, e.quotas.MaxStorageBytes
	case "database":
		return e.databaseUsed, e.quotas.MaxStorageBytes
	case "network":
		return e.networkUsed, 0 // No hard limit for network
	}
	return 0, 0
}

// QuotaExceededError is returned when a resource quota is exceeded.
type QuotaExceededError struct {
	Resource  string
	Used      int64
	Limit     int64
	Requested int64
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("quota exceeded for %s: used %d + requested %d > limit %d",
		e.Resource, e.Used, e.Requested, e.Limit)
}

// =============================================================================
// Helper Functions
// =============================================================================

// mapPermissionsToCapabilities converts manifest permissions to sandbox capabilities.
func mapPermissionsToCapabilities(perms []Permission) []sandbox.Capability {
	var caps []sandbox.Capability

	for _, perm := range perms {
		switch perm.Name {
		case "engine.api.storage":
			caps = append(caps, sandbox.CapStorageRead, sandbox.CapStorageWrite)
		case "engine.api.bus":
			caps = append(caps, sandbox.CapBusPublish, sandbox.CapBusSubscribe, sandbox.CapBusInvoke)
		case "engine.api.rpc":
			caps = append(caps, sandbox.CapNetworkOutbound)
		case "engine.api.ledger":
			caps = append(caps, sandbox.CapNetworkOutbound)
		case "engine.api.crypto":
			caps = append(caps, sandbox.CapCryptoSign, sandbox.CapCryptoEncrypt)
		case "engine.api.service.call":
			caps = append(caps, sandbox.CapServiceCall)
		case "engine.api.database":
			caps = append(caps, sandbox.CapDatabaseRead, sandbox.CapDatabaseWrite)
		}
	}

	return caps
}

// determineSecurityLevel determines the security level for a package.
func determineSecurityLevel(packageID string, manifest PackageManifest) sandbox.SecurityLevel {
	// System packages get system level
	if len(packageID) > 7 && packageID[:7] == "system." {
		return sandbox.SecurityLevelSystem
	}

	// Core R3E services get privileged level
	if len(packageID) > 15 && packageID[:15] == "com.r3e.services" {
		return sandbox.SecurityLevelPrivileged
	}

	// Check manifest metadata for trust level
	if level, ok := manifest.Metadata["security_level"]; ok {
		switch level {
		case "system":
			return sandbox.SecurityLevelSystem
		case "privileged":
			return sandbox.SecurityLevelPrivileged
		case "untrusted":
			return sandbox.SecurityLevelUntrusted
		}
	}

	return sandbox.SecurityLevelNormal
}

// extractAllowedTables extracts allowed database tables for a package.
func extractAllowedTables(packageID string) []string {
	// Generate table prefix from package ID
	prefix := sandbox.GenerateServiceID(packageID, "")[:8]
	return []string{
		prefix + "_*", // Allow all tables with this prefix
	}
}
