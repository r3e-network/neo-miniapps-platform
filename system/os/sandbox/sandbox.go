// Package sandbox implements Android-style application sandboxing for Service Layer.
//
// # Android Security Model Adaptation
//
// This package adapts Android's multi-layered security model to protect services:
//
// 1. **Process Isolation (UID-based)** → **Service Identity Isolation**
//    - Each service gets a unique ServiceID (analogous to Android UID)
//    - Services cannot directly access other services' resources
//    - All inter-service communication goes through controlled channels
//
// 2. **SELinux (MAC)** → **Capability-Based Access Control**
//    - Services declare required capabilities in manifest
//    - Engine enforces capability checks at runtime
//    - Deny-by-default policy
//
// 3. **Seccomp-BPF** → **API Surface Restriction**
//    - Services can only call whitelisted Engine APIs
//    - Syscall-like filtering for service operations
//
// 4. **App Sandbox** → **Service Sandbox**
//    - Isolated storage per service
//    - Isolated database schemas
//    - Isolated configuration
//
// 5. **Binder IPC** → **Service Bus IPC**
//    - All inter-service calls go through the Bus
//    - Permission checks on every IPC call
//    - Caller identity verification
package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// Service Identity (Android UID equivalent)
// =============================================================================

// ServiceIdentity uniquely identifies a service within the sandbox.
// Analogous to Android's UID/GID system.
type ServiceIdentity struct {
	// ServiceID is the unique identifier (like Android UID)
	ServiceID string `json:"service_id"`

	// PackageID is the parent package (like Android package name)
	PackageID string `json:"package_id"`

	// ProcessID is the runtime instance ID (for multi-instance services)
	ProcessID string `json:"process_id"`

	// SigningKey hash - verifies the service hasn't been tampered with
	SigningKeyHash string `json:"signing_key_hash"`

	// SecurityLevel indicates the trust level
	SecurityLevel SecurityLevel `json:"security_level"`

	// CreatedAt is when this identity was assigned
	CreatedAt time.Time `json:"created_at"`
}

// SecurityLevel defines the trust level of a service.
type SecurityLevel int

const (
	// SecurityLevelUntrusted - third-party services with minimal trust
	SecurityLevelUntrusted SecurityLevel = iota

	// SecurityLevelNormal - standard services with normal permissions
	SecurityLevelNormal

	// SecurityLevelPrivileged - system services with elevated permissions
	SecurityLevelPrivileged

	// SecurityLevelSystem - core engine services with full access
	SecurityLevelSystem
)

func (sl SecurityLevel) String() string {
	switch sl {
	case SecurityLevelUntrusted:
		return "untrusted"
	case SecurityLevelNormal:
		return "normal"
	case SecurityLevelPrivileged:
		return "privileged"
	case SecurityLevelSystem:
		return "system"
	default:
		return "unknown"
	}
}

// =============================================================================
// Capability System (SELinux-style MAC)
// =============================================================================

// Capability represents a permission that can be granted to a service.
// Analogous to Android permissions + SELinux contexts.
type Capability string

const (
	// Storage capabilities
	CapStorageRead      Capability = "storage.read"
	CapStorageWrite     Capability = "storage.write"
	CapStorageDelete    Capability = "storage.delete"
	CapStorageOther     Capability = "storage.other" // Access other services' storage (dangerous)

	// Database capabilities
	CapDatabaseRead     Capability = "database.read"
	CapDatabaseWrite    Capability = "database.write"
	CapDatabaseSchema   Capability = "database.schema" // Modify schema
	CapDatabaseOther    Capability = "database.other"  // Access other services' tables

	// Bus capabilities
	CapBusPublish       Capability = "bus.publish"
	CapBusSubscribe     Capability = "bus.subscribe"
	CapBusInvoke        Capability = "bus.invoke"
	CapBusBroadcast     Capability = "bus.broadcast" // Send to all services

	// Network capabilities
	CapNetworkOutbound  Capability = "network.outbound"
	CapNetworkInbound   Capability = "network.inbound"
	CapNetworkInternal  Capability = "network.internal" // Internal service mesh only

	// Crypto capabilities
	CapCryptoSign       Capability = "crypto.sign"
	CapCryptoEncrypt    Capability = "crypto.encrypt"
	CapCryptoKeyGen     Capability = "crypto.keygen"
	CapCryptoMasterKey  Capability = "crypto.masterkey" // Access master signing key

	// Service capabilities
	CapServiceCall      Capability = "service.call"      // Call other services
	CapServiceInspect   Capability = "service.inspect"   // Inspect other services
	CapServiceManage    Capability = "service.manage"    // Start/stop other services

	// System capabilities (privileged)
	CapSystemConfig     Capability = "system.config"     // Modify system config
	CapSystemAudit      Capability = "system.audit"      // Access audit logs
	CapSystemAdmin      Capability = "system.admin"      // Full admin access
)

