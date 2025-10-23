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

	if _, err := svc.SetActive(context.Background(), feed.ID, false); err != nil {
		t.Fatalf("disable feed: %v", err)
	}

	if _, err := svc.RecordSnapshot(context.Background(), feed.ID, 12.34, "oracle", time.Now()); err != nil {
		t.Fatalf("record snapshot: %v", err)
	}

	snaps, err := svc.ListSnapshots(context.Background(), feed.ID)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
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
	refresher.Stop(context.Background())

	snaps, err := svc.ListSnapshots(context.Background(), feed.ID)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snaps) == 0 {
		t.Fatalf("expected snapshot recorded")
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
