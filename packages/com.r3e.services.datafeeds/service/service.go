package datafeeds

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
	"github.com/R3E-Network/service_layer/system/sandbox"
)

// Compile-time check: Service exposes Publish for the core engine adapter.
var _ core.EventPublisher = (*Service)(nil)

// Service manages centralized Chainlink data feeds per account.
// Uses SandboxedServiceEngine for common functionality (validation, logging, manifest).
type Service struct {
	*framework.SandboxedServiceEngine // Provides: Name, Domain, Manifest, Descriptor, ValidateAccount, Logger, etc.
	store                             Store
	// aggregation defaults
	minSigners  int
	aggregation string
}

// New constructs a data feed service.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	svc := &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:         "datafeeds",
				Description:  "Aggregated data feed definitions and updates",
				DependsOn:    []string{"store", "svc-accounts"},
				RequiresAPIs: []framework.APISurface{framework.APISurfaceStore, framework.APISurfaceData},
				Capabilities: []string{"datafeeds"},
				Accounts:     accounts,
				Logger:       log,
			},
			SecurityLevel: sandbox.SecurityLevelPrivileged,
			RequestedCapabilities: []sandbox.Capability{
				sandbox.CapStorageRead,
				sandbox.CapStorageWrite,
				sandbox.CapDatabaseRead,
				sandbox.CapDatabaseWrite,
				sandbox.CapBusPublish,
				sandbox.CapServiceCall,
				sandbox.CapNetworkOutbound,
			},
			StorageQuota: 20 * 1024 * 1024,
		}),
		store: store,
	}
	return svc
}

// WithWalletChecker injects a wallet checker for ownership validation.
func (s *Service) WithWalletChecker(w WalletChecker) {
	s.SandboxedServiceEngine.WithWalletChecker(w)
}

// WithAggregationConfig sets baseline aggregation parameters.
func (s *Service) WithAggregationConfig(minSigners int, aggregation string) {
	if minSigners > 0 {
		s.minSigners = minSigners
	}
	agg, err := normalizeAggregation(aggregation)
	if err != nil {
		s.Logger().WithField("aggregation", aggregation).Warn("unsupported datafeed aggregation; defaulting to median")
	}
	s.aggregation = agg
}

// WithObservationHooks configures observability callbacks for updates.
func (s *Service) WithObservationHooks(h core.ObservationHooks) {
	s.SandboxedServiceEngine.WithObservationHooks(h)
}

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
	if err := s.ValidateAccountExists(ctx, feed.AccountID); err != nil {
		return Feed{}, err
	}
	if err := s.normalizeFeed(&feed); err != nil {
		return Feed{}, err
	}
	if err := s.ensureSignersOwned(ctx, feed.AccountID, feed.SignerSet); err != nil {
		return Feed{}, err
	}
	attrs := map[string]string{"account_id": feed.AccountID, "resource": "datafeed"}
	ctx, finish := s.StartObservation(ctx, attrs)
	created, err := s.store.CreateDataFeed(ctx, feed)
	if err == nil && created.ID != "" {
		attrs["feed_id"] = created.ID
	}
	finish(err)
	if err != nil {
		return Feed{}, err
	}
	s.Logger().WithField("feed_id", created.ID).WithField("account_id", created.AccountID).Info("data feed created")
	s.LogCreated("datafeed", created.ID, created.AccountID)
	s.IncrementCounter("datafeeds_created_total", map[string]string{"account_id": created.AccountID})
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
	attrs := map[string]string{"account_id": feed.AccountID, "feed_id": feed.ID, "resource": "datafeed"}
	ctx, finish := s.StartObservation(ctx, attrs)
	updated, err := s.store.UpdateDataFeed(ctx, feed)
	finish(err)
	if err != nil {
		return Feed{}, err
	}
	s.Logger().WithField("feed_id", feed.ID).WithField("account_id", feed.AccountID).Info("data feed updated")
	s.LogUpdated("datafeed", feed.ID, feed.AccountID)
	s.IncrementCounter("datafeeds_updated_total", map[string]string{"account_id": feed.AccountID})
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
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListDataFeeds(ctx, accountID)
}

