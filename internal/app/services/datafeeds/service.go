package datafeeds

import (
	"context"
	"fmt"
	"strings"
	"time"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	domaindf "github.com/R3E-Network/service_layer/internal/app/domain/datafeeds"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service manages centralized Chainlink data feeds per account.
type Service struct {
	base  *core.Base
	store storage.DataFeedStore
	log   *logger.Logger
	hooks core.ObservationHooks
}

// New constructs a data feed service.
func New(accounts storage.AccountStore, store storage.DataFeedStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("datafeeds")
	}
	return &Service{base: core.NewBase(accounts), store: store, log: log, hooks: core.NoopObservationHooks}
}

// WithWorkspaceWallets enforces signer set ownership when provided.
func (s *Service) WithWorkspaceWallets(store storage.WorkspaceWalletStore) {
	s.base.SetWallets(store)
}

// WithObservationHooks configures observability callbacks for updates.
func (s *Service) WithObservationHooks(h core.ObservationHooks) {
	if h.OnStart == nil && h.OnComplete == nil {
		s.hooks = core.NoopObservationHooks
		return
	}
	s.hooks = h
}

// CreateFeed validates and creates a feed.
func (s *Service) CreateFeed(ctx context.Context, feed domaindf.Feed) (domaindf.Feed, error) {
	if err := s.base.EnsureAccount(ctx, feed.AccountID); err != nil {
		return domaindf.Feed{}, err
	}
	if err := s.normalizeFeed(&feed); err != nil {
		return domaindf.Feed{}, err
	}
	if err := s.base.EnsureSignersOwned(ctx, feed.AccountID, feed.SignerSet); err != nil {
		return domaindf.Feed{}, err
	}
	created, err := s.store.CreateDataFeed(ctx, feed)
	if err != nil {
		return domaindf.Feed{}, err
	}
	s.log.WithField("feed_id", created.ID).WithField("account_id", created.AccountID).Info("data feed created")
	return created, nil
}

// UpdateFeed updates mutable fields on a feed.
func (s *Service) UpdateFeed(ctx context.Context, feed domaindf.Feed) (domaindf.Feed, error) {
	stored, err := s.store.GetDataFeed(ctx, feed.ID)
	if err != nil {
		return domaindf.Feed{}, err
	}
	if stored.AccountID != feed.AccountID {
		return domaindf.Feed{}, fmt.Errorf("feed %s does not belong to account %s", feed.ID, feed.AccountID)
	}
	feed.AccountID = stored.AccountID
	if err := s.normalizeFeed(&feed); err != nil {
		return domaindf.Feed{}, err
	}
	if err := s.base.EnsureSignersOwned(ctx, feed.AccountID, feed.SignerSet); err != nil {
		return domaindf.Feed{}, err
	}
	updated, err := s.store.UpdateDataFeed(ctx, feed)
	if err != nil {
		return domaindf.Feed{}, err
	}
	s.log.WithField("feed_id", feed.ID).WithField("account_id", feed.AccountID).Info("data feed updated")
	return updated, nil
}

// GetFeed fetches a feed ensuring ownership.
func (s *Service) GetFeed(ctx context.Context, accountID, feedID string) (domaindf.Feed, error) {
	feed, err := s.store.GetDataFeed(ctx, feedID)
	if err != nil {
		return domaindf.Feed{}, err
	}
	if feed.AccountID != accountID {
		return domaindf.Feed{}, fmt.Errorf("feed %s does not belong to account %s", feedID, accountID)
	}
	return feed, nil
}

// ListFeeds lists feeds for an account.
func (s *Service) ListFeeds(ctx context.Context, accountID string) ([]domaindf.Feed, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListDataFeeds(ctx, accountID)
}

// SubmitUpdate stores a price update for a feed.
func (s *Service) SubmitUpdate(ctx context.Context, accountID, feedID string, roundID int64, price string, ts time.Time, signature string, metadata map[string]string) (domaindf.Update, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return domaindf.Update{}, err
	}
	feed, err := s.store.GetDataFeed(ctx, feedID)
	if err != nil {
		return domaindf.Update{}, err
	}
	if feed.AccountID != accountID {
		return domaindf.Update{}, fmt.Errorf("feed %s does not belong to account %s", feedID, accountID)
	}
	if strings.TrimSpace(price) == "" {
		return domaindf.Update{}, fmt.Errorf("price is required")
	}
	if roundID <= 0 {
		return domaindf.Update{}, fmt.Errorf("round_id must be positive")
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	latest, err := s.store.GetLatestDataFeedUpdate(ctx, feedID)
	if err == nil {
		if roundID <= latest.RoundID {
			return domaindf.Update{}, fmt.Errorf("round_id must be greater than %d", latest.RoundID)
		}
	}

	upd := domaindf.Update{
		AccountID: accountID,
		FeedID:    feedID,
		RoundID:   roundID,
		Price:     strings.TrimSpace(price),
		Timestamp: ts.UTC(),
		Signature: strings.TrimSpace(signature),
		Status:    domaindf.UpdateStatusAccepted,
		Metadata:  core.NormalizeMetadata(metadata),
	}
	attrs := map[string]string{"feed_id": feedID}
	finish := core.StartObservation(ctx, s.hooks, attrs)
	created, err := s.store.CreateDataFeedUpdate(ctx, upd)
	if err != nil {
		finish(err)
		return domaindf.Update{}, err
	}
	finish(nil)
	s.log.WithField("feed_id", feedID).WithField("round_id", roundID).Info("data feed update stored")
	return created, nil
}

// ListUpdates lists recent updates for a feed.
func (s *Service) ListUpdates(ctx context.Context, accountID, feedID string, limit int) ([]domaindf.Update, error) {
	if _, err := s.GetFeed(ctx, accountID, feedID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListDataFeedUpdates(ctx, feedID, clamped)
}

// LatestUpdate returns the latest accepted update.
func (s *Service) LatestUpdate(ctx context.Context, accountID, feedID string) (domaindf.Update, error) {
	if _, err := s.GetFeed(ctx, accountID, feedID); err != nil {
		return domaindf.Update{}, err
	}
	return s.store.GetLatestDataFeedUpdate(ctx, feedID)
}

// Descriptor advertises the service placement and capabilities.
func (s *Service) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "datafeeds",
		Domain:       "datafeeds",
		Layer:        core.LayerPlatform,
		Capabilities: []string{"registry", "ingest", "dispatch"},
	}
}

func (s *Service) normalizeFeed(feed *domaindf.Feed) error {
	feed.Pair = strings.ToUpper(strings.TrimSpace(feed.Pair))
	feed.Description = strings.TrimSpace(feed.Description)
	feed.Metadata = core.NormalizeMetadata(feed.Metadata)
	feed.Tags = core.NormalizeTags(feed.Tags)
	feed.SignerSet = core.NormalizeTags(feed.SignerSet)
	if feed.Pair == "" {
		return fmt.Errorf("pair is required")
	}
	if feed.Decimals <= 0 {
		return fmt.Errorf("decimals must be positive")
	}
	if feed.Heartbeat <= 0 {
		feed.Heartbeat = time.Minute
	}
	if feed.ThresholdPPM < 0 {
		feed.ThresholdPPM = 0
	}
	return nil
}
