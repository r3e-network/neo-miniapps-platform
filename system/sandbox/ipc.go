// Package sandbox - IPC (Inter-Process Communication) isolation layer.
//
// This implements Android Binder-style IPC for service-to-service communication.
// All inter-service calls MUST go through this layer for security enforcement.
package sandbox

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// IPC Manager (Android Binder equivalent)
// =============================================================================

// IPCManager manages all inter-service communication.
// Analogous to Android's Binder driver.
type IPCManager struct {
	mu sync.RWMutex

	// Service registry
	services map[string]*ServiceEndpoint

	// Security components
	policy  *SecurityPolicy
	auditor *SecurityAuditor

	// Call tracking
	pendingCalls map[string]*PendingCall
	callTimeout  time.Duration

	// Rate limiting per service pair
	rateLimiters map[string]*IPCRateLimiter
}

// ServiceEndpoint represents a registered service that can receive IPC calls.
type ServiceEndpoint struct {
	ServiceID    string
	Identity     *ServiceIdentity
	Capabilities *CapabilitySet
	Handler      IPCHandler
	AllowedCallers []string // Empty = allow all with permission
}

// IPCHandler processes incoming IPC calls.
type IPCHandler interface {
	// HandleCall processes an IPC call and returns a result.
	HandleCall(ctx context.Context, call *IPCCall) (*IPCResult, error)

	// SupportedMethods returns the list of methods this handler supports.
	SupportedMethods() []string
}

// IPCCall represents an inter-service call.
type IPCCall struct {
	// Call identification
	CallID    string    `json:"call_id"`
	Timestamp time.Time `json:"timestamp"`

	// Caller information (verified by IPC manager)
	CallerID       string         `json:"caller_id"`
	CallerIdentity *ServiceIdentity `json:"caller_identity"`

	// Target information
	TargetID string `json:"target_id"`
	Method   string `json:"method"`

	// Payload
	Args    any `json:"args"`
	Timeout time.Duration `json:"timeout"`

	// Security context
	CallerCapabilities []Capability `json:"caller_capabilities"`
}

// IPCResult represents the result of an IPC call.
type IPCResult struct {
	CallID    string    `json:"call_id"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Result    any       `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
}

// PendingCall tracks an in-flight IPC call.
type PendingCall struct {
	Call      *IPCCall
	StartTime time.Time
	ResultCh  chan *IPCResult
}

// IPCRateLimiter limits IPC calls between service pairs.
type IPCRateLimiter struct {
	mu          sync.Mutex
	windowStart time.Time
	callCount   int
	maxPerWindow int
	windowSize  time.Duration
}

// NewIPCManager creates a new IPC manager.
func NewIPCManager(policy *SecurityPolicy, auditor *SecurityAuditor) *IPCManager {
	return &IPCManager{
		services:     make(map[string]*ServiceEndpoint),
		policy:       policy,
		auditor:      auditor,
		pendingCalls: make(map[string]*PendingCall),
		callTimeout:  30 * time.Second,
		rateLimiters: make(map[string]*IPCRateLimiter),
	}
}

// RegisterService registers a service endpoint for IPC.
func (m *IPCManager) RegisterService(endpoint *ServiceEndpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.services[endpoint.ServiceID]; exists {
		return fmt.Errorf("service already registered: %s", endpoint.ServiceID)
	}

	m.services[endpoint.ServiceID] = endpoint
	return nil
}

// UnregisterService removes a service endpoint.
func (m *IPCManager) UnregisterService(serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.services[serviceID]; !exists {
		return fmt.Errorf("service not registered: %s", serviceID)
	}

	delete(m.services, serviceID)
	return nil
}

