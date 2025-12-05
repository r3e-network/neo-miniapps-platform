// Package os provides the ServiceOS abstraction layer.
// ServiceOS is like Android OS - it abstracts TEE details and provides
// capability-based access control for services.
package os

import (
	"context"
	"time"
)

// =============================================================================
// Capabilities (Android-style Permissions)
// =============================================================================

// Capability represents a permission that a service can request.
type Capability string

const (
	// Core capabilities
	CapSecrets     Capability = "secrets"     // Access to secret storage
	CapNetwork     Capability = "network"     // Outbound network requests
	CapKeys        Capability = "keys"        // Key derivation and signing
	CapCompute     Capability = "compute"     // Confidential computation
	CapStorage     Capability = "storage"     // Persistent storage
	CapEvents      Capability = "events"      // Event bus access
	CapAttestation Capability = "attestation" // Remote attestation

	// Extended capabilities
	CapNetworkExternal Capability = "network.external" // External network access
	CapSecretsWrite    Capability = "secrets.write"    // Write secrets
	CapKeysSign        Capability = "keys.sign"        // Sign with keys

	// Neo N3 capabilities
	CapNeo     Capability = "neo"      // Neo N3 operations
	CapNeoSign Capability = "neo.sign" // Sign Neo transactions

	// Database capabilities (Supabase)
	CapDatabase      Capability = "database"       // Database access
	CapDatabaseWrite Capability = "database.write" // Write to database

	// General service capabilities
	CapConfig    Capability = "config"    // Configuration management
	CapMetrics   Capability = "metrics"   // Metrics collection
	CapScheduler Capability = "scheduler" // Task scheduling
	CapCache     Capability = "cache"     // Caching
	CapChain     Capability = "chain"     // Generic blockchain operations
	CapAuth      Capability = "auth"      // Authentication/authorization
	CapQueue     Capability = "queue"     // Message queue
	CapLock      Capability = "lock"      // Distributed locking

	// Contract capabilities (Neo N3 Service Layer contracts)
	CapContract      Capability = "contract"       // Contract interaction
	CapContractWrite Capability = "contract.write" // Write to contracts (callbacks)
)

// =============================================================================
// Service Manifest - See manifest.go for MarbleRun-style Manifest
// =============================================================================

// NOTE: The main Manifest type is defined in manifest.go with MarbleRun-style
// configuration (Packages, Marbles, Secrets, Users, Roles, TLS, etc.)
// LegacyManifest in manifest.go provides backward compatibility.

// =============================================================================
// ServiceOS Interface
// =============================================================================

// ServiceOS is the main interface for services to interact with the system.
// It's like Android's Context - provides access to system services.
// Note: Uses LegacyManifest for per-service configuration.
type ServiceOS interface {
	// Identity
	ServiceID() string
	Manifest() *LegacyManifest

	// Capability checking
	HasCapability(cap Capability) bool
	RequireCapability(cap Capability) error

	// Core TEE-backed APIs
	Secrets() SecretsAPI
	Network() NetworkAPI
	Keys() KeysAPI
	Compute() ComputeAPI
	Storage() StorageAPI
	Attestation() AttestationAPI

	// Blockchain APIs
	Neo() NeoAPI     // Neo N3 blockchain operations
	Chain() ChainAPI // Generic blockchain operations

	// Infrastructure APIs
	Database() DatabaseAPI // Supabase database operations
	Events() EventsAPI     // Event bus
	Cache() CacheAPI       // Caching
	Queue() QueueAPI       // Message queue

	// Service Management APIs
	Config() ConfigAPI       // Configuration management
	Metrics() MetricsAPI     // Metrics collection
	Scheduler() SchedulerAPI // Task scheduling
	Auth() AuthAPI           // Authentication/authorization
	Lock() LockAPI           // Distributed locking

	// Contract APIs (Neo N3 Service Layer)
	Contract() ContractAPI // Service Layer contract operations

	// Lifecycle
	Context() context.Context
	Logger() Logger
}

// =============================================================================
// API Interfaces
// =============================================================================

