// Package sdk provides the Marble SDK for Neo Service Layer services.
// Services use this SDK to activate with the Coordinator and receive
// their secrets, TLS certificates, and configuration.
//
// Architecture: MarbleRun + EGo + Supabase + Netlify + Neo N3
//
// Features:
// - Coordinator activation with retry logic
// - SGX quote generation via EGo runtime
// - TLS certificate provisioning
// - Secret management with secure access
// - Supabase client integration
// - Health monitoring and heartbeat
package sdk

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/R3E-Network/service_layer/ego"
	"github.com/google/uuid"
)

// =============================================================================
// Configuration
// =============================================================================

// Config holds Marble configuration.
type Config struct {
	// MarbleType is the marble type from the manifest
	MarbleType string

	// CoordinatorAddr is the Coordinator's address
	CoordinatorAddr string

	// UUID is optional; generated if not provided
	UUID string

	// SimulationMode enables SGX simulation
	SimulationMode bool

	// RetryConfig configures activation retry behavior
	RetryConfig RetryConfig

	// HeartbeatInterval is how often to send heartbeats (0 = disabled)
	HeartbeatInterval time.Duration

	// EgoConfig configures the EGo runtime
	EgoConfig ego.Config
}

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxRetries        int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		CoordinatorAddr: "localhost:4433",
		SimulationMode:  os.Getenv("SIMULATION_MODE") == "true",
		RetryConfig: RetryConfig{
			MaxRetries:        5,
			InitialBackoff:    500 * time.Millisecond,
			MaxBackoff:        30 * time.Second,
			BackoffMultiplier: 2.0,
		},
		HeartbeatInterval: 30 * time.Second,
		EgoConfig:         ego.DefaultConfig(),
	}
}

// =============================================================================
// Marble
// =============================================================================

// Marble represents an activated service instance.
type Marble struct {
	mu sync.RWMutex

	// Identity
	marbleType string
	uuid       string

	// Coordinator connection
	coordinatorAddr string
	httpClient      *http.Client
	retryConfig     RetryConfig

	// EGo runtime
	egoRuntime *ego.Runtime

	// Received from Coordinator
	secrets     map[string][]byte
	env         map[string]string
	files       map[string][]byte
	certificate []byte
	privateKey  []byte
	rootCA      []byte

	// TLS config for service
	tlsConfig *tls.Config

	// State
	activated       bool
	activatedAt     time.Time
	lastHeartbeat   time.Time
	heartbeatCancel context.CancelFunc

	// Metrics
	activationAttempts int64
	heartbeatsSent     int64
	heartbeatsFailed   int64
}

// New creates a new Marble instance.
func New(cfg Config) (*Marble, error) {
	if cfg.MarbleType == "" {
		return nil, fmt.Errorf("MarbleType is required")
	}

	// Apply defaults
	if cfg.CoordinatorAddr == "" {
		cfg.CoordinatorAddr = os.Getenv("COORDINATOR_ADDR")
		if cfg.CoordinatorAddr == "" {
			cfg.CoordinatorAddr = "localhost:4433"
		}
	}
	if cfg.UUID == "" {
		cfg.UUID = uuid.New().String()
	}
	if cfg.RetryConfig.MaxRetries == 0 {
		cfg.RetryConfig = DefaultConfig().RetryConfig
	}

	// Initialize EGo runtime
	cfg.EgoConfig.SimulationMode = cfg.SimulationMode
	egoRuntime := ego.NewWithConfig(cfg.EgoConfig)

	m := &Marble{
		marbleType:      cfg.MarbleType,
		uuid:            cfg.UUID,
		coordinatorAddr: cfg.CoordinatorAddr,
		retryConfig:     cfg.RetryConfig,
		egoRuntime:      egoRuntime,
		secrets:         make(map[string][]byte),
		env:             make(map[string]string),
		files:           make(map[string][]byte),
	}

	// Create HTTP client with connection pooling
	m.httpClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Will be updated after activation
			},
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}

	return m, nil
}

