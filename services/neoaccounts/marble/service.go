// Package neoaccounts provides a centralized neoaccounts service for other marbles.
// Private keys never leave this service - other services request accounts and
// submit transactions for signing.
package neoaccountsmarble

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/R3E-Network/service_layer/internal/runtime"
	commonservice "github.com/R3E-Network/service_layer/services/common/service"
	neoaccountssupabase "github.com/R3E-Network/service_layer/services/neoaccounts/supabase"
)

const (
	ServiceID   = "neoaccounts"
	ServiceName = "Account Pool Service"
	Version     = "2.0.0" // Updated for multi-token support

	// Pool configuration
	MinPoolAccounts = 200
	MaxPoolAccounts = 10000
	RotationRate    = 0.1 // 10% of accounts rotated per day
	RotationMinAge  = 24  // Minimum age in hours before rotation

	// Lock timeout - accounts locked longer than this can be force-released
	LockTimeout = 24 * time.Hour
)

// Service implements the NeoAccounts service marble.
type Service struct {
	*commonservice.BaseService
	mu sync.RWMutex

	// Secrets
	masterKey              []byte
	masterPubKey           []byte
	masterKeyHash          []byte
	masterKeyAttestationID string

	// Service-specific repository
	repo neoaccountssupabase.RepositoryInterface

	// Chain interaction (for signing)
	chainClient *chain.Client
}

// Config holds NeoAccounts service configuration.
type Config struct {
	Marble          *marble.Marble
	DB              database.RepositoryInterface
	NeoAccountsRepo neoaccountssupabase.RepositoryInterface
	ChainClient     *chain.Client
}

// New creates a new NeoAccounts service.
func New(cfg Config) (*Service, error) {
	if cfg.Marble == nil {
		return nil, fmt.Errorf("neoaccounts: marble is required")
	}

	strict := runtime.StrictIdentityMode() || cfg.Marble.IsEnclave()

	base := commonservice.NewBase(&commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	s := &Service{
		BaseService: base,
		repo:        cfg.NeoAccountsRepo,
		chainClient: cfg.ChainClient,
	}

	// Load and validate master key material.
	if err := s.loadMasterKey(cfg.Marble); err != nil {
		if strict {
			return nil, err
		}

		s.Logger().WithError(err).Warn("master key not configured; generating ephemeral key (development/testing only)")

		key, keyErr := crypto.GenerateRandomBytes(32)
		if keyErr != nil {
			return nil, fmt.Errorf("neoaccounts: generate fallback master key: %w", keyErr)
		}

		pubKeyCompressed, pubErr := deriveMasterPubKey(key)
		if pubErr != nil {
			return nil, fmt.Errorf("neoaccounts: derive fallback master pubkey: %w", pubErr)
		}

		computedHash := sha256.Sum256(pubKeyCompressed)
		s.masterKey = key
		s.masterPubKey = pubKeyCompressed
		s.masterKeyHash = computedHash[:]
	}

	base.WithHydrate(s.initializePool)
	base.AddTickerWorker(time.Hour, func(ctx context.Context) error {
		s.rotateAccounts(ctx)
		return nil
	}, commonservice.WithTickerWorkerName("account-rotation"))
	base.AddTickerWorker(time.Hour, func(ctx context.Context) error {
		s.cleanupStaleLocks(ctx)
		return nil
	}, commonservice.WithTickerWorkerName("lock-cleanup"))

	base.RegisterStandardRoutes()
	s.registerRoutes()
	return s, nil
}

// initializePool ensures the pool has at least MinPoolAccounts.
func (s *Service) initializePool(ctx context.Context) error {
	accounts, err := s.repo.List(ctx)
	if err != nil {
		// In development/testing mode, skip pool initialization if database is unavailable.
		// In strict identity/SGX mode, fail closed (database is required).
		if runtime.StrictIdentityMode() {
			return err
		}
		if runtime.IsDevelopmentOrTesting() {
			s.Logger().WithContext(ctx).WithError(err).Warn("database unavailable; skipping pool initialization")
			return nil
		}
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
// No balance is set on the account itself - balances are tracked in pool_account_balances.
func (s *Service) createAccount(ctx context.Context) (*neoaccountssupabase.Account, error) {
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

	acc := &neoaccountssupabase.Account{
		ID:         accountID,
		Address:    address,
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

	// Initialize zero balances for known tokens
	for _, tokenType := range []string{TokenTypeGAS, TokenTypeNEO} {
		scriptHash, decimals := neoaccountssupabase.GetDefaultTokenConfig(tokenType)
		if err := s.repo.UpsertBalance(ctx, accountID, tokenType, scriptHash, 0, decimals); err != nil {
			// Log but don't fail - balance can be created on first update.
			s.Logger().WithContext(ctx).WithError(err).WithFields(map[string]interface{}{
				"token_type": tokenType,
				"account_id": accountID,
			}).Warn("failed to initialize account balance")
		}
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
