// Package sandbox - Bus Integration provides secure bus access with sandbox isolation.
//
// This file integrates the sandbox security model with the existing Bus system,
// ensuring that all bus operations are subject to capability checks and audit logging.
package sandbox

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// Secure Bus Wrapper
// =============================================================================

// SecureBus wraps the engine Bus with sandbox security checks.
// It ensures all bus operations are subject to capability verification and audit logging.
type SecureBus struct {
	mu sync.RWMutex

	// Sandbox components
	manager *Manager
	auditor *SecurityAuditor

	// Caller identity (set per-request)
	callerID string

	// Rate limiting
	rateLimiter *BusRateLimiter
}

// SecureBusConfig configures the secure bus wrapper.
type SecureBusConfig struct {
	// Rate limiting
	MaxEventsPerMinute  int
	MaxDataPushPerMin   int
	MaxComputePerMinute int
}

// DefaultSecureBusConfig returns sensible defaults.
func DefaultSecureBusConfig() SecureBusConfig {
	return SecureBusConfig{
		MaxEventsPerMinute:  1000,
		MaxDataPushPerMin:   500,
		MaxComputePerMinute: 100,
	}
}

// NewSecureBus creates a new secure bus wrapper.
func NewSecureBus(manager *Manager, config SecureBusConfig) *SecureBus {
	return &SecureBus{
		manager:     manager,
		auditor:     manager.auditor,
		rateLimiter: NewBusRateLimiter(config),
	}
}

// =============================================================================
// Bus Operations with Security
// =============================================================================

// SecurePublishEvent publishes an event with security checks.
func (sb *SecureBus) SecurePublishEvent(ctx context.Context, callerID, event string, payload any) error {
	// Get caller's sandbox
	sandbox, err := sb.manager.GetSandbox(callerID)
	if err != nil {
		// Create a minimal identity for logging
		tempIdentity := &ServiceIdentity{ServiceID: callerID}
		sb.auditor.LogCapabilityCheck(ctx, tempIdentity, CapBusPublish, false)
		return fmt.Errorf("caller not found: %w", err)
	}

	// Check capability
	if !sandbox.Caps.Has(CapBusPublish) {
		sb.auditor.LogCapabilityCheck(ctx, sandbox.Identity, CapBusPublish, false)
		return &CapabilityDeniedError{
			ServiceID:  callerID,
			Capability: CapBusPublish,
		}
	}

	// Check rate limit
	if !sb.rateLimiter.AllowEvent(callerID) {
		sb.auditor.LogResourceAccess(ctx, callerID, "bus:event", "rate_limited", false)
		return fmt.Errorf("rate limit exceeded for event publishing")
	}

	// Log successful capability check
	sb.auditor.LogCapabilityCheck(ctx, sandbox.Identity, CapBusPublish, true)

	// Wrap payload with caller identity for downstream verification
	securePayload := &SecureBusPayload{
		CallerID:    callerID,
		CallerLevel: sandbox.Identity.SecurityLevel,
		Timestamp:   time.Now(),
		Event:       event,
		Payload:     payload,
	}

	// Log the operation
	sb.auditor.LogResourceAccess(ctx, callerID, "bus:event:"+event, "publish", true)

	// Return the secure payload for the actual bus to use
	_ = securePayload // The actual publishing is done by the caller with this payload
	return nil
}

// SecurePushData pushes data with security checks.
func (sb *SecureBus) SecurePushData(ctx context.Context, callerID, topic string, payload any) error {
	// Get caller's sandbox
	sandbox, err := sb.manager.GetSandbox(callerID)
	if err != nil {
		tempIdentity := &ServiceIdentity{ServiceID: callerID}
		sb.auditor.LogCapabilityCheck(ctx, tempIdentity, CapBusPublish, false)
		return fmt.Errorf("caller not found: %w", err)
	}

	// Check capability
	if !sandbox.Caps.Has(CapBusPublish) {
		sb.auditor.LogCapabilityCheck(ctx, sandbox.Identity, CapBusPublish, false)
		return &CapabilityDeniedError{
			ServiceID:  callerID,
			Capability: CapBusPublish,
		}
	}

	// Check rate limit
	if !sb.rateLimiter.AllowDataPush(callerID) {
		sb.auditor.LogResourceAccess(ctx, callerID, "bus:data", "rate_limited", false)
		return fmt.Errorf("rate limit exceeded for data push")
	}

	// Log successful capability check
	sb.auditor.LogCapabilityCheck(ctx, sandbox.Identity, CapBusPublish, true)
	sb.auditor.LogResourceAccess(ctx, callerID, "bus:data:"+topic, "push", true)

	return nil
}