// CapabilitySet is a set of capabilities granted to a service.
type CapabilitySet struct {
	mu           sync.RWMutex
	capabilities map[Capability]bool
	grantedAt    map[Capability]time.Time
	grantedBy    map[Capability]string // Who granted this capability
}

// NewCapabilitySet creates an empty capability set.
func NewCapabilitySet() *CapabilitySet {
	return &CapabilitySet{
		capabilities: make(map[Capability]bool),
		grantedAt:    make(map[Capability]time.Time),
		grantedBy:    make(map[Capability]string),
	}
}

// Grant adds a capability to the set.
func (cs *CapabilitySet) Grant(cap Capability, grantedBy string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.capabilities[cap] = true
	cs.grantedAt[cap] = time.Now()
	cs.grantedBy[cap] = grantedBy
}

// Revoke removes a capability from the set.
func (cs *CapabilitySet) Revoke(cap Capability) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.capabilities, cap)
	delete(cs.grantedAt, cap)
	delete(cs.grantedBy, cap)
}

// Has checks if a capability is granted.
func (cs *CapabilitySet) Has(cap Capability) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.capabilities[cap]
}

// HasAll checks if all specified capabilities are granted.
func (cs *CapabilitySet) HasAll(caps ...Capability) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	for _, cap := range caps {
		if !cs.capabilities[cap] {
			return false
		}
	}
	return true
}

// HasAny checks if any of the specified capabilities are granted.
func (cs *CapabilitySet) HasAny(caps ...Capability) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	for _, cap := range caps {
		if cs.capabilities[cap] {
			return true
		}
	}
	return false
}

// List returns all granted capabilities.
func (cs *CapabilitySet) List() []Capability {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	result := make([]Capability, 0, len(cs.capabilities))
	for cap := range cs.capabilities {
		result = append(result, cap)
	}
	return result
}

// =============================================================================
// Sandbox Context (Android Context equivalent)
// =============================================================================

// SandboxContext provides sandboxed access to system resources.
// This is the ONLY way services can interact with the system.
// Analogous to Android's Context class.
type SandboxContext struct {
	identity     *ServiceIdentity
	capabilities *CapabilitySet
	policy       *SecurityPolicy
	auditor      *SecurityAuditor

	// Resource handles (lazily initialized)
	storage   SandboxedStorage
	database  SandboxedDatabase
	bus       SandboxedBus
	crypto    SandboxedCrypto
	network   SandboxedNetwork
}

// NewSandboxContext creates a new sandbox context for a service.
func NewSandboxContext(
	identity *ServiceIdentity,
	capabilities *CapabilitySet,
	policy *SecurityPolicy,
	auditor *SecurityAuditor,
) *SandboxContext {
	return &SandboxContext{
		identity:     identity,
		capabilities: capabilities,
		policy:       policy,
		auditor:      auditor,
	}
}

// Identity returns the service's identity.
func (sc *SandboxContext) Identity() *ServiceIdentity {
	return sc.identity
}

// Capabilities returns the service's capability set.
func (sc *SandboxContext) Capabilities() *CapabilitySet {
	return sc.capabilities
}

// CheckCapability verifies a capability and logs the check.
func (sc *SandboxContext) CheckCapability(ctx context.Context, cap Capability) error {
	allowed := sc.capabilities.Has(cap)

	// Audit the check
	if sc.auditor != nil {
		sc.auditor.LogCapabilityCheck(ctx, sc.identity, cap, allowed)
	}

	if !allowed {
		return &CapabilityDeniedError{
			ServiceID:  sc.identity.ServiceID,
			Capability: cap,
		}
	}

	return nil
}

