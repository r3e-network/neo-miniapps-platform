// Package core implements the MarbleRun-based Coordinator for Neo Service Layer.
// The Coordinator is the control plane that verifies Marble integrity,
// manages secrets, and orchestrates the confidential microservices mesh.
package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/manifest"
)

// State represents the Coordinator's lifecycle state.
type State int

const (
	// StateUninitialized - Coordinator not yet started
	StateUninitialized State = iota
	// StateRecovery - Waiting for recovery keys
	StateRecovery
	// StateAcceptingManifest - Ready to accept manifest
	StateAcceptingManifest
	// StateAcceptingMarbles - Manifest set, accepting activations
	StateAcceptingMarbles
)

func (s State) String() string {
	switch s {
	case StateUninitialized:
		return "uninitialized"
	case StateRecovery:
		return "recovery"
	case StateAcceptingManifest:
		return "accepting-manifest"
	case StateAcceptingMarbles:
		return "accepting-marbles"
	default:
		return "unknown"
	}
}

// Coordinator is the MarbleRun-based control plane.
type Coordinator struct {
	mu sync.RWMutex

	// Configuration
	cfg Config

	// State
	state    State
	manifest *manifest.Manifest

	// Managers
	secretMgr   *SecretManager
	tlsMgr      *TLSManager
	recoveryMgr *RecoveryManager
	quoteMgr    *QuoteManager

	// Active marbles
	marbles map[string]*MarbleInstance

	// Metrics
	stats Stats
}

// Config holds Coordinator configuration.
type Config struct {
	// SealMode determines how state is sealed
	SealMode SealMode

	// DNSNames for Coordinator's TLS certificate
	DNSNames []string

	// RecoveryThreshold is minimum keys needed for recovery
	RecoveryThreshold int

	// SupabaseURL for state persistence
	SupabaseURL string

	// SupabaseKey for authentication
	SupabaseKey string

	// SimulationMode enables SGX simulation
	SimulationMode bool
}

// SealMode determines how Coordinator state is sealed.
type SealMode string

const (
	SealModeProductKey SealMode = "ProductKey"
	SealModeUniqueKey  SealMode = "UniqueKey"
	SealModeDisabled   SealMode = "Disabled"
)

// Stats holds Coordinator statistics.
type Stats struct {
	ActivationCount   uint64    `json:"activation_count"`
	FailedActivations uint64    `json:"failed_activations"`
	LastActivation    time.Time `json:"last_activation,omitempty"`
	StartTime         time.Time `json:"start_time"`
}

// MarbleInstance represents an active Marble.
type MarbleInstance struct {
	Name        string    `json:"name"`
	UUID        string    `json:"uuid"`
	Package     string    `json:"package"`
	ActivatedAt time.Time `json:"activated_at"`
	LastSeen    time.Time `json:"last_seen"`
}

// New creates a new Coordinator.
func New(cfg Config) (*Coordinator, error) {
	if cfg.SealMode == "" {
		cfg.SealMode = SealModeProductKey
	}

	c := &Coordinator{
		cfg:     cfg,
		state:   StateUninitialized,
		marbles: make(map[string]*MarbleInstance),
		stats: Stats{
			StartTime: time.Now(),
		},
	}

	return c, nil
}

// Start initializes and starts the Coordinator.
func (c *Coordinator) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != StateUninitialized {
		return fmt.Errorf("coordinator already started (state: %s)", c.state)
	}

	// Initialize managers
	var err error

	c.secretMgr, err = NewSecretManager(c.cfg.SimulationMode)
	if err != nil {
		return fmt.Errorf("create secret manager: %w", err)
	}

	c.tlsMgr, err = NewTLSManager(c.cfg.DNSNames)
	if err != nil {
		return fmt.Errorf("create TLS manager: %w", err)
	}

	c.recoveryMgr = NewRecoveryManager(c.cfg.RecoveryThreshold)

	c.quoteMgr, err = NewQuoteManager(c.cfg.SimulationMode)
	if err != nil {
		return fmt.Errorf("create quote manager: %w", err)
	}

	// Try to recover sealed state
	if c.cfg.SealMode != SealModeDisabled {
		if err := c.tryRecoverState(ctx); err != nil {
			// No sealed state, start fresh
			c.state = StateAcceptingManifest
		} else {
			c.state = StateAcceptingMarbles
		}
	} else {
		c.state = StateAcceptingManifest
	}

	return nil
}

// Stop shuts down the Coordinator.
func (c *Coordinator) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Seal state if enabled
	if c.cfg.SealMode != SealModeDisabled && c.manifest != nil {
		if err := c.sealState(ctx); err != nil {
			return fmt.Errorf("seal state: %w", err)
		}
	}

	// Zero sensitive data
	if c.secretMgr != nil {
		c.secretMgr.Zero()
	}
	if c.tlsMgr != nil {
		c.tlsMgr.Zero()
	}

	c.state = StateUninitialized
	return nil
}

// =============================================================================
// Manifest Management
// =============================================================================

