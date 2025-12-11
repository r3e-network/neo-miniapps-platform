// Package neorand provides the NeoRand verifiable random number generation service.
//
// Architecture: Request-Callback Pattern
// 1. User contract calls NeoRand contract's requestRandomness(seed, numWords, callbackGasLimit)
// 2. NeoRand contract emits RandomnessRequested event
// 3. TEE listens for events, generates VRF proof, and calls fulfillRandomness on user contract
// 4. User contract receives random words in its fulfillRandomness callback
package neorand

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"math/big"
	"sync"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	commonservice "github.com/R3E-Network/service_layer/services/common/service"
	neorandsupabase "github.com/R3E-Network/service_layer/services/neorand/supabase"
)

// =============================================================================
// Service Constants
// =============================================================================

const (
	ServiceID   = "neorand"
	ServiceName = "NeoRand Service"
	Version     = "2.0.0"
)

// =============================================================================
// Service Definition
// =============================================================================

// Service implements the VRF service.
type Service struct {
	*commonservice.BaseService
	mu sync.RWMutex

	privateKey *ecdsa.PrivateKey

	// Service-specific repository
	repo neorandsupabase.RepositoryInterface

	// Chain interaction
	chainClient   *chain.Client
	teeFulfiller  *chain.TEEFulfiller
	eventListener *chain.EventListener

	// Request tracking
	requests         map[string]*NeoRandRequest // requestID -> request (in-memory cache)
	pendingRequests  chan *NeoRandRequest       // ephemeral channel; source of truth is DB
	lastProcessedBlk uint64
}

// Config holds NeoRand service configuration.
type Config struct {
	Marble        *marble.Marble
	DB            database.RepositoryInterface
	NeoRandRepo   neorandsupabase.RepositoryInterface
	ChainClient   *chain.Client
	TEEFulfiller  *chain.TEEFulfiller
	EventListener *chain.EventListener
}

// =============================================================================
// Constructor
// =============================================================================

// New creates a new NeoRand service.
func New(cfg Config) (*Service, error) {
	base := commonservice.NewBase(commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	s := &Service{
		BaseService:     base,
		repo:            cfg.NeoRandRepo,
		chainClient:     cfg.ChainClient,
		teeFulfiller:    cfg.TEEFulfiller,
		eventListener:   cfg.EventListener,
		requests:        make(map[string]*NeoRandRequest),
		pendingRequests: make(chan *NeoRandRequest, 100),
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

	base.WithHydrate(s.hydratePendingRequests)
	base.AddWorker(s.runEventListener)
	base.AddWorker(s.runRequestFulfiller)

	// Register routes
	s.registerRoutes()

	return s, nil
}

// hydratePendingRequests loads pending requests from the repository into memory/queues.
func (s *Service) hydratePendingRequests(ctx context.Context) error {
	if s.repo == nil {
		return nil
	}
	pending, err := s.repo.ListByStatus(ctx, StatusPending)
	if err != nil {
		return err
	}
	for i := range pending {
		req := neorandReqFromRecord(&pending[i])
		s.requests[req.RequestID] = req
		select {
		case s.pendingRequests <- req:
		default:
		}
	}
	return nil
}