// Storage returns sandboxed storage access.
func (sc *SandboxContext) Storage(ctx context.Context) (SandboxedStorage, error) {
	if err := sc.CheckCapability(ctx, CapStorageRead); err != nil {
		return nil, err
	}
	return sc.storage, nil
}

// Database returns sandboxed database access.
func (sc *SandboxContext) Database(ctx context.Context) (SandboxedDatabase, error) {
	if err := sc.CheckCapability(ctx, CapDatabaseRead); err != nil {
		return nil, err
	}
	return sc.database, nil
}

// Bus returns sandboxed bus access.
func (sc *SandboxContext) Bus(ctx context.Context) (SandboxedBus, error) {
	if err := sc.CheckCapability(ctx, CapBusPublish); err != nil {
		if err := sc.CheckCapability(ctx, CapBusSubscribe); err != nil {
			return nil, err
		}
	}
	return sc.bus, nil
}

// =============================================================================
// Sandboxed Resource Interfaces
// =============================================================================

// SandboxedStorage provides isolated storage access.
type SandboxedStorage interface {
	// Get retrieves a value (only from this service's namespace)
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value (only in this service's namespace)
	Set(ctx context.Context, key string, value []byte) error

	// Delete removes a value (only from this service's namespace)
	Delete(ctx context.Context, key string) error

	// List lists keys with prefix (only in this service's namespace)
	List(ctx context.Context, prefix string) ([]string, error)

	// Quota returns storage quota information
	Quota() StorageQuota
}

// StorageQuota represents storage limits.
type StorageQuota struct {
	MaxBytes  int64
	UsedBytes int64
}

// SandboxedDatabase provides isolated database access.
type SandboxedDatabase interface {
	// Query executes a read query (only on allowed tables)
	Query(ctx context.Context, query string, args ...any) ([]map[string]any, error)

	// Exec executes a write query (only on allowed tables)
	Exec(ctx context.Context, query string, args ...any) (int64, error)

	// AllowedTables returns the list of tables this service can access
	AllowedTables() []string
}

// SandboxedBus provides isolated bus access.
type SandboxedBus interface {
	// Publish publishes an event (with caller identity attached)
	Publish(ctx context.Context, event string, payload any) error

	// Subscribe subscribes to events (filtered by allowed patterns)
	Subscribe(ctx context.Context, pattern string, handler func(event string, payload any)) error

	// Call invokes another service (with permission check)
	Call(ctx context.Context, targetService string, method string, args any) (any, error)
}

// SandboxedCrypto provides isolated crypto access.
type SandboxedCrypto interface {
	// Sign signs data with the service's key
	Sign(ctx context.Context, data []byte) ([]byte, error)

	// Verify verifies a signature
	Verify(ctx context.Context, data, signature []byte) (bool, error)

	// Encrypt encrypts data
	Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)

	// Decrypt decrypts data
	Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
}

// SandboxedNetwork provides isolated network access.
type SandboxedNetwork interface {
	// HTTPGet performs an HTTP GET (to allowed hosts only)
	HTTPGet(ctx context.Context, url string) ([]byte, error)

	// HTTPPost performs an HTTP POST (to allowed hosts only)
	HTTPPost(ctx context.Context, url string, body []byte) ([]byte, error)

	// AllowedHosts returns the list of allowed external hosts
	AllowedHosts() []string
}

// =============================================================================
// Security Policy (SELinux Policy equivalent)
// =============================================================================

// SecurityPolicy defines the security rules for the sandbox.
type SecurityPolicy struct {
	mu    sync.RWMutex
	rules []PolicyRule
}

// PolicyRule defines a single security rule.
type PolicyRule struct {
	// Subject is the service or service pattern this rule applies to
	Subject string `json:"subject"`

	// Object is the resource or resource pattern
	Object string `json:"object"`

	// Action is the operation (read, write, call, etc.)
	Action string `json:"action"`

	// Effect is allow or deny
	Effect PolicyEffect `json:"effect"`

	// Priority determines rule precedence (higher = more priority)
	Priority int `json:"priority"`

	// Conditions are additional constraints
	Conditions []PolicyCondition `json:"conditions,omitempty"`
}

