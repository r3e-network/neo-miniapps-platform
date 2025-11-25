// Package nft provides NFT domain engine interfaces for the service layer.
//
// This package defines standard abstractions for non-fungible token operations including
// collection management, minting, marketplace trading, metadata, and royalties.
//
// # Implementation Status
//
// This package contains fully-defined interface specifications and type systems. It serves as
// an architectural contract for NFT service implementations. Concrete implementations will be
// developed in internal/services/ as NFT services are built out.
//
// Services implementing these interfaces are optional extensions to the base ServiceModule.
// They should implement the NFTEngine composite interface along with specific sub-interfaces
// (CollectionEngine, MintEngine, MarketplaceEngine, etc.) for their supported capabilities.
package nft

import (
	"context"
	"math/big"
	"time"

	"github.com/R3E-Network/service_layer/internal/engine"
)

// Common NFT types

// NFT represents a non-fungible token.
type NFT struct {
	ID            string            `json:"id"`
	TokenID       string            `json:"token_id"`
	ContractAddr  string            `json:"contract_address"`
	ChainID       string            `json:"chain_id"`
	CollectionID  string            `json:"collection_id"`
	Owner         string            `json:"owner"`
	Creator       string            `json:"creator"`
	Standard      TokenStandard     `json:"standard"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	ImageURL      string            `json:"image_url,omitempty"`
	AnimationURL  string            `json:"animation_url,omitempty"`
	ExternalURL   string            `json:"external_url,omitempty"`
	Attributes    []Attribute       `json:"attributes,omitempty"`
	Royalty       *Royalty          `json:"royalty,omitempty"`
	MintedAt      time.Time         `json:"minted_at"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// TokenStandard identifies the NFT token standard.
type TokenStandard string

const (
	StandardERC721    TokenStandard = "erc721"
	StandardERC1155   TokenStandard = "erc1155"
	StandardNEP11     TokenStandard = "nep11"  // Neo N3
	StandardNEP17     TokenStandard = "nep17"  // Neo N3 (for semi-fungible)
	StandardMetaplex  TokenStandard = "metaplex" // Solana
	StandardSPL       TokenStandard = "spl"    // Solana
)

// Attribute represents an NFT trait/attribute.
type Attribute struct {
	TraitType   string `json:"trait_type"`
	Value       any    `json:"value"`
	DisplayType string `json:"display_type,omitempty"`
	MaxValue    any    `json:"max_value,omitempty"`
}

// Royalty represents royalty configuration.
type Royalty struct {
	Recipient string  `json:"recipient"`
	Percent   float64 `json:"percent"` // e.g., 2.5 for 2.5%
	Amount    *big.Int `json:"amount,omitempty"` // For specific amount
}

// Collection represents an NFT collection.
type Collection struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Symbol       string            `json:"symbol"`
	Description  string            `json:"description,omitempty"`
	ContractAddr string            `json:"contract_address"`
	ChainID      string            `json:"chain_id"`
	Creator      string            `json:"creator"`
	Owner        string            `json:"owner"`
	Standard     TokenStandard     `json:"standard"`
	ImageURL     string            `json:"image_url,omitempty"`
	BannerURL    string            `json:"banner_url,omitempty"`
	ExternalURL  string            `json:"external_url,omitempty"`
	TotalSupply  uint64            `json:"total_supply"`
	MaxSupply    uint64            `json:"max_supply,omitempty"`
	Royalty      *Royalty          `json:"royalty,omitempty"`
	Categories   []string          `json:"categories,omitempty"`
	Verified     bool              `json:"verified"`
	CreatedAt    time.Time         `json:"created_at"`
	Stats        CollectionStats   `json:"stats"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// CollectionStats holds collection statistics.
type CollectionStats struct {
	FloorPrice     float64 `json:"floor_price_usd"`
	TotalVolume    float64 `json:"total_volume_usd"`
	Volume24h      float64 `json:"volume_24h_usd"`
	Volume7d       float64 `json:"volume_7d_usd"`
	Sales24h       uint64  `json:"sales_24h"`
	AveragePrice   float64 `json:"average_price_usd"`
	NumOwners      uint64  `json:"num_owners"`
	NumListed      uint64  `json:"num_listed"`
	MarketCap      float64 `json:"market_cap_usd"`
}

// MintRequest represents a minting request.
type MintRequest struct {
	CollectionID string            `json:"collection_id"`
	Recipient    string            `json:"recipient"`
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	ImageURL     string            `json:"image_url,omitempty"`
	AnimationURL string            `json:"animation_url,omitempty"`
	ExternalURL  string            `json:"external_url,omitempty"`
	Attributes   []Attribute       `json:"attributes,omitempty"`
	Royalty      *Royalty          `json:"royalty,omitempty"`
	Amount       uint64            `json:"amount,omitempty"` // For ERC1155
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// MintResult represents the result of a minting operation.
type MintResult struct {
	NFT       *NFT   `json:"nft"`
	TxHash    string `json:"tx_hash"`
	GasUsed   uint64 `json:"gas_used"`
}

// Listing represents a marketplace listing.
type Listing struct {
	ID           string        `json:"id"`
	NFTID        string        `json:"nft_id"`
	TokenID      string        `json:"token_id"`
	ContractAddr string        `json:"contract_address"`
	ChainID      string        `json:"chain_id"`
	Seller       string        `json:"seller"`
	Price        *big.Int      `json:"price"`
	Currency     string        `json:"currency"`
	PriceUSD     float64       `json:"price_usd"`
	ListingType  ListingType   `json:"listing_type"`
	Status       ListingStatus `json:"status"`
	Quantity     uint64        `json:"quantity,omitempty"` // For ERC1155
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
}

// ListingType identifies the type of listing.
type ListingType string

const (
	ListingTypeFixedPrice ListingType = "fixed_price"
	ListingTypeAuction    ListingType = "auction"
	ListingTypeDutchAuction ListingType = "dutch_auction"
)

// ListingStatus indicates listing status.
type ListingStatus string

const (
	ListingStatusActive    ListingStatus = "active"
	ListingStatusSold      ListingStatus = "sold"
	ListingStatusCancelled ListingStatus = "cancelled"
	ListingStatusExpired   ListingStatus = "expired"
)

// Bid represents an auction bid.
type Bid struct {
	ID         string    `json:"id"`
	ListingID  string    `json:"listing_id"`
	Bidder     string    `json:"bidder"`
	Amount     *big.Int  `json:"amount"`
	Currency   string    `json:"currency"`
	AmountUSD  float64   `json:"amount_usd"`
	Status     BidStatus `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
}

