package datafeeds

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/pkg/metrics"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Compile-time check: Service exposes Publish for the core engine adapter.
type eventPublisher interface {
	Publish(context.Context, string, any) error
}

var _ eventPublisher = (*Service)(nil)

// Service manages centralized Chainlink data feeds per account.
type Service struct {
	framework.ServiceBase
	accounts AccountChecker
	wallets  WalletChecker
	store    Store
	log      *logger.Logger
	hooks    core.ObservationHooks
	// aggregation defaults
	minSigners  int
	aggregation string
}

// Name returns the stable engine module name.
func (s *Service) Name() string { return "datafeeds" }

// Domain reports the service domain for engine grouping.
func (s *Service) Domain() string { return "datafeeds" }

// Manifest describes the service contract for the engine OS.
func (s *Service) Manifest() *framework.Manifest {
	return &framework.Manifest{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Description:  "Aggregated data feed definitions and updates",
		Layer:        "service",
		DependsOn:    []string{"store", "svc-accounts"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceData},
		Capabilities: []string{"datafeeds"},
	}
}

// Descriptor advertises the service for system discovery.
func (s *Service) Descriptor() core.Descriptor { return s.Manifest().ToDescriptor() }

// New constructs a data feed service.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("datafeeds")
	}
	svc := &Service{accounts: accounts, store: store, log: log, hooks: core.NoopObservationHooks}
	svc.SetName(svc.Name())
	return svc
}

// WithWalletChecker injects a wallet checker for ownership validation.
func (s *Service) WithWalletChecker(w WalletChecker) {
	s.wallets = w
}

// WithAggregationConfig sets baseline aggregation parameters.
func (s *Service) WithAggregationConfig(minSigners int, aggregation string) {
	if minSigners > 0 {
		s.minSigners = minSigners
	}
	agg, err := normalizeAggregation(aggregation)
	if err != nil {
		s.log.WithField("aggregation", aggregation).Warn("unsupported datafeed aggregation; defaulting to median")
	}
	s.aggregation = agg
}

// WithObservationHooks configures observability callbacks for updates.
func (s *Service) WithObservationHooks(h core.ObservationHooks) {
	s.hooks = core.NormalizeHooks(h)
}

// Start/Stop/Ready are inherited from framework.ServiceBase.

// Publish implements EventEngine: accept a feed update event.
func (s *Service) Publish(ctx context.Context, event string, payload any) error {
	if strings.ToLower(strings.TrimSpace(event)) != "update" {
		return fmt.Errorf("unsupported event: %s", event)
	}
	body, ok := payload.(map[string]any)
	if !ok {
		return fmt.Errorf("payload must be a map")
	}
	accountID, _ := body["account_id"].(string)
	feedID, _ := body["feed_id"].(string)
	priceStr, _ := body["price"].(string)
	roundID, _ := body["round_id"].(int64)
	if roundID == 0 {
		if rid, ok := body["round_id"].(float64); ok {
			roundID = int64(rid)
		}
	}
	if accountID == "" || feedID == "" || priceStr == "" || roundID <= 0 {
		return fmt.Errorf("account_id, feed_id, price, round_id required")
	}
	_, err := s.SubmitUpdate(ctx, accountID, feedID, roundID, priceStr, time.Now().UTC(), "submitted", "", nil)
	return err
}

// CreateFeed validates and creates a feed.
func (s *Service) CreateFeed(ctx context.Context, feed Feed) (Feed, error) {
	if err := s.accounts.AccountExists(ctx, feed.AccountID); err != nil {
		return Feed{}, err
	}
	if err := s.normalizeFeed(&feed); err != nil {
		return Feed{}, err
	}
	if err := s.ensureSignersOwned(ctx, feed.AccountID, feed.SignerSet); err != nil {
		return Feed{}, err
	}
	created, err := s.store.CreateDataFeed(ctx, feed)
	if err != nil {
		return Feed{}, err
	}
	s.log.WithField("feed_id", created.ID).WithField("account_id", created.AccountID).Info("data feed created")
	return created, nil
}

