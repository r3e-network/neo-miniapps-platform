package oracle

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	domain "github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

const (
	defaultHTTPResolverTimeout   = 10 * time.Second
	defaultHTTPResolverRetry     = 5 * time.Second
	defaultHTTPResolverBodyLimit = int64(1 << 20) // 1 MiB
)

// HTTPResolver executes oracle data sources using plain HTTP requests.
type HTTPResolver struct {
	service   *Service
	client    *http.Client
	log       *logger.Logger
	bodyLimit int64
	tracer    core.Tracer
}

// NewHTTPResolver constructs an HTTP resolver. When client is nil a sensible
// default with per-request timeouts is used.
func NewHTTPResolver(service *Service, client *http.Client, log *logger.Logger) *HTTPResolver {
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPResolverTimeout}
	}
	if log == nil {
		log = logger.NewDefault("oracle-http-resolver")
	}
	return &HTTPResolver{
		service:   service,
		client:    client,
		log:       log,
		bodyLimit: defaultHTTPResolverBodyLimit,
		tracer:    core.NoopTracer,
	}
}

// WithTracer configures an optional tracer for HTTP executions.
func (r *HTTPResolver) WithTracer(tracer core.Tracer) {
	if tracer == nil {
		r.tracer = core.NoopTracer
		return
	}
	r.tracer = tracer
}

// Resolve satisfies the RequestResolver interface. It resolves pending oracle
// requests by issuing an HTTP request based on the configured data source.
func (r *HTTPResolver) Resolve(ctx context.Context, req domain.Request) (bool, bool, string, string, time.Duration, error) {
	attrs := map[string]string{"request_id": req.ID}
	if req.DataSourceID != "" {
		attrs["data_source_id"] = req.DataSourceID
	}
	spanCtx, finishSpan := r.tracer.StartSpan(ctx, "oracle.http_resolve", attrs)
	var spanErr error
	defer func() {
		finishSpan(spanErr)
	}()
	ctx = spanCtx

	if req.Status == domain.StatusSucceeded || req.Status == domain.StatusFailed {
		return true, req.Status == domain.StatusSucceeded, req.Result, req.Error, 0, nil
	}
	if r.service == nil {
		err := fmt.Errorf("oracle service not configured")
		spanErr = err
		return false, false, "", "", defaultHTTPResolverRetry, err
	}

	payload := strings.TrimSpace(req.Payload)
	var alternateSources []string
	if payload != "" {
		var raw map[string]any
		if err := json.Unmarshal([]byte(payload), &raw); err == nil {
			if alts, ok := raw["alternate_source_ids"]; ok {
				delete(raw, "alternate_source_ids")
				for _, v := range toStringSlice(alts) {
					if trimmed := strings.TrimSpace(v); trimmed != "" {
						alternateSources = append(alternateSources, trimmed)
					}
				}
			}
			if updated, err := json.Marshal(raw); err == nil {
				payload = strings.TrimSpace(string(updated))
			}
		}
	}

	sourceIDs := uniqueSources(req.DataSourceID, alternateSources)
	if len(sourceIDs) == 0 {
		err := fmt.Errorf("no data sources specified")
		spanErr = err
		return true, false, "", err.Error(), 0, nil
	}

	// Single-source path preserves previous behaviour.
	if len(sourceIDs) == 1 {
		src, err := r.service.GetSource(ctx, sourceIDs[0])
		if err != nil {
			return true, false, "", fmt.Sprintf("load data source: %v", err), 0, nil
		}
		return r.executeSource(ctx, req.ID, src, payload)
	}

	var (
		successful []string
		numerics   []float64
		retryAfter time.Duration
		retryErr   error
	)

	for _, srcID := range sourceIDs {
		src, err := r.service.GetSource(ctx, srcID)
		if err != nil {
			continue
		}
		done, success, result, errMsg, nextRetry, err := r.executeSource(ctx, req.ID, src, payload)
		if err != nil {
			if nextRetry > 0 && retryAfter == 0 {
				retryAfter = nextRetry
				retryErr = err
			}
			continue
		}
		if !done {
			if nextRetry > 0 && retryAfter == 0 {
				retryAfter = nextRetry
				retryErr = err
			}
			continue
		}
		if success {
			successful = append(successful, result)
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(result), 64); err == nil {
				numerics = append(numerics, parsed)
			}
		} else if retryAfter == 0 && errMsg != "" {
			retryErr = fmt.Errorf("%s", errMsg)
		}
	}

	if len(successful) > 0 {
		aggregated := successful[0]
		if len(numerics) > 0 {
			aggregated = fmt.Sprintf("%v", medianFloat(numerics))
		}
		return true, true, aggregated, "", 0, nil
	}
	if retryAfter > 0 {
		return false, false, "", "", retryAfter, retryErr
	}
	errMsg := "all oracle sources failed"
	if retryErr != nil {
		spanErr = retryErr
	}
	return true, false, "", errMsg, 0, nil
}