// BidStatus indicates bid status.
type BidStatus string

const (
	BidStatusPending   BidStatus = "pending"
	BidStatusAccepted  BidStatus = "accepted"
	BidStatusRejected  BidStatus = "rejected"
	BidStatusOutbid    BidStatus = "outbid"
	BidStatusExpired   BidStatus = "expired"
	BidStatusCancelled BidStatus = "cancelled"
)

// Transfer represents an NFT transfer event.
type Transfer struct {
	ID           string    `json:"id"`
	NFTID        string    `json:"nft_id"`
	TokenID      string    `json:"token_id"`
	ContractAddr string    `json:"contract_address"`
	ChainID      string    `json:"chain_id"`
	From         string    `json:"from"`
	To           string    `json:"to"`
	TxHash       string    `json:"tx_hash"`
	BlockNumber  uint64    `json:"block_number"`
	Timestamp    time.Time `json:"timestamp"`
	Value        *big.Int  `json:"value,omitempty"` // Sale price if applicable
}

// Offer represents an offer to buy an NFT.
type Offer struct {
	ID           string      `json:"id"`
	NFTID        string      `json:"nft_id"`
	CollectionID string      `json:"collection_id,omitempty"` // For collection offers
	Offerer      string      `json:"offerer"`
	Amount       *big.Int    `json:"amount"`
	Currency     string      `json:"currency"`
	AmountUSD    float64     `json:"amount_usd"`
	Status       OfferStatus `json:"status"`
	CreatedAt    time.Time   `json:"created_at"`
	ExpiresAt    time.Time   `json:"expires_at"`
}

// OfferStatus indicates offer status.
type OfferStatus string

const (
	OfferStatusPending   OfferStatus = "pending"
	OfferStatusAccepted  OfferStatus = "accepted"
	OfferStatusRejected  OfferStatus = "rejected"
	OfferStatusExpired   OfferStatus = "expired"
	OfferStatusCancelled OfferStatus = "cancelled"
)

// MetadataUpdate represents metadata update request.
type MetadataUpdate struct {
	NFTID        string      `json:"nft_id"`
	Name         string      `json:"name,omitempty"`
	Description  string      `json:"description,omitempty"`
	ImageURL     string      `json:"image_url,omitempty"`
	AnimationURL string      `json:"animation_url,omitempty"`
	ExternalURL  string      `json:"external_url,omitempty"`
	Attributes   []Attribute `json:"attributes,omitempty"`
}

// Engine interfaces

