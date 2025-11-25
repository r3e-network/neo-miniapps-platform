package nft

import (
	"math/big"
	"testing"
	"time"
)

func TestNFT(t *testing.T) {
	nft := NFT{
		ID:           "nft-1",
		TokenID:      "1234",
		ContractAddr: "0xabcd",
		ChainID:      "neo-mainnet",
		CollectionID: "collection-1",
		Owner:        "NeoAddr123",
		Creator:      "NeoAddr456",
		Standard:     StandardNEP11,
		Name:         "Cyber Punk #1234",
		Description:  "A unique digital artwork",
		ImageURL:     "https://example.com/nft.png",
		Attributes: []Attribute{
			{TraitType: "Background", Value: "Blue"},
			{TraitType: "Rarity", Value: 95, DisplayType: "number"},
		},
		Royalty: &Royalty{
			Recipient: "NeoAddr456",
			Percent:   2.5,
		},
		MintedAt: time.Now(),
	}

	if nft.Standard != StandardNEP11 {
		t.Errorf("expected NEP11, got %s", nft.Standard)
	}
	if len(nft.Attributes) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(nft.Attributes))
	}
}

func TestTokenStandards(t *testing.T) {
	standards := []TokenStandard{
		StandardERC721,
		StandardERC1155,
		StandardNEP11,
		StandardNEP17,
		StandardMetaplex,
		StandardSPL,
	}

	for _, s := range standards {
		nft := NFT{Standard: s}
		if nft.Standard != s {
			t.Errorf("expected %s, got %s", s, nft.Standard)
		}
	}
}

func TestAttribute(t *testing.T) {
	attrs := []Attribute{
		{TraitType: "Color", Value: "Red"},
		{TraitType: "Power", Value: 100, DisplayType: "number", MaxValue: 200},
		{TraitType: "Level", Value: 5, DisplayType: "boost_number"},
	}

	if attrs[0].Value != "Red" {
		t.Errorf("expected Red, got %v", attrs[0].Value)
	}
	if attrs[1].MaxValue != 200 {
		t.Errorf("expected max 200, got %v", attrs[1].MaxValue)
	}
}

func TestRoyalty(t *testing.T) {
	royalty := Royalty{
		Recipient: "creator-addr",
		Percent:   5.0,
		Amount:    big.NewInt(50000000),
	}

	if royalty.Percent != 5.0 {
		t.Errorf("expected 5.0%%, got %f%%", royalty.Percent)
	}
}

func TestCollection(t *testing.T) {
	collection := Collection{
		ID:           "collection-1",
		Name:         "Neo Punks",
		Symbol:       "NPUNK",
		Description:  "10,000 unique punks on Neo",
		ContractAddr: "0x1234",
		ChainID:      "neo-mainnet",
		Creator:      "NeoAddr123",
		Owner:        "NeoAddr123",
		Standard:     StandardNEP11,
		ImageURL:     "https://example.com/collection.png",
		TotalSupply:  5000,
		MaxSupply:    10000,
		Royalty:      &Royalty{Recipient: "NeoAddr123", Percent: 2.5},
		Categories:   []string{"PFP", "Art"},
		Verified:     true,
		CreatedAt:    time.Now(),
		Stats: CollectionStats{
			FloorPrice:   100.0,
			TotalVolume:  500000.0,
			Volume24h:    10000.0,
			NumOwners:    3000,
			NumListed:    500,
		},
	}

	if collection.Standard != StandardNEP11 {
		t.Errorf("expected NEP11, got %s", collection.Standard)
	}
	if collection.TotalSupply != 5000 {
		t.Errorf("expected 5000 supply, got %d", collection.TotalSupply)
	}
	if collection.Stats.FloorPrice != 100.0 {
		t.Errorf("expected floor 100, got %f", collection.Stats.FloorPrice)
	}
}

func TestMintRequest(t *testing.T) {
	req := MintRequest{
		CollectionID: "collection-1",
		Recipient:    "NeoAddr123",
		Name:         "Neo Punk #5001",
		Description:  "A new punk",
		ImageURL:     "https://example.com/5001.png",
		Attributes:   []Attribute{{TraitType: "Hat", Value: "Crown"}},
		Royalty:      &Royalty{Recipient: "creator", Percent: 2.5},
		Amount:       1,
	}

	if req.CollectionID != "collection-1" {
		t.Errorf("expected collection-1, got %s", req.CollectionID)
	}
}

func TestMintResult(t *testing.T) {
	result := MintResult{
		NFT: &NFT{
			ID:      "nft-5001",
			TokenID: "5001",
		},
		TxHash:  "0xabcdef",
		GasUsed: 150000,
	}

	if result.TxHash != "0xabcdef" {
		t.Errorf("expected tx hash 0xabcdef, got %s", result.TxHash)
	}
}

