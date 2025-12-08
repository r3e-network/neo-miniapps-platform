// Package accountpool provides a centralized account pool service for other marbles.
// Private keys never leave this service - other services request accounts and
// submit transactions for signing.
package accountpoolmarble

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	accountpoolsupabase "github.com/R3E-Network/service_layer/services/accountpool/supabase"
	"github.com/google/uuid"
)

const (
	ServiceID   = "accountpool"
	ServiceName = "Account Pool Service"
	Version     = "1.0.0"

	// Pool configuration
	MinPoolAccounts = 200
	MaxPoolAccounts = 10000
	RotationRate    = 0.1 // 10% of accounts rotated per day
	RotationMinAge  = 24  // Minimum age in hours before rotation

	// Lock timeout - accounts locked longer than this can be force-released
	LockTimeout = 24 * time.Hour
)

// Service implements the AccountPool service marble.
type Service struct {
	*marble.Service
	mu sync.RWMutex

	// Secrets
	masterKey []byte

	// Service-specific repository
	repo accountpoolsupabase.RepositoryInterface

	// Chain interaction (for signing)
	chainClient *chain.Client

	// Background workers
	stopCh chan struct{}
}

// Config holds AccountPool service configuration.
type Config struct {
	Marble         *marble.Marble
	DB             database.RepositoryInterface
	AccountPoolRepo accountpoolsupabase.RepositoryInterface
	ChainClient    *chain.Client
}

// New creates a new AccountPool service.
func New(cfg Config) (*Service, error) {
	base := marble.NewService(marble.ServiceConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	s := &Service{
		Service:     base,
		repo:        cfg.AccountPoolRepo,
		chainClient: cfg.ChainClient,
		stopCh:      make(chan struct{}),
	}

	// Load master key from Marble secrets
	// UPGRADE SAFETY: POOL_MASTER_KEY is injected by MarbleRun Coordinator from
	// manifest-defined secrets. All account keys are derived from this master key
	// using HKDF without enclave identity, ensuring keys remain stable across upgrades.
	if key, ok := cfg.Marble.Secret("POOL_MASTER_KEY"); ok {
		s.masterKey = key
	}

	s.registerRoutes()
	return s, nil
}


// initializePool ensures the pool has at least MinPoolAccounts.
func (s *Service) initializePool(ctx context.Context) error {
	accounts, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	if len(accounts) >= MaxPoolAccounts {
		return nil
	}

	need := MinPoolAccounts - len(accounts)
	if need < 0 {
		need = 0
	}
	if need > MaxPoolAccounts-len(accounts) {
		need = MaxPoolAccounts - len(accounts)
	}
	for i := 0; i < need; i++ {
		if _, err := s.createAccount(ctx); err != nil {
			return err
		}
	}
	return nil
}

// createAccount creates and persists a new pool account with HD derivation.
func (s *Service) createAccount(ctx context.Context) (*accountpoolsupabase.Account, error) {
	accountID := uuid.New().String()

	derivedKey, err := s.deriveAccountKey(accountID)
	if err != nil {
		return nil, err
	}

	curve := elliptic.P256()
	d := new(big.Int).SetBytes(derivedKey)
	n := new(big.Int).Sub(curve.Params().N, big.NewInt(1))
	d.Mod(d, n)
	d.Add(d, big.NewInt(1)) // ensure non-zero
	priv := &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve}, D: d}
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	pubBytes := crypto.PublicKeyToBytes(&priv.PublicKey)
	scriptHash := crypto.PublicKeyToScriptHash(pubBytes)
	address := crypto.ScriptHashToAddress(scriptHash)

	acc := &accountpoolsupabase.Account{
		ID:         accountID,
		Address:    address,
		Balance:    0,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		TxCount:    0,
		IsRetiring: false,
		LockedBy:   "",
		LockedAt:   time.Time{},
	}
	if err := s.repo.Create(ctx, acc); err != nil {
		return nil, err
	}

	return acc, nil
}

// deriveAccountKey derives an account's private key from the master key.
// UPGRADE SAFETY: Uses crypto.DeriveKey which derives keys based only on:
//   - masterKey: From MarbleRun injection (manifest-defined, stable across upgrades)
//   - accountID: Business identifier (stable)
//   - "pool-account": Service context (code constant, stable)
//
// NO enclave identity (MRENCLAVE/MRSIGNER) is used in derivation.
func (s *Service) deriveAccountKey(accountID string) ([]byte, error) {
	return crypto.DeriveKey(s.masterKey, []byte(accountID), "pool-account", 32)
}

// getPrivateKey derives and returns the private key for an account.
// This is internal only - private keys never leave this service.
func (s *Service) getPrivateKey(accountID string) (*ecdsa.PrivateKey, error) {
	derivedKey, err := s.deriveAccountKey(accountID)
	if err != nil {
		return nil, err
	}

	curve := elliptic.P256()
	d := new(big.Int).SetBytes(derivedKey)
	n := new(big.Int).Sub(curve.Params().N, big.NewInt(1))
	d.Mod(d, n)
	d.Add(d, big.NewInt(1))
	priv := &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve}, D: d}
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	return priv, nil
}