// SecretsAPI provides access to secret storage.
type SecretsAPI interface {
	// Store stores a secret (requires secrets.write capability)
	Store(ctx context.Context, name string, value []byte) error

	// Use executes a function with access to a secret
	// The secret is ONLY available inside the callback
	Use(ctx context.Context, name string, fn func(secret []byte) error) error

	// UseMultiple executes a function with multiple secrets
	UseMultiple(ctx context.Context, names []string, fn func(secrets map[string][]byte) error) error

	// Delete removes a secret
	Delete(ctx context.Context, name string) error

	// List returns secret names
	List(ctx context.Context) ([]string, error)

	// Exists checks if a secret exists
	Exists(ctx context.Context, name string) (bool, error)
}

// NetworkAPI provides secure networking.
type NetworkAPI interface {
	// Fetch performs an HTTP request
	Fetch(ctx context.Context, req HTTPRequest) (*HTTPResponse, error)

	// FetchWithSecret performs HTTP with secret-based auth
	FetchWithSecret(ctx context.Context, req HTTPRequest, secretName string, authType AuthType) (*HTTPResponse, error)

	// RPC performs a JSON-RPC call
	RPC(ctx context.Context, endpoint, method string, params any) ([]byte, error)

	// RPCWithSecret performs RPC with secret-based auth
	RPCWithSecret(ctx context.Context, endpoint, method string, params any, secretName string) ([]byte, error)
}

// HTTPRequest represents an HTTP request.
type HTTPRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
	Timeout time.Duration     `json:"timeout,omitempty"`
}

// HTTPResponse represents an HTTP response.
type HTTPResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
}

// AuthType specifies authentication type.
type AuthType string

const (
	AuthBearer AuthType = "bearer"
	AuthBasic  AuthType = "basic"
	AuthAPIKey AuthType = "api_key"
)

// KeysAPI provides key management.
type KeysAPI interface {
	// DeriveKey derives a key from the service's key path
	DeriveKey(ctx context.Context, subPath string) (KeyHandle, error)

	// Sign signs data with a key
	Sign(ctx context.Context, handle KeyHandle, data []byte) ([]byte, error)

	// Verify verifies a signature
	Verify(ctx context.Context, publicKey, data, signature []byte) (bool, error)

	// GetPublicKey returns the public key
	GetPublicKey(ctx context.Context, handle KeyHandle) ([]byte, error)

	// GetAddress returns the blockchain address
	GetAddress(ctx context.Context, handle KeyHandle, chain ChainType) (string, error)
}

// KeyHandle is an opaque reference to a key.
type KeyHandle string

// ChainType specifies blockchain type.
type ChainType string

const (
	ChainEthereum ChainType = "ethereum"
	ChainNeo      ChainType = "neo"
)

// ComputeAPI provides confidential computation.
type ComputeAPI interface {
	// Execute runs code inside the enclave
	Execute(ctx context.Context, req ComputeRequest) (*ComputeResult, error)

	// ExecuteWithSecrets runs code with access to secrets
	ExecuteWithSecrets(ctx context.Context, req ComputeRequest, secretNames []string) (*ComputeResult, error)
}

// ComputeRequest specifies computation parameters.
type ComputeRequest struct {
	Script     string         `json:"script"`
	EntryPoint string         `json:"entry_point"`
	Input      map[string]any `json:"input,omitempty"`
	Timeout    time.Duration  `json:"timeout,omitempty"`
}

// ComputeResult contains computation results.
type ComputeResult struct {
	Success bool           `json:"success"`
	Output  map[string]any `json:"output,omitempty"`
	Logs    []string       `json:"logs,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// StorageAPI provides persistent storage.
type StorageAPI interface {
	// Get retrieves a value
	Get(ctx context.Context, key string) ([]byte, error)

	// Use executes a callback with the unsealed value; plaintext stays enclave-side.
	Use(ctx context.Context, key string, fn func(value []byte) error) error

	// Exists checks if a key exists
	Exists(ctx context.Context, key string) (bool, error)

	// Put stores a value
	Put(ctx context.Context, key string, value []byte) error

	// Delete removes a value
	Delete(ctx context.Context, key string) error

	// List returns keys with prefix
	List(ctx context.Context, prefix string) ([]string, error)
}

// EventsAPI provides event bus access.
type EventsAPI interface {
	// Publish publishes an event
	Publish(ctx context.Context, topic string, data any) error

	// Subscribe subscribes to events
	Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error)
}

// EventHandler handles events.
type EventHandler func(ctx context.Context, event Event) error

// Event represents an event.
type Event struct {
	Topic     string    `json:"topic"`
	Data      any       `json:"data"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
}