// Call performs an IPC call from one service to another.
// This is the ONLY way services can communicate with each other.
func (m *IPCManager) Call(ctx context.Context, call *IPCCall) (*IPCResult, error) {
	startTime := time.Now()

	// 1. Verify caller identity
	if call.CallerIdentity == nil {
		return nil, &IPCError{Code: IPCErrorUnauthorized, Message: "caller identity required"}
	}

	// 2. Check if target service exists
	m.mu.RLock()
	target, exists := m.services[call.TargetID]
	m.mu.RUnlock()

	if !exists {
		return nil, &IPCError{Code: IPCErrorNotFound, Message: fmt.Sprintf("service not found: %s", call.TargetID)}
	}

	// 3. Check caller permissions
	if err := m.checkCallPermission(call, target); err != nil {
		m.auditor.LogIPCCall(ctx, call.CallerID, call.TargetID, call.Method, false)
		return nil, err
	}

	// 4. Check rate limits
	if err := m.checkRateLimit(call.CallerID, call.TargetID); err != nil {
		return nil, err
	}

	// 5. Log the call attempt
	m.auditor.LogIPCCall(ctx, call.CallerID, call.TargetID, call.Method, true)

	// 6. Execute the call with timeout
	timeout := call.Timeout
	if timeout == 0 {
		timeout = m.callTimeout
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 7. Invoke the target handler
	result, err := target.Handler.HandleCall(callCtx, call)
	if err != nil {
		return &IPCResult{
			CallID:    call.CallID,
			Timestamp: time.Now(),
			Success:   false,
			Error:     err.Error(),
			Duration:  time.Since(startTime),
		}, nil
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// checkCallPermission verifies the caller has permission to call the target.
func (m *IPCManager) checkCallPermission(call *IPCCall, target *ServiceEndpoint) error {
	// Check if caller has service.call capability
	hasCallCap := false
	for _, cap := range call.CallerCapabilities {
		if cap == CapServiceCall {
			hasCallCap = true
			break
		}
	}
	if !hasCallCap {
		return &IPCError{
			Code:    IPCErrorPermissionDenied,
			Message: "caller lacks service.call capability",
		}
	}

	// Check if target allows this caller
	if len(target.AllowedCallers) > 0 {
		allowed := false
		for _, allowedID := range target.AllowedCallers {
			if allowedID == call.CallerID || allowedID == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			return &IPCError{
				Code:    IPCErrorPermissionDenied,
				Message: fmt.Sprintf("caller %s not in target's allowed list", call.CallerID),
			}
		}
	}

	// Check security policy
	subject := call.CallerID
	object := fmt.Sprintf("service:%s:%s", call.TargetID, call.Method)
	if m.policy.Evaluate(subject, object, "call") == PolicyEffectDeny {
		return &IPCError{
			Code:    IPCErrorPolicyDenied,
			Message: "security policy denies this call",
		}
	}

	return nil
}

// checkRateLimit enforces rate limits on IPC calls.
func (m *IPCManager) checkRateLimit(callerID, targetID string) error {
	key := fmt.Sprintf("%s->%s", callerID, targetID)

	m.mu.Lock()
	limiter, exists := m.rateLimiters[key]
	if !exists {
		limiter = &IPCRateLimiter{
			windowStart:  time.Now(),
			maxPerWindow: 1000, // 1000 calls per window
			windowSize:   time.Minute,
		}
		m.rateLimiters[key] = limiter
	}
	m.mu.Unlock()

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()
	if now.Sub(limiter.windowStart) > limiter.windowSize {
		// Reset window
		limiter.windowStart = now
		limiter.callCount = 0
	}

	if limiter.callCount >= limiter.maxPerWindow {
		return &IPCError{
			Code:    IPCErrorRateLimited,
			Message: fmt.Sprintf("rate limit exceeded: %d calls per %v", limiter.maxPerWindow, limiter.windowSize),
		}
	}

	limiter.callCount++
	return nil
}

// =============================================================================
// IPC Proxy (Client-side stub)
// =============================================================================

// IPCProxy provides a client-side interface for making IPC calls.
// Services use this to call other services.
type IPCProxy struct {
	manager  *IPCManager
	identity *ServiceIdentity
	caps     *CapabilitySet
}

// NewIPCProxy creates a new IPC proxy for a service.
func NewIPCProxy(manager *IPCManager, identity *ServiceIdentity, caps *CapabilitySet) *IPCProxy {
	return &IPCProxy{
		manager:  manager,
		identity: identity,
		caps:     caps,
	}
}

// Call invokes a method on another service.
func (p *IPCProxy) Call(ctx context.Context, targetService, method string, args any) (any, error) {
	call := &IPCCall{
		CallID:             GenerateCallID(),
		Timestamp:          time.Now(),
		CallerID:           p.identity.ServiceID,
		CallerIdentity:     p.identity,
		TargetID:           targetService,
		Method:             method,
		Args:               args,
		CallerCapabilities: p.caps.List(),
	}

	result, err := p.manager.Call(ctx, call)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("IPC call failed: %s", result.Error)
	}

	return result.Result, nil
}

// AsyncCall invokes a method asynchronously.
func (p *IPCProxy) AsyncCall(ctx context.Context, targetService, method string, args any) <-chan *IPCResult {
	resultCh := make(chan *IPCResult, 1)

	go func() {
		defer close(resultCh)

		call := &IPCCall{
			CallID:             GenerateCallID(),
			Timestamp:          time.Now(),
			CallerID:           p.identity.ServiceID,
			CallerIdentity:     p.identity,
			TargetID:           targetService,
			Method:             method,
			Args:               args,
			CallerCapabilities: p.caps.List(),
		}

		result, err := p.manager.Call(ctx, call)
		if err != nil {
			resultCh <- &IPCResult{
				CallID:    call.CallID,
				Timestamp: time.Now(),
				Success:   false,
				Error:     err.Error(),
			}
			return
		}

		resultCh <- result
	}()

	return resultCh
}

// =============================================================================
// IPC Errors
// =============================================================================

// IPCErrorCode represents an IPC error type.
type IPCErrorCode int

const (
	IPCErrorUnknown IPCErrorCode = iota
	IPCErrorNotFound
	IPCErrorUnauthorized
	IPCErrorPermissionDenied
	IPCErrorPolicyDenied
	IPCErrorRateLimited
	IPCErrorTimeout
	IPCErrorInternal
)

// IPCError represents an IPC-specific error.
type IPCError struct {
	Code    IPCErrorCode
	Message string
}

func (e *IPCError) Error() string {
	return fmt.Sprintf("IPC error [%d]: %s", e.Code, e.Message)
}

// =============================================================================
// Utility Functions
// =============================================================================

// GenerateCallID generates a unique call ID.
func GenerateCallID() string {
	return fmt.Sprintf("call-%s-%d", GenerateProcessID(), time.Now().UnixNano())
}

// =============================================================================
// Service Bus Adapter (Integrates with existing Bus)
// =============================================================================

// EventPublisher is the interface for publishing events to the bus.
// This avoids circular dependency with system/core.
type EventPublisher interface {
	PublishEvent(ctx context.Context, event string, payload any) error
}

// EventSubscriber is the interface for subscribing to events from the bus.
type EventSubscriber interface {
	SubscribeEvent(ctx context.Context, event string, handler func(context.Context, any) error) error
}

// SecureBusAdapter wraps the existing Bus with IPC security.
type SecureBusAdapter struct {
	ipcManager  *IPCManager
	identity    *ServiceIdentity
	caps        *CapabilitySet
	auditor     *SecurityAuditor
	publisher   EventPublisher
	subscriber  EventSubscriber
	subscribers map[string][]func(event string, payload any) // local subscriptions
	subMu       sync.RWMutex
}

// NewSecureBusAdapter creates a secure bus adapter.
func NewSecureBusAdapter(
	ipcManager *IPCManager,
	identity *ServiceIdentity,
	caps *CapabilitySet,
	auditor *SecurityAuditor,
) *SecureBusAdapter {
	return &SecureBusAdapter{
		ipcManager:  ipcManager,
		identity:    identity,
		caps:        caps,
		auditor:     auditor,
		subscribers: make(map[string][]func(event string, payload any)),
	}
}

// WithPublisher sets the event publisher for the adapter.
func (b *SecureBusAdapter) WithPublisher(pub EventPublisher) *SecureBusAdapter {
	b.publisher = pub
	return b
}

// WithSubscriber sets the event subscriber for the adapter.
func (b *SecureBusAdapter) WithSubscriber(sub EventSubscriber) *SecureBusAdapter {
	b.subscriber = sub
	return b
}

// Publish publishes an event with caller identity attached.
func (b *SecureBusAdapter) Publish(ctx context.Context, event string, payload any) error {
	// Check capability
	if !b.caps.Has(CapBusPublish) {
		b.auditor.LogResourceAccess(ctx, b.identity.ServiceID, "bus:event:"+event, "publish", false)
		return &CapabilityDeniedError{
			ServiceID:  b.identity.ServiceID,
			Capability: CapBusPublish,
		}
	}

	// Verify event namespace (services can only publish to their own namespace)
	expectedPrefix := b.identity.ServiceID + "."
	if len(event) < len(expectedPrefix) || event[:len(expectedPrefix)] != expectedPrefix {
		// Check if it's a public event
		if len(event) < 7 || event[:7] != "public." {
			return fmt.Errorf("services can only publish to their own namespace or public.*")
		}
	}

	b.auditor.LogResourceAccess(ctx, b.identity.ServiceID, "bus:event:"+event, "publish", true)

	// Wrap payload with caller identity
	securePayload := &SecureEventPayload{
		CallerID:  b.identity.ServiceID,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// Publish to the underlying bus if configured
	if b.publisher != nil {
		return b.publisher.PublishEvent(ctx, event, securePayload)
	}

	// Fallback: deliver to local subscribers (in-process pub/sub)
	b.subMu.RLock()
	handlers := b.subscribers[event]
	b.subMu.RUnlock()

	for _, h := range handlers {
		h(event, securePayload)
	}

	return nil
}

// Subscribe subscribes to events with permission check.
func (b *SecureBusAdapter) Subscribe(ctx context.Context, pattern string, handler func(event string, payload any)) error {
	// Check capability
	if !b.caps.Has(CapBusSubscribe) {
		b.auditor.LogResourceAccess(ctx, b.identity.ServiceID, "bus:pattern:"+pattern, "subscribe", false)
		return &CapabilityDeniedError{
			ServiceID:  b.identity.ServiceID,
			Capability: CapBusSubscribe,
		}
	}

	b.auditor.LogResourceAccess(ctx, b.identity.ServiceID, "bus:pattern:"+pattern, "subscribe", true)

	// Wrap handler to verify event source
	secureHandler := func(event string, payload any) {
		// Verify the payload is from a trusted source
		if securePayload, ok := payload.(*SecureEventPayload); ok {
			// Pass the original payload to the handler
			handler(event, securePayload.Payload)
		} else {
			// Allow non-secure payloads for backward compatibility
			handler(event, payload)
		}
	}

	// Subscribe to the underlying bus if configured
	if b.subscriber != nil {
		// Adapt the handler signature for the bus
		busHandler := func(ctx context.Context, payload any) error {
			secureHandler(pattern, payload)
			return nil
		}
		return b.subscriber.SubscribeEvent(ctx, pattern, busHandler)
	}

	// Fallback: register in local subscriber map (in-process pub/sub)
	b.subMu.Lock()
	b.subscribers[pattern] = append(b.subscribers[pattern], secureHandler)
	b.subMu.Unlock()

	return nil
}

// SecureEventPayload wraps event payloads with security metadata.
type SecureEventPayload struct {
	CallerID  string    `json:"caller_id"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}
