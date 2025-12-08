// Package vrfmarble provides the Verifiable Random Function service.
//
// Architecture: Request-Callback Pattern
// 1. User contract calls VRF contract's requestRandomness(seed, numWords, callbackGasLimit)
// 2. VRF contract emits RandomnessRequested event
// 3. TEE listens for events, generates VRF proof, and calls fulfillRandomness on user contract
// 4. User contract receives random words in its fulfillRandomness callback
package vrfmarble

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"math/big"
	"sync"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	vrfsupabase "github.com/R3E-Network/service_layer/services/vrf/supabase"
)

// =============================================================================
// Service Definition
// =============================================================================

// Service implements the VRF service.
type Service struct {
	*marble.Service
	mu sync.RWMutex

	privateKey *ecdsa.PrivateKey

	// Service-specific repository
	repo vrfsupabase.RepositoryInterface

	// Chain interaction
	chainClient   *chain.Client
	teeFulfiller  *chain.TEEFulfiller
	eventListener *chain.EventListener

	// Request tracking
	requests         map[string]*VRFRequest // requestID -> request (in-memory cache)
	pendingRequests  chan *VRFRequest       // ephemeral channel; source of truth is DB
	lastProcessedBlk uint64

	// Background workers
	stopCh chan struct{}
}

// Config holds VRF service configuration.
type Config struct {
	Marble        *marble.Marble
	DB            database.RepositoryInterface
	VRFRepo       vrfsupabase.RepositoryInterface
	ChainClient   *chain.Client
	TEEFulfiller  *chain.TEEFulfiller
	EventListener *chain.EventListener
}

// =============================================================================
// Constructor
// =============================================================================

// New creates a new VRF service.
func New(cfg Config) (*Service, error) {
	base := marble.NewService(marble.ServiceConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	s := &Service{
		Service:         base,
		repo:            cfg.VRFRepo,
		chainClient:     cfg.ChainClient,
		teeFulfiller:    cfg.TEEFulfiller,
		eventListener:   cfg.EventListener,
		requests:        make(map[string]*VRFRequest),
		pendingRequests: make(chan *VRFRequest, 100),
		stopCh:          make(chan struct{}),
	}

	// Load VRF private key from Marble secrets
	// UPGRADE SAFETY: VRF_PRIVATE_KEY is injected by MarbleRun Coordinator from
	// manifest-defined secrets. This key remains constant across enclave upgrades
	// (MRENCLAVE changes) as long as the manifest secret is unchanged.
	// The key is NOT derived from SGX sealing keys or enclave identity.
	if keyBytes, ok := cfg.Marble.Secret("VRF_PRIVATE_KEY"); ok {
		privateKey := new(ecdsa.PrivateKey)
		privateKey.Curve = elliptic.P256()
		privateKey.D = new(big.Int).SetBytes(keyBytes)
		privateKey.PublicKey.X, privateKey.PublicKey.Y = privateKey.Curve.ScalarBaseMult(keyBytes)
		s.privateKey = privateKey
	} else {
		// Generate new key pair if not provided
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			return nil, fmt.Errorf("generate key pair: %w", err)
		}
		s.privateKey = keyPair.PrivateKey
	}

	// Register routes
	s.registerRoutes()

	return s, nil
}