// PolicyEffect is the result of a policy evaluation.
type PolicyEffect string

const (
	PolicyEffectAllow PolicyEffect = "allow"
	PolicyEffectDeny  PolicyEffect = "deny"
)

// PolicyCondition is an additional constraint on a rule.
type PolicyCondition struct {
	Type  string `json:"type"`  // time, rate, count, etc.
	Value string `json:"value"` // condition value
}

// NewSecurityPolicy creates a new security policy with default rules.
func NewSecurityPolicy() *SecurityPolicy {
	return &SecurityPolicy{
		rules: defaultSecurityRules(),
	}
}

// defaultSecurityRules returns the default deny-by-default rules.
func defaultSecurityRules() []PolicyRule {
	return []PolicyRule{
		// Default deny all
		{Subject: "*", Object: "*", Action: "*", Effect: PolicyEffectDeny, Priority: 0},

		// Allow services to access their own storage
		{Subject: "${service}", Object: "storage:${service}/*", Action: "read", Effect: PolicyEffectAllow, Priority: 100},
		{Subject: "${service}", Object: "storage:${service}/*", Action: "write", Effect: PolicyEffectAllow, Priority: 100},

		// Allow services to access their own database tables
		{Subject: "${service}", Object: "database:${service}_*", Action: "read", Effect: PolicyEffectAllow, Priority: 100},
		{Subject: "${service}", Object: "database:${service}_*", Action: "write", Effect: PolicyEffectAllow, Priority: 100},

		// Allow services to publish events with their own prefix
		{Subject: "${service}", Object: "bus:event:${service}.*", Action: "publish", Effect: PolicyEffectAllow, Priority: 100},

		// Allow services to subscribe to public events
		{Subject: "*", Object: "bus:event:public.*", Action: "subscribe", Effect: PolicyEffectAllow, Priority: 50},

		// System services have elevated access
		{Subject: "system.*", Object: "*", Action: "*", Effect: PolicyEffectAllow, Priority: 1000},
	}
}

// Evaluate checks if an action is allowed.
func (sp *SecurityPolicy) Evaluate(subject, object, action string) PolicyEffect {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	// Find the highest priority matching rule
	var matchedRule *PolicyRule
	for i := range sp.rules {
		rule := &sp.rules[i]
		if sp.matchRule(rule, subject, object, action) {
			if matchedRule == nil || rule.Priority > matchedRule.Priority {
				matchedRule = rule
			}
		}
	}

	if matchedRule != nil {
		return matchedRule.Effect
	}

	// Default deny
	return PolicyEffectDeny
}

// matchRule checks if a rule matches the given subject, object, and action.
func (sp *SecurityPolicy) matchRule(rule *PolicyRule, subject, object, action string) bool {
	// Simple wildcard matching (production would use proper glob/regex)
	return matchPattern(rule.Subject, subject) &&
		matchPattern(rule.Object, object) &&
		matchPattern(rule.Action, action)
}

// matchPattern performs glob-style pattern matching with variable substitution.
// Supports:
//   - "*" matches any sequence of characters
//   - "?" matches any single character
//   - Exact string matching
//
// Variable substitution (e.g., ${service}) should be performed before calling this function.
func matchPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == "" {
		return value == ""
	}

	// Use simple glob matching algorithm
	return globMatch(pattern, value)
}

// globMatch implements glob-style pattern matching.
// Supports * (match any sequence) and ? (match single character).
func globMatch(pattern, value string) bool {
	px, vx := 0, 0
	nextPx, nextVx := 0, -1

	for vx < len(value) || px < len(pattern) {
		if px < len(pattern) {
			switch pattern[px] {
			case '*':
				// Try to match at current position, remember fallback
				nextPx = px
				nextVx = vx + 1
				px++
				continue
			case '?':
				// Match any single character
				if vx < len(value) {
					px++
					vx++
					continue
				}
			default:
				// Match exact character
				if vx < len(value) && pattern[px] == value[vx] {
					px++
					vx++
					continue
				}
			}
		}
		// Mismatch - try fallback if we have a * to backtrack to
		if nextVx > 0 && nextVx <= len(value) {
			px = nextPx + 1
			vx = nextVx
			nextVx++
			continue
		}
		return false
	}
	return true
}

