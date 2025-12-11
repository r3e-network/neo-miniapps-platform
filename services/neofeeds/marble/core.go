// Package neofeeds provides core logic for the price feed aggregation service.
package neofeeds

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

// =============================================================================
// Core Logic
// =============================================================================

// GetPrice fetches and aggregates price from multiple sources.
// Priority: Chainlink (free, on-chain) -> HTTP sources (Binance)
func (s *Service) GetPrice(ctx context.Context, pair string) (*PriceResponse, error) {
	// Try to find feed config for this pair
	feed := s.findFeedByPair(pair)

	feedID := pair
	if feed != nil {
		feedID = feed.ID
	}

	var prices []float64
	var sources []string
	decimals := 8
	if feed != nil && feed.Decimals > 0 {
		decimals = feed.Decimals
	}

	// Try Chainlink first (free, on-chain data)
	if s.chainlinkClient != nil && s.chainlinkClient.HasFeed(feedID) {
		price, _, err := s.chainlinkClient.GetPrice(ctx, feedID)
		if err == nil && price > 0 {
			prices = append(prices, price, price, price) // Weight 3 for Chainlink
			sources = append(sources, "chainlink")
		}
	}

	// Fall back to HTTP sources (Binance) if Chainlink not available or failed
	if len(prices) == 0 {
		var wg sync.WaitGroup
		var mu sync.Mutex

		sourcesToUse := s.getSourcesForFeed(feed)

		for _, srcConfig := range sourcesToUse {
			wg.Add(1)
			go func(src *SourceConfig) {
				defer wg.Done()

				price, err := s.fetchPriceFromSource(ctx, pair, feed, src)
				if err != nil {
					return
				}

				mu.Lock()
				for i := 0; i < src.Weight; i++ {
					prices = append(prices, price)
				}
				sources = append(sources, src.ID)
				mu.Unlock()
			}(srcConfig)
		}

		wg.Wait()
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no prices available for %s", pair)
	}

	medianPrice := s.calculateMedian(prices)
	priceInt := int64(medianPrice * float64(pow10(decimals)))

	response := &PriceResponse{
		FeedID:    feedID,
		Pair:      pair,
		Price:     priceInt,
		Decimals:  decimals,
		Timestamp: time.Now(),
		Sources:   sources,
	}

	if len(s.signingKey) > 0 {
		sig, pub, _ := s.signPrice(response)
		response.Signature = append([]byte{}, sig...)
		response.PublicKey = append([]byte{}, pub...)
	}

	if s.DB() != nil {
		_ = s.DB().CreatePriceFeed(ctx, &database.PriceFeed{
			ID:        uuid.New().String(),
			FeedID:    feedID,
			Pair:      pair,
			Price:     priceInt,
			Decimals:  response.Decimals,
			Timestamp: response.Timestamp,
			Sources:   response.Sources,
			Signature: response.Signature,
		})
	}

	return response, nil
}

// findFeedByPair finds a feed config by pair or feed ID.
func (s *Service) findFeedByPair(pair string) *FeedConfig {
	for i := range s.config.Feeds {
		f := &s.config.Feeds[i]
		if f.Pair == pair || f.ID == pair {
			return f
		}
	}
	return nil
}

// getSourcesForFeed returns sources to use for a feed.
func (s *Service) getSourcesForFeed(feed *FeedConfig) []*SourceConfig {
	if feed != nil && len(feed.Sources) > 0 {
		sources := make([]*SourceConfig, 0, len(feed.Sources))
		for _, srcID := range feed.Sources {
			if src := s.sources[srcID]; src != nil {
				sources = append(sources, src)
			}
		}
		return sources
	}
	// Return all sources if no feed config
	sources := make([]*SourceConfig, 0, len(s.sources))
	for _, src := range s.sources {
		sources = append(sources, src)
	}
	return sources
}