// CollectionEngine handles NFT collection management.
type CollectionEngine interface {
	engine.ServiceModule

	// CreateCollection creates a new NFT collection.
	CreateCollection(ctx context.Context, chainID string, collection *Collection) (*Collection, error)

	// GetCollection retrieves collection information.
	GetCollection(ctx context.Context, collectionID string) (*Collection, error)

	// GetCollectionByContract retrieves collection by contract address.
	GetCollectionByContract(ctx context.Context, chainID, contractAddr string) (*Collection, error)

	// ListCollections lists collections with filters.
	ListCollections(ctx context.Context, chainID string, creator string, verified bool, limit int) ([]Collection, error)

	// UpdateCollection updates collection metadata.
	UpdateCollection(ctx context.Context, collectionID string, update *Collection) error

	// GetCollectionStats retrieves collection statistics.
	GetCollectionStats(ctx context.Context, collectionID string) (*CollectionStats, error)

	// VerifyCollection marks a collection as verified.
	VerifyCollection(ctx context.Context, collectionID string) error
}

// MintEngine handles NFT minting operations.
type MintEngine interface {
	engine.ServiceModule

	// Mint mints a new NFT.
	Mint(ctx context.Context, req *MintRequest) (*MintResult, error)

	// BatchMint mints multiple NFTs in one transaction.
	BatchMint(ctx context.Context, reqs []*MintRequest) ([]*MintResult, error)

	// Burn burns an NFT.
	Burn(ctx context.Context, chainID, contractAddr, tokenID, owner string) (string, error)

	// EstimateMintGas estimates gas for minting.
	EstimateMintGas(ctx context.Context, req *MintRequest) (uint64, error)

	// GetMintStatus checks the status of a pending mint.
	GetMintStatus(ctx context.Context, txHash string) (*MintResult, error)
}

// TokenEngine handles individual NFT operations.
type TokenEngine interface {
	engine.ServiceModule

	// GetNFT retrieves NFT information.
	GetNFT(ctx context.Context, chainID, contractAddr, tokenID string) (*NFT, error)

	// GetNFTsByOwner retrieves NFTs owned by an address.
	GetNFTsByOwner(ctx context.Context, chainID, owner string, limit, offset int) ([]NFT, error)

	// GetNFTsByCollection retrieves NFTs in a collection.
	GetNFTsByCollection(ctx context.Context, collectionID string, limit, offset int) ([]NFT, error)

	// Transfer transfers an NFT.
	Transfer(ctx context.Context, chainID, contractAddr, tokenID, from, to string) (*Transfer, error)

	// GetTransferHistory retrieves transfer history for an NFT.
	GetTransferHistory(ctx context.Context, chainID, contractAddr, tokenID string, limit int) ([]Transfer, error)

	// GetOwnershipHistory retrieves ownership history for an NFT.
	GetOwnershipHistory(ctx context.Context, chainID, contractAddr, tokenID string) ([]string, error)
}

// MarketplaceEngine handles NFT marketplace operations.
type MarketplaceEngine interface {
	engine.ServiceModule

	// CreateListing creates a new listing.
	CreateListing(ctx context.Context, listing *Listing) (*Listing, error)

	// GetListing retrieves listing information.
	GetListing(ctx context.Context, listingID string) (*Listing, error)

	// ListListings lists active listings with filters.
	ListListings(ctx context.Context, chainID, collectionID string, listingType ListingType, limit int) ([]Listing, error)

	// CancelListing cancels a listing.
	CancelListing(ctx context.Context, listingID, seller string) error

	// UpdateListingPrice updates listing price.
	UpdateListingPrice(ctx context.Context, listingID string, newPrice *big.Int) error

	// Buy executes a purchase.
	Buy(ctx context.Context, listingID, buyer string) (*Transfer, error)

	// PlaceBid places a bid on an auction.
	PlaceBid(ctx context.Context, listingID string, bid *Bid) (*Bid, error)

	// AcceptBid accepts a bid.
	AcceptBid(ctx context.Context, listingID, bidID, seller string) (*Transfer, error)

	// GetBids retrieves bids for a listing.
	GetBids(ctx context.Context, listingID string) ([]Bid, error)

	// MakeOffer makes an offer on an NFT.
	MakeOffer(ctx context.Context, offer *Offer) (*Offer, error)

	// AcceptOffer accepts an offer.
	AcceptOffer(ctx context.Context, offerID, owner string) (*Transfer, error)

	// GetOffers retrieves offers for an NFT or collection.
	GetOffers(ctx context.Context, nftID, collectionID string) ([]Offer, error)
}

// MetadataEngine handles NFT metadata operations.
type MetadataEngine interface {
	engine.ServiceModule

	// GetMetadata retrieves raw metadata for an NFT.
	GetMetadata(ctx context.Context, chainID, contractAddr, tokenID string) (map[string]any, error)

	// UpdateMetadata updates NFT metadata (for mutable NFTs).
	UpdateMetadata(ctx context.Context, update *MetadataUpdate) error

	// RefreshMetadata triggers a metadata refresh from source.
	RefreshMetadata(ctx context.Context, chainID, contractAddr, tokenID string) error

	// GetTokenURI retrieves the token URI.
	GetTokenURI(ctx context.Context, chainID, contractAddr, tokenID string) (string, error)

	// ResolveMetadata resolves metadata from a token URI.
	ResolveMetadata(ctx context.Context, tokenURI string) (map[string]any, error)

	// ValidateMetadata validates metadata against standards.
	ValidateMetadata(ctx context.Context, metadata map[string]any, standard TokenStandard) ([]string, error)
}