func TestListing(t *testing.T) {
	listing := Listing{
		ID:           "listing-1",
		NFTID:        "nft-1",
		TokenID:      "1234",
		ContractAddr: "0xabcd",
		ChainID:      "neo-mainnet",
		Seller:       "NeoAddr123",
		Price:        big.NewInt(1000000000000),
		Currency:     "GAS",
		PriceUSD:     150.0,
		ListingType:  ListingTypeFixedPrice,
		Status:       ListingStatusActive,
		Quantity:     1,
		StartTime:    time.Now(),
		CreatedAt:    time.Now(),
	}

	if listing.ListingType != ListingTypeFixedPrice {
		t.Errorf("expected fixed_price, got %s", listing.ListingType)
	}
	if listing.Status != ListingStatusActive {
		t.Errorf("expected active, got %s", listing.Status)
	}
}

func TestListingTypes(t *testing.T) {
	types := []ListingType{
		ListingTypeFixedPrice,
		ListingTypeAuction,
		ListingTypeDutchAuction,
	}

	for _, lt := range types {
		listing := Listing{ListingType: lt}
		if listing.ListingType != lt {
			t.Errorf("expected %s, got %s", lt, listing.ListingType)
		}
	}
}

func TestBid(t *testing.T) {
	bid := Bid{
		ID:        "bid-1",
		ListingID: "listing-1",
		Bidder:    "NeoAddr456",
		Amount:    big.NewInt(2000000000000),
		Currency:  "GAS",
		AmountUSD: 300.0,
		Status:    BidStatusPending,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if bid.Status != BidStatusPending {
		t.Errorf("expected pending, got %s", bid.Status)
	}
}

func TestBidStatuses(t *testing.T) {
	statuses := []BidStatus{
		BidStatusPending,
		BidStatusAccepted,
		BidStatusRejected,
		BidStatusOutbid,
		BidStatusExpired,
		BidStatusCancelled,
	}

	for _, s := range statuses {
		bid := Bid{Status: s}
		if bid.Status != s {
			t.Errorf("expected %s, got %s", s, bid.Status)
		}
	}
}

func TestTransfer(t *testing.T) {
	transfer := Transfer{
		ID:           "transfer-1",
		NFTID:        "nft-1",
		TokenID:      "1234",
		ContractAddr: "0xabcd",
		ChainID:      "neo-mainnet",
		From:         "NeoAddr123",
		To:           "NeoAddr456",
		TxHash:       "0xdefabc",
		BlockNumber:  12345678,
		Timestamp:    time.Now(),
		Value:        big.NewInt(1500000000000),
	}

	if transfer.From != "NeoAddr123" {
		t.Errorf("expected from NeoAddr123, got %s", transfer.From)
	}
	if transfer.To != "NeoAddr456" {
		t.Errorf("expected to NeoAddr456, got %s", transfer.To)
	}
}

func TestOffer(t *testing.T) {
	offer := Offer{
		ID:           "offer-1",
		NFTID:        "nft-1",
		CollectionID: "collection-1",
		Offerer:      "NeoAddr789",
		Amount:       big.NewInt(1000000000000),
		Currency:     "GAS",
		AmountUSD:    150.0,
		Status:       OfferStatusPending,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(72 * time.Hour),
	}

	if offer.Status != OfferStatusPending {
		t.Errorf("expected pending, got %s", offer.Status)
	}
}

func TestSearchFilters(t *testing.T) {
	filters := SearchFilters{
		ChainID:      "neo-mainnet",
		CollectionID: "collection-1",
		MinPrice:     10.0,
		MaxPrice:     1000.0,
		Currency:     "GAS",
		Attributes:   map[string]any{"trait": "rare"},
		Status:       []ListingStatus{ListingStatusActive},
		SortBy:       "price",
		SortOrder:    "asc",
	}

	if filters.MinPrice != 10.0 {
		t.Errorf("expected min 10, got %f", filters.MinPrice)
	}
	if len(filters.Status) != 1 {
		t.Errorf("expected 1 status, got %d", len(filters.Status))
	}
}

func TestNFTInfo(t *testing.T) {
	info := NFTInfo{
		SupportedStandards: []TokenStandard{StandardNEP11, StandardERC721},
		Chains:             []string{"neo-mainnet", "eth-mainnet"},
		Capabilities:       []string{"collection", "mint", "marketplace", "metadata"},
		Marketplaces:       []string{"opensea", "ghostmarket"},
	}

	if len(info.SupportedStandards) != 2 {
		t.Errorf("expected 2 standards, got %d", len(info.SupportedStandards))
	}
	if len(info.Capabilities) != 4 {
		t.Errorf("expected 4 capabilities, got %d", len(info.Capabilities))
	}
}
