// Package enclave provides TEE-protected data feed operations.
// This enclave focuses solely on data fetching and aggregation logic.
// Attestation, signing, and HTTP capabilities are provided by the Enclave SDK.
package enclave

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/system/tee/sdk"
)

// DataFeedsEnclave handles data feed operations within the TEE.
// It embeds BaseEnclave for common functionality (attestation, signing, keys).
type DataFeedsEnclave struct {
	*sdk.BaseEnclave
	httpClient sdk.SecureHTTPClient
	feeds      map[string]*FeedData
}

// FeedData represents aggregated feed data with signature.
type FeedData struct {
	FeedID    string
	Value     *big.Int
	Timestamp int64
	Signature []byte
}

// SourceConfig defines how to fetch data from an external source.
type SourceConfig struct {
	URL        string            `json:"url"`
	Method     string            `json:"method,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	AuthType   string            `json:"auth_type,omitempty"`
	AuthToken  string            `json:"auth_token,omitempty"`
	JSONPath   string            `json:"json_path,omitempty"`
	Timeout    time.Duration     `json:"timeout,omitempty"`
	RetryCount int               `json:"retry_count,omitempty"`
	RetryDelay time.Duration     `json:"retry_delay,omitempty"`
}

// FetchResult contains the result of fetching from a source.
type FetchResult struct {
	SourceURL    string    `json:"source_url"`
	Value        string    `json:"value"`
	NumericValue *float64  `json:"numeric_value,omitempty"`
	FetchedAt    time.Time `json:"fetched_at"`
	Latency      int64     `json:"latency_ms"`
	Error        string    `json:"error,omitempty"`
	Success      bool      `json:"success"`
}

// AggregatedResult contains the aggregated result from multiple sources.
type AggregatedResult struct {
	FeedID       string        `json:"feed_id"`
	Value        *big.Int      `json:"value"`
	StringValue  string        `json:"string_value"`
	NumericValue *float64      `json:"numeric_value,omitempty"`
	Timestamp    int64         `json:"timestamp"`
	Signature    []byte        `json:"signature"`
	SourceCount  int           `json:"source_count"`
	Confidence   float64       `json:"confidence"`
	Results      []FetchResult `json:"results"`
}

// New creates a new DataFeedsEnclave using the SDK's base enclave.
func New() (*DataFeedsEnclave, error) {
	base, err := sdk.NewBaseEnclave("datafeeds")
	if err != nil {
		return nil, err
	}
	return &DataFeedsEnclave{
		BaseEnclave: base,
		httpClient:  sdk.NewSecureHTTPClient(nil),
		feeds:       make(map[string]*FeedData),
	}, nil
}

// NewWithSDK creates a DataFeedsEnclave with full SDK integration.
func NewWithSDK(cfg *sdk.BaseConfig) (*DataFeedsEnclave, error) {
	base, err := sdk.NewBaseEnclaveWithSDK(cfg)
	if err != nil {
		return nil, err
	}
	return &DataFeedsEnclave{
		BaseEnclave: base,
		httpClient:  sdk.NewSecureHTTPClient(nil),
		feeds:       make(map[string]*FeedData),
	}, nil
}

// FetchAndAggregate fetches data from multiple sources and aggregates the results.
// This is the core enclave logic that runs in the TEE.
func (e *DataFeedsEnclave) FetchAndAggregate(ctx context.Context, feedID string, sources []SourceConfig, aggregation string, decimals int) (*AggregatedResult, error) {
	e.Lock()
	defer e.Unlock()

	if len(sources) == 0 {
		return nil, errors.New("no sources configured")
	}

	results := make([]FetchResult, 0, len(sources))
	var numericValues []float64

	// Fetch from each source
	for _, src := range sources {
		result := e.fetchFromSource(ctx, src)
		results = append(results, result)

		if result.Success && result.NumericValue != nil {
			numericValues = append(numericValues, *result.NumericValue)
		}
	}

	if len(numericValues) == 0 {
		return nil, errors.New("no successful fetches from any source")
	}

	// Aggregate values
	aggregatedValue := aggregate(numericValues, aggregation)

	// Convert to big.Int with decimals
	valueBigInt := toBigInt(aggregatedValue, decimals)
	timestamp := time.Now().Unix()

	// Sign using BaseEnclave's signing capability
	signature, err := e.signFeedData(feedID, valueBigInt, timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to sign result: %w", err)
	}

	// Calculate confidence based on source agreement
	confidence := calculateConfidence(numericValues, aggregatedValue)

	// Store the feed data
	e.feeds[feedID] = &FeedData{
		FeedID:    feedID,
		Value:     valueBigInt,
		Timestamp: timestamp,
		Signature: signature,
	}

	return &AggregatedResult{
		FeedID:       feedID,
		Value:        valueBigInt,
		StringValue:  formatValue(aggregatedValue, decimals),
		NumericValue: &aggregatedValue,
		Timestamp:    timestamp,
		Signature:    signature,
		SourceCount:  len(numericValues),
		Confidence:   confidence,
		Results:      results,
	}, nil
}

// fetchFromSource fetches data from a single source using SDK's secure HTTP client.
func (e *DataFeedsEnclave) fetchFromSource(ctx context.Context, src SourceConfig) FetchResult {
	startTime := time.Now()
	result := FetchResult{
		SourceURL: src.URL,
		FetchedAt: startTime,
	}

	// Build HTTP options
	var opts []sdk.HTTPOption
	if src.Timeout > 0 {
		opts = append(opts, sdk.WithTimeout(src.Timeout))
	}
	for k, v := range src.Headers {
		opts = append(opts, sdk.WithHeader(k, v))
	}
	if src.AuthType != "" && src.AuthToken != "" {
		opts = append(opts, sdk.WithAuth(&sdk.HTTPAuth{
			Type:  src.AuthType,
			Token: src.AuthToken,
		}))
	}

	// Execute request with retries
	var resp *sdk.HTTPResponse
	var err error
	retries := src.RetryCount
	if retries == 0 {
		retries = 1
	}

	for i := 0; i < retries; i++ {
		method := src.Method
		if method == "" {
			method = "GET"
		}

		if method == http.MethodGet {
			resp, err = e.httpClient.Get(ctx, src.URL, opts...)
		} else if method == http.MethodPost {
			resp, err = e.httpClient.Post(ctx, src.URL, []byte(src.Body), opts...)
		}

		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			break
		}
		if i < retries-1 {
			delay := src.RetryDelay
			if delay == 0 {
				delay = time.Second
			}
			time.Sleep(delay)
		}
	}

	result.Latency = time.Since(startTime).Milliseconds()

	if err != nil {
		result.Error = fmt.Sprintf("request failed: %v", err)
		return result
	}

	if resp == nil || resp.StatusCode != http.StatusOK {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		result.Error = fmt.Sprintf("unexpected status code: %d", statusCode)
		return result
	}

	// Extract value using JSONPath
	value, err := extractValue(resp.Body, src.JSONPath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to extract value: %v", err)
		return result
	}

	result.Value = value
	result.Success = true

	// Try to parse as numeric
	if numVal, err := strconv.ParseFloat(value, 64); err == nil {
		result.NumericValue = &numVal
	}

	return result
}

// signFeedData signs feed data using the enclave's signing key.
func (e *DataFeedsEnclave) signFeedData(feedID string, value *big.Int, timestamp int64) ([]byte, error) {
	// Prepare data to sign
	data := append([]byte(feedID), value.Bytes()...)
	data = append(data, big.NewInt(timestamp).Bytes()...)

	// Use BaseEnclave's SignData method
	return e.SignData(data)
}

// GetFeedData returns the current data for a feed.
func (e *DataFeedsEnclave) GetFeedData(feedID string) (*FeedData, bool) {
	e.RLock()
	defer e.RUnlock()
	data, ok := e.feeds[feedID]
	return data, ok
}

// AggregateValues aggregates pre-collected values (for backward compatibility).
func (e *DataFeedsEnclave) AggregateValues(feedID string, values []*big.Int, timestamp int64) (*FeedData, error) {
	e.Lock()
	defer e.Unlock()

	if len(values) == 0 {
		return nil, errors.New("no values to aggregate")
	}

	// Calculate median
	aggregated := values[len(values)/2]

	signature, err := e.signFeedData(feedID, aggregated, timestamp)
	if err != nil {
		return nil, err
	}

	feed := &FeedData{
		FeedID:    feedID,
		Value:     aggregated,
		Timestamp: timestamp,
		Signature: signature,
	}
	e.feeds[feedID] = feed
	return feed, nil
}

// ============================================================================
// Pure functions - no enclave state dependency
// ============================================================================

// aggregate aggregates numeric values using the specified method.
func aggregate(values []float64, method string) float64 {
	if len(values) == 0 {
		return 0
	}

	switch method {
	case "mean", "average":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values))

	case "min":
		min := values[0]
		for _, v := range values[1:] {
			if v < min {
				min = v
			}
		}
		return min

	case "max":
		max := values[0]
		for _, v := range values[1:] {
			if v > max {
				max = v
			}
		}
		return max

	case "median":
		fallthrough
	default:
		sorted := make([]float64, len(values))
		copy(sorted, values)
		sort.Float64s(sorted)
		mid := len(sorted) / 2
		if len(sorted)%2 == 0 {
			return (sorted[mid-1] + sorted[mid]) / 2
		}
		return sorted[mid]
	}
}

// calculateConfidence calculates confidence based on source agreement.
func calculateConfidence(values []float64, aggregated float64) float64 {
	if len(values) <= 1 {
		return 1.0
	}

	// Calculate variance
	sum := 0.0
	for _, v := range values {
		diff := v - aggregated
		sum += diff * diff
	}
	variance := sum / float64(len(values))

	// Confidence decreases with higher deviation
	if aggregated == 0 {
		return 1.0
	}
	cv := variance / (aggregated * aggregated) // coefficient of variation squared
	confidence := 1.0 - cv
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	return confidence
}

// toBigInt converts a float64 to big.Int with decimal scaling.
func toBigInt(value float64, decimals int) *big.Int {
	multiplier := new(big.Float).SetFloat64(float64(pow10(decimals)))
	valueFloat := new(big.Float).SetFloat64(value)
	valueFloat.Mul(valueFloat, multiplier)

	result := new(big.Int)
	valueFloat.Int(result)
	return result
}

// extractValue extracts a value from JSON using a simple JSONPath.
func extractValue(body []byte, jsonPath string) (string, error) {
	if jsonPath == "" {
		return string(body), nil
	}

	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	path := strings.TrimPrefix(jsonPath, "$")
	path = strings.TrimPrefix(path, ".")

	current := data
	for _, part := range splitPath(path) {
		if part == "" {
			continue
		}

		// Check for array index
		if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") {
			indexStr := part[1 : len(part)-1]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return "", fmt.Errorf("invalid array index: %s", part)
			}
			arr, ok := current.([]interface{})
			if !ok {
				return "", fmt.Errorf("expected array at %s", part)
			}
			if index < 0 || index >= len(arr) {
				return "", fmt.Errorf("array index out of bounds: %d", index)
			}
			current = arr[index]
			continue
		}

		// Handle field access
		obj, ok := current.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("expected object at %s", part)
		}
		current, ok = obj[part]
		if !ok {
			return "", fmt.Errorf("field not found: %s", part)
		}
	}

	// Convert to string
	switch v := current.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(v), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func splitPath(path string) []string {
	var parts []string
	current := ""
	inBracket := false

	for _, c := range path {
		switch c {
		case '.':
			if !inBracket && current != "" {
				parts = append(parts, current)
				current = ""
			} else if inBracket {
				current += string(c)
			}
		case '[':
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
			current = "["
			inBracket = true
		case ']':
			current += "]"
			parts = append(parts, current)
			current = ""
			inBracket = false
		default:
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func pow10(n int) int {
	result := 1
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

func formatValue(value float64, decimals int) string {
	format := fmt.Sprintf("%%.%df", decimals)
	return fmt.Sprintf(format, value)
}