// SetManifest sets the Coordinator's manifest.
func (c *Coordinator) SetManifest(ctx context.Context, m *manifest.Manifest) (*SetManifestResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != StateAcceptingManifest {
		return nil, fmt.Errorf("cannot set manifest in state %s", c.state)
	}

	// Validate manifest
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	// Set recovery keys
	if len(m.RecoveryKeys) > 0 {
		if err := c.recoveryMgr.SetRecoveryKeys(m.RecoveryKeys); err != nil {
			return nil, fmt.Errorf("set recovery keys: %w", err)
		}
	}

	// Generate secrets
	for name, def := range m.Secrets {
		if !def.UserDefined {
			if err := c.secretMgr.GenerateSecret(ctx, name, &def); err != nil {
				return nil, fmt.Errorf("generate secret %q: %w", name, err)
			}
		}
	}

	// Initialize TLS with manifest config
	if err := c.tlsMgr.Initialize(ctx, &m.TLS); err != nil {
		return nil, fmt.Errorf("initialize TLS: %w", err)
	}

	c.manifest = m
	c.state = StateAcceptingMarbles

	// Generate recovery data
	var recoveryData map[string][]byte
	if c.recoveryMgr.HasRecoveryKeys() {
		sealingKey := c.secretMgr.GetSealingKey()
		var err error
		recoveryData, err = c.recoveryMgr.GenerateRecoveryData(sealingKey)
		if err != nil {
			return nil, fmt.Errorf("generate recovery data: %w", err)
		}
	}

	return &SetManifestResponse{
		RecoverySecrets: recoveryData,
	}, nil
}

// SetManifestResponse is returned after setting manifest.
type SetManifestResponse struct {
	RecoverySecrets map[string][]byte `json:"recovery_secrets,omitempty"`
}

// GetManifest returns the current manifest.
func (c *Coordinator) GetManifest() (*manifest.Manifest, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.manifest == nil {
		return nil, ""
	}

	return c.manifest, c.manifestFingerprint()
}

func (c *Coordinator) manifestFingerprint() string {
	if c.manifest == nil {
		return ""
	}
	data, _ := json.Marshal(c.manifest)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// =============================================================================
// Marble Activation
// =============================================================================

// ActivationRequest is sent by a Marble to activate.
type ActivationRequest struct {
	MarbleType string `json:"marble_type"`
	UUID       string `json:"uuid"`
	Quote      []byte `json:"quote"`
	CSRPem     []byte `json:"csr_pem,omitempty"`
}

// ActivationResponse is returned to an activated Marble.
type ActivationResponse struct {
	Secrets     map[string][]byte `json:"secrets"`
	Env         map[string]string `json:"env"`
	Files       map[string][]byte `json:"files"`
	Certificate []byte            `json:"certificate"`
	PrivateKey  []byte            `json:"private_key"`
	RootCA      []byte            `json:"root_ca"`
}

// Activate activates a Marble after verification.
func (c *Coordinator) Activate(ctx context.Context, req *ActivationRequest) (*ActivationResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != StateAcceptingMarbles {
		return nil, fmt.Errorf("not accepting marbles (state: %s)", c.state)
	}

	// Get marble definition
	marble, ok := c.manifest.Marbles[req.MarbleType]
	if !ok {
		c.stats.FailedActivations++
		return nil, fmt.Errorf("unknown marble type: %s", req.MarbleType)
	}

	// Check max activations
	if marble.MaxActivations > 0 {
		count := c.countActiveMarbles(req.MarbleType)
		if count >= marble.MaxActivations {
			c.stats.FailedActivations++
			return nil, fmt.Errorf("max activations reached for %s", req.MarbleType)
		}
	}

	// Verify quote against package
	pkg, ok := c.manifest.Packages[marble.Package]
	if !ok {
		c.stats.FailedActivations++
		return nil, fmt.Errorf("unknown package: %s", marble.Package)
	}

	if err := c.quoteMgr.VerifyQuote(req.Quote, &pkg); err != nil {
		c.stats.FailedActivations++
		return nil, fmt.Errorf("quote verification failed: %w", err)
	}

	// Collect secrets for this marble
	secretNames := c.manifest.GetMarbleSecrets(req.MarbleType)
	secrets := make(map[string][]byte, len(secretNames))
	for _, name := range secretNames {
		secret, err := c.secretMgr.GetSecret(ctx, name)
		if err != nil {
			c.stats.FailedActivations++
			return nil, fmt.Errorf("get secret %q: %w", name, err)
		}
		secrets[name] = secret
	}

	// Process environment variables
	env := make(map[string]string, len(marble.Parameters.Env))
	for k, v := range marble.Parameters.Env {
		env[k] = substituteSecrets(v, secrets)
	}

	// Process files
	files := make(map[string][]byte, len(marble.Parameters.Files))
	for path, content := range marble.Parameters.Files {
		files[path] = []byte(substituteSecrets(content, secrets))
	}

	// Issue TLS certificate
	var cert, key, rootCA []byte
	if len(marble.TLS.Incoming) > 0 || len(marble.TLS.Outgoing) > 0 {
		var err error
		cert, key, rootCA, err = c.tlsMgr.IssueCertificate(ctx, req.MarbleType, req.UUID)
		if err != nil {
			c.stats.FailedActivations++
			return nil, fmt.Errorf("issue certificate: %w", err)
		}
	}

	// Register marble instance
	instance := &MarbleInstance{
		Name:        req.MarbleType,
		UUID:        req.UUID,
		Package:     marble.Package,
		ActivatedAt: time.Now(),
		LastSeen:    time.Now(),
	}
	c.marbles[req.UUID] = instance

	c.stats.ActivationCount++
	c.stats.LastActivation = time.Now()

	return &ActivationResponse{
		Secrets:     secrets,
		Env:         env,
		Files:       files,
		Certificate: cert,
		PrivateKey:  key,
		RootCA:      rootCA,
	}, nil
}

// Deactivate removes a Marble from the active set.
func (c *Coordinator) Deactivate(ctx context.Context, uuid string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.marbles[uuid]; !ok {
		return fmt.Errorf("marble not found: %s", uuid)
	}

	delete(c.marbles, uuid)
	return nil
}

// =============================================================================
// Secrets Management
// =============================================================================

// SetSecret sets a user-defined secret.
func (c *Coordinator) SetSecret(ctx context.Context, name string, value []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.manifest == nil {
		return fmt.Errorf("manifest not set")
	}

	// Verify secret is user-defined
	def, ok := c.manifest.Secrets[name]
	if !ok {
		return fmt.Errorf("unknown secret: %s", name)
	}
	if !def.UserDefined {
		return fmt.Errorf("secret %s is not user-defined", name)
	}

	return c.secretMgr.SetUserSecret(ctx, name, value)
}

// GetSecrets retrieves secrets by names.
func (c *Coordinator) GetSecrets(ctx context.Context, names []string) (map[string][]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string][]byte, len(names))
	for _, name := range names {
		secret, err := c.secretMgr.GetSecret(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("get secret %q: %w", name, err)
		}
		result[name] = secret
	}

	return result, nil
}

