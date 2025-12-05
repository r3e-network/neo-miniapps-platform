// Package os provides the ServiceOS abstraction layer.
// ServiceOS is like Android OS - it abstracts TEE details and provides
// capability-based access control for services.
package os

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/tee"
)

// ServiceContext implements ServiceOS for a specific service.
// It provides capability-checked access to all platform APIs.
// Note: Uses LegacyManifest for per-service configuration.
// The new Manifest type is for mesh-wide configuration (MarbleRun-style).
type ServiceContext struct {
	mu sync.RWMutex

	manifest     *LegacyManifest
	trustRoot    *tee.TrustRoot
	capabilities map[Capability]bool
	ctx          context.Context
	cancel       context.CancelFunc
	logger       Logger

	// API implementations (lazy initialized)
	secretsAPI     *secretsAPIImpl
	networkAPI     *networkAPIImpl
	keysAPI        *keysAPIImpl
	computeAPI     *computeAPIImpl
	storageAPI     *storageAPIImpl
	eventsAPI      *eventsAPIImpl
	attestationAPI *attestationAPIImpl
	neoAPI         *neoAPIImpl
	databaseAPI    *databaseAPIImpl
	chainAPI       *chainAPIImpl
	configAPI      *configAPIImpl
	metricsAPI     *metricsAPIImpl
	schedulerAPI   *schedulerAPIImpl
	cacheAPI       *cacheAPIImpl
	authAPI        *authAPIImpl
	queueAPI       *queueAPIImpl
	lockAPI        *lockAPIImpl
	contractAPI    *contractAPIImpl
}

// NewServiceContext creates a new ServiceContext for a service.
// Uses LegacyManifest for per-service configuration.
func NewServiceContext(manifest *LegacyManifest, trustRoot *tee.TrustRoot, logger Logger) (*ServiceContext, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest is required")
	}
	if trustRoot == nil {
		return nil, fmt.Errorf("trust_root is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Build capability set from manifest
	caps := make(map[Capability]bool)
	for _, cap := range manifest.RequiredCapabilities {
		caps[cap] = true
	}
	for _, cap := range manifest.OptionalCapabilities {
		caps[cap] = true
	}

	if logger == nil {
		logger = newDefaultLogger(manifest.ServiceID)
	}

	return &ServiceContext{
		manifest:     manifest,
		trustRoot:    trustRoot,
		capabilities: caps,
		ctx:          ctx,
		cancel:       cancel,
		logger:       logger,
	}, nil
}

// =============================================================================
// Identity Methods
// =============================================================================

// ServiceID returns the service identifier.
func (c *ServiceContext) ServiceID() string {
	return c.manifest.ServiceID
}

// Manifest returns the service manifest (legacy format).
func (c *ServiceContext) Manifest() *LegacyManifest {
	return c.manifest
}

// =============================================================================
// Capability Methods
// =============================================================================

// HasCapability checks if the service has a capability.
func (c *ServiceContext) HasCapability(cap Capability) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capabilities[cap]
}

// RequireCapability returns an error if capability is not granted.
func (c *ServiceContext) RequireCapability(cap Capability) error {
	if !c.HasCapability(cap) {
		return ErrCapabilityDenied(cap)
	}
	return nil
}

// =============================================================================
// Core TEE-backed APIs
// =============================================================================

// Secrets returns the SecretsAPI.
func (c *ServiceContext) Secrets() SecretsAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.secretsAPI == nil {
		c.secretsAPI = newSecretsAPI(c, c.trustRoot.Vault(), c.manifest.ServiceID)
	}
	return c.secretsAPI
}

// Network returns the NetworkAPI.
func (c *ServiceContext) Network() NetworkAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.networkAPI == nil {
		c.networkAPI = newNetworkAPI(c, c.trustRoot.Network(), c.manifest.ServiceID, c.manifest.AllowedHosts)
	}
	return c.networkAPI
}

// Keys returns the KeysAPI.
func (c *ServiceContext) Keys() KeysAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.keysAPI == nil {
		c.keysAPI = newKeysAPI(c, c.trustRoot.Keys(), c.manifest.ServiceID)
	}
	return c.keysAPI
}