// UpdateFeed updates mutable fields on a feed.
func (s *Service) UpdateFeed(ctx context.Context, feed Feed) (Feed, error) {
	stored, err := s.store.GetDataFeed(ctx, feed.ID)
	if err != nil {
		return Feed{}, err
	}
	if err := core.EnsureOwnership(stored.AccountID, feed.AccountID, "feed", feed.ID); err != nil {
		return Feed{}, err
	}
	feed.AccountID = stored.AccountID
	if err := s.normalizeFeed(&feed); err != nil {
		return Feed{}, err
	}
	if err := s.ensureSignersOwned(ctx, feed.AccountID, feed.SignerSet); err != nil {
		return Feed{}, err
	}
	start := time.Now()
	s.hooks.OnStart(ctx, map[string]string{"account_id": feed.AccountID, "feed_id": feed.ID})
	defer func() {
		s.hooks.OnComplete(ctx, map[string]string{"account_id": feed.AccountID, "feed_id": feed.ID}, nil, time.Since(start))
	}()
	updated, err := s.store.UpdateDataFeed(ctx, feed)
	if err != nil {
		return Feed{}, err
	}
	s.log.WithField("feed_id", feed.ID).WithField("account_id", feed.AccountID).Info("data feed updated")
	return updated, nil
}

// GetFeed fetches a feed ensuring ownership.
func (s *Service) GetFeed(ctx context.Context, accountID, feedID string) (Feed, error) {
	feed, err := s.store.GetDataFeed(ctx, feedID)
	if err != nil {
		return Feed{}, err
	}
	if err := core.EnsureOwnership(feed.AccountID, accountID, "feed", feedID); err != nil {
		return Feed{}, err
	}
	return feed, nil
}

// ListFeeds lists feeds for an account.
func (s *Service) ListFeeds(ctx context.Context, accountID string) ([]Feed, error) {
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListDataFeeds(ctx, accountID)
}

// SubmitUpdate stores a price update for a feed, enforcing signer verification,
// heartbeat/deviation thresholds, and the configured aggregation strategy.
func (s *Service) SubmitUpdate(ctx context.Context, accountID, feedID string, roundID int64, price string, ts time.Time, signer string, signature string, metadata map[string]string) (Update, error) {
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return Update{}, err
	}
	feed, err := s.store.GetDataFeed(ctx, feedID)
	if err != nil {
		return Update{}, err
	}
	if err := core.EnsureOwnership(feed.AccountID, accountID, "feed", feedID); err != nil {
		return Update{}, err
	}
	if roundID <= 0 {
		return Update{}, fmt.Errorf("round_id must be positive")
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	ts = ts.UTC()

	signer = strings.TrimSpace(signer)
	if len(feed.SignerSet) > 0 {
		if signer == "" {
			return Update{}, core.RequiredError("signer")
		}
		if !core.ContainsCaseInsensitive(feed.SignerSet, signer) {
			return Update{}, fmt.Errorf("signer %s is not authorized for feed %s", signer, feedID)
		}
	}

	sig := strings.TrimSpace(signature)
	if signer != "" && sig == "" {
		return Update{}, fmt.Errorf("signature is required for signer submissions")
	}

	priceInt, normalizedPrice, err := normalizePrice(price, feed.Decimals)
	if err != nil {
		return Update{}, err
	}

	latest, err := s.store.GetLatestDataFeedUpdate(ctx, feedID)
	if err == nil {
		if roundID < latest.RoundID {
			return Update{}, fmt.Errorf("round_id must be at least %d", latest.RoundID)
		}
		if roundID > latest.RoundID {
			latestInt, _, parseErr := normalizePrice(latest.Price, feed.Decimals)
			if parseErr == nil && !shouldPublishDatafeed(latestInt, priceInt, latest.Timestamp, ts, feed.ThresholdPPM, feed.Heartbeat) {
				return Update{}, fmt.Errorf("heartbeat/deviation thresholds not met for new round %d", roundID)
			}
		}
	}

	existingRound, err := s.store.ListDataFeedUpdatesByRound(ctx, feedID, roundID)
	if err != nil {
		return Update{}, err
	}
	for _, upd := range existingRound {
		if signer != "" && strings.EqualFold(upd.Signer, signer) {
			return Update{}, fmt.Errorf("signer %s already submitted for round %d", signer, roundID)
		}
	}

	meta := core.NormalizeMetadata(metadata)
	if meta == nil {
		meta = make(map[string]string)
	}
	aggregation := feed.Aggregation
	if aggregation == "" {
		aggregation = s.aggregation
	}
	aggregation, aggErr := normalizeAggregation(aggregation)
	if aggErr != nil {
		s.log.WithField("aggregation", aggregation).WithError(aggErr).Warn("falling back to median aggregation")
	}

	threshold := s.signerThreshold(feed)
	submissions := len(existingRound) + 1
	meta["aggregation"] = aggregation
	meta["signer_count"] = strconv.Itoa(submissions)
	meta["quorum"] = strconv.Itoa(threshold)

	allPrices := append(make([]*big.Int, 0, len(existingRound)+1), priceInt)
	for _, upd := range existingRound {
		parsed, parseErr := normalizePriceInt(upd.Price, feed.Decimals)
		if parseErr != nil {
			continue
		}
		allPrices = append(allPrices, parsed)
	}

	status := UpdateStatusPending
	if submissions >= threshold {
		status = UpdateStatusAccepted
		aggPrice := aggregatePrices(allPrices, aggregation)
		meta["aggregated_price"] = formatPrice(aggPrice, feed.Decimals)
		meta["quorum_met"] = "true"
	} else {
		meta["quorum_met"] = "false"
	}

	upd := Update{
		AccountID: accountID,
		FeedID:    feedID,
		RoundID:   roundID,
		Price:     normalizedPrice,
		Signer:    signer,
		Timestamp: ts,
		Signature: sig,
		Status:    status,
		Metadata:  meta,
	}
	attrs := map[string]string{"feed_id": feedID}
	finish := core.StartObservation(ctx, s.hooks, attrs)
	created, err := s.store.CreateDataFeedUpdate(ctx, upd)
	if err != nil {
		finish(err)
		return Update{}, err
	}
	finish(nil)
	s.log.WithField("feed_id", feedID).WithField("round_id", roundID).Info("data feed update stored")
	s.recordStaleness(feedID, upd.Timestamp)
	return created, nil
}