// =============================================================================
// Recovery
// =============================================================================

// Recover recovers the Coordinator using provided key shares.
func (c *Coordinator) Recover(ctx context.Context, shares map[string][]byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != StateRecovery {
		return fmt.Errorf("not in recovery state")
	}

	sealingKey, err := c.recoveryMgr.RecoverSealingKey(shares)
	if err != nil {
		return fmt.Errorf("recover sealing key: %w", err)
	}

	if err := c.unsealState(ctx, sealingKey); err != nil {
		return fmt.Errorf("unseal state: %w", err)
	}

	c.state = StateAcceptingMarbles
	return nil
}

// =============================================================================
// Status
// =============================================================================

// Status returns the Coordinator's status.
type Status struct {
	State           string           `json:"state"`
	ManifestHash    string           `json:"manifest_hash,omitempty"`
	ActiveMarbles   int              `json:"active_marbles"`
	Stats           Stats            `json:"stats"`
	RecoveryKeyHolders []string      `json:"recovery_key_holders,omitempty"`
}

// GetStatus returns the Coordinator's status.
func (c *Coordinator) GetStatus() *Status {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := &Status{
		State:         c.state.String(),
		ActiveMarbles: len(c.marbles),
		Stats:         c.stats,
	}

	if c.manifest != nil {
		status.ManifestHash = c.manifestFingerprint()
	}

	if c.recoveryMgr != nil {
		status.RecoveryKeyHolders = c.recoveryMgr.GetKeyHolders()
	}

	return status
}

// GetQuote returns the Coordinator's attestation quote.
func (c *Coordinator) GetQuote(ctx context.Context, nonce []byte) ([]byte, error) {
	return c.quoteMgr.GenerateQuote(ctx, nonce)
}

// =============================================================================
// Internal Methods
// =============================================================================

func (c *Coordinator) countActiveMarbles(name string) uint {
	var count uint
	for _, m := range c.marbles {
		if m.Name == name {
			count++
		}
	}
	return count
}

func (c *Coordinator) tryRecoverState(ctx context.Context) error {
	// TODO: Implement state recovery from sealed storage
	return fmt.Errorf("no sealed state found")
}

func (c *Coordinator) sealState(ctx context.Context) error {
	// TODO: Implement state sealing
	return nil
}

func (c *Coordinator) unsealState(ctx context.Context, key []byte) error {
	// TODO: Implement state unsealing
	return nil
}

func substituteSecrets(template string, secrets map[string][]byte) string {
	result := template
	for name, value := range secrets {
		// Replace {{ .Secrets.name }} patterns
		patterns := []string{
			"{{ .Secrets." + name + " }}",
			"{{.Secrets." + name + "}}",
			"{{ raw .Secrets." + name + " }}",
			"{{raw .Secrets." + name + "}}",
		}
		for _, p := range patterns {
			for {
				idx := indexOf(result, p)
				if idx == -1 {
					break
				}
				result = result[:idx] + string(value) + result[idx+len(p):]
			}
		}
	}
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
