package neofeeds

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
)

type pendingPublish struct {
	startedAt time.Time
}

type pricePublishState struct {
	lastRoundID        int64
	lastPublishedPrice int64
	lastPublishedAt    time.Time

	pending      *pendingPublish
	publishTimes []time.Time
}

func (s *Service) hydratePriceFeedState(ctx context.Context) error {
	if s == nil || s.priceFeed == nil {
		return nil
	}

	feeds := s.GetEnabledFeeds()
	if len(feeds) == 0 {
		return nil
	}

	for i := range feeds {
		symbol := strings.TrimSpace(feeds[i].ID)
		if symbol == "" {
			continue
		}

		rec, err := s.priceFeed.GetLatest(ctx, symbol)
		if err != nil {
			s.Logger().WithContext(ctx).WithError(err).WithFields(map[string]interface{}{
				"symbol": symbol,
			}).Warn("pricefeed state hydration failed")
			continue
		}

		var roundID int64
		if rec != nil && rec.RoundID != nil {
			roundID = rec.RoundID.Int64()
		}

		var lastPrice int64
		if rec != nil && rec.Price != nil {
			lastPrice = rec.Price.Int64()
		}

		var lastAt time.Time
		if rec != nil && rec.Timestamp > 0 {
			lastAt = time.Unix(int64(rec.Timestamp), 0)
		}

		s.publishMu.Lock()
		state := s.publishState[symbol]
		if state == nil {
			state = &pricePublishState{}
			s.publishState[symbol] = state
		}
		if roundID > state.lastRoundID {
			state.lastRoundID = roundID
		}
		if lastPrice > 0 {
			state.lastPublishedPrice = lastPrice
		}
		if !lastAt.IsZero() {
			state.lastPublishedAt = lastAt
		}
		s.publishMu.Unlock()
	}

	return nil
}

func (s *Service) pushPricesToPriceFeed(ctx context.Context) {
	if s == nil || s.priceFeed == nil {
		return
	}
	if s.txProxy == nil {
		s.Logger().WithContext(ctx).Warn("pricefeed push enabled but txproxy not configured")
		return
	}
	if len(s.attestationHash) == 0 {
		s.Logger().WithContext(ctx).Warn("pricefeed push enabled but attestation hash missing")
		return
	}

	feeds := s.GetEnabledFeeds()
	if len(feeds) == 0 {
		return
	}

	for i := range feeds {
		symbol := strings.TrimSpace(feeds[i].ID)
		if symbol == "" {
			continue
		}

		price, err := s.GetPrice(ctx, symbol)
		if err != nil {
			continue
		}
		if price == nil {
			continue
		}

		ts := price.Timestamp.Unix()
		if ts <= 0 {
			continue
		}

		sourceSetID := sourceSetIDFromSources(price.Sources)
		s.tryPublishPrice(ctx, symbol, price.Price, uint64(ts), sourceSetID)
	}
}

