package datafeeds

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domaindf "github.com/R3E-Network/service_layer/internal/app/domain/datafeeds"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
)

const testFeedSigner = "0xbbbbccccddddeeeeffffaaaabbbbccccddddeeee"

func TestService_CreateFeed(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testFeedSigner}); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	feed, err := svc.CreateFeed(context.Background(), domaindf.Feed{
		AccountID: acct.ID,
		Pair:      "ETH/USD",
		Decimals:  8,
		SignerSet: []string{testFeedSigner},
	})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	if feed.Pair != "ETH/USD" {
		t.Fatalf("expected pair normalized to upper, got %s", feed.Pair)
	}

	feeds, err := svc.ListFeeds(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list feeds: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("expected 1 feed, got %d", len(feeds))
	}
}

func TestService_CreateFeedValidation(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	if _, err := svc.CreateFeed(context.Background(), domaindf.Feed{AccountID: acct.ID, Decimals: 0}); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestService_CreateFeedRequiresRegisteredSigners(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	_, err = svc.CreateFeed(context.Background(), domaindf.Feed{
		AccountID: acct.ID,
		Pair:      "ETH/USD",
		Decimals:  8,
		SignerSet: []string{"unknown"},
	})
	if err == nil {
		t.Fatalf("expected signer validation error")
	}
}

func TestService_SubmitUpdate(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	feed, err := svc.CreateFeed(context.Background(), domaindf.Feed{
		AccountID: acct.ID,
		Pair:      "BTC/USD",
		Decimals:  8,
	})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}

	upd, err := svc.SubmitUpdate(context.Background(), acct.ID, feed.ID, 1, "35000", time.Now(), "", "sig", map[string]string{"Env": "Prod"})
	if err != nil {
		t.Fatalf("submit update: %v", err)
	}
	if upd.RoundID != 1 {
		t.Fatalf("unexpected round id")
	}

	if _, err := svc.SubmitUpdate(context.Background(), acct.ID, feed.ID, 1, "35100", time.Now(), "", "sig", nil); err == nil {
		t.Fatalf("expected round monotonicity error")
	}

	updates, err := svc.ListUpdates(context.Background(), acct.ID, feed.ID, 10)
	if err != nil {
		t.Fatalf("list updates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update")
	}

	latest, err := svc.LatestUpdate(context.Background(), acct.ID, feed.ID)
	if err != nil {
		t.Fatalf("latest update: %v", err)
	}
	if latest.ID != upd.ID {
		t.Fatalf("expected latest to match first update")
	}
}

func TestService_SubmitUpdateSignerVerificationAndAggregation(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: testFeedSigner}); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	secondSigner := "0x1111aaaa2222bbbb3333cccc4444dddd5555eeee"
	if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: secondSigner}); err != nil {
		t.Fatalf("seed wallet2: %v", err)
	}
	svc.WithAggregationConfig(2, "median")
	feed, err := svc.CreateFeed(context.Background(), domaindf.Feed{
		AccountID: acct.ID,
		Pair:      "ETH/USD",
		Decimals:  2,
		SignerSet: []string{testFeedSigner, secondSigner},
	})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}

	if _, err := svc.SubmitUpdate(context.Background(), acct.ID, feed.ID, 1, "100.00", time.Now(), "unknown", "sig", nil); err == nil {
		t.Fatalf("expected unknown signer to be rejected")
	}

	first, err := svc.SubmitUpdate(context.Background(), acct.ID, feed.ID, 1, "100.00", time.Now(), testFeedSigner, "sig1", nil)
	if err != nil {
		t.Fatalf("submit first signer: %v", err)
	}
	if first.Status != domaindf.UpdateStatusPending {
		t.Fatalf("expected pending status before quorum, got %s", first.Status)
	}

	second, err := svc.SubmitUpdate(context.Background(), acct.ID, feed.ID, 1, "200.00", time.Now(), secondSigner, "sig2", nil)
	if err != nil {
		t.Fatalf("submit second signer: %v", err)
	}
	if second.Status != domaindf.UpdateStatusAccepted {
		t.Fatalf("expected accepted after quorum, got %s", second.Status)
	}
	if agg := second.Metadata["aggregated_price"]; agg != "150" && agg != "150.00" {
		t.Fatalf("expected aggregated median price, got %q", agg)
	}
	if second.Metadata["quorum_met"] != "true" {
		t.Fatalf("expected quorum metadata flag")
	}
}

