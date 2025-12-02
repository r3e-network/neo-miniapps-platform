// Package sandbox - Sandbox Manager orchestrates all isolation components.
//
// This is the main entry point for the sandbox system, analogous to
// Android's ActivityManagerService + PackageManagerService.
package sandbox

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// Sandbox Manager
// =============================================================================

// Manager orchestrates all sandbox components.
// This is the central authority for service isolation.
type Manager struct {
	mu sync.RWMutex

	// Core components
	policy  *SecurityPolicy
	auditor *SecurityAuditor
	ipc     *IPCManager

	// Storage backend
	storageBackend StorageBackend

	// Database connection (for isolated database access)
	db *sql.DB

	// Registered sandboxes
	sandboxes map[string]*ServiceSandbox

	// Configuration
	config ManagerConfig
}

// ManagerConfig configures the sandbox manager.
type ManagerConfig struct {
	// Default storage quota per service (bytes)
	DefaultStorageQuota int64

	// Default rate limits
	DefaultIPCRateLimit int // calls per minute

	// Audit settings
	MaxAuditEvents int

	// Security level for new services
	DefaultSecurityLevel SecurityLevel
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		DefaultStorageQuota:  100 * 1024 * 1024, // 100MB
		DefaultIPCRateLimit:  1000,               // 1000 calls/min
		MaxAuditEvents:       10000,
		DefaultSecurityLevel: SecurityLevelNormal,
	}
}

// NewManager creates a new sandbox manager.
func NewManager(db *sql.DB, config ManagerConfig) *Manager {
	policy := NewSecurityPolicy()
	auditor := NewSecurityAuditor(config.MaxAuditEvents)

	return &Manager{
		policy:         policy,
		auditor:        auditor,
		ipc:            NewIPCManager(policy, auditor),
		storageBackend: NewMemoryStorageBackend(),
		db:             db,
		sandboxes:      make(map[string]*ServiceSandbox),
		config:         config,
	}
}

// CreateSandbox creates a new sandbox for a service.
func (m *Manager) CreateSandbox(ctx context.Context, req CreateSandboxRequest) (*ServiceSandbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if sandbox already exists
	if _, exists := m.sandboxes[req.ServiceID]; exists {
		return nil, fmt.Errorf("sandbox already exists for service: %s", req.ServiceID)
	}

	// Create service identity
	identity := &ServiceIdentity{
		ServiceID:      req.ServiceID,
		PackageID:      req.PackageID,
		ProcessID:      GenerateProcessID(),
		SigningKeyHash: req.SigningKeyHash,
		SecurityLevel:  req.SecurityLevel,
		CreatedAt:      time.Now(),
	}

	// Create capability set
	caps := NewCapabilitySet()
	for _, cap := range req.RequestedCapabilities {
		// Evaluate if capability should be granted
		if m.shouldGrantCapability(identity, cap) {
			caps.Grant(cap, "sandbox_manager")
		}
	}

	// Create isolated storage
	storageQuota := req.StorageQuota
	if storageQuota == 0 {
		storageQuota = m.config.DefaultStorageQuota
	}
	storage := NewIsolatedStorage(req.ServiceID, m.storageBackend, storageQuota, m.auditor)

	// Create isolated database (if DB available)
	var database *IsolatedDatabase
	if m.db != nil {
		database = NewIsolatedDatabase(req.ServiceID, m.db, m.auditor)
		// Register allowed tables
		for _, table := range req.AllowedTables {
			database.RegisterTable(table)
		}
	}

	// Create sandbox context
	sandboxCtx := NewSandboxContext(identity, caps, m.policy, m.auditor)

	// Create IPC proxy
	ipcProxy := NewIPCProxy(m.ipc, identity, caps)

	// Create the sandbox
	sandbox := &ServiceSandbox{
		Identity:    identity,
		Caps:        caps,
		Context:     sandboxCtx,
		Storage:     storage,
		Database:    database,
		IPC:         ipcProxy,
		CreatedAt:   time.Now(),
		LastAccess:  time.Now(),
	}

	m.sandboxes[req.ServiceID] = sandbox

	// Log sandbox creation
	m.auditor.LogResourceAccess(ctx, req.ServiceID, "sandbox", "create", true)

	return sandbox, nil
}

// GetSandbox retrieves an existing sandbox.
func (m *Manager) GetSandbox(serviceID string) (*ServiceSandbox, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sandbox, exists := m.sandboxes[serviceID]
	if !exists {
		return nil, fmt.Errorf("sandbox not found: %s", serviceID)
	}

	sandbox.LastAccess = time.Now()
	return sandbox, nil
}

// DestroySandbox removes a sandbox.
func (m *Manager) DestroySandbox(ctx context.Context, serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sandbox, exists := m.sandboxes[serviceID]
	if !exists {
		return fmt.Errorf("sandbox not found: %s", serviceID)
	}

	// Unregister from IPC
	_ = m.ipc.UnregisterService(serviceID)

	// Log destruction
	m.auditor.LogResourceAccess(ctx, serviceID, "sandbox", "destroy", true)

	delete(m.sandboxes, serviceID)

	_ = sandbox // Could add cleanup logic here
	return nil
}

