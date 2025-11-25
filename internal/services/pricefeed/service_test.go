package pricefeed

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	pricefeedDomain "github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

func TestService_FeedLifecycle(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, store, nil)
	feed, err := svc.CreateFeed(context.Background(), acct.ID, "neo", "usd", "@every 5m", "@every 1h", 0.5)
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	if !feed.Active || feed.Pair != "NEO/USD" {
		t.Fatalf("unexpected feed state: %#v", feed)
	}

	if _, err := svc.CreateFeed(context.Background(), acct.ID, "NEO", "USD", "@every 5m", "@every 1h", 0.5); err == nil {
		t.Fatalf("expected duplicate pair error")
	}

	newInterval := "@every 10m"
	newHeartbeat := "@every 2h"
	newDeviation := 0.75
	updated, err := svc.UpdateFeed(context.Background(), feed.ID, &newInterval, &newHeartbeat, &newDeviation)
	if err != nil {
		t.Fatalf("update feed: %v", err)
	}
	if updated.UpdateInterval != newInterval || updated.Heartbeat != newHeartbeat || updated.DeviationPercent != newDeviation {
		t.Fatalf("feed update not applied: %#v", updated)
	}

	if _, err := svc.RecordSnapshot(context.Background(), feed.ID, 12.34, "oracle", time.Now()); err != nil {
		t.Fatalf("record snapshot: %v", err)
	}

	rounds, err := svc.ListRounds(context.Background(), feed.ID, 1)
	if err != nil {
		t.Fatalf("list rounds: %v", err)
	}
	if len(rounds) != 1 || rounds[0].AggregatedPrice != 12.34 {
		t.Fatalf("unexpected round data: %#v", rounds)
	}
	obs, err := svc.ListObservations(context.Background(), feed.AccountID, feed.ID, rounds[0].RoundID, 10)
	if err != nil {
		t.Fatalf("list observations: %v", err)
	}
	if len(obs) != 1 || obs[0].Price != 12.34 {
		t.Fatalf("observation mismatch: %#v", obs)
	}

	if _, err := svc.SetActive(context.Background(), feed.ID, false); err != nil {
		t.Fatalf("disable feed: %v", err)
	}

	if _, _, err := svc.SubmitObservation(context.Background(), feed.ID, 15.5, "oracle", time.Now()); err == nil {
		t.Fatalf("expected submit observation to fail for inactive feed")
	}

	snaps, err := svc.ListSnapshots(context.Background(), feed.ID)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
}

func TestService_SubmitObservationThreshold(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	svc.SetMinimumSubmissions(2)

	feed, err := svc.CreateFeed(context.Background(), acct.ID, "neo", "usd", "@every 1m", "@every 5m", 0.5)
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}

	if round, snap, err := svc.SubmitObservation(context.Background(), feed.ID, 10, "provider-a", time.Now()); err != nil {
		t.Fatalf("first observation: %v", err)
	} else if round.Finalized || snap.ID != "" {
		t.Fatalf("expected pending round, got finalized=%v snapshot=%v", round.Finalized, snap.ID)
	}

	if round, snap, err := svc.SubmitObservation(context.Background(), feed.ID, 14, "provider-b", time.Now()); err != nil {
		t.Fatalf("second observation: %v", err)
	} else if !round.Finalized || snap.ID == "" {
		t.Fatalf("expected finalized round with snapshot")
	} else if round.AggregatedPrice != 12 {
		t.Fatalf("unexpected aggregated price: %v", round.AggregatedPrice)
	}
}

func TestService_SubmitObservationDeviationGate(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	svc.SetMinimumSubmissions(1)
	feed, err := svc.CreateFeed(context.Background(), acct.ID, "neo", "usd", "@every 1m", "@every 1h", 1.0)
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}

	if round, snap, err := svc.SubmitObservation(context.Background(), feed.ID, 100, "provider-a", time.Now()); err != nil || !round.Finalized || snap.ID == "" {
		t.Fatalf("expected first round to finalize: round=%#v snap=%#v err=%v", round, snap, err)
	}

	if round, snap, err := svc.SubmitObservation(context.Background(), feed.ID, 100.2, "provider-b", time.Now()); err != nil {
		t.Fatalf("second observation: %v", err)
	} else if round.Finalized || snap.ID != "" {
		t.Fatalf("expected deviation gate to defer publish")
	}

	if round, snap, err := svc.SubmitObservation(context.Background(), feed.ID, 110, "provider-c", time.Now()); err != nil {
		t.Fatalf("third observation: %v", err)
	} else if !round.Finalized || snap.ID == "" {
		t.Fatalf("expected round to finalize after deviation threshold")
	} else if round.AggregatedPrice <= 100 {
		t.Fatalf("expected updated price, got %v", round.AggregatedPrice)
	}
}