// RoyaltyEngine handles royalty configuration and distribution.
type RoyaltyEngine interface {
	engine.ServiceModule

	// GetRoyaltyInfo retrieves royalty info for an NFT.
	GetRoyaltyInfo(ctx context.Context, chainID, contractAddr, tokenID string, salePrice *big.Int) (*Royalty, error)

	// SetRoyalty sets royalty for an NFT or collection.
	SetRoyalty(ctx context.Context, chainID, contractAddr string, tokenID string, royalty *Royalty) error

	// CalculateRoyalty calculates royalty amount for a sale.
	CalculateRoyalty(ctx context.Context, chainID, contractAddr, tokenID string, salePrice *big.Int) (*big.Int, error)

	// GetRoyaltyRecipients retrieves all royalty recipients.
	GetRoyaltyRecipients(ctx context.Context, chainID, contractAddr, tokenID string) ([]Royalty, error)

	// DistributeRoyalty distributes royalties from a sale.
	DistributeRoyalty(ctx context.Context, chainID, contractAddr, tokenID string, salePrice *big.Int) (string, error)
}

// SearchEngine handles NFT search and discovery.
type SearchEngine interface {
	engine.ServiceModule

	// SearchNFTs searches NFTs by query.
	SearchNFTs(ctx context.Context, query string, filters SearchFilters, limit, offset int) ([]NFT, error)

	// SearchCollections searches collections by query.
	SearchCollections(ctx context.Context, query string, limit int) ([]Collection, error)

	// GetTrendingNFTs retrieves trending NFTs.
	GetTrendingNFTs(ctx context.Context, chainID string, period string, limit int) ([]NFT, error)

	// GetTrendingCollections retrieves trending collections.
	GetTrendingCollections(ctx context.Context, chainID string, period string, limit int) ([]Collection, error)

	// GetRecentSales retrieves recent NFT sales.
	GetRecentSales(ctx context.Context, chainID, collectionID string, limit int) ([]Transfer, error)
}

// SearchFilters defines NFT search filters.
type SearchFilters struct {
	ChainID      string            `json:"chain_id,omitempty"`
	CollectionID string            `json:"collection_id,omitempty"`
	Owner        string            `json:"owner,omitempty"`
	Creator      string            `json:"creator,omitempty"`
	MinPrice     float64           `json:"min_price,omitempty"`
	MaxPrice     float64           `json:"max_price,omitempty"`
	Currency     string            `json:"currency,omitempty"`
	Attributes   map[string]any    `json:"attributes,omitempty"`
	Status       []ListingStatus   `json:"status,omitempty"`
	SortBy       string            `json:"sort_by,omitempty"`
	SortOrder    string            `json:"sort_order,omitempty"`
}

// NFTEngine is the composite interface for NFT services.
type NFTEngine interface {
	engine.ServiceModule

	// NFTInfo returns NFT engine information.
	NFTInfo() NFTInfo
}

// NFTInfo provides metadata about an NFT engine implementation.
type NFTInfo struct {
	SupportedStandards []TokenStandard `json:"supported_standards"`
	Chains             []string        `json:"chains"`
	Capabilities       []string        `json:"capabilities"` // collection, mint, marketplace, metadata, royalty, search
	Marketplaces       []string        `json:"marketplaces,omitempty"`
}

// Capability markers

// CollectionCapable indicates collection management support.
type CollectionCapable interface {
	HasCollectionEngine() bool
	CollectionEngine() CollectionEngine
}

// MintCapable indicates minting support.
type MintCapable interface {
	HasMintEngine() bool
	MintEngine() MintEngine
}

// TokenCapable indicates token operations support.
type TokenCapable interface {
	HasTokenEngine() bool
	TokenEngine() TokenEngine
}

// MarketplaceCapable indicates marketplace support.
type MarketplaceCapable interface {
	HasMarketplaceEngine() bool
	MarketplaceEngine() MarketplaceEngine
}

// MetadataCapable indicates metadata support.
type MetadataCapable interface {
	HasMetadataEngine() bool
	MetadataEngine() MetadataEngine
}

// RoyaltyCapable indicates royalty support.
type RoyaltyCapable interface {
	HasRoyaltyEngine() bool
	RoyaltyEngine() RoyaltyEngine
}

// SearchCapable indicates search support.
type SearchCapable interface {
	HasSearchEngine() bool
	SearchEngine() SearchEngine
}
