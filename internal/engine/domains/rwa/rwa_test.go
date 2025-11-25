package rwa

import (
	"math/big"
	"testing"
	"time"
)

func TestAsset(t *testing.T) {
	asset := Asset{
		ID:           "asset-1",
		TokenID:      "token-1",
		ContractAddr: "0xabcd",
		ChainID:      "neo-mainnet",
		Type:         AssetTypeRealEstate,
		Name:         "Manhattan Office Building",
		Description:  "Class A office building in Midtown",
		Issuer:       "RealEstate Corp",
		Owner:        "SPV Holdings LLC",
		Custodian:    "Trust Bank",
		Status:       AssetStatusTokenized,
		Valuation: Valuation{
			ValueUSD:      50000000.0,
			Currency:      "USD",
			ValueLocal:    50000000.0,
			Method:        "appraisal",
			Appraiser:     "Jones Lang LaSalle",
			AppraisalDate: time.Now().Add(-30 * 24 * time.Hour),
			Confidence:    0.95,
		},
		Legal: LegalInfo{
			Jurisdiction:  "US-NY",
			LegalEntity:   "SPV Holdings LLC",
			OwnershipType: "spv",
			Title:         "Fee Simple",
			TitleNumber:   "NY-12345",
		},
		Compliance: ComplianceInfo{
			KYCRequired:    true,
			AccreditedOnly: true,
			Jurisdictions:  []string{"US", "EU"},
			Regulations:    []string{"reg_d", "reg_s"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if asset.Type != AssetTypeRealEstate {
		t.Errorf("expected real_estate, got %s", asset.Type)
	}
	if asset.Status != AssetStatusTokenized {
		t.Errorf("expected tokenized, got %s", asset.Status)
	}
	if asset.Valuation.ValueUSD != 50000000.0 {
		t.Errorf("expected 50M, got %f", asset.Valuation.ValueUSD)
	}
}

func TestAssetTypes(t *testing.T) {
	types := []AssetType{
		AssetTypeRealEstate,
		AssetTypeCommodity,
		AssetTypeSecurity,
		AssetTypeDebt,
		AssetTypeEquity,
		AssetTypeIntellectual,
		AssetTypeCollectible,
		AssetTypeInfrastructure,
		AssetTypeRevenue,
		AssetTypeOther,
	}

	for _, at := range types {
		asset := Asset{Type: at}
		if asset.Type != at {
			t.Errorf("expected %s, got %s", at, asset.Type)
		}
	}
}

func TestAssetStatuses(t *testing.T) {
	statuses := []AssetStatus{
		AssetStatusDraft,
		AssetStatusPending,
		AssetStatusVerified,
		AssetStatusTokenized,
		AssetStatusActive,
		AssetStatusFrozen,
		AssetStatusRedeemed,
		AssetStatusDelisted,
	}

	for _, s := range statuses {
		asset := Asset{Status: s}
		if asset.Status != s {
			t.Errorf("expected %s, got %s", s, asset.Status)
		}
	}
}

func TestValuation(t *testing.T) {
	val := Valuation{
		ValueUSD:      10000000.0,
		Currency:      "EUR",
		ValueLocal:    9500000.0,
		Method:        "dcf",
		Appraiser:     "CBRE",
		AppraisalDate: time.Now(),
		NextReview:    time.Now().Add(365 * 24 * time.Hour),
		Confidence:    0.9,
	}

	if val.Confidence != 0.9 {
		t.Errorf("expected confidence 0.9, got %f", val.Confidence)
	}
}

func TestLegalInfo(t *testing.T) {
	legal := LegalInfo{
		Jurisdiction:  "US-DE",
		LegalEntity:   "Asset Holdings Inc",
		OwnershipType: "trust",
		Title:         "Leasehold",
		TitleNumber:   "DE-99999",
		Encumbrances:  []string{"mortgage-1"},
		Restrictions:  []string{"no-subdivision"},
		ExpiryDate:    time.Now().Add(99 * 365 * 24 * time.Hour),
	}

	if legal.OwnershipType != "trust" {
		t.Errorf("expected trust, got %s", legal.OwnershipType)
	}
	if len(legal.Encumbrances) != 1 {
		t.Errorf("expected 1 encumbrance, got %d", len(legal.Encumbrances))
	}
}

func TestComplianceInfo(t *testing.T) {
	compliance := ComplianceInfo{
		KYCRequired:       true,
		AccreditedOnly:    true,
		Jurisdictions:     []string{"US", "UK", "SG"},
		Regulations:       []string{"reg_d", "mifid2"},
		LastAudit:         time.Now().Add(-90 * 24 * time.Hour),
		AuditReport:       "ipfs://Qm...",
		ComplianceOfficer: "John Smith",
	}

	if !compliance.KYCRequired {
		t.Error("expected KYC required")
	}
	if len(compliance.Jurisdictions) != 3 {
		t.Errorf("expected 3 jurisdictions, got %d", len(compliance.Jurisdictions))
	}
}

func TestDocument(t *testing.T) {
	doc := Document{
		ID:         "doc-1",
		Type:       "deed",
		Name:       "Property Deed",
		Hash:       "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
		URL:        "https://example.com/deed.pdf",
		UploadedAt: time.Now(),
		ExpiresAt:  time.Now().Add(365 * 24 * time.Hour),
		Verified:   true,
	}

	if doc.Type != "deed" {
		t.Errorf("expected deed, got %s", doc.Type)
	}
	if !doc.Verified {
		t.Error("expected verified")
	}
}

func TestTokenConfig(t *testing.T) {
	totalSupply := new(big.Int)
	totalSupply.SetString("1000000000000000000000000", 10) // 1M tokens (18 decimals)

	minInvestment := new(big.Int)
	minInvestment.SetString("1000000000000000000000", 10) // 1000 tokens

	maxInvestment := new(big.Int)
	maxInvestment.SetString("100000000000000000000000", 10) // 100000 tokens

	config := TokenConfig{
		Standard:      "erc20",
		TotalSupply:   totalSupply,
		Decimals:      18,
		Transferable:  true,
		Divisible:     true,
		MinInvestment: minInvestment,
		MaxInvestment: maxInvestment,
		LockupPeriod:  365 * 24 * time.Hour,
		VestingSchedule: &VestingSchedule{
			StartDate:       time.Now(),
			CliffDuration:   90 * 24 * time.Hour,
			VestingDuration: 365 * 24 * time.Hour,
			Interval:        30 * 24 * time.Hour,
		},
	}

	if config.Standard != "erc20" {
		t.Errorf("expected erc20, got %s", config.Standard)
	}
	if config.Decimals != 18 {
		t.Errorf("expected 18 decimals, got %d", config.Decimals)
	}
}

func TestInvestor(t *testing.T) {
	investor := Investor{
		ID:           "investor-1",
		Wallet:       "NeoAddr123",
		KYCStatus:    KYCStatusVerified,
		Accredited:   true,
		Jurisdiction: "US",
		InvestorType: "individual",
		Holdings: map[string]*big.Int{
			"asset-1": big.NewInt(10000),
			"asset-2": big.NewInt(5000),
		},
		JoinedAt: time.Now(),
	}

	if investor.KYCStatus != KYCStatusVerified {
		t.Errorf("expected verified, got %s", investor.KYCStatus)
	}
	if !investor.Accredited {
		t.Error("expected accredited")
	}
	if len(investor.Holdings) != 2 {
		t.Errorf("expected 2 holdings, got %d", len(investor.Holdings))
	}
}

func TestKYCStatuses(t *testing.T) {
	statuses := []KYCStatus{
		KYCStatusPending,
		KYCStatusVerified,
		KYCStatusRejected,
		KYCStatusExpired,
	}

	for _, s := range statuses {
		inv := Investor{KYCStatus: s}
		if inv.KYCStatus != s {
			t.Errorf("expected %s, got %s", s, inv.KYCStatus)
		}
	}
}

func TestDistribution(t *testing.T) {
	dist := Distribution{
		ID:          "dist-1",
		AssetID:     "asset-1",
		Type:        "dividend",
		Amount:      big.NewInt(1000000000000),
		Currency:    "USDC",
		RecordDate:  time.Now().Add(-7 * 24 * time.Hour),
		PaymentDate: time.Now(),
		Status:      "paid",
		TxHash:      "0xabcdef",
	}

	if dist.Type != "dividend" {
		t.Errorf("expected dividend, got %s", dist.Type)
	}
	if dist.Status != "paid" {
		t.Errorf("expected paid, got %s", dist.Status)
	}
}

func TestRedemptionRequest(t *testing.T) {
	req := RedemptionRequest{
		ID:          "redeem-1",
		AssetID:     "asset-1",
		Investor:    "investor-1",
		Amount:      big.NewInt(5000),
		Status:      "pending",
		RequestedAt: time.Now(),
	}

	if req.Status != "pending" {
		t.Errorf("expected pending, got %s", req.Status)
	}
}

func TestRWAInfo(t *testing.T) {
	info := RWAInfo{
		SupportedAssetTypes: []AssetType{AssetTypeRealEstate, AssetTypeSecurity},
		Chains:              []string{"neo-mainnet", "eth-mainnet"},
		Jurisdictions:       []string{"US", "EU", "SG"},
		Capabilities:        []string{"tokenization", "compliance", "custody", "distribution"},
	}

	if len(info.SupportedAssetTypes) != 2 {
		t.Errorf("expected 2 asset types, got %d", len(info.SupportedAssetTypes))
	}
	if len(info.Capabilities) != 4 {
		t.Errorf("expected 4 capabilities, got %d", len(info.Capabilities))
	}
}