// Subscription represents an event subscription.
type Subscription interface {
	Unsubscribe() error
}

// AttestationAPI provides remote attestation.
type AttestationAPI interface {
	// GenerateQuote generates an attestation quote
	GenerateQuote(ctx context.Context, userData []byte) (*Quote, error)

	// VerifyQuote verifies a quote
	VerifyQuote(ctx context.Context, quote *Quote) (*QuoteVerification, error)

	// GetReport returns the attestation report
	GetReport(ctx context.Context) (*AttestationReport, error)
}

// Quote represents an attestation quote.
type Quote struct {
	RawQuote  []byte    `json:"raw_quote"`
	UserData  []byte    `json:"user_data"`
	MREnclave string    `json:"mr_enclave"`
	MRSigner  string    `json:"mr_signer"`
	Timestamp time.Time `json:"timestamp"`
}

// QuoteVerification contains verification results.
type QuoteVerification struct {
	Valid      bool      `json:"valid"`
	MREnclave  string    `json:"mr_enclave"`
	MRSigner   string    `json:"mr_signer"`
	VerifiedAt time.Time `json:"verified_at"`
}

// AttestationReport contains attestation information.
type AttestationReport struct {
	EnclaveID string    `json:"enclave_id"`
	Mode      string    `json:"mode"`
	MREnclave string    `json:"mr_enclave"`
	MRSigner  string    `json:"mr_signer"`
	Timestamp time.Time `json:"timestamp"`
}

// =============================================================================
// Logger Interface
// =============================================================================

// Logger provides logging for services.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// =============================================================================
// Errors
// =============================================================================

// Error codes
const (
	ErrCodeCapabilityDenied = "CAPABILITY_DENIED"
	ErrCodeSecretNotFound   = "SECRET_NOT_FOUND"
	ErrCodeKeyNotFound      = "KEY_NOT_FOUND"
	ErrCodeNetworkError     = "NETWORK_ERROR"
	ErrCodeComputeError     = "COMPUTE_ERROR"
	ErrCodeStorageError     = "STORAGE_ERROR"
)

// OSError represents a ServiceOS error.
type OSError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *OSError) Error() string {
	return e.Message
}

// NewOSError creates a new OS error.
func NewOSError(code, message string) *OSError {
	return &OSError{Code: code, Message: message}
}

// ErrCapabilityDenied returns a capability denied error.
func ErrCapabilityDenied(cap Capability) *OSError {
	return NewOSError(ErrCodeCapabilityDenied, "capability denied: "+string(cap))
}

// =============================================================================
// NeoAPI - Neo N3 Blockchain Operations
// =============================================================================

// WalletHandle is an opaque reference to a Neo wallet.
type WalletHandle string

// NeoAPI provides Neo N3 blockchain operations.
// All signing happens inside the TEE enclave - private keys never leave.
type NeoAPI interface {
	// CreateWallet creates a new Neo wallet inside the enclave.
	CreateWallet(ctx context.Context, path string) (WalletHandle, error)

	// DeriveWallet derives a wallet from a seed path.
	DeriveWallet(ctx context.Context, seedPath string) (WalletHandle, error)

	// GetAddress returns the Neo N3 address for a wallet.
	GetAddress(ctx context.Context, handle WalletHandle) (string, error)

	// GetPublicKey returns the compressed public key.
	GetPublicKey(ctx context.Context, handle WalletHandle) ([]byte, error)

	// GetScriptHash returns the script hash.
	GetScriptHash(ctx context.Context, handle WalletHandle) ([]byte, error)

	// SignData signs arbitrary data (signing inside enclave).
	SignData(ctx context.Context, handle WalletHandle, data []byte) ([]byte, error)

	// SignTransaction signs a Neo transaction hash (signing inside enclave).
	SignTransaction(ctx context.Context, handle WalletHandle, txHash []byte) ([]byte, error)

	// VerifySignature verifies a signature.
	VerifySignature(ctx context.Context, publicKey, data, signature []byte) (bool, error)

	// ListWallets returns all wallet handles for this service.
	ListWallets(ctx context.Context) ([]WalletHandle, error)

	// DeleteWallet removes a wallet.
	DeleteWallet(ctx context.Context, handle WalletHandle) error
}