func TestService_Refresher(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	feed, err := svc.CreateFeed(context.Background(), acct.ID, "NEO", "USD", "@every 1m", "@every 5m", 0.5)
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	refresher := NewRefresher(svc, nil)
	fetcher := FetcherFunc(func(ctx context.Context, f pricefeedDomain.Feed) (float64, string, error) {
		return 12.34, "test", nil
	})
	refresher.WithFetcher(fetcher)
	refresher.interval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := refresher.Start(ctx); err != nil {
		t.Fatalf("start refresher: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if err := refresher.Stop(context.Background()); err != nil {
		t.Fatalf("stop refresher: %v", err)
	}

	snaps, err := svc.ListSnapshots(context.Background(), feed.ID)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snaps) == 0 {
		t.Fatalf("expected snapshot recorded")
	}
}

func TestService_DeleteFeed(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, store, nil)
	feed, err := svc.CreateFeed(context.Background(), acct.ID, "neo", "usd", "@every 5m", "@every 1h", 0.5)
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}

	// Record some data first
	if _, err := svc.RecordSnapshot(context.Background(), feed.ID, 12.34, "oracle", time.Now()); err != nil {
		t.Fatalf("record snapshot: %v", err)
	}

	// Delete should fail for wrong account
	otherAcct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "other"})
	if err := svc.DeleteFeed(context.Background(), otherAcct.ID, feed.ID); err == nil {
		t.Fatalf("expected delete to fail for wrong account")
	}

	// Delete should succeed for correct account
	if err := svc.DeleteFeed(context.Background(), acct.ID, feed.ID); err != nil {
		t.Fatalf("delete feed: %v", err)
	}

	// Feed should no longer exist
	if _, err := svc.GetFeed(context.Background(), feed.ID); err == nil {
		t.Fatalf("expected feed to be deleted")
	}
}

func TestService_AccountOwnershipValidation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	otherAcct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "other"})

	svc := New(store, store, nil)
	feed, err := svc.CreateFeed(context.Background(), acct.ID, "btc", "usd", "@every 1m", "@every 5m", 0.5)
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}

	// GetFeedForAccount should fail for wrong account
	if _, err := svc.GetFeedForAccount(context.Background(), otherAcct.ID, feed.ID); err == nil {
		t.Fatalf("expected GetFeedForAccount to fail for wrong account")
	}

	// GetFeedForAccount should succeed for correct account
	if _, err := svc.GetFeedForAccount(context.Background(), acct.ID, feed.ID); err != nil {
		t.Fatalf("GetFeedForAccount failed: %v", err)
	}

	// UpdateFeedForAccount should fail for wrong account
	newInterval := "@every 10m"
	if _, err := svc.UpdateFeedForAccount(context.Background(), otherAcct.ID, feed.ID, &newInterval, nil, nil); err == nil {
		t.Fatalf("expected UpdateFeedForAccount to fail for wrong account")
	}

	// SetActiveForAccount should fail for wrong account
	if _, err := svc.SetActiveForAccount(context.Background(), otherAcct.ID, feed.ID, false); err == nil {
		t.Fatalf("expected SetActiveForAccount to fail for wrong account")
	}
}

func TestService_HealthCheck(t *testing.T) {
	store := memory.New()
	svc := New(store, store, nil)

	hc := svc.HealthCheck(context.Background())
	if hc.Service != "pricefeed" {
		t.Errorf("expected service name 'pricefeed', got %s", hc.Service)
	}
	if !hc.IsHealthy() {
		t.Errorf("expected healthy status, got %s", hc.Status)
	}
}

func ExampleService_CreateFeed() {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "desk"})
	log := logger.NewDefault("example-pricefeed")
	log.SetOutput(io.Discard)
	svc := New(store, store, log)
	feed, _ := svc.CreateFeed(context.Background(), acct.ID, "btc", "usd", "@every 1m", "@every 5m", 0.5)
	fmt.Println(feed.Pair, feed.Active)
	// Output:
	// BTC/USD true
}