// RegisterIPCHandler registers an IPC handler for a service.
func (m *Manager) RegisterIPCHandler(serviceID string, handler IPCHandler) error {
	m.mu.RLock()
	sandbox, exists := m.sandboxes[serviceID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("sandbox not found: %s", serviceID)
	}

	endpoint := &ServiceEndpoint{
		ServiceID:    serviceID,
		Identity:     sandbox.Identity,
		Capabilities: sandbox.Caps,
		Handler:      handler,
	}

	return m.ipc.RegisterService(endpoint)
}

// shouldGrantCapability determines if a capability should be granted.
func (m *Manager) shouldGrantCapability(identity *ServiceIdentity, cap Capability) bool {
	// System services get all capabilities
	if identity.SecurityLevel == SecurityLevelSystem {
		return true
	}

	// Privileged services get most capabilities
	if identity.SecurityLevel == SecurityLevelPrivileged {
		// Deny only the most dangerous capabilities
		switch cap {
		case CapSystemAdmin, CapCryptoMasterKey:
			return false
		default:
			return true
		}
	}

	// Normal services get standard capabilities
	if identity.SecurityLevel == SecurityLevelNormal {
		switch cap {
		// Deny dangerous capabilities
		case CapSystemAdmin, CapSystemConfig, CapCryptoMasterKey,
			CapStorageOther, CapDatabaseOther, CapServiceManage:
			return false
		default:
			return true
		}
	}

	// Untrusted services get minimal capabilities
	switch cap {
	case CapStorageRead, CapStorageWrite, CapDatabaseRead, CapDatabaseWrite,
		CapBusPublish, CapBusSubscribe:
		return true
	default:
		return false
	}
}

// GetAuditEvents returns recent audit events.
func (m *Manager) GetAuditEvents(limit int) []AuditEvent {
	return m.auditor.GetEvents(limit)
}

// GetPolicy returns the security policy.
func (m *Manager) GetPolicy() *SecurityPolicy {
	return m.policy
}

// ListSandboxes returns all active sandboxes.
func (m *Manager) ListSandboxes() []SandboxInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []SandboxInfo
	for _, sb := range m.sandboxes {
		result = append(result, SandboxInfo{
			ServiceID:     sb.Identity.ServiceID,
			PackageID:     sb.Identity.PackageID,
			SecurityLevel: sb.Identity.SecurityLevel,
			Capabilities:  sb.Caps.List(),
			CreatedAt:     sb.CreatedAt,
			LastAccess:    sb.LastAccess,
		})
	}
	return result
}

// =============================================================================
// Service Sandbox
// =============================================================================

// ServiceSandbox contains all isolated resources for a service.
type ServiceSandbox struct {
	Identity   *ServiceIdentity
	Caps       *CapabilitySet
	Context    *SandboxContext
	Storage    *IsolatedStorage
	Database   *IsolatedDatabase
	IPC        *IPCProxy
	CreatedAt  time.Time
	LastAccess time.Time
}

// CreateSandboxRequest contains parameters for creating a sandbox.
type CreateSandboxRequest struct {
	ServiceID             string
	PackageID             string
	SigningKeyHash        string
	SecurityLevel         SecurityLevel
	RequestedCapabilities []Capability
	StorageQuota          int64
	AllowedTables         []string
}

// SandboxInfo contains summary information about a sandbox.
type SandboxInfo struct {
	ServiceID     string
	PackageID     string
	SecurityLevel SecurityLevel
	Capabilities  []Capability
	CreatedAt     time.Time
	LastAccess    time.Time
}

// =============================================================================
// Integration with Existing Runtime
// =============================================================================

// RuntimeAdapter adapts the sandbox to the existing PackageRuntime interface.
type RuntimeAdapter struct {
	sandbox *ServiceSandbox
	manager *Manager
}

// NewRuntimeAdapter creates a runtime adapter for a sandbox.
func NewRuntimeAdapter(sandbox *ServiceSandbox, manager *Manager) *RuntimeAdapter {
	return &RuntimeAdapter{
		sandbox: sandbox,
		manager: manager,
	}
}

// CheckCapability checks if the service has a capability.
func (r *RuntimeAdapter) CheckCapability(ctx context.Context, cap Capability) error {
	return r.sandbox.Context.CheckCapability(ctx, cap)
}

// Storage returns the isolated storage.
func (r *RuntimeAdapter) Storage() *IsolatedStorage {
	return r.sandbox.Storage
}

// Database returns the isolated database.
func (r *RuntimeAdapter) Database() *IsolatedDatabase {
	return r.sandbox.Database
}

// IPC returns the IPC proxy.
func (r *RuntimeAdapter) IPC() *IPCProxy {
	return r.sandbox.IPC
}

// Identity returns the service identity.
func (r *RuntimeAdapter) Identity() *ServiceIdentity {
	return r.sandbox.Identity
}
