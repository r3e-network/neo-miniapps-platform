// Package rwa provides Real-World Asset domain engine interfaces for the service layer.
//
// This package defines standard abstractions for tokenizing and managing real-world assets
// including real estate, commodities, securities, intellectual property, and other tangible
// or intangible assets on blockchain.
//
// # Implementation Status
//
// This package contains interface specifications and type systems for RWA operations.
// It serves as an architectural contract for RWA service implementations. Concrete
// implementations will be developed in internal/services/ as RWA services are built out.
//
// Services implementing these interfaces are optional extensions to the base ServiceModule.
// They should implement the RWAEngine composite interface along with specific sub-interfaces
// (AssetEngine, ComplianceEngine, CustodyEngine, etc.) for their supported capabilities.
package rwa

import (
	"context"
	"math/big"
	"time"

	"github.com/R3E-Network/service_layer/internal/engine"
)

// Common RWA types

// Asset represents a tokenized real-world asset.
type Asset struct {
	ID             string            `json:"id"`
	TokenID        string            `json:"token_id,omitempty"`
	ContractAddr   string            `json:"contract_address,omitempty"`
	ChainID        string            `json:"chain_id"`
	Type           AssetType         `json:"type"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	Issuer         string            `json:"issuer"`
	Owner          string            `json:"owner"`
	Custodian      string            `json:"custodian,omitempty"`
	Status         AssetStatus       `json:"status"`
	Valuation      Valuation         `json:"valuation"`
	Legal          LegalInfo         `json:"legal"`
	Compliance     ComplianceInfo    `json:"compliance"`
	Documents      []Document        `json:"documents,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// AssetType identifies the type of real-world asset.
type AssetType string

const (
	AssetTypeRealEstate     AssetType = "real_estate"
	AssetTypeCommodity      AssetType = "commodity"
	AssetTypeSecurity       AssetType = "security"
	AssetTypeDebt           AssetType = "debt"
	AssetTypeEquity         AssetType = "equity"
	AssetTypeIntellectual   AssetType = "intellectual_property"
	AssetTypeCollectible    AssetType = "collectible"
	AssetTypeInfrastructure AssetType = "infrastructure"
	AssetTypeRevenue        AssetType = "revenue_stream"
	AssetTypeOther          AssetType = "other"
)

// AssetStatus indicates the current status of an asset.
type AssetStatus string

const (
	AssetStatusDraft      AssetStatus = "draft"
	AssetStatusPending    AssetStatus = "pending"
	AssetStatusVerified   AssetStatus = "verified"
	AssetStatusTokenized  AssetStatus = "tokenized"
	AssetStatusActive     AssetStatus = "active"
	AssetStatusFrozen     AssetStatus = "frozen"
	AssetStatusRedeemed   AssetStatus = "redeemed"
	AssetStatusDelisted   AssetStatus = "delisted"
)

// Valuation represents asset valuation information.
type Valuation struct {
	ValueUSD       float64   `json:"value_usd"`
	Currency       string    `json:"currency"`
	ValueLocal     float64   `json:"value_local"`
	Method         string    `json:"method"` // appraisal, market, book, dcf
	Appraiser      string    `json:"appraiser,omitempty"`
	AppraisalDate  time.Time `json:"appraisal_date"`
	NextReview     time.Time `json:"next_review,omitempty"`
	Confidence     float64   `json:"confidence"` // 0-1
}

// LegalInfo contains legal information about an asset.
type LegalInfo struct {
	Jurisdiction   string    `json:"jurisdiction"`
	LegalEntity    string    `json:"legal_entity"`
	OwnershipType  string    `json:"ownership_type"` // direct, spv, trust
	Title          string    `json:"title,omitempty"`
	TitleNumber    string    `json:"title_number,omitempty"`
	Encumbrances   []string  `json:"encumbrances,omitempty"`
	Restrictions   []string  `json:"restrictions,omitempty"`
	ExpiryDate     time.Time `json:"expiry_date,omitempty"`
}

// ComplianceInfo contains compliance and regulatory information.
type ComplianceInfo struct {
	KYCRequired      bool      `json:"kyc_required"`
	AccreditedOnly   bool      `json:"accredited_only"`
	Jurisdictions    []string  `json:"jurisdictions"`
	Regulations      []string  `json:"regulations"` // reg_d, reg_s, mifid2
	LastAudit        time.Time `json:"last_audit,omitempty"`
	AuditReport      string    `json:"audit_report,omitempty"`
	ComplianceOfficer string   `json:"compliance_officer,omitempty"`
}

// Document represents a legal or supporting document.
type Document struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // deed, appraisal, audit, prospectus
	Name        string    `json:"name"`
	Hash        string    `json:"hash"` // IPFS or content hash
	URL         string    `json:"url,omitempty"`
	UploadedAt  time.Time `json:"uploaded_at"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	Verified    bool      `json:"verified"`
}

// TokenConfig defines tokenization parameters.
type TokenConfig struct {
	Standard       string    `json:"standard"` // erc20, erc1155, nep17
	TotalSupply    *big.Int  `json:"total_supply"`
	Decimals       uint8     `json:"decimals"`
	Transferable   bool      `json:"transferable"`
	Divisible      bool      `json:"divisible"`
	MinInvestment  *big.Int  `json:"min_investment,omitempty"`
	MaxInvestment  *big.Int  `json:"max_investment,omitempty"`
	LockupPeriod   time.Duration `json:"lockup_period,omitempty"`
	VestingSchedule *VestingSchedule `json:"vesting,omitempty"`
}

// VestingSchedule defines token vesting parameters.
type VestingSchedule struct {
	StartDate      time.Time `json:"start_date"`
	CliffDuration  time.Duration `json:"cliff_duration"`
	VestingDuration time.Duration `json:"vesting_duration"`
	Interval       time.Duration `json:"interval"`
}

// Investor represents an investor in an RWA.
type Investor struct {
	ID            string            `json:"id"`
	Wallet        string            `json:"wallet"`
	KYCStatus     KYCStatus         `json:"kyc_status"`
	Accredited    bool              `json:"accredited"`
	Jurisdiction  string            `json:"jurisdiction"`
	InvestorType  string            `json:"investor_type"` // individual, institutional, fund
	Holdings      map[string]*big.Int `json:"holdings"` // assetID -> amount
	JoinedAt      time.Time         `json:"joined_at"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// KYCStatus indicates KYC verification status.
type KYCStatus string

const (
	KYCStatusPending  KYCStatus = "pending"
	KYCStatusVerified KYCStatus = "verified"
	KYCStatusRejected KYCStatus = "rejected"
	KYCStatusExpired  KYCStatus = "expired"
)

// Distribution represents a dividend or income distribution.
type Distribution struct {
	ID            string    `json:"id"`
	AssetID       string    `json:"asset_id"`
	Type          string    `json:"type"` // dividend, interest, rent, royalty
	Amount        *big.Int  `json:"amount"`
	Currency      string    `json:"currency"`
	RecordDate    time.Time `json:"record_date"`
	PaymentDate   time.Time `json:"payment_date"`
	Status        string    `json:"status"` // announced, processing, paid
	TxHash        string    `json:"tx_hash,omitempty"`
}

// RedemptionRequest represents a request to redeem tokens for underlying asset.
type RedemptionRequest struct {
	ID          string    `json:"id"`
	AssetID     string    `json:"asset_id"`
	Investor    string    `json:"investor"`
	Amount      *big.Int  `json:"amount"`
	Status      string    `json:"status"` // pending, approved, processing, completed, rejected
	RequestedAt time.Time `json:"requested_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	TxHash      string    `json:"tx_hash,omitempty"`
}

// Engine interfaces

// AssetEngine handles RWA asset management.
type AssetEngine interface {
	engine.ServiceModule

	// CreateAsset creates a new RWA.
	CreateAsset(ctx context.Context, asset *Asset) (*Asset, error)

	// GetAsset retrieves asset information.
	GetAsset(ctx context.Context, assetID string) (*Asset, error)

	// ListAssets lists assets with filters.
	ListAssets(ctx context.Context, assetType AssetType, status AssetStatus, issuer string, limit int) ([]Asset, error)

	// UpdateAsset updates asset information.
	UpdateAsset(ctx context.Context, assetID string, update *Asset) error

	// UpdateStatus updates asset status.
	UpdateStatus(ctx context.Context, assetID string, status AssetStatus) error

	// GetValuation retrieves current valuation.
	GetValuation(ctx context.Context, assetID string) (*Valuation, error)

	// UpdateValuation updates asset valuation.
	UpdateValuation(ctx context.Context, assetID string, valuation *Valuation) error
}

// TokenizationEngine handles asset tokenization.
type TokenizationEngine interface {
	engine.ServiceModule

	// Tokenize tokenizes an asset.
	Tokenize(ctx context.Context, assetID string, config *TokenConfig) (string, error)

	// GetTokenInfo retrieves token information for an asset.
	GetTokenInfo(ctx context.Context, assetID string) (*TokenConfig, error)

	// Mint mints new tokens.
	Mint(ctx context.Context, assetID string, to string, amount *big.Int) (string, error)

	// Burn burns tokens.
	Burn(ctx context.Context, assetID string, from string, amount *big.Int) (string, error)

	// Transfer transfers tokens between addresses.
	Transfer(ctx context.Context, assetID string, from, to string, amount *big.Int) (string, error)

	// GetBalance retrieves token balance.
	GetBalance(ctx context.Context, assetID string, holder string) (*big.Int, error)

	// GetHolders retrieves all token holders.
	GetHolders(ctx context.Context, assetID string, limit int) ([]Investor, error)
}

// ComplianceEngine handles regulatory compliance.
type ComplianceEngine interface {
	engine.ServiceModule

	// CheckTransferCompliance checks if a transfer is compliant.
	CheckTransferCompliance(ctx context.Context, assetID string, from, to string, amount *big.Int) (bool, []string, error)

	// WhitelistInvestor adds an investor to whitelist.
	WhitelistInvestor(ctx context.Context, assetID string, investor string) error

	// BlacklistInvestor removes an investor from whitelist.
	BlacklistInvestor(ctx context.Context, assetID string, investor string) error

	// IsWhitelisted checks if an investor is whitelisted.
	IsWhitelisted(ctx context.Context, assetID string, investor string) (bool, error)

	// GetComplianceInfo retrieves compliance information.
	GetComplianceInfo(ctx context.Context, assetID string) (*ComplianceInfo, error)

	// UpdateComplianceInfo updates compliance information.
	UpdateComplianceInfo(ctx context.Context, assetID string, info *ComplianceInfo) error
}

// InvestorEngine handles investor management.
type InvestorEngine interface {
	engine.ServiceModule

	// RegisterInvestor registers a new investor.
	RegisterInvestor(ctx context.Context, investor *Investor) (*Investor, error)

	// GetInvestor retrieves investor information.
	GetInvestor(ctx context.Context, investorID string) (*Investor, error)

	// GetInvestorByWallet retrieves investor by wallet.
	GetInvestorByWallet(ctx context.Context, wallet string) (*Investor, error)

	// UpdateKYCStatus updates investor KYC status.
	UpdateKYCStatus(ctx context.Context, investorID string, status KYCStatus) error

	// GetHoldings retrieves investor holdings.
	GetHoldings(ctx context.Context, investorID string) (map[string]*big.Int, error)

	// ListInvestors lists investors with filters.
	ListInvestors(ctx context.Context, kycStatus KYCStatus, accredited bool, limit int) ([]Investor, error)
}

// CustodyEngine handles asset custody.
type CustodyEngine interface {
	engine.ServiceModule

	// GetCustodian retrieves custodian for an asset.
	GetCustodian(ctx context.Context, assetID string) (string, error)

	// SetCustodian sets custodian for an asset.
	SetCustodian(ctx context.Context, assetID string, custodian string) error

	// RequestCustodyTransfer requests custody transfer.
	RequestCustodyTransfer(ctx context.Context, assetID string, newCustodian string) (string, error)

	// ApproveCustodyTransfer approves custody transfer.
	ApproveCustodyTransfer(ctx context.Context, requestID string) error

	// GetCustodyProof retrieves proof of custody.
	GetCustodyProof(ctx context.Context, assetID string) (*Document, error)
}

// DocumentEngine handles document management.
type DocumentEngine interface {
	engine.ServiceModule

	// UploadDocument uploads a document.
	UploadDocument(ctx context.Context, assetID string, doc *Document) (*Document, error)

	// GetDocument retrieves a document.
	GetDocument(ctx context.Context, documentID string) (*Document, error)

	// ListDocuments lists documents for an asset.
	ListDocuments(ctx context.Context, assetID string, docType string) ([]Document, error)

	// VerifyDocument verifies a document.
	VerifyDocument(ctx context.Context, documentID string) error

	// DeleteDocument deletes a document.
	DeleteDocument(ctx context.Context, documentID string) error
}

// DistributionEngine handles income/dividend distribution.
type DistributionEngine interface {
	engine.ServiceModule

	// CreateDistribution creates a new distribution.
	CreateDistribution(ctx context.Context, dist *Distribution) (*Distribution, error)

	// GetDistribution retrieves distribution information.
	GetDistribution(ctx context.Context, distributionID string) (*Distribution, error)

	// ListDistributions lists distributions for an asset.
	ListDistributions(ctx context.Context, assetID string, limit int) ([]Distribution, error)

	// ProcessDistribution processes a distribution.
	ProcessDistribution(ctx context.Context, distributionID string) error

	// GetInvestorDistributions retrieves distributions for an investor.
	GetInvestorDistributions(ctx context.Context, investorID string, limit int) ([]Distribution, error)
}

// RedemptionEngine handles token redemption.
type RedemptionEngine interface {
	engine.ServiceModule

	// RequestRedemption requests token redemption.
	RequestRedemption(ctx context.Context, req *RedemptionRequest) (*RedemptionRequest, error)

	// GetRedemption retrieves redemption request.
	GetRedemption(ctx context.Context, redemptionID string) (*RedemptionRequest, error)

	// ApproveRedemption approves a redemption request.
	ApproveRedemption(ctx context.Context, redemptionID string) error

	// RejectRedemption rejects a redemption request.
	RejectRedemption(ctx context.Context, redemptionID string, reason string) error

	// ProcessRedemption processes approved redemption.
	ProcessRedemption(ctx context.Context, redemptionID string) error

	// ListRedemptions lists redemption requests.
	ListRedemptions(ctx context.Context, assetID, investorID string, status string, limit int) ([]RedemptionRequest, error)
}

// RWAEngine is the composite interface for RWA services.
type RWAEngine interface {
	engine.ServiceModule

	// RWAInfo returns RWA engine information.
	RWAInfo() RWAInfo
}

// RWAInfo provides metadata about an RWA engine implementation.
type RWAInfo struct {
	SupportedAssetTypes []AssetType `json:"supported_asset_types"`
	Chains              []string    `json:"chains"`
	Jurisdictions       []string    `json:"jurisdictions"`
	Capabilities        []string    `json:"capabilities"` // tokenization, compliance, custody, distribution
}

// Capability markers

// TokenizationCapable indicates tokenization support.
type TokenizationCapable interface {
	HasTokenizationEngine() bool
	TokenizationEngine() TokenizationEngine
}

// ComplianceCapable indicates compliance support.
type ComplianceCapable interface {
	HasComplianceEngine() bool
	ComplianceEngine() ComplianceEngine
}

// InvestorCapable indicates investor management support.
type InvestorCapable interface {
	HasInvestorEngine() bool
	InvestorEngine() InvestorEngine
}

// CustodyCapable indicates custody support.
type CustodyCapable interface {
	HasCustodyEngine() bool
	CustodyEngine() CustodyEngine
}

// DocumentCapable indicates document management support.
type DocumentCapable interface {
	HasDocumentEngine() bool
	DocumentEngine() DocumentEngine
}

// DistributionCapable indicates distribution support.
type DistributionCapable interface {
	HasDistributionEngine() bool
	DistributionEngine() DistributionEngine
}

// RedemptionCapable indicates redemption support.
type RedemptionCapable interface {
	HasRedemptionEngine() bool
	RedemptionEngine() RedemptionEngine
}