// SubmitUpdate stores a price update for a feed, enforcing signer verification,
// heartbeat/deviation thresholds, and the configured aggregation strategy.
func (s *Service) SubmitUpdate(ctx context.Context, accountID, feedID string, roundID int64, price string, ts time.Time, signer string, signature string, metadata map[string]string) (Update, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
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
			latestInt, _, parseErr := normalizePrice(latest.Value, feed.Decimals)
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
		s.Logger().WithField("aggregation", aggregation).WithError(aggErr).Warn("falling back to median aggregation")
	}

	threshold := s.signerThreshold(feed)
	submissions := len(existingRound) + 1
	meta["aggregation"] = aggregation
	meta["signer_count"] = strconv.Itoa(submissions)
	meta["quorum"] = strconv.Itoa(threshold)

	allPrices := append(make([]*big.Int, 0, len(existingRound)+1), priceInt)
	for _, upd := range existingRound {
		parsed, parseErr := normalizePriceInt(upd.Value, feed.Decimals)
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
		Value:     normalizedPrice,
		Signer:    signer,
		Timestamp: ts,
		Signature: sig,
		Status:    status,
		Metadata:  meta,
	}
	attrs := map[string]string{"feed_id": feedID, "account_id": accountID, "resource": "datafeed_update"}
	ctx, finish := s.StartObservation(ctx, attrs)
	created, err := s.store.CreateDataFeedUpdate(ctx, upd)
	if err != nil {
		finish(err)
		return Update{}, err
	}
	finish(nil)
	s.Logger().WithField("feed_id", feedID).WithField("round_id", roundID).Info("data feed update stored")
	s.IncrementCounter("datafeeds_updates_total", map[string]string{"feed_id": feedID})
	s.recordStaleness(feedID, upd.Timestamp)
	dataPayload := map[string]any{
		"feed_id":   feedID,
		"round_id":  roundID,
		"price":     normalizedPrice,
		"status":    status,
		"metadata":  meta,
		"timestamp": ts,
	}
	topic := fmt.Sprintf("datafeeds/%s", feedID)
	if err := s.PushData(ctx, topic, dataPayload); err != nil {
		if errors.Is(err, core.ErrBusUnavailable) {
			s.Logger().WithError(err).Warn("bus unavailable for datafeed push")
		} else {
			return Update{}, fmt.Errorf("push datafeed update: %w", err)
		}
	}
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
	s.SetGauge("datafeeds_feed_staleness_seconds", map[string]string{"feed_id": feedID, "status": status}, age.Seconds())
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
	return s.ValidateSigners(ctx, accountID, signers)
}

// HTTP API Methods
// These methods follow the HTTP{Method}{Path} naming convention for automatic route discovery.

// HTTPGetFeeds handles GET /feeds - list all data feeds for an account.
func (s *Service) HTTPGetFeeds(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListFeeds(ctx, req.AccountID)
}

// HTTPPostFeeds handles POST /feeds - create a new data feed.
func (s *Service) HTTPPostFeeds(ctx context.Context, req core.APIRequest) (any, error) {
	pair, _ := req.Body["pair"].(string)
	description, _ := req.Body["description"].(string)
	decimals := 8
	if d, ok := req.Body["decimals"].(float64); ok {
		decimals = int(d)
	}
	heartbeat := time.Minute
	if h, ok := req.Body["heartbeat"].(string); ok {
		if parsed, err := time.ParseDuration(h); err == nil {
			heartbeat = parsed
		}
	}
	thresholdPPM := 0
	if t, ok := req.Body["threshold_ppm"].(float64); ok {
		thresholdPPM = int(t)
	}
	aggregation, _ := req.Body["aggregation"].(string)

	var signerSet []string
	if rawSigners, ok := req.Body["signer_set"].([]any); ok {
		for _, s := range rawSigners {
			if str, ok := s.(string); ok {
				signerSet = append(signerSet, str)
			}
		}
	}

	var tags []string
	if rawTags, ok := req.Body["tags"].([]any); ok {
		for _, t := range rawTags {
			if str, ok := t.(string); ok {
				tags = append(tags, str)
			}
		}
	}

	metadata := core.ExtractMetadataRaw(req.Body, "")

	feed := Feed{
		AccountID:    req.AccountID,
		Pair:         pair,
		Description:  description,
		Decimals:     decimals,
		Heartbeat:    heartbeat,
		ThresholdPPM: thresholdPPM,
		Aggregation:  aggregation,
		SignerSet:    signerSet,
		Tags:         tags,
		Metadata:     metadata,
	}

	return s.CreateFeed(ctx, feed)
}