// Compute returns the ComputeAPI.
func (c *ServiceContext) Compute() ComputeAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.computeAPI == nil {
		c.computeAPI = newComputeAPI(c, c.trustRoot.Compute(), c.manifest.ServiceID)
	}
	return c.computeAPI
}

// Storage returns the StorageAPI.
func (c *ServiceContext) Storage() StorageAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.storageAPI == nil {
		c.storageAPI = newStorageAPI(c, c.trustRoot.Vault(), c.manifest.ServiceID)
	}
	return c.storageAPI
}

// Events returns the EventsAPI.
func (c *ServiceContext) Events() EventsAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.eventsAPI == nil {
		c.eventsAPI = newEventsAPI(c, c.manifest.ServiceID)
	}
	return c.eventsAPI
}

// Attestation returns the AttestationAPI.
func (c *ServiceContext) Attestation() AttestationAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.attestationAPI == nil {
		c.attestationAPI = newAttestationAPI(c, c.trustRoot.Attestation())
	}
	return c.attestationAPI
}

// =============================================================================
// Blockchain APIs
// =============================================================================

// Neo returns the NeoAPI.
func (c *ServiceContext) Neo() NeoAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.neoAPI == nil {
		c.neoAPI = newNeoAPI(c, c.trustRoot.Neo(), c.manifest.ServiceID)
	}
	return c.neoAPI
}

// Chain returns the ChainAPI.
func (c *ServiceContext) Chain() ChainAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.chainAPI == nil {
		c.chainAPI = newChainAPI(c, c.manifest.ServiceID)
	}
	return c.chainAPI
}

// =============================================================================
// Infrastructure APIs
// =============================================================================

// Database returns the DatabaseAPI.
func (c *ServiceContext) Database() DatabaseAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.databaseAPI == nil {
		c.databaseAPI = newDatabaseAPI(c, c.manifest.ServiceID)
	}
	return c.databaseAPI
}

// Cache returns the CacheAPI.
func (c *ServiceContext) Cache() CacheAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cacheAPI == nil {
		c.cacheAPI = newCacheAPI(c, c.manifest.ServiceID)
	}
	return c.cacheAPI
}

// Queue returns the QueueAPI.
func (c *ServiceContext) Queue() QueueAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.queueAPI == nil {
		c.queueAPI = newQueueAPI(c, c.manifest.ServiceID)
	}
	return c.queueAPI
}

// =============================================================================
// Service Management APIs
// =============================================================================

// Config returns the ConfigAPI.
func (c *ServiceContext) Config() ConfigAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.configAPI == nil {
		c.configAPI = newConfigAPI(c, c.manifest.ServiceID)
	}
	return c.configAPI
}

// Metrics returns the MetricsAPI.
func (c *ServiceContext) Metrics() MetricsAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.metricsAPI == nil {
		c.metricsAPI = newMetricsAPI(c, c.manifest.ServiceID)
	}
	return c.metricsAPI
}

// Scheduler returns the SchedulerAPI.
func (c *ServiceContext) Scheduler() SchedulerAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.schedulerAPI == nil {
		c.schedulerAPI = newSchedulerAPI(c, c.manifest.ServiceID)
	}
	return c.schedulerAPI
}

// Auth returns the AuthAPI.
func (c *ServiceContext) Auth() AuthAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.authAPI == nil {
		c.authAPI = newAuthAPI(c, c.manifest.ServiceID)
	}
	return c.authAPI
}

// Lock returns the LockAPI.
func (c *ServiceContext) Lock() LockAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lockAPI == nil {
		c.lockAPI = newLockAPI(c, c.manifest.ServiceID)
	}
	return c.lockAPI
}

// =============================================================================
// Contract APIs
// =============================================================================

// Contract returns the ContractAPI.
func (c *ServiceContext) Contract() ContractAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.contractAPI == nil {
		c.contractAPI = newContractAPI(c, c.manifest.ServiceID)
	}
	return c.contractAPI
}

// =============================================================================
// Lifecycle Methods
// =============================================================================

// Context returns the service context.
func (c *ServiceContext) Context() context.Context {
	return c.ctx
}

// Logger returns the service logger.
func (c *ServiceContext) Logger() Logger {
	return c.logger
}

// Close closes the service context and releases resources.
func (c *ServiceContext) Close() error {
	c.cancel()
	return nil
}