// fetchPriceFromSource fetches price from a single source.
func (s *Service) fetchPriceFromSource(ctx context.Context, pair string, feed *FeedConfig, src *SourceConfig) (float64, error) {
	// Use feed.Pair (e.g., NEOUSDT) for URL template if available, otherwise use raw pair (e.g., NEO/USD)
	pairForURL := pair
	if feed != nil && feed.Pair != "" {
		pairForURL = feed.Pair
	}
	url := formatSourceURLNew(src.URL, pairForURL, feed)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	for k, v := range src.Headers {
		req.Header.Set(k, resolveEnvVar(v))
	}

	client := s.httpClient
	if src.Timeout > 0 {
		client = &http.Client{
			Timeout: src.Timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	jsonPath := formatJSONPath(src.JSONPath, feed)
	result := gjson.GetBytes(body, jsonPath)
	if !result.Exists() {
		return 0, fmt.Errorf("price not found in response")
	}

	return result.Float(), nil
}

func (s *Service) fetchPrice(ctx context.Context, pair string, source PriceSource) (float64, error) {
	url := formatSourceURL(source.URL, pair)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	result := gjson.GetBytes(body, source.JSONPath)
	if !result.Exists() {
		return 0, fmt.Errorf("price not found in response")
	}

	return result.Float(), nil
}

func (s *Service) calculateMedian(prices []float64) float64 {
	sort.Float64s(prices)
	n := len(prices)
	if n%2 == 0 {
		return (prices[n/2-1] + prices[n/2]) / 2
	}
	return prices[n/2]
}

func (s *Service) signPrice(price *PriceResponse) ([]byte, []byte, error) {
	data, _ := json.Marshal(map[string]interface{}{
		"pair":      price.Pair,
		"price":     price.Price,
		"decimals":  price.Decimals,
		"timestamp": price.Timestamp.Unix(),
	})

	seed, err := crypto.DeriveKey(s.signingKey, nil, "price-signing", 32)
	if err != nil {
		return nil, nil, err
	}
	defer crypto.ZeroBytes(seed)

	curve := elliptic.P256()
	d := new(big.Int).SetBytes(seed)
	n := new(big.Int).Sub(curve.Params().N, big.NewInt(1))
	d.Mod(d, n)
	d.Add(d, big.NewInt(1))
	priv := &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve}, D: d}
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	sig, err := crypto.Sign(priv, data)
	if err != nil {
		return nil, nil, err
	}
	pubBytes := crypto.PublicKeyToBytes(&priv.PublicKey)
	return sig, pubBytes, nil
}

func formatSourceURL(tmpl, pair string) string {
	if strings.Contains(tmpl, "%sPAIR%s") {
		return fmt.Sprintf(tmpl, "", pair, "")
	}
	return strings.ReplaceAll(tmpl, "{pair}", pair)
}

// formatSourceURLNew formats URL template with feed-specific placeholders.
func formatSourceURLNew(tmpl, pair string, feed *FeedConfig) string {
	url := tmpl
	url = strings.ReplaceAll(url, "{pair}", pair)

	if feed != nil {
		url = strings.ReplaceAll(url, "{base}", feed.Base)
		url = strings.ReplaceAll(url, "{quote}", feed.Quote)
	} else {
		// Parse base/quote from pair (e.g., BTCUSDT -> BTC, USDT)
		if len(pair) >= 6 {
			base := strings.ToLower(pair[:3])
			quote := strings.ToLower(pair[3:])
			url = strings.ReplaceAll(url, "{base}", base)
			url = strings.ReplaceAll(url, "{quote}", quote)
		}
	}

	// Legacy format support
	if strings.Contains(url, "%sPAIR%s") {
		url = fmt.Sprintf(url, "", pair, "")
	}

	return url
}

// formatJSONPath formats JSON path with feed-specific placeholders.
func formatJSONPath(tmpl string, feed *FeedConfig) string {
	if feed == nil {
		return tmpl
	}
	path := tmpl
	path = strings.ReplaceAll(path, "{base}", feed.Base)
	path = strings.ReplaceAll(path, "{quote}", feed.Quote)
	return path
}

// pow10 returns 10^n.
func pow10(n int) int64 {
	result := int64(1)
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

// resolveEnvVar resolves ${VAR_NAME} placeholders with environment values.
func resolveEnvVar(value string) string {
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		envKey := value[2 : len(value)-1]
		if envVal := os.Getenv(envKey); envVal != "" {
			return envVal
		}
	}
	return value
}
