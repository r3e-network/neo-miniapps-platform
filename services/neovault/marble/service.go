// Package neovault provides privacy-preserving transaction mixing service.
//
// Architecture: Off-Chain Mixing with TEE Proofs + On-Chain Dispute Only
//
// Privacy-First Fee Model:
// Users NEVER connect to any known service layer account. Fees are collected
// by sending less tokens to the target address (fee deducted from delivery).
//
// Flow:
//  1. User requests mix via API → NeoVault returns deposit address (pool account)
//  2. NeoVault returns RequestProof to user (for 7-day dispute claim)
//  3. User deposits DIRECTLY to pool account on-chain (NO gasbank, NO service layer account)
//  4. NeoVault verifies deposit on-chain via event listener
//  5. NeoVault processes off-chain (pool account mixing)
//  6. NeoVault delivers (TotalAmount - ServiceFee) to target addresses
//  7. ServiceFee remains distributed in pool (no explicit fee collection address)
//  8. Normal path: Tokens delivered, user happy, nothing on-chain
//  9. Dispute path: User submits dispute → NeoVault submits CompletionProof on-chain
//
// Privacy Guarantees:
// - User deposits to anonymous pool account (not linked to service layer)
// - Fee is implicit (user receives less, no separate fee transaction)
// - No on-chain link between user and service layer
//
// Security:
// - NeoAccounts service owns HD-derived pool accounts; neovault only locks/uses them
// - RequestProof = Hash256(request) + TEE signature (user can verify)
// - CompletionProof = Hash256(outputs) + TEE signature (dispute evidence)
// - Compliance limits: ≤10,000 per request, ≤100,000 total pool
package neovaultmarble

import (
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	commonservice "github.com/R3E-Network/service_layer/services/common/service"
	neovaultsupabase "github.com/R3E-Network/service_layer/services/neovault/supabase"
)

const (
	ServiceID   = "neovault"
	ServiceName = "NeoVault Service"
	Version     = "3.4.0"

	// Default token for backward compatibility
	DefaultToken = "GAS"

	// Mixing configuration
	MinMixingTxPerMinute = 5
	MaxMixingTxPerMinute = 20

	// Dispute grace period
	DisputeGracePeriod = 7 * 24 * time.Hour
)

// TokenConfig holds per-token configuration (limits and fees only).
// Pool accounts are shared across all tokens.
type TokenConfig struct {
	TokenType        string  `json:"token_type"`
	ScriptHash       string  `json:"script_hash"` // NEP-17 contract hash
	MinTxAmount      int64   `json:"min_tx_amount"`
	MaxTxAmount      int64   `json:"max_tx_amount"`
	MaxRequestAmount int64   `json:"max_request_amount"`
	MaxPoolBalance   int64   `json:"max_pool_balance"`
	ServiceFeeRate   float64 `json:"service_fee_rate"`
}

// DefaultTokenConfigs returns the default per-token configurations.
func DefaultTokenConfigs() map[string]*TokenConfig {
	return map[string]*TokenConfig{
		"GAS": {
			TokenType:        "GAS",
			ScriptHash:       "0xd2a4cff31913016155e38e474a2c06d08be276cf", // GAS on Neo N3
			MinTxAmount:      100000,
			MaxTxAmount:      100000000,
			MaxRequestAmount: 1000000000000,
			MaxPoolBalance:   10000000000000,
			ServiceFeeRate:   0.005,
		},
		"NEO": {
			TokenType:        "NEO",
			ScriptHash:       "0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5", // NEO on Neo N3
			MinTxAmount:      1,                                            // NEO is indivisible
			MaxTxAmount:      1000,
			MaxRequestAmount: 100000,
			MaxPoolBalance:   1000000,
			ServiceFeeRate:   0.005,
		},
	}
}

// Service implements the NeoVault service.
type Service struct {
	*commonservice.BaseService
	mu sync.RWMutex

	// Secrets (for TEE signing of requests/proofs only)
	masterKey []byte

	// Service-specific repository
	repo neovaultsupabase.RepositoryInterface

	// Per-token configuration
	tokenConfigs map[string]*TokenConfig

	// Account pool client (for requesting/releasing accounts)
	neoAccountsURL string

	// Fee collection: master account address to receive collected fees
	// Fees are collected from random pool accounts after mixing completes
	feeCollectionAddress string

	// Chain interaction
	chainClient  *chain.Client
	teeFulfiller *chain.TEEFulfiller
	gateway      *chain.GatewayContract
}

// GetTokenConfig returns the configuration for a specific token.
func (s *Service) GetTokenConfig(tokenType string) *TokenConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if cfg, ok := s.tokenConfigs[tokenType]; ok {
		return cfg
	}
	return s.tokenConfigs[DefaultToken]
}

// GetSupportedTokens returns all supported token types.
func (s *Service) GetSupportedTokens() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tokens := make([]string, 0, len(s.tokenConfigs))
	for t := range s.tokenConfigs {
		tokens = append(tokens, t)
	}
	return tokens
}

// Config holds NeoVault service configuration.
type Config struct {
	Marble               *marble.Marble
	DB                   database.RepositoryInterface
	NeoVaultRepo            neovaultsupabase.RepositoryInterface
	ChainClient          *chain.Client
	TEEFulfiller         *chain.TEEFulfiller
	Gateway              *chain.GatewayContract
	TokenConfigs         map[string]*TokenConfig // Optional custom token configs
	NeoAccountsURL       string                  // URL for neoaccounts service
	FeeCollectionAddress string                  // Master account address for fee collection
}

// New creates a new NeoVault service.
func New(cfg Config) (*Service, error) {
	base := commonservice.NewBase(commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	// Use provided token configs or defaults
	tokenConfigs := cfg.TokenConfigs
	if tokenConfigs == nil {
		tokenConfigs = DefaultTokenConfigs()
	}

	s := &Service{
		BaseService:          base,
		repo:                 cfg.NeoVaultRepo,
		tokenConfigs:         tokenConfigs,
		neoAccountsURL:       cfg.NeoAccountsURL,
		feeCollectionAddress: cfg.FeeCollectionAddress,
		chainClient:          cfg.ChainClient,
		teeFulfiller:         cfg.TEEFulfiller,
		gateway:              cfg.Gateway,
	}

	// Load neovault master key from Marble secrets
	// UPGRADE SAFETY: NEOVAULT_MASTER_KEY is injected by MarbleRun Coordinator from
	// manifest-defined secrets. It is used only for TEE HMAC signatures on
	// request/completion proofs (account keys are managed by the neoaccounts service).
	if key, ok := cfg.Marble.Secret("NEOVAULT_MASTER_KEY"); ok {
		s.masterKey = key
	}

	// Load fee collection address from Marble secrets if not provided in config
	if s.feeCollectionAddress == "" {
		if addr, ok := cfg.Marble.Secret("NEOVAULT_FEE_ADDRESS"); ok {
			s.feeCollectionAddress = string(addr)
		}
	}

	// Register standard /health route (we keep custom /info handler for async pool info)
	base.RegisterStandardRoutes()
	s.registerRoutes()

	return s, nil
}