// ListUpdates lists recent updates for a feed.
func (s *Service) ListUpdates(ctx context.Context, accountID, feedID string, limit int) ([]Update, error) {
	if _, err := s.GetFeed(ctx, accountID, feedID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListDataFeedUpdates(ctx, feedID, clamped)
}

// LatestUpdate returns the latest accepted update.
func (s *Service) LatestUpdate(ctx context.Context, accountID, feedID string) (Update, error) {
	if _, err := s.GetFeed(ctx, accountID, feedID); err != nil {
		return Update{}, err
	}
	upd, err := s.store.GetLatestDataFeedUpdate(ctx, feedID)
	if err == nil {
		s.recordStaleness(feedID, upd.Timestamp)
	}
	return upd, err
}

func (s *Service) recordStaleness(feedID string, ts time.Time) {
	status := "empty"
	age := time.Duration(0)
	if !ts.IsZero() {
		age = time.Since(ts)
		status = "healthy"
		if age <= 0 {
			age = 0
		}
	}
	metrics.RecordDatafeedStaleness(feedID, status, age)
}

func (s *Service) signerThreshold(feed Feed) int {
	threshold := s.minSigners
	if threshold <= 0 {
		threshold = len(feed.SignerSet)
	}
	if threshold <= 0 {
		threshold = 1
	}
	if len(feed.SignerSet) > 0 && threshold > len(feed.SignerSet) {
		threshold = len(feed.SignerSet)
	}
	return threshold
}

func shouldPublishDatafeed(prevPrice *big.Int, newPrice *big.Int, prevTs, newTs time.Time, thresholdPPM int, heartbeat time.Duration) bool {
	if prevPrice == nil || prevPrice.Sign() == 0 {
		return true
	}
	if heartbeat > 0 && !prevTs.IsZero() && newTs.Sub(prevTs) >= heartbeat {
		return true
	}
	if thresholdPPM <= 0 {
		return true
	}
	diff := new(big.Int).Sub(newPrice, prevPrice)
	diff.Abs(diff)

	limit := new(big.Int).Mul(prevPrice, big.NewInt(int64(thresholdPPM)))
	diffScaled := new(big.Int).Mul(diff, big.NewInt(1_000_000))
	return diffScaled.Cmp(limit) >= 0
}

func normalizePrice(value string, decimals int) (*big.Int, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, "", core.RequiredError("price")
	}
	scaled, err := normalizePriceInt(value, decimals)
	if err != nil {
		return nil, "", err
	}
	if scaled.Sign() <= 0 {
		return nil, "", fmt.Errorf("price must be positive")
	}
	return scaled, formatPrice(scaled, decimals), nil
}