// HTTPGetFeedsById handles GET /feeds/{id} - get a specific data feed.
func (s *Service) HTTPGetFeedsById(ctx context.Context, req core.APIRequest) (any, error) {
	feedID := req.PathParams["id"]
	return s.GetFeed(ctx, req.AccountID, feedID)
}

// HTTPPatchFeedsById handles PATCH /feeds/{id} - update a data feed.
func (s *Service) HTTPPatchFeedsById(ctx context.Context, req core.APIRequest) (any, error) {
	feedID := req.PathParams["id"]

	// Get existing feed first
	existing, err := s.GetFeed(ctx, req.AccountID, feedID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if pair, ok := req.Body["pair"].(string); ok {
		existing.Pair = pair
	}
	if description, ok := req.Body["description"].(string); ok {
		existing.Description = description
	}
	if decimals, ok := req.Body["decimals"].(float64); ok {
		existing.Decimals = int(decimals)
	}
	if h, ok := req.Body["heartbeat"].(string); ok {
		if parsed, err := time.ParseDuration(h); err == nil {
			existing.Heartbeat = parsed
		}
	}
	if t, ok := req.Body["threshold_ppm"].(float64); ok {
		existing.ThresholdPPM = int(t)
	}
	if aggregation, ok := req.Body["aggregation"].(string); ok {
		existing.Aggregation = aggregation
	}

	existing.AccountID = req.AccountID
	return s.UpdateFeed(ctx, existing)
}

// HTTPGetFeedsIdUpdates handles GET /feeds/{id}/updates - list updates for a feed.
func (s *Service) HTTPGetFeedsIdUpdates(ctx context.Context, req core.APIRequest) (any, error) {
	feedID := req.PathParams["id"]
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListUpdates(ctx, req.AccountID, feedID, limit)
}

// HTTPPostFeedsIdUpdates handles POST /feeds/{id}/updates - submit a price update.
func (s *Service) HTTPPostFeedsIdUpdates(ctx context.Context, req core.APIRequest) (any, error) {
	feedID := req.PathParams["id"]
	roundID := int64(0)
	if r, ok := req.Body["round_id"].(float64); ok {
		roundID = int64(r)
	}
	price, _ := req.Body["price"].(string)
	signer, _ := req.Body["signer"].(string)
	signature, _ := req.Body["signature"].(string)

	metadata := core.ExtractMetadataRaw(req.Body, "")

	return s.SubmitUpdate(ctx, req.AccountID, feedID, roundID, price, time.Now().UTC(), signer, signature, metadata)
}

// HTTPGetFeedsIdLatest handles GET /feeds/{id}/latest - get latest update.
func (s *Service) HTTPGetFeedsIdLatest(ctx context.Context, req core.APIRequest) (any, error) {
	feedID := req.PathParams["id"]
	return s.LatestUpdate(ctx, req.AccountID, feedID)
}

// ============================================================================
// Frontend Display Endpoints - For showing feed status and values
// ============================================================================

// HTTPGetValues handles GET /values - get current values for all feeds (frontend display).
func (s *Service) HTTPGetValues(ctx context.Context, req core.APIRequest) (any, error) {
	return s.GetAllFeedValues(ctx, req.AccountID)
}

// HTTPGetValuesById handles GET /values/{id} - get current value for a specific feed.
func (s *Service) HTTPGetValuesById(ctx context.Context, req core.APIRequest) (any, error) {
	feedID := req.PathParams["id"]
	return s.GetFeedValue(ctx, req.AccountID, feedID)
}

// HTTPGetStatus handles GET /status - get status overview of all feeds.
func (s *Service) HTTPGetStatus(ctx context.Context, req core.APIRequest) (any, error) {
	return s.GetFeedStatusOverview(ctx, req.AccountID)
}

// HTTPGetFeedsIdStats handles GET /feeds/{id}/stats - get statistics for a feed.
func (s *Service) HTTPGetFeedsIdStats(ctx context.Context, req core.APIRequest) (any, error) {
	feedID := req.PathParams["id"]
	return s.GetFeedStats(ctx, req.AccountID, feedID)
}

// HTTPPostFeedsIdSources handles POST /feeds/{id}/sources - configure data sources for a feed.
func (s *Service) HTTPPostFeedsIdSources(ctx context.Context, req core.APIRequest) (any, error) {
	feedID := req.PathParams["id"]

	// Parse sources from request body
	var sources []SourceConfig
	if rawSources, ok := req.Body["sources"].([]any); ok {
		for _, raw := range rawSources {
			if srcMap, ok := raw.(map[string]any); ok {
				src := SourceConfig{
					URL:      getString(srcMap, "url"),
					Method:   getString(srcMap, "method"),
					JSONPath: getString(srcMap, "json_path"),
					AuthType: getString(srcMap, "auth_type"),
				}
				if headers, ok := srcMap["headers"].(map[string]any); ok {
					src.Headers = make(map[string]string)
					for k, v := range headers {
						if str, ok := v.(string); ok {
							src.Headers[k] = str
						}
					}
				}
				if timeout, ok := srcMap["timeout"].(string); ok {
					if d, err := time.ParseDuration(timeout); err == nil {
						src.Timeout = d
					}
				}
				if retries, ok := srcMap["retry_count"].(float64); ok {
					src.RetryCount = int(retries)
				}
				sources = append(sources, src)
			}
		}
	}

	return s.ConfigureFeedSources(ctx, req.AccountID, feedID, sources)
}

// HTTPPostFeedsIdFetch handles POST /feeds/{id}/fetch - trigger enclave fetch for a feed.
func (s *Service) HTTPPostFeedsIdFetch(ctx context.Context, req core.APIRequest) (any, error) {
	feedID := req.PathParams["id"]
	return s.TriggerEnclaveFetch(ctx, req.AccountID, feedID)
}

// ============================================================================
// Service Methods for Frontend Display
// ============================================================================

// GetAllFeedValues returns current values for all feeds owned by the account.
func (s *Service) GetAllFeedValues(ctx context.Context, accountID string) ([]FeedValue, error) {
	feeds, err := s.ListFeeds(ctx, accountID)
	if err != nil {
		return nil, err
	}

	values := make([]FeedValue, 0, len(feeds))
	for _, feed := range feeds {
		value, err := s.GetFeedValue(ctx, accountID, feed.ID)
		if err != nil {
			// Include feed with error status
			values = append(values, FeedValue{
				FeedID:   feed.ID,
				FeedName: feed.Name,
				Pair:     feed.Pair,
				DataType: feed.DataType,
				Status:   FeedStatusError,
				Decimals: feed.Decimals,
			})
			continue
		}
		values = append(values, value)
	}

	return values, nil
}

// GetFeedValue returns the current value for a specific feed.
func (s *Service) GetFeedValue(ctx context.Context, accountID, feedID string) (FeedValue, error) {
	feed, err := s.GetFeed(ctx, accountID, feedID)
	if err != nil {
		return FeedValue{}, err
	}

	latest, err := s.LatestUpdate(ctx, accountID, feedID)
	if err != nil {
		// Return feed info without value
		return FeedValue{
			FeedID:      feed.ID,
			FeedName:    feed.Name,
			Pair:        feed.Pair,
			DataType:    feed.DataType,
			Decimals:    feed.Decimals,
			Status:      feed.Status,
			SourceCount: len(feed.Sources),
		}, nil
	}

	return FeedValue{
		FeedID:       feed.ID,
		FeedName:     feed.Name,
		Pair:         feed.Pair,
		DataType:     feed.DataType,
		Value:        latest.Value,
		NumericValue: latest.NumericValue,
		Decimals:     feed.Decimals,
		RoundID:      latest.RoundID,
		Timestamp:    latest.Timestamp,
		Status:       feed.Status,
		LastUpdated:  latest.UpdatedAt,
		SourceCount:  len(feed.Sources),
	}, nil
}

// FeedStatusSummary provides an overview of feed health.
type FeedStatusSummary struct {
	TotalFeeds     int            `json:"total_feeds"`
	ActiveFeeds    int            `json:"active_feeds"`
	PausedFeeds    int            `json:"paused_feeds"`
	ErrorFeeds     int            `json:"error_feeds"`
	DisabledFeeds  int            `json:"disabled_feeds"`
	Feeds          []FeedListItem `json:"feeds"`
	LastRefreshed  time.Time      `json:"last_refreshed"`
}

// GetFeedStatusOverview returns a status overview of all feeds.
func (s *Service) GetFeedStatusOverview(ctx context.Context, accountID string) (FeedStatusSummary, error) {
	feeds, err := s.ListFeeds(ctx, accountID)
	if err != nil {
		return FeedStatusSummary{}, err
	}

	summary := FeedStatusSummary{
		TotalFeeds:    len(feeds),
		Feeds:         make([]FeedListItem, 0, len(feeds)),
		LastRefreshed: time.Now().UTC(),
	}

	for _, feed := range feeds {
		// Count by status
		switch feed.Status {
		case FeedStatusActive:
			summary.ActiveFeeds++
		case FeedStatusPaused:
			summary.PausedFeeds++
		case FeedStatusError:
			summary.ErrorFeeds++
		case FeedStatusDisabled:
			summary.DisabledFeeds++
		}

		// Get latest value for display
		var value string
		var lastUpdated time.Time
		if latest, err := s.LatestUpdate(ctx, accountID, feed.ID); err == nil {
			value = latest.Value
			lastUpdated = latest.UpdatedAt
		}

		summary.Feeds = append(summary.Feeds, FeedListItem{
			ID:          feed.ID,
			Name:        feed.Name,
			Pair:        feed.Pair,
			DataType:    feed.DataType,
			Status:      feed.Status,
			Value:       value,
			LastUpdated: lastUpdated,
			SourceCount: len(feed.Sources),
			Tags:        feed.Tags,
		})
	}

	return summary, nil
}

// GetFeedStats returns statistics for a specific feed.
func (s *Service) GetFeedStats(ctx context.Context, accountID, feedID string) (FeedStats, error) {
	feed, err := s.GetFeed(ctx, accountID, feedID)
	if err != nil {
		return FeedStats{}, err
	}

	updates, err := s.ListUpdates(ctx, accountID, feedID, 1000)
	if err != nil {
		return FeedStats{}, err
	}

	stats := FeedStats{
		FeedID:       feedID,
		TotalUpdates: int64(len(updates)),
	}

	// Count updates in last 24 hours and calculate error rate
	now := time.Now().UTC()
	dayAgo := now.Add(-24 * time.Hour)
	var errorCount int64

	for _, upd := range updates {
		if upd.CreatedAt.After(dayAgo) {
			stats.UpdatesLast24h++
		}
		if upd.Status == UpdateStatusRejected {
			errorCount++
		}
	}

	if stats.TotalUpdates > 0 {
		stats.ErrorRate = float64(errorCount) / float64(stats.TotalUpdates)
	}

	// Calculate uptime based on heartbeat
	if feed.Heartbeat > 0 && len(updates) > 0 {
		latest := updates[0]
		timeSinceUpdate := now.Sub(latest.UpdatedAt)
		if timeSinceUpdate <= feed.Heartbeat {
			stats.UptimePercent = 100.0
		} else {
			// Simplified uptime calculation
			stats.UptimePercent = float64(feed.Heartbeat) / float64(timeSinceUpdate) * 100
			if stats.UptimePercent > 100 {
				stats.UptimePercent = 100
			}
		}
		stats.LastHeartbeat = latest.UpdatedAt
	}

	// Count healthy sources
	stats.SourcesHealthy = len(feed.Sources)
	if feed.ErrorCount > 0 {
		stats.SourcesUnhealthy = feed.ErrorCount
		if stats.SourcesUnhealthy > stats.SourcesHealthy {
			stats.SourcesUnhealthy = stats.SourcesHealthy
		}
		stats.SourcesHealthy -= stats.SourcesUnhealthy
	}

	return stats, nil
}

// ConfigureFeedSources updates the data sources for a feed.
func (s *Service) ConfigureFeedSources(ctx context.Context, accountID, feedID string, sources []SourceConfig) (Feed, error) {
	feed, err := s.GetFeed(ctx, accountID, feedID)
	if err != nil {
		return Feed{}, err
	}

	feed.Sources = sources
	feed.UpdatedAt = time.Now().UTC()

	return s.UpdateFeed(ctx, feed)
}

// TriggerEnclaveFetch triggers an enclave fetch for a feed and returns the result.
func (s *Service) TriggerEnclaveFetch(ctx context.Context, accountID, feedID string) (Update, error) {
	feed, err := s.GetFeed(ctx, accountID, feedID)
	if err != nil {
		return Update{}, err
	}

	if len(feed.Sources) == 0 {
		return Update{}, fmt.Errorf("no sources configured for feed %s", feedID)
	}

	// Convert service SourceConfig to enclave SourceConfig
	enclaveSources := make([]enclaveSourceConfig, len(feed.Sources))
	for i, src := range feed.Sources {
		enclaveSources[i] = enclaveSourceConfig{
			URL:        src.URL,
			Method:     src.Method,
			Headers:    src.Headers,
			Body:       src.Body,
			AuthType:   src.AuthType,
			JSONPath:   src.JSONPath,
			Timeout:    src.Timeout,
			RetryCount: src.RetryCount,
			RetryDelay: src.RetryDelay,
		}
	}

	// Get latest round ID
	var roundID int64 = 1
	if latest, err := s.LatestUpdate(ctx, accountID, feedID); err == nil {
		roundID = latest.RoundID + 1
	}

	// Create update record
	now := time.Now().UTC()
	update := Update{
		AccountID:        accountID,
		FeedID:           feedID,
		RoundID:          roundID,
		Timestamp:        now,
		Status:           UpdateStatusFetched,
		FetchedInEnclave: true,
		Source:           "enclave",
	}

	// Note: In production, this would call the actual enclave
	// For now, we create a placeholder that indicates enclave fetch was triggered
	update.Value = "enclave_fetch_pending"

	created, err := s.store.CreateDataFeedUpdate(ctx, update)
	if err != nil {
		return Update{}, err
	}

	// Update feed status
	feed.LastFetchAt = now
	feed.Status = FeedStatusActive
	feed.ErrorCount = 0
	feed.LastError = ""
	_, _ = s.UpdateFeed(ctx, feed)

	s.Logger().WithField("feed_id", feedID).
		WithField("round_id", roundID).
		Info("enclave fetch triggered")

	return created, nil
}

// enclaveSourceConfig is a local type for enclave source configuration.
type enclaveSourceConfig struct {
	URL        string
	Method     string
	Headers    map[string]string
	Body       string
	AuthType   string
	JSONPath   string
	Timeout    time.Duration
	RetryCount int
	RetryDelay time.Duration
}

// Helper function to safely get string from map
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
