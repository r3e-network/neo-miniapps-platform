package engine

import "context"

// ServiceModule is the common contract every service must implement to plug into the Engine.
// Each module advertises a name and domain, and exposes lifecycle hooks for Start/Stop.
type ServiceModule interface {
	Name() string
	Domain() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// AccountEngine covers account lifecycle and tenancy.
type AccountEngine interface {
	ServiceModule
	CreateAccount(ctx context.Context, owner string, metadata map[string]string) (string, error)
	ListAccounts(ctx context.Context) ([]any, error)
}

// StoreEngine abstracts persistence (e.g., Postgres, in-memory).
type StoreEngine interface {
	ServiceModule
	Ping(ctx context.Context) error
}

// ComputeEngine abstracts execution of user functions or jobs.
type ComputeEngine interface {
	ServiceModule
	Invoke(ctx context.Context, payload any) (any, error)
}

// DataEngine abstracts data-plane services like feeds/streams/datalink.
type DataEngine interface {
	ServiceModule
	Push(ctx context.Context, topic string, payload any) error
}

// EventEngine abstracts event dispatch/subscribe.
type EventEngine interface {
	ServiceModule
	Publish(ctx context.Context, event string, payload any) error
	Subscribe(ctx context.Context, event string, handler func(context.Context, any) error) error
}

// LedgerEngine abstracts a full node for a specific network (e.g., Neo).
type LedgerEngine interface {
	ServiceModule
	LedgerInfo() string
}

// IndexerEngine abstracts a chain indexer.
type IndexerEngine interface {
	ServiceModule
	IndexerInfo() string
}

// RPCEngine exposes generic chain RPC fan-out (btc/eth/neox/etc.).
type RPCEngine interface {
	ServiceModule
	RPCInfo() string
	RPCEndpoints() map[string]string
}

// DataSourceEngine exposes upstream data sources usable by feeds/triggers.
type DataSourceEngine interface {
	ServiceModule
	DataSourcesInfo() string
}

// ContractsEngine manages deployment/invocation of service-layer contracts.
// Extended to support the full contract lifecycle including deployment,
// invocation, and service binding.
type ContractsEngine interface {
	ServiceModule
	ContractsNetwork() string
	// Deploy initiates a contract deployment.
	Deploy(ctx context.Context, contractID string, args map[string]any) (string, error)
	// Invoke calls a contract method.
	Invoke(ctx context.Context, contractID, method string, args map[string]any) (any, error)
	// GetContractAddress returns the deployed address for a contract.
	GetContractAddress(ctx context.Context, contractID string) (string, error)
}

// ServiceBankEngine controls GAS usage owned by the service layer.
type ServiceBankEngine interface {
	ServiceModule
	ServiceBankInfo() string
}

// CryptoEngine exposes advanced cryptography helpers (ZKP/FHE/MPC).
type CryptoEngine interface {
	ServiceModule
	CryptoInfo() string
}

// SecretsEngine provides secure secret storage and resolution for services.
// It handles encrypted storage, access control, and secret lifecycle management.
type SecretsEngine interface {
	ServiceModule
	// StoreSecret stores an encrypted secret for an account.
	StoreSecret(ctx context.Context, accountID, name string, value []byte) error
	// GetSecret retrieves a decrypted secret by name.
	GetSecret(ctx context.Context, accountID, name string) ([]byte, error)
	// DeleteSecret removes a secret.
	DeleteSecret(ctx context.Context, accountID, name string) error
	// ListSecrets returns secret names (not values) for an account.
	ListSecrets(ctx context.Context, accountID string) ([]string, error)
	// ResolveSecrets resolves multiple secrets by name, returning a map.
	ResolveSecrets(ctx context.Context, accountID string, names []string) (map[string][]byte, error)
	// SecretsInfo returns secrets engine metadata.
	SecretsInfo() string
}

// =============================================================================
// Security & Access Control Engines
// =============================================================================

// SecurityEngine provides security policy enforcement and threat detection.
// It handles authentication, authorization policies, and security monitoring.
type SecurityEngine interface {
	ServiceModule
	// ValidateToken validates an authentication token and returns claims.
	ValidateToken(ctx context.Context, token string) (SecurityClaims, error)
	// EnforcePolicy checks if an action is allowed by security policies.
	EnforcePolicy(ctx context.Context, subject, action, resource string) error
	// SecurityInfo returns security engine metadata.
	SecurityInfo() string
}

// SecurityClaims represents validated authentication claims.
type SecurityClaims struct {
	Subject   string            // User or service identifier
	Issuer    string            // Token issuer
	Audience  []string          // Intended audiences
	ExpiresAt int64             // Expiration timestamp
	IssuedAt  int64             // Issue timestamp
	Claims    map[string]any    // Additional claims
}

// PermissionEngine manages fine-grained permissions and RBAC.
type PermissionEngine interface {
	ServiceModule
	// CheckPermission verifies if a subject has permission for an action on a resource.
	CheckPermission(ctx context.Context, subject, action, resource string) (bool, error)
	// GrantPermission grants a permission to a subject.
	GrantPermission(ctx context.Context, subject, action, resource string) error
	// RevokePermission revokes a permission from a subject.
	RevokePermission(ctx context.Context, subject, action, resource string) error
	// ListPermissions lists permissions for a subject.
	ListPermissions(ctx context.Context, subject string) ([]Permission, error)
}

// Permission represents a single permission grant.
type Permission struct {
	Subject   string `json:"subject"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
	GrantedAt int64  `json:"granted_at"`
	GrantedBy string `json:"granted_by"`
}

// AuditEngine provides audit logging and compliance tracking.
type AuditEngine interface {
	ServiceModule
	// LogAuditEvent records an audit event.
	LogAuditEvent(ctx context.Context, event AuditEvent) error
	// QueryAuditLog queries audit events with filters.
	QueryAuditLog(ctx context.Context, filter AuditFilter) ([]AuditEvent, error)
	// AuditInfo returns audit engine metadata.
	AuditInfo() string
}

// AuditEvent represents a single audit log entry.
type AuditEvent struct {
	ID         string         `json:"id"`
	Timestamp  int64          `json:"timestamp"`
	Actor      string         `json:"actor"`       // Who performed the action
	Action     string         `json:"action"`      // What action was performed
	Resource   string         `json:"resource"`    // What resource was affected
	ResourceID string         `json:"resource_id"` // Specific resource identifier
	Outcome    string         `json:"outcome"`     // success, failure, denied
	Details    map[string]any `json:"details"`     // Additional context
	IPAddress  string         `json:"ip_address"`  // Source IP
	UserAgent  string         `json:"user_agent"`  // Client user agent
}

// AuditFilter specifies criteria for querying audit logs.
type AuditFilter struct {
	Actor      string `json:"actor,omitempty"`
	Action     string `json:"action,omitempty"`
	Resource   string `json:"resource,omitempty"`
	Outcome    string `json:"outcome,omitempty"`
	StartTime  int64  `json:"start_time,omitempty"`
	EndTime    int64  `json:"end_time,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
}

// =============================================================================
// Infrastructure Engines
// =============================================================================

// CacheEngine abstracts caching operations (Redis, Memcached, in-memory).
type CacheEngine interface {
	ServiceModule
	// Get retrieves a value from cache.
	Get(ctx context.Context, key string) ([]byte, error)
	// Set stores a value in cache with optional TTL (seconds, 0 = no expiry).
	Set(ctx context.Context, key string, value []byte, ttlSeconds int) error
	// Delete removes a value from cache.
	Delete(ctx context.Context, key string) error
	// Exists checks if a key exists in cache.
	Exists(ctx context.Context, key string) (bool, error)
	// CacheInfo returns cache engine metadata.
	CacheInfo() string
}

// QueueEngine abstracts message queue operations (RabbitMQ, Kafka, SQS).
type QueueEngine interface {
	ServiceModule
	// Enqueue adds a message to a queue.
	Enqueue(ctx context.Context, queue string, message []byte) error
	// Dequeue retrieves and removes a message from a queue.
	Dequeue(ctx context.Context, queue string) ([]byte, error)
	// Subscribe registers a handler for messages on a queue.
	Subscribe(ctx context.Context, queue string, handler func(context.Context, []byte) error) error
	// QueueInfo returns queue engine metadata.
	QueueInfo() string
}

// SchedulerEngine manages scheduled tasks and cron jobs.
type SchedulerEngine interface {
	ServiceModule
	// Schedule registers a task to run at specified intervals.
	Schedule(ctx context.Context, task ScheduledTask) (string, error)
	// Cancel cancels a scheduled task.
	Cancel(ctx context.Context, taskID string) error
	// ListTasks lists all scheduled tasks.
	ListTasks(ctx context.Context) ([]ScheduledTask, error)
	// SchedulerInfo returns scheduler engine metadata.
	SchedulerInfo() string
}

// ScheduledTask represents a scheduled job.
type ScheduledTask struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Schedule string         `json:"schedule"` // Cron expression
	Payload  map[string]any `json:"payload"`
	Enabled  bool           `json:"enabled"`
	LastRun  int64          `json:"last_run"`
	NextRun  int64          `json:"next_run"`
}