// =============================================================================
// DatabaseAPI - Supabase Database Operations
// =============================================================================

// DatabaseAPI provides database operations via Supabase.
type DatabaseAPI interface {
	// From starts a query builder for a table.
	From(table string) QueryBuilder

	// RPC calls a Postgres function.
	RPC(ctx context.Context, fn string, params any) ([]byte, error)
}

// QueryBuilder builds database queries.
type QueryBuilder interface {
	// Select specifies columns to select.
	Select(columns string) QueryBuilder

	// Insert inserts records.
	Insert(data any) QueryBuilder

	// Update updates records.
	Update(data any) QueryBuilder

	// Delete deletes records.
	Delete() QueryBuilder

	// Filters
	Eq(column string, value any) QueryBuilder
	Neq(column string, value any) QueryBuilder
	Gt(column string, value any) QueryBuilder
	Gte(column string, value any) QueryBuilder
	Lt(column string, value any) QueryBuilder
	Lte(column string, value any) QueryBuilder
	Like(column, pattern string) QueryBuilder
	ILike(column, pattern string) QueryBuilder
	Is(column string, value any) QueryBuilder
	In(column string, values []any) QueryBuilder

	// Ordering and pagination
	Order(column string, ascending bool) QueryBuilder
	Limit(n int) QueryBuilder
	Offset(n int) QueryBuilder

	// Single expects a single row result.
	Single() QueryBuilder

	// Execute executes the query and returns raw bytes.
	Execute(ctx context.Context) ([]byte, error)

	// ExecuteInto executes and unmarshals into dest.
	ExecuteInto(ctx context.Context, dest any) error
}

// =============================================================================
// ConfigAPI - Configuration Management
// =============================================================================

// ConfigAPI provides configuration management for services.
type ConfigAPI interface {
	// Get retrieves a configuration value.
	Get(ctx context.Context, key string) (any, error)

	// GetString retrieves a string configuration value.
	GetString(ctx context.Context, key string) (string, error)

	// GetInt retrieves an integer configuration value.
	GetInt(ctx context.Context, key string) (int, error)

	// GetBool retrieves a boolean configuration value.
	GetBool(ctx context.Context, key string) (bool, error)

	// GetDuration retrieves a duration configuration value.
	GetDuration(ctx context.Context, key string) (time.Duration, error)

	// Set sets a configuration value.
	Set(ctx context.Context, key string, value any) error

	// Delete removes a configuration value.
	Delete(ctx context.Context, key string) error

	// All returns all configuration values.
	All(ctx context.Context) (map[string]any, error)

	// Watch watches for configuration changes.
	Watch(ctx context.Context, key string, handler func(key string, value any)) error
}

// =============================================================================
// MetricsAPI - Metrics Collection
// =============================================================================

// MetricsAPI provides metrics collection for services.
type MetricsAPI interface {
	// Counter increments a counter metric.
	Counter(name string, value float64, labels ...string)

	// Gauge sets a gauge metric.
	Gauge(name string, value float64, labels ...string)

	// Histogram records a histogram observation.
	Histogram(name string, value float64, labels ...string)

	// Timer records a duration.
	Timer(name string, duration time.Duration, labels ...string)

	// RegisterCounter registers a new counter metric.
	RegisterCounter(name, help string, labelNames ...string) error

	// RegisterGauge registers a new gauge metric.
	RegisterGauge(name, help string, labelNames ...string) error

	// RegisterHistogram registers a new histogram metric.
	RegisterHistogram(name, help string, buckets []float64, labelNames ...string) error
}

