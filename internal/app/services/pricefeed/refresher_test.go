package pricefeed

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
)

type recordingFetcher struct {
	calls []string
}

func (f *recordingFetcher) Fetch(ctx context.Context, feed pricefeed.Feed) (float64, string, error) {
	f.calls = append(f.calls, feed.ID)
	return 42.0, "test-source", nil
}

func TestRefresherSkipsInactiveFeeds(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, store, nil)
	active, err := svc.CreateFeed(context.Background(), acct.ID, "BTC", "USD", "@every 1m", "@every 5m", 0.5)
	if err != nil {
		t.Fatalf("create active feed: %v", err)
	}
	inactive, err := svc.CreateFeed(context.Background(), acct.ID, "ETH", "USD", "@every 1m", "@every 5m", 0.5)
	if err != nil {
		t.Fatalf("create inactive feed: %v", err)
	}
	if _, err := svc.SetActive(context.Background(), inactive.ID, false); err != nil {
		t.Fatalf("deactivate feed: %v", err)
	}

	refresher := NewRefresher(svc, nil)
	fetcher := &recordingFetcher{}
	refresher.WithFetcher(fetcher)

	refresher.tick(context.Background())

	if len(fetcher.calls) != 1 || fetcher.calls[0] != active.ID {
		t.Fatalf("expected only active feed to be fetched, got %v", fetcher.calls)
	}

	snapshots, err := svc.ListSnapshots(context.Background(), active.ID)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected snapshot for active feed, got %d", len(snapshots))
	}
	if !snapshots[0].CollectedAt.After(time.Time{}) {
		t.Fatalf("snapshot timestamp not set")
	}

	inactiveSnaps, err := svc.ListSnapshots(context.Background(), inactive.ID)
	if err != nil {
		t.Fatalf("list inactive snapshots: %v", err)
	}
	if len(inactiveSnaps) != 0 {
		t.Fatalf("expected no snapshots for inactive feed, got %d", len(inactiveSnaps))
	}
}