// substituteServiceVar replaces ${service} placeholders in a pattern.
func substituteServiceVar(pattern, serviceID string) string {
	return strings.ReplaceAll(pattern, "${service}", serviceID)
}

// =============================================================================
// Security Auditor
// =============================================================================

// SecurityAuditor logs all security-relevant events.
type SecurityAuditor struct {
	mu     sync.Mutex
	events []AuditEvent
	maxLen int
}

// AuditEvent represents a security audit event.
type AuditEvent struct {
	Timestamp   time.Time         `json:"timestamp"`
	EventType   string            `json:"event_type"`
	ServiceID   string            `json:"service_id"`
	Action      string            `json:"action"`
	Resource    string            `json:"resource"`
	Allowed     bool              `json:"allowed"`
	Details     map[string]string `json:"details,omitempty"`
}

// NewSecurityAuditor creates a new auditor.
func NewSecurityAuditor(maxEvents int) *SecurityAuditor {
	return &SecurityAuditor{
		events: make([]AuditEvent, 0, maxEvents),
		maxLen: maxEvents,
	}
}

// LogCapabilityCheck logs a capability check.
func (sa *SecurityAuditor) LogCapabilityCheck(ctx context.Context, identity *ServiceIdentity, cap Capability, allowed bool) {
	_ = ctx
	sa.log(AuditEvent{
		Timestamp: time.Now(),
		EventType: "capability_check",
		ServiceID: identity.ServiceID,
		Action:    string(cap),
		Allowed:   allowed,
	})
}

// LogResourceAccess logs a resource access attempt.
func (sa *SecurityAuditor) LogResourceAccess(ctx context.Context, serviceID, resource, action string, allowed bool) {
	_ = ctx
	sa.log(AuditEvent{
		Timestamp: time.Now(),
		EventType: "resource_access",
		ServiceID: serviceID,
		Action:    action,
		Resource:  resource,
		Allowed:   allowed,
	})
}

// LogIPCCall logs an inter-service call.
func (sa *SecurityAuditor) LogIPCCall(ctx context.Context, callerID, targetID, method string, allowed bool) {
	_ = ctx
	sa.log(AuditEvent{
		Timestamp: time.Now(),
		EventType: "ipc_call",
		ServiceID: callerID,
		Action:    method,
		Resource:  targetID,
		Allowed:   allowed,
	})
}

func (sa *SecurityAuditor) log(event AuditEvent) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	if len(sa.events) >= sa.maxLen {
		// Remove oldest event
		sa.events = sa.events[1:]
	}
	sa.events = append(sa.events, event)
}

// GetEvents returns recent audit events.
func (sa *SecurityAuditor) GetEvents(limit int) []AuditEvent {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	if limit <= 0 || limit > len(sa.events) {
		limit = len(sa.events)
	}

	// Return most recent events
	start := len(sa.events) - limit
	result := make([]AuditEvent, limit)
	copy(result, sa.events[start:])
	return result
}

// =============================================================================
// Errors
// =============================================================================

// CapabilityDeniedError is returned when a capability check fails.
type CapabilityDeniedError struct {
	ServiceID  string
	Capability Capability
}

func (e *CapabilityDeniedError) Error() string {
	return fmt.Sprintf("capability denied: service %s does not have %s", e.ServiceID, e.Capability)
}

// PolicyDeniedError is returned when a policy check fails.
type PolicyDeniedError struct {
	Subject string
	Object  string
	Action  string
}

func (e *PolicyDeniedError) Error() string {
	return fmt.Sprintf("policy denied: %s cannot %s on %s", e.Subject, e.Action, e.Object)
}

// =============================================================================
// Utility Functions
// =============================================================================

// GenerateServiceID generates a unique service ID from package and service name.
func GenerateServiceID(packageID, serviceName string) string {
	data := fmt.Sprintf("%s:%s", packageID, serviceName)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // First 8 bytes = 16 hex chars
}

// GenerateProcessID generates a unique process ID for a service instance.
func GenerateProcessID() string {
	data := fmt.Sprintf("%d:%d", time.Now().UnixNano(), time.Now().Nanosecond())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:4]) // First 4 bytes = 8 hex chars
}