func (s *Service) tryPublishPrice(ctx context.Context, symbol string, newPrice int64, timestamp uint64, sourceSetID *big.Int) {
	now := time.Now()
	thresholdBps := int64(s.publishPolicy.ThresholdBps)
	hysteresisBps := int64(s.publishPolicy.HysteresisBps)

	if thresholdBps <= 0 {
		thresholdBps = 10
	}
	if hysteresisBps <= 0 {
		hysteresisBps = 8
	}

	minInterval := s.publishPolicy.MinInterval
	if minInterval <= 0 {
		minInterval = 5 * time.Second
	}

	maxPerMinute := s.publishPolicy.MaxPerMinute
	if maxPerMinute <= 0 {
		maxPerMinute = 30
	}

	var (
		lastRoundID int64
		lastPrice   int64
		lastAt      time.Time
		confirm     bool
		nextRoundID int64
	)

	s.publishMu.Lock()
	state := s.publishState[symbol]
	if state == nil {
		state = &pricePublishState{}
		s.publishState[symbol] = state
	}

	// Enforce per-symbol minimum publish interval.
	if !state.lastPublishedAt.IsZero() && now.Sub(state.lastPublishedAt) < minInterval {
		s.publishMu.Unlock()
		return
	}

	// Enforce per-symbol max frequency.
	state.publishTimes = pruneRecentPublishes(state.publishTimes, now)
	if len(state.publishTimes) >= maxPerMinute {
		s.publishMu.Unlock()
		return
	}

	lastRoundID = state.lastRoundID
	lastPrice = state.lastPublishedPrice
	lastAt = state.lastPublishedAt

	change := changeBps(lastPrice, newPrice)

	// Two-step publish confirmation:
	// - first observation must cross threshold (0.1% default)
	// - second observation must stay beyond hysteresis (0.08% default)
	if state.pending == nil {
		if change < thresholdBps {
			s.publishMu.Unlock()
			return
		}
		state.pending = &pendingPublish{startedAt: now}
		s.publishMu.Unlock()
		return
	}

	if change < hysteresisBps {
		state.pending = nil
		s.publishMu.Unlock()
		return
	}

	// Confirm publish.
	state.pending = nil
	confirm = true
	nextRoundID = lastRoundID + 1
	if nextRoundID <= 0 {
		nextRoundID = 1
	}
	s.publishMu.Unlock()

	if !confirm {
		return
	}

	priceBig := big.NewInt(newPrice)
	roundBig := big.NewInt(nextRoundID)
	if sourceSetID == nil {
		sourceSetID = big.NewInt(0)
	}

	_, err := s.invokePriceFeedUpdate(ctx, symbol, roundBig, priceBig, timestamp, sourceSetID, false)
	if err != nil {
		// If we got out of sync (e.g., restart), resync once and retry with the correct round.
		if s.resyncRoundID(ctx, symbol) {
			s.publishMu.Lock()
			state := s.publishState[symbol]
			if state != nil {
				next := state.lastRoundID + 1
				if next <= 0 {
					next = 1
				}
				roundBig = big.NewInt(next)
			}
			s.publishMu.Unlock()
			_, err = s.invokePriceFeedUpdate(ctx, symbol, roundBig, priceBig, timestamp, sourceSetID, false)
		}
		if err != nil {
			s.Logger().WithContext(ctx).WithError(err).WithFields(map[string]interface{}{
				"symbol":     symbol,
				"new_price":  newPrice,
				"last_price": lastPrice,
				"last_ts":    lastAt.Unix(),
			}).Warn("failed to anchor price on-chain")
			return
		}
	}

	s.publishMu.Lock()
	state = s.publishState[symbol]
	if state == nil {
		state = &pricePublishState{}
		s.publishState[symbol] = state
	}
	state.lastRoundID = roundBig.Int64()
	state.lastPublishedPrice = newPrice
	state.lastPublishedAt = now
	state.publishTimes = append(state.publishTimes, now)
	s.publishMu.Unlock()
}

func (s *Service) resyncRoundID(ctx context.Context, symbol string) bool {
	if s == nil || s.priceFeed == nil {
		return false
	}
	rec, err := s.priceFeed.GetLatest(ctx, symbol)
	if err != nil || rec == nil || rec.RoundID == nil {
		return false
	}

	roundID := rec.RoundID.Int64()
	if roundID < 0 {
		return false
	}

	s.publishMu.Lock()
	state := s.publishState[symbol]
	if state == nil {
		state = &pricePublishState{}
		s.publishState[symbol] = state
	}
	if roundID > state.lastRoundID {
		state.lastRoundID = roundID
	}
	s.publishMu.Unlock()

	return true
}

func pruneRecentPublishes(times []time.Time, now time.Time) []time.Time {
	if len(times) == 0 {
		return times
	}
	cutoff := now.Add(-1 * time.Minute)
	out := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			out = append(out, t)
		}
	}
	return out
}

func changeBps(oldPrice, newPrice int64) int64 {
	if oldPrice <= 0 || newPrice <= 0 {
		// Force publish when we don't have a baseline.
		return 10000
	}
	diff := newPrice - oldPrice
	if diff < 0 {
		diff = -diff
	}
	if diff == 0 {
		return 0
	}

	n := big.NewInt(diff)
	n.Mul(n, big.NewInt(10000))
	d := big.NewInt(oldPrice)
	if d.Sign() <= 0 {
		return 10000
	}
	n.Div(n, d)
	if !n.IsInt64() {
		return 10000
	}
	return n.Int64()
}

func sourceSetIDFromSources(sources []string) *big.Int {
	if len(sources) == 0 {
		return big.NewInt(0)
	}
	clone := append([]string{}, sources...)
	sort.Strings(clone)
	value := strings.Join(clone, ",")
	sum := sha256.Sum256([]byte(value))
	u := binary.BigEndian.Uint64(sum[:8])
	return new(big.Int).SetUint64(u)
}

func (s *Service) publishPolicySummary() map[string]any {
	if s == nil {
		return map[string]any{}
	}
	return map[string]any{
		"threshold_bps":    s.publishPolicy.ThresholdBps,
		"hysteresis_bps":   s.publishPolicy.HysteresisBps,
		"min_interval":     s.publishPolicy.MinInterval.String(),
		"max_per_minute":   s.publishPolicy.MaxPerMinute,
		"attestation_hash": fmt.Sprintf("%x", s.attestationHash),
	}
}
