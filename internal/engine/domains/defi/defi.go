// Package defi provides DeFi domain engine interfaces for the service layer.
//
// This package defines standard abstractions for decentralized finance operations including
// token management, liquidity pools, lending/borrowing, staking, and yield farming.
//
// # Implementation Status
//
// This package contains fully-defined interface specifications and type systems. It serves as
// an architectural contract for DeFi service implementations. Concrete implementations will be
// developed in internal/services/ as DeFi services are built out.
//
// Services implementing these interfaces are optional extensions to the base ServiceModule.
// They should implement the DeFiEngine composite interface along with specific sub-interfaces
// (TokenEngine, SwapEngine, LiquidityEngine, etc.) for their supported capabilities.
package defi

import (
	"context"
	"math/big"
	"time"

	"github.com/R3E-Network/service_layer/internal/engine"
)

// Common DeFi types

// Token represents a fungible token.
type Token struct {
	Address  string            `json:"address"`
	Symbol   string            `json:"symbol"`
	Name     string            `json:"name"`
	Decimals uint8             `json:"decimals"`
	ChainID  string            `json:"chain_id"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// TokenAmount represents a token with its amount.
type TokenAmount struct {
	Token  Token    `json:"token"`
	Amount *big.Int `json:"amount"`
}

// Price represents a token price.
type Price struct {
	Token     Token     `json:"token"`
	PriceUSD  float64   `json:"price_usd"`
	UpdatedAt time.Time `json:"updated_at"`
	Source    string    `json:"source,omitempty"`
}

// Pool represents a liquidity pool.
type Pool struct {
	ID         string       `json:"id"`
	Protocol   string       `json:"protocol"`
	Tokens     []Token      `json:"tokens"`
	Reserves   []*big.Int   `json:"reserves"`
	Fee        float64      `json:"fee"`
	TVL        float64      `json:"tvl_usd"`
	APY        float64      `json:"apy"`
	ChainID    string       `json:"chain_id"`
	Address    string       `json:"address"`
	Type       PoolType     `json:"type"`
}

// PoolType identifies the pool mechanism.
type PoolType string

const (
	PoolTypeAMM        PoolType = "amm"        // Automated Market Maker
	PoolTypeOrderBook  PoolType = "orderbook"  // Order book based
	PoolTypeStable     PoolType = "stable"     // Stable swap curve
	PoolTypeConcentrated PoolType = "concentrated" // Concentrated liquidity
)

// SwapQuote represents a quote for token swap.
type SwapQuote struct {
	TokenIn      TokenAmount `json:"token_in"`
	TokenOut     TokenAmount `json:"token_out"`
	PriceImpact  float64     `json:"price_impact"`
	Route        []string    `json:"route"`
	Protocol     string      `json:"protocol"`
	ExpiresAt    time.Time   `json:"expires_at"`
	GasEstimate  uint64      `json:"gas_estimate"`
}

// SwapResult represents the result of a swap execution.
type SwapResult struct {
	TxHash      string      `json:"tx_hash"`
	TokenIn     TokenAmount `json:"token_in"`
	TokenOut    TokenAmount `json:"token_out"`
	Fee         TokenAmount `json:"fee"`
	ExecutedAt  time.Time   `json:"executed_at"`
	BlockNumber uint64      `json:"block_number"`
}

// Position represents a user's DeFi position.
type Position struct {
	ID          string            `json:"id"`
	Owner       string            `json:"owner"`
	Protocol    string            `json:"protocol"`
	Type        PositionType      `json:"type"`
	Assets      []TokenAmount     `json:"assets"`
	ValueUSD    float64           `json:"value_usd"`
	PnL         float64           `json:"pnl"`
	OpenedAt    time.Time         `json:"opened_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// PositionType identifies the type of DeFi position.
type PositionType string

const (
	PositionTypeLiquidity PositionType = "liquidity"
	PositionTypeStake     PositionType = "stake"
	PositionTypeLend      PositionType = "lend"
	PositionTypeBorrow    PositionType = "borrow"
	PositionTypeYield     PositionType = "yield"
)

// LendingMarket represents a lending market.
type LendingMarket struct {
	ID            string  `json:"id"`
	Protocol      string  `json:"protocol"`
	Asset         Token   `json:"asset"`
	SupplyAPY     float64 `json:"supply_apy"`
	BorrowAPY     float64 `json:"borrow_apy"`
	TotalSupply   *big.Int `json:"total_supply"`
	TotalBorrow   *big.Int `json:"total_borrow"`
	Utilization   float64 `json:"utilization"`
	CollateralFactor float64 `json:"collateral_factor"`
	ChainID       string  `json:"chain_id"`
}

// StakingPool represents a staking pool.
type StakingPool struct {
	ID            string    `json:"id"`
	Protocol      string    `json:"protocol"`
	StakeToken    Token     `json:"stake_token"`
	RewardTokens  []Token   `json:"reward_tokens"`
	APY           float64   `json:"apy"`
	TVL           float64   `json:"tvl_usd"`
	LockPeriod    time.Duration `json:"lock_period"`
	MinStake      *big.Int  `json:"min_stake"`
	ChainID       string    `json:"chain_id"`
}

// Engine interfaces

// TokenEngine handles token operations.
type TokenEngine interface {
	engine.ServiceModule

	// GetToken retrieves token information by address.
	GetToken(ctx context.Context, chainID, address string) (*Token, error)

	// GetTokenPrice retrieves current token price.
	GetTokenPrice(ctx context.Context, chainID, address string) (*Price, error)

	// GetTokenBalance retrieves token balance for an account.
	GetTokenBalance(ctx context.Context, chainID, address, account string) (*big.Int, error)

	// ListTokens lists tokens matching criteria.
	ListTokens(ctx context.Context, chainID string, limit int) ([]Token, error)
}

// SwapEngine handles token swap operations.
type SwapEngine interface {
	engine.ServiceModule

	// GetSwapQuote gets a quote for swapping tokens.
	GetSwapQuote(ctx context.Context, chainID string, tokenIn, tokenOut string, amountIn *big.Int) (*SwapQuote, error)

	// ExecuteSwap executes a token swap.
	ExecuteSwap(ctx context.Context, quote *SwapQuote, sender string) (*SwapResult, error)

	// GetSwapRoutes finds optimal swap routes.
	GetSwapRoutes(ctx context.Context, chainID string, tokenIn, tokenOut string) ([][]string, error)
}

// LiquidityEngine handles liquidity pool operations.
type LiquidityEngine interface {
	engine.ServiceModule

	// GetPool retrieves pool information.
	GetPool(ctx context.Context, chainID, poolID string) (*Pool, error)

	// ListPools lists pools matching criteria.
	ListPools(ctx context.Context, chainID string, tokens []string, limit int) ([]Pool, error)

	// AddLiquidity adds liquidity to a pool.
	AddLiquidity(ctx context.Context, chainID, poolID string, amounts []TokenAmount, sender string) (*Position, error)

	// RemoveLiquidity removes liquidity from a pool.
	RemoveLiquidity(ctx context.Context, positionID string, percentage float64, sender string) (*SwapResult, error)

	// GetPoolTVL gets total value locked in a pool.
	GetPoolTVL(ctx context.Context, chainID, poolID string) (float64, error)
}

// LendingEngine handles lending and borrowing operations.
type LendingEngine interface {
	engine.ServiceModule

	// GetMarket retrieves lending market information.
	GetMarket(ctx context.Context, chainID, marketID string) (*LendingMarket, error)

	// ListMarkets lists available lending markets.
	ListMarkets(ctx context.Context, chainID string, limit int) ([]LendingMarket, error)

	// Supply supplies assets to a lending market.
	Supply(ctx context.Context, chainID, marketID string, amount TokenAmount, sender string) (*Position, error)

	// Withdraw withdraws supplied assets.
	Withdraw(ctx context.Context, positionID string, amount *big.Int, sender string) (*SwapResult, error)

	// Borrow borrows assets from a lending market.
	Borrow(ctx context.Context, chainID, marketID string, amount *big.Int, collateral []TokenAmount, sender string) (*Position, error)

	// Repay repays borrowed assets.
	Repay(ctx context.Context, positionID string, amount *big.Int, sender string) (*SwapResult, error)

	// GetHealthFactor calculates account health factor.
	GetHealthFactor(ctx context.Context, chainID, account string) (float64, error)
}

// StakingEngine handles staking operations.
type StakingEngine interface {
	engine.ServiceModule

	// GetStakingPool retrieves staking pool information.
	GetStakingPool(ctx context.Context, chainID, poolID string) (*StakingPool, error)

	// ListStakingPools lists available staking pools.
	ListStakingPools(ctx context.Context, chainID string, limit int) ([]StakingPool, error)

	// Stake stakes tokens in a pool.
	Stake(ctx context.Context, chainID, poolID string, amount TokenAmount, sender string) (*Position, error)

	// Unstake unstakes tokens from a pool.
	Unstake(ctx context.Context, positionID string, amount *big.Int, sender string) (*SwapResult, error)

	// ClaimRewards claims staking rewards.
	ClaimRewards(ctx context.Context, positionID string, sender string) ([]TokenAmount, error)

	// GetStakingRewards gets pending rewards for a position.
	GetStakingRewards(ctx context.Context, positionID string) ([]TokenAmount, error)
}

// YieldEngine handles yield farming and aggregation.
type YieldEngine interface {
	engine.ServiceModule

	// GetBestYield finds the best yield opportunity for a token.
	GetBestYield(ctx context.Context, chainID, tokenAddress string) ([]YieldOpportunity, error)

	// Deposit deposits into a yield strategy.
	Deposit(ctx context.Context, strategyID string, amount TokenAmount, sender string) (*Position, error)

	// Withdraw withdraws from a yield strategy.
	Withdraw(ctx context.Context, positionID string, amount *big.Int, sender string) (*SwapResult, error)

	// Harvest harvests yield from a position.
	Harvest(ctx context.Context, positionID string, sender string) ([]TokenAmount, error)
}

// YieldOpportunity represents a yield farming opportunity.
type YieldOpportunity struct {
	ID        string   `json:"id"`
	Protocol  string   `json:"protocol"`
	Token     Token    `json:"token"`
	APY       float64  `json:"apy"`
	TVL       float64  `json:"tvl_usd"`
	Risk      RiskLevel `json:"risk"`
	ChainID   string   `json:"chain_id"`
}

// RiskLevel indicates the risk level of a DeFi opportunity.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// PositionEngine handles DeFi position management.
type PositionEngine interface {
	engine.ServiceModule

	// GetPosition retrieves a position by ID.
	GetPosition(ctx context.Context, positionID string) (*Position, error)

	// ListPositions lists positions for an account.
	ListPositions(ctx context.Context, account string, positionType PositionType) ([]Position, error)

	// GetPortfolioValue gets total portfolio value for an account.
	GetPortfolioValue(ctx context.Context, account string) (float64, error)

	// GetPositionHistory gets historical data for a position.
	GetPositionHistory(ctx context.Context, positionID string, from, to time.Time) ([]PositionSnapshot, error)
}

// PositionSnapshot represents a point-in-time snapshot of a position.
type PositionSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	ValueUSD  float64   `json:"value_usd"`
	PnL       float64   `json:"pnl"`
}

