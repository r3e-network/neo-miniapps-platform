package pricefeed

import "testing"

func TestFeedPairAssignment(t *testing.T) {
	feed := Feed{BaseAsset: "NEO", QuoteAsset: "USD", Pair: "NEO/USD"}
	if feed.Pair != feed.BaseAsset+"/"+feed.QuoteAsset {
		t.Fatalf("expected pair to match derived symbols")
	}
}

func TestSnapshotFields(t *testing.T) {
	snap := Snapshot{Price: 12.34, Source: "oracle"}
	if snap.Price <= 0 {
		t.Fatalf("expected positive price")
	}
	if snap.Source != "oracle" {
		t.Fatalf("expected source to persist")
	}
}
