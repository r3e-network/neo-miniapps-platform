package defi

import (
	"math/big"
	"testing"
	"time"
)

func TestTokenTypes(t *testing.T) {
	token := Token{
		Address:  "0x1234567890abcdef",
		Symbol:   "NEO",
		Name:     "Neo",
		Decimals: 8,
		ChainID:  "neo-mainnet",
		Metadata: map[string]string{"type": "native"},
	}

	if token.Symbol != "NEO" {
		t.Errorf("expected symbol NEO, got %s", token.Symbol)
	}
	if token.Decimals != 8 {
		t.Errorf("expected decimals 8, got %d", token.Decimals)
	}
}

func TestTokenAmount(t *testing.T) {
	token := Token{Symbol: "GAS", Decimals: 8}
	amount := TokenAmount{
		Token:  token,
		Amount: big.NewInt(100000000), // 1 GAS
	}

	if amount.Token.Symbol != "GAS" {
		t.Errorf("expected GAS, got %s", amount.Token.Symbol)
	}
	if amount.Amount.Cmp(big.NewInt(100000000)) != 0 {
		t.Errorf("unexpected amount")
	}
}

func TestPrice(t *testing.T) {
	price := Price{
		Token:     Token{Symbol: "NEO"},
		PriceUSD:  12.34,
		UpdatedAt: time.Now(),
		Source:    "coingecko",
	}

	if price.PriceUSD != 12.34 {
		t.Errorf("expected 12.34, got %f", price.PriceUSD)
	}
}

func TestPoolTypes(t *testing.T) {
	tests := []struct {
		name     string
		poolType PoolType
	}{
		{"AMM", PoolTypeAMM},
		{"OrderBook", PoolTypeOrderBook},
		{"Stable", PoolTypeStable},
		{"Concentrated", PoolTypeConcentrated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := Pool{
				ID:       "pool-1",
				Protocol: "flamingo",
				Type:     tt.poolType,
				Fee:      0.003,
			}
			if pool.Type != tt.poolType {
				t.Errorf("expected %s, got %s", tt.poolType, pool.Type)
			}
		})
	}
}

func TestSwapQuote(t *testing.T) {
	quote := SwapQuote{
		TokenIn: TokenAmount{
			Token:  Token{Symbol: "NEO"},
			Amount: big.NewInt(1000000000),
		},
		TokenOut: TokenAmount{
			Token:  Token{Symbol: "GAS"},
			Amount: big.NewInt(500000000),
		},
		PriceImpact: 0.5,
		Route:       []string{"NEO", "GAS"},
		Protocol:    "flamingo",
		ExpiresAt:   time.Now().Add(5 * time.Minute),
		GasEstimate: 100000,
	}

	if quote.PriceImpact != 0.5 {
		t.Errorf("expected price impact 0.5, got %f", quote.PriceImpact)
	}
	if len(quote.Route) != 2 {
		t.Errorf("expected 2 hops, got %d", len(quote.Route))
	}
}

func TestPositionTypes(t *testing.T) {
	tests := []struct {
		name string
		pt   PositionType
	}{
		{"Liquidity", PositionTypeLiquidity},
		{"Stake", PositionTypeStake},
		{"Lend", PositionTypeLend},
		{"Borrow", PositionTypeBorrow},
		{"Yield", PositionTypeYield},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := Position{
				ID:       "pos-1",
				Owner:    "addr-1",
				Protocol: "aave",
				Type:     tt.pt,
				ValueUSD: 1000.0,
			}
			if pos.Type != tt.pt {
				t.Errorf("expected %s, got %s", tt.pt, pos.Type)
			}
		})
	}
}

func TestLendingMarket(t *testing.T) {
	market := LendingMarket{
		ID:               "market-1",
		Protocol:         "compound",
		Asset:            Token{Symbol: "USDC"},
		SupplyAPY:        3.5,
		BorrowAPY:        5.2,
		TotalSupply:      big.NewInt(1000000000000),
		TotalBorrow:      big.NewInt(500000000000),
		Utilization:      0.5,
		CollateralFactor: 0.8,
		ChainID:          "eth-mainnet",
	}

	if market.Utilization != 0.5 {
		t.Errorf("expected utilization 0.5, got %f", market.Utilization)
	}
	if market.CollateralFactor != 0.8 {
		t.Errorf("expected CF 0.8, got %f", market.CollateralFactor)
	}
}

func TestStakingPool(t *testing.T) {
	pool := StakingPool{
		ID:           "stake-1",
		Protocol:     "lido",
		StakeToken:   Token{Symbol: "ETH"},
		RewardTokens: []Token{{Symbol: "stETH"}},
		APY:          4.5,
		TVL:          10000000.0,
		LockPeriod:   24 * time.Hour * 30,
		MinStake:     big.NewInt(100000000000000000), // 0.1 ETH
		ChainID:      "eth-mainnet",
	}

	if pool.APY != 4.5 {
		t.Errorf("expected APY 4.5, got %f", pool.APY)
	}
	if len(pool.RewardTokens) != 1 {
		t.Errorf("expected 1 reward token, got %d", len(pool.RewardTokens))
	}
}

func TestRiskLevels(t *testing.T) {
	tests := []struct {
		level RiskLevel
		valid bool
	}{
		{RiskLow, true},
		{RiskMedium, true},
		{RiskHigh, true},
	}

	for _, tt := range tests {
		opp := YieldOpportunity{
			ID:       "yield-1",
			Protocol: "yearn",
			Token:    Token{Symbol: "USDC"},
			APY:      10.5,
			Risk:     tt.level,
		}
		if opp.Risk != tt.level {
			t.Errorf("expected risk %s, got %s", tt.level, opp.Risk)
		}
	}
}

func TestDeFiInfo(t *testing.T) {
	info := DeFiInfo{
		Protocols:    []string{"uniswap", "aave", "compound"},
		Chains:       []string{"eth-mainnet", "neo-mainnet"},
		Capabilities: []string{"swap", "liquidity", "lending", "staking"},
	}

	if len(info.Protocols) != 3 {
		t.Errorf("expected 3 protocols, got %d", len(info.Protocols))
	}
	if len(info.Capabilities) != 4 {
		t.Errorf("expected 4 capabilities, got %d", len(info.Capabilities))
	}
}