// =============================================================================
// Activation
// =============================================================================

// Activate activates the Marble with the Coordinator.
// Uses exponential backoff retry on failure.
func (m *Marble) Activate(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activated {
		return fmt.Errorf("already activated")
	}

	var lastErr error

	for attempt := 0; attempt <= m.retryConfig.MaxRetries; attempt++ {
		atomic.AddInt64(&m.activationAttempts, 1)

		if attempt > 0 {
			backoff := m.calculateBackoff(attempt)
			log.Printf("[marble:%s] Activation attempt %d failed, retrying in %v...", m.marbleType, attempt, backoff)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := m.doActivate(ctx)
		if err == nil {
			m.activated = true
			m.activatedAt = time.Now()
			log.Printf("[marble:%s] Activated successfully (UUID: %s)", m.marbleType, m.uuid)
			return nil
		}

		lastErr = err
		log.Printf("[marble:%s] Activation error: %v", m.marbleType, err)

		// Don't retry on certain errors
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
	}

	return fmt.Errorf("activation failed after %d attempts: %w", m.retryConfig.MaxRetries+1, lastErr)
}

func (m *Marble) doActivate(ctx context.Context) error {
	// Generate attestation quote using EGo runtime
	reportData := []byte(m.marbleType + ":" + m.uuid)
	quote, err := m.egoRuntime.GenerateQuote(reportData)
	if err != nil {
		return fmt.Errorf("generate quote: %w", err)
	}

	// Get attestation info
	attestInfo, err := m.egoRuntime.GetAttestationInfo()
	if err != nil {
		return fmt.Errorf("get attestation info: %w", err)
	}

	// Send activation request
	req := ActivationRequest{
		MarbleType:      m.marbleType,
		UUID:            m.uuid,
		Quote:           quote,
		UniqueID:        attestInfo.UniqueID,
		SignerID:        attestInfo.SignerID,
		ProductID:       attestInfo.ProductID,
		SecurityVersion: attestInfo.SecurityVersion,
		SimulationMode:  attestInfo.SimulationMode,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("https://%s/api/v2/marble/activate", m.coordinatorAddr)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Marble-Type", m.marbleType)
	httpReq.Header.Set("X-Marble-UUID", m.uuid)

	resp, err := m.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("activation failed: %s - %s", resp.Status, string(body))
	}

	var activationResp ActivationResponse
	if err := json.Unmarshal(body, &activationResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	// Store received data
	m.secrets = activationResp.Secrets
	m.env = activationResp.Env
	m.files = activationResp.Files
	m.certificate = activationResp.Certificate
	m.privateKey = activationResp.PrivateKey
	m.rootCA = activationResp.RootCA

	// Setup TLS config
	if err := m.setupTLS(); err != nil {
		return fmt.Errorf("setup TLS: %w", err)
	}

	// Write files to filesystem
	for path, content := range m.files {
		if err := os.WriteFile(path, content, 0600); err != nil {
			log.Printf("[marble:%s] Warning: failed to write file %s: %v", m.marbleType, path, err)
		}
	}

	// Set environment variables
	for k, v := range m.env {
		os.Setenv(k, v)
	}

	return nil
}

func (m *Marble) calculateBackoff(attempt int) time.Duration {
	backoff := float64(m.retryConfig.InitialBackoff) * math.Pow(m.retryConfig.BackoffMultiplier, float64(attempt-1))

	if backoff > float64(m.retryConfig.MaxBackoff) {
		backoff = float64(m.retryConfig.MaxBackoff)
	}

	// Add jitter (±10%)
	jitter := backoff * 0.1 * (rand.Float64()*2 - 1)
	backoff += jitter

	return time.Duration(backoff)
}

// =============================================================================
// Request/Response Types
// =============================================================================

// ActivationRequest is sent to the Coordinator.
type ActivationRequest struct {
	MarbleType      string `json:"marble_type"`
	UUID            string `json:"uuid"`
	Quote           []byte `json:"quote"`
	CSRPem          []byte `json:"csr_pem,omitempty"`
	UniqueID        string `json:"unique_id,omitempty"`
	SignerID        string `json:"signer_id,omitempty"`
	ProductID       uint16 `json:"product_id,omitempty"`
	SecurityVersion uint16 `json:"security_version,omitempty"`
	SimulationMode  bool   `json:"simulation_mode,omitempty"`
}

// ActivationResponse is received from the Coordinator.
type ActivationResponse struct {
	Secrets     map[string][]byte `json:"secrets"`
	Env         map[string]string `json:"env"`
	Files       map[string][]byte `json:"files"`
	Certificate []byte            `json:"certificate"`
	PrivateKey  []byte            `json:"private_key"`
	RootCA      []byte            `json:"root_ca"`
}

// =============================================================================
// Secret Access
// =============================================================================

// GetSecret returns a secret by name.
func (m *Marble) GetSecret(name string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.activated {
		return nil, fmt.Errorf("not activated")
	}

	secret, ok := m.secrets[name]
	if !ok {
		return nil, fmt.Errorf("secret not found: %s", name)
	}

	// Return a copy to prevent modification
	result := make([]byte, len(secret))
	copy(result, secret)
	return result, nil
}

// GetSecretString returns a secret as a string.
func (m *Marble) GetSecretString(name string) (string, error) {
	secret, err := m.GetSecret(name)
	if err != nil {
		return "", err
	}
	return string(secret), nil
}

// HasSecret checks if a secret exists.
func (m *Marble) HasSecret(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.secrets[name]
	return ok
}

// ListSecrets returns the names of all secrets.
func (m *Marble) ListSecrets() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.secrets))
	for name := range m.secrets {
		names = append(names, name)
	}
	return names
}