// DeFiEngine is the composite interface for DeFi services.
// Services can implement the full interface or specific sub-interfaces.
type DeFiEngine interface {
	engine.ServiceModule

	// Info returns DeFi engine information.
	DeFiInfo() DeFiInfo
}

// DeFiInfo provides metadata about a DeFi engine implementation.
type DeFiInfo struct {
	Protocols    []string `json:"protocols"`
	Chains       []string `json:"chains"`
	Capabilities []string `json:"capabilities"` // swap, liquidity, lending, staking, yield
}

// Capability markers

// TokenCapable indicates token management support.
type TokenCapable interface {
	HasTokenEngine() bool
	TokenEngine() TokenEngine
}

// SwapCapable indicates swap support.
type SwapCapable interface {
	HasSwapEngine() bool
	SwapEngine() SwapEngine
}

// LiquidityCapable indicates liquidity pool support.
type LiquidityCapable interface {
	HasLiquidityEngine() bool
	LiquidityEngine() LiquidityEngine
}

// LendingCapable indicates lending/borrowing support.
type LendingCapable interface {
	HasLendingEngine() bool
	LendingEngine() LendingEngine
}

// StakingCapable indicates staking support.
type StakingCapable interface {
	HasStakingEngine() bool
	StakingEngine() StakingEngine
}

// YieldCapable indicates yield farming support.
type YieldCapable interface {
	HasYieldEngine() bool
	YieldEngine() YieldEngine
}