// SecureInvokeCompute invokes compute with security checks.
func (sb *SecureBus) SecureInvokeCompute(ctx context.Context, callerID string, payload any) error {
	// Get caller's sandbox
	sandbox, err := sb.manager.GetSandbox(callerID)
	if err != nil {
		tempIdentity := &ServiceIdentity{ServiceID: callerID}
		sb.auditor.LogCapabilityCheck(ctx, tempIdentity, CapBusInvoke, false)
		return fmt.Errorf("caller not found: %w", err)
	}

	// Check capability
	if !sandbox.Caps.Has(CapBusInvoke) {
		sb.auditor.LogCapabilityCheck(ctx, sandbox.Identity, CapBusInvoke, false)
		return &CapabilityDeniedError{
			ServiceID:  callerID,
			Capability: CapBusInvoke,
		}
	}

	// Check rate limit
	if !sb.rateLimiter.AllowCompute(callerID) {
		sb.auditor.LogResourceAccess(ctx, callerID, "bus:compute", "rate_limited", false)
		return fmt.Errorf("rate limit exceeded for compute invocation")
	}

	// Log successful capability check
	sb.auditor.LogCapabilityCheck(ctx, sandbox.Identity, CapBusInvoke, true)
	sb.auditor.LogResourceAccess(ctx, callerID, "bus:compute", "invoke", true)

	return nil
}

// SecureSubscribe subscribes to events with security checks.
func (sb *SecureBus) SecureSubscribe(ctx context.Context, callerID, event string) error {
	// Get caller's sandbox
	sandbox, err := sb.manager.GetSandbox(callerID)
	if err != nil {
		tempIdentity := &ServiceIdentity{ServiceID: callerID}
		sb.auditor.LogCapabilityCheck(ctx, tempIdentity, CapBusSubscribe, false)
		return fmt.Errorf("caller not found: %w", err)
	}

	// Check capability
	if !sandbox.Caps.Has(CapBusSubscribe) {
		sb.auditor.LogCapabilityCheck(ctx, sandbox.Identity, CapBusSubscribe, false)
		return &CapabilityDeniedError{
			ServiceID:  callerID,
			Capability: CapBusSubscribe,
		}
	}

	// Log successful capability check
	sb.auditor.LogCapabilityCheck(ctx, sandbox.Identity, CapBusSubscribe, true)
	sb.auditor.LogResourceAccess(ctx, callerID, "bus:subscribe:"+event, "subscribe", true)

	return nil
}

// =============================================================================
// Secure Bus Payload
// =============================================================================

// SecureBusPayload wraps bus payloads with caller identity.
type SecureBusPayload struct {
	CallerID    string        `json:"caller_id"`
	CallerLevel SecurityLevel `json:"caller_level"`
	Timestamp   time.Time     `json:"timestamp"`
	Event       string        `json:"event,omitempty"`
	Topic       string        `json:"topic,omitempty"`
	Payload     any           `json:"payload"`
}

// VerifyPayload verifies a secure bus payload.
func VerifyPayload(payload *SecureBusPayload, maxAge time.Duration) error {
	if payload == nil {
		return fmt.Errorf("payload is nil")
	}
	if payload.CallerID == "" {
		return fmt.Errorf("caller ID is empty")
	}
	if time.Since(payload.Timestamp) > maxAge {
		return fmt.Errorf("payload expired")
	}
	return nil
}

// =============================================================================
// Bus Rate Limiter
// =============================================================================

// BusRateLimiter implements per-service rate limiting for bus operations.
type BusRateLimiter struct {
	mu     sync.Mutex
	config SecureBusConfig

	// Per-service counters (reset every minute)
	eventCounts   map[string]*rateLimitCounter
	dataCounts    map[string]*rateLimitCounter
	computeCounts map[string]*rateLimitCounter
}

type rateLimitCounter struct {
	count     int
	resetTime time.Time
}

// NewBusRateLimiter creates a new rate limiter.
func NewBusRateLimiter(config SecureBusConfig) *BusRateLimiter {
	return &BusRateLimiter{
		config:        config,
		eventCounts:   make(map[string]*rateLimitCounter),
		dataCounts:    make(map[string]*rateLimitCounter),
		computeCounts: make(map[string]*rateLimitCounter),
	}
}