// =============================================================================
// Environment Access
// =============================================================================

// GetEnv returns an environment variable.
func (m *Marble) GetEnv(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.env[name]
}

// GetEnvWithDefault returns an environment variable with a default.
func (m *Marble) GetEnvWithDefault(name, defaultValue string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if v, ok := m.env[name]; ok {
		return v
	}
	return defaultValue
}

// =============================================================================
// TLS
// =============================================================================

// TLSConfig returns the TLS configuration for the service.
func (m *Marble) TLSConfig() *tls.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tlsConfig
}

func (m *Marble) setupTLS() error {
	if m.certificate == nil || m.privateKey == nil {
		return nil // No TLS configured
	}

	cert, err := tls.X509KeyPair(m.certificate, m.privateKey)
	if err != nil {
		return fmt.Errorf("parse certificate: %w", err)
	}

	var rootCAs *x509.CertPool
	if m.rootCA != nil {
		rootCAs = x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(m.rootCA) {
			return fmt.Errorf("failed to parse root CA")
		}
	}

	m.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      rootCAs,
		ClientCAs:    rootCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}

	return nil
}

// =============================================================================
// Identity
// =============================================================================

// UUID returns the Marble's UUID.
func (m *Marble) UUID() string {
	return m.uuid
}

// MarbleType returns the Marble's type.
func (m *Marble) MarbleType() string {
	return m.marbleType
}

// IsActivated returns true if the Marble is activated.
func (m *Marble) IsActivated() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activated
}

// ActivatedAt returns when the Marble was activated.
func (m *Marble) ActivatedAt() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activatedAt
}

// =============================================================================
// EGo Runtime Access
// =============================================================================

// EgoRuntime returns the EGo runtime.
func (m *Marble) EgoRuntime() *ego.Runtime {
	return m.egoRuntime
}

// InEnclave returns true if running in an SGX enclave.
func (m *Marble) InEnclave() bool {
	return m.egoRuntime.InEnclave()
}

// IsSimulation returns true if running in simulation mode.
func (m *Marble) IsSimulation() bool {
	return m.egoRuntime.IsSimulation()
}