// NotificationEngine handles notifications across channels (email, SMS, push).
type NotificationEngine interface {
	ServiceModule
	// Send sends a notification through the specified channel.
	Send(ctx context.Context, notification Notification) error
	// NotificationInfo returns notification engine metadata.
	NotificationInfo() string
}

// Notification represents a notification to be sent.
type Notification struct {
	Channel   string         `json:"channel"`   // email, sms, push, webhook
	Recipient string         `json:"recipient"` // Target address/ID
	Subject   string         `json:"subject"`
	Body      string         `json:"body"`
	Metadata  map[string]any `json:"metadata"`
}

// =============================================================================
// Observability Engines
// =============================================================================

// MetricsEngine provides metrics collection and export.
type MetricsEngine interface {
	ServiceModule
	// Counter increments a counter metric.
	Counter(name string, labels map[string]string, delta float64)
	// Gauge sets a gauge metric.
	Gauge(name string, labels map[string]string, value float64)
	// Histogram records a histogram observation.
	Histogram(name string, labels map[string]string, value float64)
	// MetricsInfo returns metrics engine metadata.
	MetricsInfo() string
}

// TracingEngine provides distributed tracing capabilities.
type TracingEngine interface {
	ServiceModule
	// StartSpan starts a new trace span.
	StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, func(error))
	// TracingInfo returns tracing engine metadata.
	TracingInfo() string
}

// Capability markers allow adapters to avoid advertising interfaces they cannot serve.

// AccountCapable indicates whether a module supports account operations.
type AccountCapable interface {
	HasAccount() bool
}

// ComputeCapable indicates whether a module supports compute operations.
type ComputeCapable interface {
	HasCompute() bool
}

// DataCapable indicates whether a module supports data operations.
type DataCapable interface {
	HasData() bool
}

// EventCapable indicates whether a module supports event operations.
type EventCapable interface {
	HasEvent() bool
}

// ReadyChecker reports whether a module is currently ready to serve traffic.
type ReadyChecker interface {
	Ready(ctx context.Context) error
}

// ReadySetter can be implemented by modules to allow the engine to mark readiness explicitly.
type ReadySetter interface {
	SetReady(status string, errMsg string)
}

// EventHandler is a callback used by SubscribeEvent for in-process consumers.
type EventHandler func(context.Context, any) error

// InvokeResult captures the outcome of a ComputeEngine invocation.
type InvokeResult struct {
	Module string
	Result any
	Err    error
}