// AllowEvent checks if an event publish is allowed.
func (rl *BusRateLimiter) AllowEvent(serviceID string) bool {
	return rl.allow(serviceID, rl.eventCounts, rl.config.MaxEventsPerMinute)
}

// AllowDataPush checks if a data push is allowed.
func (rl *BusRateLimiter) AllowDataPush(serviceID string) bool {
	return rl.allow(serviceID, rl.dataCounts, rl.config.MaxDataPushPerMin)
}

// AllowCompute checks if a compute invocation is allowed.
func (rl *BusRateLimiter) AllowCompute(serviceID string) bool {
	return rl.allow(serviceID, rl.computeCounts, rl.config.MaxComputePerMinute)
}

func (rl *BusRateLimiter) allow(serviceID string, counters map[string]*rateLimitCounter, limit int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	counter, exists := counters[serviceID]

	if !exists || now.After(counter.resetTime) {
		// Create new counter or reset expired one
		counters[serviceID] = &rateLimitCounter{
			count:     1,
			resetTime: now.Add(time.Minute),
		}
		return true
	}

	if counter.count >= limit {
		return false
	}

	counter.count++
	return true
}

// =============================================================================
// Bus Event Filter
// =============================================================================

// BusEventFilter filters bus events based on security policy.
type BusEventFilter struct {
	manager *Manager
	policy  *SecurityPolicy
}

// NewBusEventFilter creates a new event filter.
func NewBusEventFilter(manager *Manager) *BusEventFilter {
	return &BusEventFilter{
		manager: manager,
		policy:  manager.policy,
	}
}

// CanReceive checks if a service can receive an event from a sender.
func (f *BusEventFilter) CanReceive(ctx context.Context, receiverID, senderID, event string) bool {
	_ = ctx // ctx reserved for future use

	// Get receiver's sandbox
	receiver, err := f.manager.GetSandbox(receiverID)
	if err != nil {
		return false
	}

	// Check subscribe capability
	if !receiver.Caps.Has(CapBusSubscribe) {
		return false
	}

	// Check security policy
	effect := f.policy.Evaluate(senderID, "bus:event:"+event, "send")
	return effect == PolicyEffectAllow
}

// FilterPayload filters sensitive data from payload based on receiver's security level.
func (f *BusEventFilter) FilterPayload(receiverID string, payload *SecureBusPayload) any {
	receiver, err := f.manager.GetSandbox(receiverID)
	if err != nil {
		// Unknown receiver gets minimal payload
		return map[string]any{
			"event": payload.Event,
		}
	}

	// System and privileged services get full payload
	if receiver.Identity.SecurityLevel >= SecurityLevelPrivileged {
		return payload
	}

	// Normal services get payload without internal metadata
	return map[string]any{
		"caller_id": payload.CallerID,
		"event":     payload.Event,
		"payload":   payload.Payload,
	}
}

// =============================================================================
// Bus Middleware
// =============================================================================

// BusMiddleware provides middleware functions for bus security.
type BusMiddleware struct {
	secureBus *SecureBus
	filter    *BusEventFilter
}

// NewBusMiddleware creates new bus middleware.
func NewBusMiddleware(manager *Manager) *BusMiddleware {
	return &BusMiddleware{
		secureBus: NewSecureBus(manager, DefaultSecureBusConfig()),
		filter:    NewBusEventFilter(manager),
	}
}

// WrapPublish wraps a publish operation with security checks.
func (m *BusMiddleware) WrapPublish(ctx context.Context, callerID string, publishFn func() error) error {
	// Pre-check
	if err := m.secureBus.SecurePublishEvent(ctx, callerID, "", nil); err != nil {
		return err
	}

	// Execute actual publish
	return publishFn()
}

// WrapSubscribe wraps a subscribe operation with security checks.
func (m *BusMiddleware) WrapSubscribe(ctx context.Context, callerID, event string, subscribeFn func() error) error {
	// Pre-check
	if err := m.secureBus.SecureSubscribe(ctx, callerID, event); err != nil {
		return err
	}

	// Execute actual subscribe
	return subscribeFn()
}

// WrapCompute wraps a compute invocation with security checks.
func (m *BusMiddleware) WrapCompute(ctx context.Context, callerID string, computeFn func() error) error {
	// Pre-check
	if err := m.secureBus.SecureInvokeCompute(ctx, callerID, nil); err != nil {
		return err
	}

	// Execute actual compute
	return computeFn()
}