// Seal seals data using SGX sealing.
func (m *Marble) Seal(data []byte) ([]byte, error) {
	return m.egoRuntime.Seal(data, ego.SealPolicyProduct)
}

// Unseal unseals SGX-sealed data.
func (m *Marble) Unseal(sealedData []byte) ([]byte, error) {
	return m.egoRuntime.Unseal(sealedData)
}

// GenerateQuote generates an SGX quote for attestation.
func (m *Marble) GenerateQuote(data []byte) ([]byte, error) {
	return m.egoRuntime.GenerateQuote(data)
}

// =============================================================================
// Health and Metrics
// =============================================================================

// Health returns health status.
type Health struct {
	Status             string    `json:"status"`
	MarbleType         string    `json:"marble_type"`
	UUID               string    `json:"uuid"`
	Activated          bool      `json:"activated"`
	ActivatedAt        time.Time `json:"activated_at,omitempty"`
	InEnclave          bool      `json:"in_enclave"`
	SimulationMode     bool      `json:"simulation_mode"`
	Uptime             string    `json:"uptime,omitempty"`
	ActivationAttempts int64     `json:"activation_attempts"`
	HeartbeatsSent     int64     `json:"heartbeats_sent"`
	HeartbeatsFailed   int64     `json:"heartbeats_failed"`
}

// Health returns the Marble's health status.
func (m *Marble) Health() Health {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := "healthy"
	if !m.activated {
		status = "not_activated"
	}

	var uptime string
	if m.activated {
		uptime = time.Since(m.activatedAt).String()
	}

	return Health{
		Status:             status,
		MarbleType:         m.marbleType,
		UUID:               m.uuid,
		Activated:          m.activated,
		ActivatedAt:        m.activatedAt,
		InEnclave:          m.egoRuntime.InEnclave(),
		SimulationMode:     m.egoRuntime.IsSimulation(),
		Uptime:             uptime,
		ActivationAttempts: atomic.LoadInt64(&m.activationAttempts),
		HeartbeatsSent:     atomic.LoadInt64(&m.heartbeatsSent),
		HeartbeatsFailed:   atomic.LoadInt64(&m.heartbeatsFailed),
	}
}

// =============================================================================
// Heartbeat
// =============================================================================

// StartHeartbeat starts sending heartbeats to the Coordinator.
func (m *Marble) StartHeartbeat(ctx context.Context, interval time.Duration) {
	m.mu.Lock()
	if m.heartbeatCancel != nil {
		m.mu.Unlock()
		return // Already running
	}

	heartbeatCtx, cancel := context.WithCancel(ctx)
	m.heartbeatCancel = cancel
	m.mu.Unlock()

	go m.heartbeatLoop(heartbeatCtx, interval)
}

// StopHeartbeat stops the heartbeat goroutine.
func (m *Marble) StopHeartbeat() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.heartbeatCancel != nil {
		m.heartbeatCancel()
		m.heartbeatCancel = nil
	}
}

func (m *Marble) heartbeatLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.sendHeartbeat(ctx); err != nil {
				atomic.AddInt64(&m.heartbeatsFailed, 1)
				log.Printf("[marble:%s] Heartbeat failed: %v", m.marbleType, err)
			} else {
				atomic.AddInt64(&m.heartbeatsSent, 1)
				m.mu.Lock()
				m.lastHeartbeat = time.Now()
				m.mu.Unlock()
			}
		}
	}
}

func (m *Marble) sendHeartbeat(ctx context.Context) error {
	url := fmt.Sprintf("https://%s/api/v2/marble/heartbeat", m.coordinatorAddr)

	payload := map[string]any{
		"uuid":        m.uuid,
		"marble_type": m.marbleType,
		"timestamp":   time.Now().Unix(),
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Marble-UUID", m.uuid)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed: %s", resp.Status)
	}

	return nil
}

// =============================================================================
// Supabase Integration
// =============================================================================

