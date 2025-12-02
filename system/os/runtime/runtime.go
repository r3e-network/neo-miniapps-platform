package pkg

import (
	"context"
	"fmt"
	"sync"
	"time"

	engine "github.com/R3E-Network/service_layer/system/os/core"
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// runtime is the default implementation of PackageRuntime.
// It provides sandboxed access to engine resources based on granted permissions.
type runtime struct {
	packageID   string
	manifest    PackageManifest
	engine      *engine.Engine
	config      PackageConfig
	storage     PackageStorage
	permissions map[string]bool // granted permissions

	// StoreProvider for typed database access (Android ContentResolver equivalent)
	storeProvider StoreProvider

	// Resource tracking
	quotaEnforcer *quotaEnforcer
	tracer        core.Tracer
	metrics       framework.Metrics
}

// NewPackageRuntime creates a runtime context for a service package.
func NewPackageRuntime(
	packageID string,
	manifest PackageManifest,
	eng *engine.Engine,
	config PackageConfig,
	permissions map[string]bool,
	storeProvider StoreProvider,
	tracer core.Tracer,
	metrics framework.Metrics,
) PackageRuntime {
	r := &runtime{
		packageID:     packageID,
		manifest:      manifest,
		engine:        eng,
		config:        config,
		permissions:   permissions,
		storeProvider: storeProvider,
		quotaEnforcer: newQuotaEnforcer(manifest.Resources),
		tracer:        tracer,
		metrics:       metrics,
	}
	if r.tracer == nil {
		r.tracer = core.NoopTracer
	}
	if r.metrics == nil {
		r.metrics = framework.NoopMetrics()
	}

	// Initialize package-specific storage
	r.storage = newPackageStorage(packageID, manifest.Resources.MaxStorageBytes)

	return r
}

func (r *runtime) Logger() any {
	if r.engine == nil {
		return nil
	}
	return r.engine.Logger()
}

func (r *runtime) Config() PackageConfig {
	return r.config
}

func (r *runtime) Storage() (PackageStorage, error) {
	if !r.hasPermission("engine.api.storage") {
		return nil, fmt.Errorf("permission denied: engine.api.storage")
	}
	return r.storage, nil
}

// StoreProvider returns the typed database store provider.
// This provides Android ContentResolver-style access to domain-specific stores.
func (r *runtime) StoreProvider() StoreProvider {
	if r.storeProvider == nil {
		return NilStoreProvider()
	}
	return r.storeProvider
}

func (r *runtime) Bus() (framework.BusClient, error) {
	if !r.hasPermission("engine.api.bus") {
		return nil, fmt.Errorf("permission denied: engine.api.bus")
	}
	// Return a wrapped bus client that enforces quotas
	return newQuotaEnforcedBus(r.engine.Bus(), r.quotaEnforcer), nil
}

func (r *runtime) RPCClient() (any, error) {
	if !r.hasPermission("engine.api.rpc") {
		return nil, fmt.Errorf("permission denied: engine.api.rpc")
	}
	// Return RPC client from engine
	rpcEngines := r.engine.RPCEngines()
	if len(rpcEngines) == 0 {
		return nil, fmt.Errorf("no RPC engines available")
	}
	return rpcEngines[0], nil
}

func (r *runtime) LedgerClient() (any, error) {
	if !r.hasPermission("engine.api.ledger") {
		return nil, fmt.Errorf("permission denied: engine.api.ledger")
	}
	ledgerEngines := r.engine.LedgerEngines()
	if len(ledgerEngines) == 0 {
		return nil, fmt.Errorf("no ledger engines available")
	}
	return ledgerEngines[0], nil
}

func (r *runtime) EnforceQuota(resource string, amount int64) error {
	return r.quotaEnforcer.Enforce(resource, amount)
}

func (r *runtime) Quota() framework.QuotaEnforcer {
	if r.quotaEnforcer == nil {
		return framework.NoopQuota()
	}
	return r.quotaEnforcer
}

func (r *runtime) Metrics() framework.Metrics {
	if r.metrics == nil {
		return framework.NoopMetrics()
	}
	return r.metrics
}

func (r *runtime) Tracer() core.Tracer {
	if r.tracer == nil {
		return core.NoopTracer
	}
	return r.tracer
}

func (r *runtime) hasPermission(perm string) bool {
	if r.permissions == nil {
		return false
	}
	return r.permissions[perm]
}

// =============================================================================
// Package Storage Implementation
// =============================================================================

// packageStorage provides isolated key-value storage for a package.
type packageStorage struct {
	packageID string
	mu        sync.RWMutex
	data      map[string][]byte
	usedBytes int64
	maxBytes  int64
}

func newPackageStorage(packageID string, maxBytes int64) *packageStorage {
	return &packageStorage{
		packageID: packageID,
		data:      make(map[string][]byte),
		maxBytes:  maxBytes,
	}
}

func (s *packageStorage) Set(ctx context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = ctx

	// Calculate size change
	oldSize := int64(0)
	if old, exists := s.data[key]; exists {
		oldSize = int64(len(old))
	}
	newSize := int64(len(value))
	sizeDelta := newSize - oldSize

	// Check quota
	if s.maxBytes > 0 && s.usedBytes+sizeDelta > s.maxBytes {
		return fmt.Errorf("storage quota exceeded: %d/%d bytes", s.usedBytes+sizeDelta, s.maxBytes)
	}

	s.data[key] = value
	s.usedBytes += sizeDelta
	return nil
}

func (s *packageStorage) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_ = ctx

	value, exists := s.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	// Return a copy to prevent external modification
	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

func (s *packageStorage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = ctx

	if value, exists := s.data[key]; exists {
		s.usedBytes -= int64(len(value))
		delete(s.data, key)
	}
	return nil
}

func (s *packageStorage) List(ctx context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_ = ctx

	var keys []string
	for key := range s.data {
		if prefix == "" || len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (s *packageStorage) UsedBytes() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.usedBytes
}

func (s *packageStorage) AvailableBytes() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.maxBytes <= 0 {
		return -1 // unlimited
	}
	return s.maxBytes - s.usedBytes
}

// =============================================================================
// Package Config Implementation
// =============================================================================

// simpleConfig is a simple map-based configuration implementation.
type simpleConfig struct {
	data map[string]string
}

// NewPackageConfig creates a package configuration from a map.
func NewPackageConfig(data map[string]string) PackageConfig {
	if data == nil {
		data = make(map[string]string)
	}
	return &simpleConfig{data: data}
}

func (c *simpleConfig) Get(key string) (string, bool) {
	val, ok := c.data[key]
	return val, ok
}

func (c *simpleConfig) GetInt(key string) (int, bool) {
	val, ok := c.data[key]
	if !ok {
		return 0, false
	}
	var i int
	_, err := fmt.Sscanf(val, "%d", &i)
	if err != nil {
		return 0, false
	}
	return i, true
}

func (c *simpleConfig) GetBool(key string) (bool, bool) {
	val, ok := c.data[key]
	if !ok {
		return false, false
	}
	switch val {
	case "true", "1", "yes":
		return true, true
	case "false", "0", "no":
		return false, true
	default:
		return false, false
	}
}

func (c *simpleConfig) GetAll() map[string]string {
	// Return a copy
	copy := make(map[string]string, len(c.data))
	for k, v := range c.data {
		copy[k] = v
	}
	return copy
}

// =============================================================================
// Quota Enforcement
// =============================================================================

// quotaEnforcer tracks and enforces resource quotas.
type quotaEnforcer struct {
	mu       sync.RWMutex
	quotas   ResourceQuotas
	counters map[string]*rateLimiter
}

var _ framework.QuotaEnforcer = (*quotaEnforcer)(nil)

func newQuotaEnforcer(quotas ResourceQuotas) *quotaEnforcer {
	return &quotaEnforcer{
		quotas:   quotas,
		counters: make(map[string]*rateLimiter),
	}
}

func (qe *quotaEnforcer) Enforce(resource string, amount int64) error {
	qe.mu.RLock()
	limiter, exists := qe.counters[resource]
	qe.mu.RUnlock()

	if !exists {
		qe.mu.Lock()
		// Double-check after acquiring write lock
		limiter, exists = qe.counters[resource]
		if !exists {
			limiter = newRateLimiter()
			qe.counters[resource] = limiter
		}
		qe.mu.Unlock()
	}

	// Check resource-specific limits
	switch resource {
	case "events":
		if qe.quotas.MaxEventsPerSecond > 0 {
			return limiter.Allow(qe.quotas.MaxEventsPerSecond)
		}
	case "data_push":
		if qe.quotas.MaxDataPushPerSecond > 0 {
			return limiter.Allow(qe.quotas.MaxDataPushPerSecond)
		}
	case "concurrent_requests":
		if qe.quotas.MaxConcurrentRequests > 0 {
			return limiter.AllowConcurrent(qe.quotas.MaxConcurrentRequests)
		}
	}

	return nil // No quota defined for this resource
}

// rateLimiter implements a simple token bucket rate limiter.
type rateLimiter struct {
	mu               sync.Mutex
	lastCheck        time.Time
	current          int
	concurrentActive int
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		lastCheck: time.Now(),
	}
}