func addPayloadToQuery(u *url.URL, payload string) error {
	if u == nil {
		return fmt.Errorf("url is nil")
	}

	q := u.Query()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(payload), &parsed); err == nil {
		for key, value := range parsed {
			q.Set(key, fmt.Sprint(value))
		}
	} else {
		q.Set("payload", payload)
	}
	u.RawQuery = q.Encode()
	return nil
}

func (r *HTTPResolver) executeSource(ctx context.Context, requestID string, source domain.DataSource, payload string) (bool, bool, string, string, time.Duration, error) {
	method := strings.ToUpper(strings.TrimSpace(source.Method))
	if method == "" {
		method = http.MethodGet
	}

	bodyContent := strings.TrimSpace(payload)
	if bodyContent == "" {
		bodyContent = strings.TrimSpace(source.Body)
	}

	endpoint := strings.TrimSpace(source.URL)
	if endpoint == "" {
		return true, false, "", "data source url is empty", 0, nil
	}

	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return true, false, "", fmt.Sprintf("parse data source url: %v", err), 0, nil
	}

	var reader io.Reader
	if method == http.MethodGet {
		if bodyContent != "" {
			if err := addPayloadToQuery(requestURL, bodyContent); err != nil {
				return true, false, "", fmt.Sprintf("encode payload: %v", err), 0, nil
			}
		}
	} else if bodyContent != "" {
		reader = strings.NewReader(bodyContent)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, requestURL.String(), reader)
	if err != nil {
		err = fmt.Errorf("build http request: %w", err)
		return false, false, "", "", defaultHTTPResolverRetry, err
	}

	for key, value := range source.Headers {
		httpReq.Header.Set(key, value)
	}
	if reader != nil && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := r.client.Do(httpReq)
	if err != nil {
		err = fmt.Errorf("execute http request: %w", err)
		return false, false, "", "", defaultHTTPResolverRetry, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, r.bodyLimit)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		err = fmt.Errorf("read http response: %w", err)
		return false, false, "", "", defaultHTTPResolverRetry, err
	}

	// Retry on transient failures.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		r.log.WithField("status", resp.StatusCode).
			WithField("request_id", requestID).
			WithField("source_id", source.ID).
			Warn("oracle http resolver received retryable status")
		err := fmt.Errorf("upstream status %d", resp.StatusCode)
		return false, false, "", "", defaultHTTPResolverRetry, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := strings.TrimSpace(string(responseBody))
		if errMsg == "" {
			errMsg = fmt.Sprintf("upstream returned status %d", resp.StatusCode)
		}
		return true, false, "", errMsg, 0, nil
	}

	duration := time.Since(start)
	r.log.WithField("request_id", requestID).
		WithField("source_id", source.ID).
		WithField("status", resp.StatusCode).
		WithField("duration", duration).
		Debug("oracle http resolver completed")

	return true, true, string(responseBody), "", 0, nil
}

func uniqueSources(primary string, alternates []string) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(id string) {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	add(primary)
	for _, alt := range alternates {
		add(alt)
	}
	return out
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func toStringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