// =============================================================================
// SchedulerAPI - Task Scheduling
// =============================================================================

// SchedulerAPI provides task scheduling for services.
type SchedulerAPI interface {
	// Schedule schedules a task to run at a specific time.
	Schedule(ctx context.Context, task *ScheduledTask) (string, error)

	// ScheduleCron schedules a task using cron expression.
	ScheduleCron(ctx context.Context, cronExpr string, task *ScheduledTask) (string, error)

	// ScheduleInterval schedules a task to run at regular intervals.
	ScheduleInterval(ctx context.Context, interval time.Duration, task *ScheduledTask) (string, error)

	// Cancel cancels a scheduled task.
	Cancel(ctx context.Context, taskID string) error

	// List lists all scheduled tasks.
	List(ctx context.Context) ([]*ScheduledTask, error)

	// Get retrieves a scheduled task by ID.
	Get(ctx context.Context, taskID string) (*ScheduledTask, error)
}

// ScheduledTask represents a scheduled task.
type ScheduledTask struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Handler     string         `json:"handler"`
	Payload     map[string]any `json:"payload,omitempty"`
	ScheduledAt time.Time      `json:"scheduled_at"`
	Interval    time.Duration  `json:"interval,omitempty"`
	CronExpr    string         `json:"cron_expr,omitempty"`
	MaxRetries  int            `json:"max_retries"`
	Timeout     time.Duration  `json:"timeout"`
	Status      TaskStatus     `json:"status"`
	LastRunAt   *time.Time     `json:"last_run_at,omitempty"`
	NextRunAt   *time.Time     `json:"next_run_at,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// TaskStatus represents the status of a scheduled task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// =============================================================================
// CacheAPI - Caching
// =============================================================================

// CacheAPI provides caching for services.
type CacheAPI interface {
	// Get retrieves a value from cache.
	Get(ctx context.Context, key string) ([]byte, error)

	// GetInto retrieves and unmarshals a value from cache.
	GetInto(ctx context.Context, key string, dest any) error

	// Set stores a value in cache with optional TTL.
	Set(ctx context.Context, key string, value any, ttl time.Duration) error

	// Delete removes a value from cache.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in cache.
	Exists(ctx context.Context, key string) (bool, error)

	// Expire sets expiration on a key.
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// TTL returns the remaining TTL for a key.
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Clear clears all cache entries for this service.
	Clear(ctx context.Context) error

	// GetOrSet gets a value or sets it if not exists.
	GetOrSet(ctx context.Context, key string, fn func() (any, error), ttl time.Duration) ([]byte, error)
}

// =============================================================================
// ChainAPI - Generic Blockchain Operations
// =============================================================================

// ChainAPI provides generic blockchain operations.
type ChainAPI interface {
	// GetBlock retrieves a block by height or hash.
	GetBlock(ctx context.Context, chainID string, blockRef string) (*Block, error)

	// GetTransaction retrieves a transaction by hash.
	GetTransaction(ctx context.Context, chainID string, txHash string) (*Transaction, error)

	// GetBalance retrieves the balance of an address.
	GetBalance(ctx context.Context, chainID string, address string, asset string) (*Balance, error)

	// SendTransaction sends a signed transaction.
	SendTransaction(ctx context.Context, chainID string, signedTx []byte) (string, error)

	// CallContract calls a smart contract (read-only).
	CallContract(ctx context.Context, chainID string, call *ContractCall) ([]byte, error)

	// InvokeContract invokes a smart contract (state-changing).
	InvokeContract(ctx context.Context, chainID string, invoke *ContractInvoke) (string, error)

	// SubscribeEvents subscribes to blockchain events.
	SubscribeEvents(ctx context.Context, chainID string, filter *EventFilter, handler func(*ChainEvent)) (Subscription, error)

	// GetChainInfo retrieves chain information.
	GetChainInfo(ctx context.Context, chainID string) (*ChainInfo, error)
}

// Block represents a blockchain block.
type Block struct {
	ChainID       string    `json:"chain_id"`
	Height        uint64    `json:"height"`
	Hash          string    `json:"hash"`
	PreviousHash  string    `json:"previous_hash"`
	Timestamp     time.Time `json:"timestamp"`
	Transactions  []string  `json:"transactions"`
	StateRoot     string    `json:"state_root,omitempty"`
	Confirmations uint64    `json:"confirmations"`
}

// Transaction represents a blockchain transaction.
type Transaction struct {
	ChainID       string         `json:"chain_id"`
	Hash          string         `json:"hash"`
	BlockHash     string         `json:"block_hash,omitempty"`
	BlockHeight   uint64         `json:"block_height,omitempty"`
	From          string         `json:"from"`
	To            string         `json:"to,omitempty"`
	Value         string         `json:"value,omitempty"`
	Data          []byte         `json:"data,omitempty"`
	Status        string         `json:"status"`
	Confirmations uint64         `json:"confirmations"`
	Timestamp     time.Time      `json:"timestamp"`
	Extra         map[string]any `json:"extra,omitempty"`
}

// Balance represents an account balance.
type Balance struct {
	ChainID  string `json:"chain_id"`
	Address  string `json:"address"`
	Asset    string `json:"asset"`
	Amount   string `json:"amount"`
	Decimals int    `json:"decimals"`
}

// ContractCall represents a contract call (read-only).
type ContractCall struct {
	Contract string   `json:"contract"`
	Method   string   `json:"method"`
	Args     []any    `json:"args,omitempty"`
	Caller   string   `json:"caller,omitempty"`
}

// ContractInvoke represents a contract invocation (state-changing).
type ContractInvoke struct {
	Contract   string   `json:"contract"`
	Method     string   `json:"method"`
	Args       []any    `json:"args,omitempty"`
	Signer     string   `json:"signer"`
	GasLimit   uint64   `json:"gas_limit,omitempty"`
	GasPrice   string   `json:"gas_price,omitempty"`
	Value      string   `json:"value,omitempty"`
}

// EventFilter for blockchain event subscription.
type EventFilter struct {
	Contract   string   `json:"contract,omitempty"`
	EventNames []string `json:"event_names,omitempty"`
	FromBlock  uint64   `json:"from_block,omitempty"`
	ToBlock    uint64   `json:"to_block,omitempty"`
}

// ChainEvent represents a blockchain event.
type ChainEvent struct {
	ChainID     string         `json:"chain_id"`
	BlockHeight uint64         `json:"block_height"`
	TxHash      string         `json:"tx_hash"`
	Contract    string         `json:"contract"`
	EventName   string         `json:"event_name"`
	Data        map[string]any `json:"data"`
	Timestamp   time.Time      `json:"timestamp"`
}

// ChainInfo represents blockchain information.
type ChainInfo struct {
	ChainID     string `json:"chain_id"`
	Name        string `json:"name"`
	BlockHeight uint64 `json:"block_height"`
	NetworkID   string `json:"network_id"`
	Protocol    string `json:"protocol"` // e.g., "neo", "ethereum", "bitcoin"
}

// =============================================================================
// AuthAPI - Authentication/Authorization
// =============================================================================

// AuthAPI provides authentication and authorization for services.
type AuthAPI interface {
	// VerifyToken verifies an access token and returns claims.
	VerifyToken(ctx context.Context, token string) (*TokenClaims, error)

	// GetUser retrieves user information from a token.
	GetUser(ctx context.Context, token string) (*AuthUser, error)

	// HasPermission checks if a user has a specific permission.
	HasPermission(ctx context.Context, userID string, permission string) (bool, error)

	// HasRole checks if a user has a specific role.
	HasRole(ctx context.Context, userID string, role string) (bool, error)

	// GetPermissions retrieves all permissions for a user.
	GetPermissions(ctx context.Context, userID string) ([]string, error)

	// GetRoles retrieves all roles for a user.
	GetRoles(ctx context.Context, userID string) ([]string, error)
}

// TokenClaims represents JWT token claims.
type TokenClaims struct {
	Subject   string         `json:"sub"`
	Issuer    string         `json:"iss"`
	Audience  string         `json:"aud"`
	ExpiresAt int64          `json:"exp"`
	IssuedAt  int64          `json:"iat"`
	Roles     []string       `json:"roles,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// AuthUser represents an authenticated user (from AuthAPI).