// SupabaseClient returns a configured Supabase client.
func (m *Marble) SupabaseClient() (*SupabaseClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.activated {
		return nil, fmt.Errorf("not activated")
	}

	url := m.env["SUPABASE_URL"]
	if url == "" {
		url = os.Getenv("SUPABASE_URL")
	}

	key := string(m.secrets["supabase_service_key"])
	if key == "" {
		key = os.Getenv("SUPABASE_SERVICE_KEY")
		if key == "" {
			key = os.Getenv("SUPABASE_SECRET_KEY")
		}
	}

	if url == "" || key == "" {
		return nil, fmt.Errorf("Supabase credentials not configured")
	}

	return NewSupabaseClient(url, key), nil
}

// SupabaseClient is a simple Supabase REST client.
type SupabaseClient struct {
	url    string
	key    string
	client *http.Client
}

// NewSupabaseClient creates a new Supabase client.
func NewSupabaseClient(url, key string) *SupabaseClient {
	return &SupabaseClient{
		url: url,
		key: key,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// From starts a query builder for a table.
func (c *SupabaseClient) From(table string) *QueryBuilder {
	return &QueryBuilder{
		client: c,
		table:  table,
	}
}

// QueryBuilder builds Supabase queries.
type QueryBuilder struct {
	client  *SupabaseClient
	table   string
	columns string
	filters []string
	order   string
	limit   int
	single  bool
}

// Select specifies columns to select.
func (q *QueryBuilder) Select(columns string) *QueryBuilder {
	q.columns = columns
	return q
}

// Eq adds an equality filter.
func (q *QueryBuilder) Eq(column string, value any) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=eq.%v", column, value))
	return q
}

// Neq adds a not-equal filter.
func (q *QueryBuilder) Neq(column string, value any) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=neq.%v", column, value))
	return q
}

// Order sets the order.
func (q *QueryBuilder) Order(column string, ascending bool) *QueryBuilder {
	dir := "asc"
	if !ascending {
		dir = "desc"
	}
	q.order = fmt.Sprintf("%s.%s", column, dir)
	return q
}

// Limit sets the limit.
func (q *QueryBuilder) Limit(n int) *QueryBuilder {
	q.limit = n
	return q
}

// Single expects a single result.
func (q *QueryBuilder) Single() *QueryBuilder {
	q.single = true
	return q
}

// Execute executes the query.
func (q *QueryBuilder) Execute(ctx context.Context) ([]byte, error) {
	url := fmt.Sprintf("%s/rest/v1/%s", q.client.url, q.table)

	// Build query string
	params := []string{}
	if q.columns != "" {
		params = append(params, "select="+q.columns)
	}
	params = append(params, q.filters...)
	if q.order != "" {
		params = append(params, "order="+q.order)
	}
	if q.limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", q.limit))
	}

	if len(params) > 0 {
		url += "?" + joinParams(params)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	q.client.setHeaders(req)
	if q.single {
		req.Header.Set("Accept", "application/vnd.pgrst.object+json")
	}

	resp, err := q.client.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// ExecuteInsert executes an INSERT operation.
func (q *QueryBuilder) ExecuteInsert(ctx context.Context, data any) ([]byte, error) {
	url := fmt.Sprintf("%s/rest/v1/%s", q.client.url, q.table)

	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	q.client.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	resp, err := q.client.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// ExecuteUpdate executes an UPDATE operation.
func (q *QueryBuilder) ExecuteUpdate(ctx context.Context, data any) ([]byte, error) {
	url := fmt.Sprintf("%s/rest/v1/%s", q.client.url, q.table)

	// Add filters to URL
	if len(q.filters) > 0 {
		url += "?" + joinParams(q.filters)
	}

	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	q.client.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	resp, err := q.client.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *SupabaseClient) setHeaders(req *http.Request) {
	req.Header.Set("apikey", c.key)
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Accept", "application/json")
}

func joinParams(params []string) string {
	result := ""
	for i, p := range params {
		if i > 0 {
			result += "&"
		}
		result += p
	}
	return result
}