func normalizePriceInt(value string, decimals int) (*big.Int, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return nil, fmt.Errorf("invalid price format")
	}
	intPart := parts[0]
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}
	if intPart == "" {
		intPart = "0"
	}
	intPart = strings.TrimPrefix(intPart, "+")
	if strings.HasPrefix(intPart, "-") {
		return nil, fmt.Errorf("price must be positive")
	}
	if len(fracPart) > decimals {
		return nil, fmt.Errorf("price exceeds maximum decimals (%d)", decimals)
	}
	fracPart = fracPart + strings.Repeat("0", decimals-len(fracPart))
	combined := strings.TrimLeft(intPart, "0") + fracPart
	if combined == "" {
		combined = "0"
	}
	valueInt := new(big.Int)
	if _, ok := valueInt.SetString(combined, 10); !ok {
		return nil, fmt.Errorf("invalid price digits")
	}
	return valueInt, nil
}

func formatPrice(value *big.Int, decimals int) string {
	if value == nil {
		return "0"
	}
	str := value.String()
	if decimals == 0 {
		return str
	}
	if len(str) <= decimals {
		str = strings.Repeat("0", decimals-len(str)+1) + str
	}
	intPart := str[:len(str)-decimals]
	fracPart := strings.TrimRight(str[len(str)-decimals:], "0")
	if fracPart == "" {
		return intPart
	}
	return intPart + "." + fracPart
}

func normalizeAggregation(strategy string) (string, error) {
	agg := strings.ToLower(strings.TrimSpace(strategy))
	if agg == "" {
		return "median", nil
	}
	switch agg {
	case "median":
		return "median", nil
	case "mean", "avg", "average":
		return "mean", nil
	case "min":
		return "min", nil
	case "max":
		return "max", nil
	default:
		return "median", fmt.Errorf("unsupported aggregation %q", agg)
	}
}

func aggregatePrices(prices []*big.Int, strategy string) *big.Int {
	if len(prices) == 0 {
		return big.NewInt(0)
	}
	agg, _ := normalizeAggregation(strategy)
	switch agg {
	case "mean":
		return meanPrice(prices)
	case "min":
		return extremumPrice(prices, false)
	case "max":
		return extremumPrice(prices, true)
	default:
		return medianPrice(prices)
	}
}

func medianPrice(prices []*big.Int) *big.Int {
	sorted := make([]*big.Int, len(prices))
	for i, p := range prices {
		sorted[i] = new(big.Int).Set(p)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Cmp(sorted[j]) < 0 })
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		sum := new(big.Int).Add(sorted[mid-1], sorted[mid])
		return sum.Div(sum, big.NewInt(2))
	}
	return sorted[mid]
}

func meanPrice(prices []*big.Int) *big.Int {
	if len(prices) == 0 {
		return big.NewInt(0)
	}
	sum := big.NewInt(0)
	for _, p := range prices {
		sum = sum.Add(sum, p)
	}
	return sum.Div(sum, big.NewInt(int64(len(prices))))
}

func extremumPrice(prices []*big.Int, wantMax bool) *big.Int {
	if len(prices) == 0 {
		return big.NewInt(0)
	}
	choice := new(big.Int).Set(prices[0])
	for i := 1; i < len(prices); i++ {
		switch prices[i].Cmp(choice) {
		case -1:
			if !wantMax {
				choice = prices[i]
			}
		case 1:
			if wantMax {
				choice = prices[i]
			}
		}
	}
	return choice
}

func (s *Service) normalizeFeed(feed *Feed) error {
	feed.Pair = strings.ToUpper(strings.TrimSpace(feed.Pair))
	feed.Description = strings.TrimSpace(feed.Description)
	feed.Metadata = core.NormalizeMetadata(feed.Metadata)
	feed.Tags = core.NormalizeTags(feed.Tags)
	feed.SignerSet = core.NormalizeTags(feed.SignerSet)
	if feed.Aggregation == "" {
		feed.Aggregation = s.aggregation
	}
	agg, err := normalizeAggregation(feed.Aggregation)
	if err != nil {
		return err
	}
	feed.Aggregation = agg
	if s.minSigners > 0 && len(feed.SignerSet) < s.minSigners {
		return fmt.Errorf("signer_set must include at least %d signers", s.minSigners)
	}
	if feed.Pair == "" {
		return core.RequiredError("pair")
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

func (s *Service) ensureSignersOwned(ctx context.Context, accountID string, signers []string) error {
	if len(signers) == 0 {
		return nil
	}
	if s.wallets == nil {
		return nil
	}
	for _, signer := range signers {
		if err := s.wallets.WalletOwnedBy(ctx, accountID, signer); err != nil {
			return err
		}
	}
	return nil
}