// Note: This is different from manifest.User which defines manifest users.
type AuthUser struct {
	ID       string         `json:"id"`
	Email    string         `json:"email,omitempty"`
	Name     string         `json:"name,omitempty"`
	Roles    []string       `json:"roles,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// =============================================================================
// QueueAPI - Message Queue
// =============================================================================

// QueueAPI provides message queue operations for services.
type QueueAPI interface {
	// Publish publishes a message to a queue.
	Publish(ctx context.Context, queue string, message *QueueMessage) error

	// Subscribe subscribes to a queue.
	Subscribe(ctx context.Context, queue string, handler func(*QueueMessage) error) (Subscription, error)

	// Ack acknowledges a message.
	Ack(ctx context.Context, messageID string) error

	// Nack negatively acknowledges a message (requeue).
	Nack(ctx context.Context, messageID string, requeue bool) error

	// GetQueueInfo retrieves queue information.
	GetQueueInfo(ctx context.Context, queue string) (*QueueInfo, error)
}

// QueueMessage represents a queue message.
type QueueMessage struct {
	ID          string         `json:"id"`
	Queue       string         `json:"queue"`
	Body        []byte         `json:"body"`
	Headers     map[string]any `json:"headers,omitempty"`
	Priority    int            `json:"priority"`
	Delay       time.Duration  `json:"delay,omitempty"`
	Expiration  time.Duration  `json:"expiration,omitempty"`
	PublishedAt time.Time      `json:"published_at"`
	Attempts    int            `json:"attempts"`
}

// QueueInfo represents queue information.
type QueueInfo struct {
	Name         string `json:"name"`
	MessageCount int64  `json:"message_count"`
	ConsumerCount int   `json:"consumer_count"`
}

// =============================================================================
// LockAPI - Distributed Locking
// =============================================================================

// LockAPI provides distributed locking for services.
type LockAPI interface {
	// Acquire acquires a lock with the given key.
	Acquire(ctx context.Context, key string, ttl time.Duration) (Lock, error)

	// TryAcquire tries to acquire a lock without blocking.
	TryAcquire(ctx context.Context, key string, ttl time.Duration) (Lock, bool, error)

	// IsLocked checks if a key is locked.
	IsLocked(ctx context.Context, key string) (bool, error)
}

// Lock represents a distributed lock.
type Lock interface {
	// Release releases the lock.
	Release(ctx context.Context) error

	// Extend extends the lock TTL.
	Extend(ctx context.Context, ttl time.Duration) error

	// Key returns the lock key.
	Key() string
}

// =============================================================================
// ContractAPI - Neo N3 Service Layer Contract Operations
// =============================================================================

// ContractAPI provides interaction with Neo N3 Service Layer contracts.
// This API enables services to:
// - Monitor contract events for service requests
// - Send callbacks with results
// - Push data to contracts (DataFeeds)
// - Execute automation triggers
type ContractAPI interface {
	// SubscribeRequests subscribes to service request events from the Gateway contract.
	// The handler is called for each new request targeting this service.
	SubscribeRequests(ctx context.Context, handler ServiceRequestHandler) (Subscription, error)

	// SendCallback sends a service response callback to the Gateway contract.
	// This delivers the result back to the user contract.
	SendCallback(ctx context.Context, response *ServiceResponse) error

	// GetRequest retrieves a pending request by ID.
	GetRequest(ctx context.Context, requestID string) (*ServiceRequest, error)

	// GetPendingRequests retrieves all pending requests for this service.
	GetPendingRequests(ctx context.Context) ([]*ServiceRequest, error)

	// UpdatePrice pushes a price update to the DataFeeds contract.
	// Only available for DataFeeds service.
	UpdatePrice(ctx context.Context, update *PriceUpdate) error

	// ExecuteTrigger executes an automation trigger.
	// Only available for Automation service.
	ExecuteTrigger(ctx context.Context, triggerID string) error

	// GetTrigger retrieves a trigger configuration.
	GetTrigger(ctx context.Context, triggerID string) (*AutomationTrigger, error)

	// GetContractAddresses returns the configured contract addresses.
	GetContractAddresses() *ContractAddresses
}

// ServiceRequestHandler handles incoming service requests from contracts.
type ServiceRequestHandler func(ctx context.Context, request *ServiceRequest) error

// ServiceRequest represents a request from a user contract via the Gateway.
type ServiceRequest struct {
	// RequestID is the unique identifier for this request
	RequestID string `json:"request_id"`

	// Requester is the script hash of the requesting contract
	Requester string `json:"requester"`

	// CallbackContract is where to send the result
	CallbackContract string `json:"callback_contract"`

	// CallbackMethod is the method to call with the result
	CallbackMethod string `json:"callback_method"`

	// Payload contains service-specific request data
	Payload []byte `json:"payload"`

	// GasDeposit is the amount of GAS deposited for this request
	GasDeposit int64 `json:"gas_deposit"`

	// CreatedAt is when the request was created
	CreatedAt time.Time `json:"created_at"`

	// TxHash is the transaction hash that created this request
	TxHash string `json:"tx_hash"`

	// BlockHeight is the block height when request was created
	BlockHeight uint32 `json:"block_height"`
}

// ServiceResponse represents a response to send back via the Gateway.
type ServiceResponse struct {
	// RequestID matches the original request
	RequestID string `json:"request_id"`

	// Success indicates if the service call succeeded
	Success bool `json:"success"`

	// Result contains the service response data
	Result []byte `json:"result"`

	// Error contains error message if failed
	Error string `json:"error,omitempty"`

	// GasUsed is the amount of GAS consumed
	GasUsed int64 `json:"gas_used"`

	// Signature is the TEE signature over the response
	Signature []byte `json:"signature"`
}

// PriceUpdate represents a price update for the DataFeeds contract.
type PriceUpdate struct {
	// FeedID identifies the price feed
	FeedID string `json:"feed_id"`

	// Price is the aggregated price (scaled by decimals)
	Price int64 `json:"price"`

	// Decimals is the number of decimal places
	Decimals uint8 `json:"decimals"`

	// Timestamp is when the price was determined
	Timestamp time.Time `json:"timestamp"`

	// Signature is the TEE signature over the price data
	Signature []byte `json:"signature"`
}

// AutomationTrigger represents an automation trigger configuration.
type AutomationTrigger struct {
	// TriggerID is the unique identifier
	TriggerID string `json:"trigger_id"`

	// Owner is the script hash of the owner contract
	Owner string `json:"owner"`

	// Type is the trigger type (cron, interval, event, once)
	Type TriggerType `json:"type"`

	// Target is the contract to call when triggered
	Target string `json:"target"`

	// Method is the method to call
	Method string `json:"method"`

	// Args are the arguments to pass
	Args []byte `json:"args"`

	// Schedule is the cron expression or interval
	Schedule string `json:"schedule"`

	// GasLimit is the maximum gas to use per execution
	GasLimit int64 `json:"gas_limit"`

	// Active indicates if the trigger is active
	Active bool `json:"active"`

	// LastExecuted is when the trigger last fired
	LastExecuted *time.Time `json:"last_executed,omitempty"`

	// NextExecution is when the trigger will next fire
	NextExecution *time.Time `json:"next_execution,omitempty"`

	// ExecutionCount is the number of times executed
	ExecutionCount uint64 `json:"execution_count"`
}

// TriggerType identifies the type of automation trigger.
type TriggerType uint8

const (
	TriggerTypeCron     TriggerType = 0 // Cron-based periodic trigger
	TriggerTypeInterval TriggerType = 1 // Fixed interval trigger
	TriggerTypeEvent    TriggerType = 2 // Event-based trigger
	TriggerTypeOnce     TriggerType = 3 // One-time trigger
)

// ContractAddresses holds the deployed contract addresses.
type ContractAddresses struct {
	Gateway    string `json:"gateway"`
	Oracle     string `json:"oracle"`
	VRF        string `json:"vrf"`
	Secrets    string `json:"secrets"`
	GasBank    string `json:"gasbank"`
	DataFeeds  string `json:"datafeeds"`
	Automation string `json:"automation"`
}