func (rl *rateLimiter) Allow(maxPerSecond int) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastCheck)

	// Reset counter every second
	if elapsed >= time.Second {
		rl.current = 0
		rl.lastCheck = now
	}

	if rl.current >= maxPerSecond {
		return fmt.Errorf("rate limit exceeded: %d/%d per second", rl.current, maxPerSecond)
	}

	rl.current++
	return nil
}

func (rl *rateLimiter) AllowConcurrent(maxConcurrent int) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.concurrentActive >= maxConcurrent {
		return fmt.Errorf("concurrent limit exceeded: %d/%d", rl.concurrentActive, maxConcurrent)
	}

	rl.concurrentActive++
	return nil
}

func (rl *rateLimiter) Release() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.concurrentActive > 0 {
		rl.concurrentActive--
	}
}

// =============================================================================
// Quota-Enforced Bus Wrapper
// =============================================================================

// quotaEnforcedBus wraps the engine bus and enforces quotas on operations.
type quotaEnforcedBus struct {
	bus      *engine.Bus
	enforcer *quotaEnforcer
}

func newQuotaEnforcedBus(bus *engine.Bus, enforcer *quotaEnforcer) framework.BusClient {
	return &quotaEnforcedBus{
		bus:      bus,
		enforcer: enforcer,
	}
}

func (b *quotaEnforcedBus) PublishEvent(ctx context.Context, event string, payload any) error {
	if err := b.enforcer.Enforce("events", 1); err != nil {
		return fmt.Errorf("quota check failed: %w", err)
	}
	return b.bus.PublishEvent(ctx, event, payload)
}

func (b *quotaEnforcedBus) PushData(ctx context.Context, topic string, payload any) error {
	if err := b.enforcer.Enforce("data_push", 1); err != nil {
		return fmt.Errorf("quota check failed: %w", err)
	}
	return b.bus.PushData(ctx, topic, payload)
}

func (b *quotaEnforcedBus) InvokeCompute(ctx context.Context, payload any) ([]framework.ComputeResult, error) {
	if err := b.enforcer.Enforce("concurrent_requests", 1); err != nil {
		return nil, fmt.Errorf("quota check failed: %w", err)
	}
	defer b.enforcer.counters["concurrent_requests"].Release()

	// Convert engine.InvokeResult to framework.ComputeResult
	results, err := b.bus.InvokeComputeAll(ctx, payload)
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