func TestService_SubmitUpdateUnknownAggregationDefaultsToMedian(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)

	signerA := testFeedSigner
	signerB := "0xffffeeeeaaaa9999888877776666555544443333"
	for _, signer := range []string{signerA, signerB} {
		if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: signer}); err != nil {
			t.Fatalf("seed wallet %s: %v", signer, err)
		}
	}

	svc.WithAggregationConfig(2, "bogus-strategy") // unsupported strategy should fall back to median
	feed, err := svc.CreateFeed(context.Background(), domaindf.Feed{
		AccountID: acct.ID,
		Pair:      "N3/USD",
		Decimals:  2,
		SignerSet: []string{signerA, signerB},
	})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}

	_, err = svc.SubmitUpdate(context.Background(), acct.ID, feed.ID, 1, "10.00", time.Now(), signerA, "sig-a", nil)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	second, err := svc.SubmitUpdate(context.Background(), acct.ID, feed.ID, 1, "20.00", time.Now(), signerB, "sig-b", nil)
	if err != nil {
		t.Fatalf("second update: %v", err)
	}
	if second.Metadata["aggregation"] != "median" {
		t.Fatalf("expected aggregation metadata to reflect median fallback, got %q", second.Metadata["aggregation"])
	}
	if second.Metadata["aggregated_price"] != "15" && second.Metadata["aggregated_price"] != "15.00" {
		t.Fatalf("expected median aggregated price, got %q", second.Metadata["aggregated_price"])
	}
	if second.Status != domaindf.UpdateStatusAccepted {
		t.Fatalf("expected accepted status after quorum")
	}
}

func TestService_SubmitUpdateMeanAggregation(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	svc.WithWorkspaceWallets(store)
	signerA := testFeedSigner
	signerB := "0xaaaabbbbccccddddeeeeffff0000111122223333"
	for _, signer := range []string{signerA, signerB} {
		if _, err := store.CreateWorkspaceWallet(context.Background(), account.WorkspaceWallet{WorkspaceID: acct.ID, WalletAddress: signer}); err != nil {
			t.Fatalf("seed wallet %s: %v", signer, err)
		}
	}
	svc.WithAggregationConfig(2, "mean")
	feed, err := svc.CreateFeed(context.Background(), domaindf.Feed{
		AccountID: acct.ID,
		Pair:      "GAS/USD",
		Decimals:  2,
		SignerSet: []string{signerA, signerB},
	})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}

	if _, err := svc.SubmitUpdate(context.Background(), acct.ID, feed.ID, 1, "10.00", time.Now(), signerA, "sig-a", nil); err != nil {
		t.Fatalf("first update: %v", err)
	}
	second, err := svc.SubmitUpdate(context.Background(), acct.ID, feed.ID, 1, "20.00", time.Now(), signerB, "sig-b", nil)
	if err != nil {
		t.Fatalf("second update: %v", err)
	}
	if second.Metadata["aggregation"] != "mean" {
		t.Fatalf("expected aggregation metadata mean, got %q", second.Metadata["aggregation"])
	}
	if second.Metadata["aggregated_price"] != "15" && second.Metadata["aggregated_price"] != "15.00" {
		t.Fatalf("expected mean aggregated price, got %q", second.Metadata["aggregated_price"])
	}
}

func TestService_SubmitUpdateHeartbeatDeviation(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	now := time.Now()
	feed, err := svc.CreateFeed(context.Background(), domaindf.Feed{
		AccountID:    acct.ID,
		Pair:         "BTC/USD",
		Decimals:     2,
		Heartbeat:    time.Minute,
		ThresholdPPM: 50000, // 5%
	})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	_, err = svc.SubmitUpdate(context.Background(), acct.ID, feed.ID, 1, "100.00", now, "", "", nil)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	if _, err := svc.SubmitUpdate(context.Background(), acct.ID, feed.ID, 2, "101.00", now.Add(30*time.Second), "", "", nil); err == nil {
		t.Fatalf("expected deviation/heartbeat rejection before heartbeat interval")
	}
	if _, err := svc.SubmitUpdate(context.Background(), acct.ID, feed.ID, 2, "103.00", now.Add(30*time.Second), "", "", nil); err == nil {
		t.Fatalf("expected deviation rejection below threshold")
	}
	if _, err := svc.SubmitUpdate(context.Background(), acct.ID, feed.ID, 2, "101.00", now.Add(2*time.Minute), "", "", nil); err != nil {
		t.Fatalf("expected heartbeat to allow next round, got %v", err)
	}
}
