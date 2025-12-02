package oracle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/sandbox"
	
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Compile-time check: Service exposes Publish for the core engine adapter.
var _ core.EventPublisher = (*Service)(nil)

// Service manages oracle data sources and requests.
// Uses SandboxedServiceEngine for capability-based access control.
type Service struct {
	*framework.SandboxedServiceEngine
	store        Store
	feeCollector framework.FeeCollector // Use engine-level interface for decoupling
	defaultFee   int64               // default fee per request in smallest unit
}

// Option configures the oracle service.
type Option func(*Service)

// WithFeeCollector sets a fee collector for charging oracle request fees.
// Aligned with OracleHub.cs contract fee model.
// Uses framework.FeeCollector interface for service decoupling.
func WithFeeCollector(fc framework.FeeCollector) Option {
	return func(s *Service) { s.feeCollector = fc }
}

// WithDefaultFee sets the default fee per request in smallest unit.
func WithDefaultFee(fee int64) Option {
	return func(s *Service) { s.defaultFee = fee }
}

// New constructs a new oracle service.
func New(accounts AccountChecker, store Store, log *logger.Logger, opts ...Option) *Service {
	svc := &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:         "oracle",
				Domain:       "oracle",
				Description:  "Oracle sources and request lifecycle",
				DependsOn:    []string{"store", "svc-accounts"},
				RequiresAPIs: []framework.APISurface{framework.APISurfaceStore, framework.APISurfaceData, framework.APISurfaceEvent},
				Capabilities: []string{"oracle"},
				Quotas:       map[string]string{"rpc": "oracle-callbacks"},
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
				sandbox.CapNetworkOutbound, // For external data fetching
			},
			StorageQuota: 20 * 1024 * 1024, // 20MB
		}),
		store:      store,
		defaultFee: 0, // free by default
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// Start/Stop/Ready are inherited from framework.ServiceEngine.

// Publish implements EventEngine: enqueue a request (simplified).
func (s *Service) Publish(ctx context.Context, event string, payload any) error {
	if strings.ToLower(strings.TrimSpace(event)) != "request" {
		return fmt.Errorf("unsupported event: %s", event)
	}
	body, ok := payload.(map[string]any)
	if !ok {
		return fmt.Errorf("payload must be a map")
	}
	accountID, _ := body["account_id"].(string)
	sourceID, _ := body["source_id"].(string)
	reqPayload, _ := body["payload"].(string)
	if reqPayload == "" {
		if pm, ok := body["payload"].(map[string]any); ok {
			if enc, err := json.Marshal(pm); err == nil {
				reqPayload = string(enc)
			}
		}
	}
	if accountID == "" || sourceID == "" {
		return fmt.Errorf("account_id and source_id required")
	}
	attrs := map[string]string{"account_id": strings.TrimSpace(accountID), "source_id": strings.TrimSpace(sourceID), "resource": "oracle_publish"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	_, err := s.CreateRequest(ctx, accountID, sourceID, reqPayload)
	return err
}

// CreateSource registers a new data source.
func (s *Service) CreateSource(ctx context.Context, accountID, name, url, method, description string, headers map[string]string, body string) (DataSource, error) {
	accountID = strings.TrimSpace(accountID)
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	method = strings.ToUpper(strings.TrimSpace(method))
	description = strings.TrimSpace(description)

	if accountID == "" {
		return DataSource{}, core.RequiredError("account_id")
	}
	if name == "" {
		return DataSource{}, core.RequiredError("name")
	}
	if url == "" {
		return DataSource{}, core.RequiredError("url")
	}
	if method == "" {
		method = "GET"
	}

	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return DataSource{}, fmt.Errorf("account validation failed: %w", err)
	}
	attrs := map[string]string{"account_id": accountID, "resource": "oracle_source"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)

	existing, err := s.store.ListDataSources(ctx, accountID)
	if err != nil {
		return DataSource{}, err
	}
	for _, src := range existing {
		if strings.EqualFold(src.Name, name) {
			return DataSource{}, fmt.Errorf("data source with name %q already exists", name)
		}
	}

	src := DataSource{
		AccountID:   accountID,
		Name:        name,
		Description: description,
		URL:         url,
		Method:      method,
		Headers:     headers,
		Body:        body,
		Enabled:     true,
	}
	src, err = s.store.CreateDataSource(ctx, src)
	if err != nil {
		return DataSource{}, err
	}
	s.Logger().WithField("source_id", src.ID).
		WithField("account_id", accountID).
		WithField("name", name).
		Info("oracle source created")
	s.LogCreated("oracle_source", src.ID, accountID)
	s.IncrementCounter("oracle_sources_created_total", map[string]string{"account_id": accountID})
	return src, nil
}

// UpdateSource modifies mutable fields of a data source.
func (s *Service) UpdateSource(ctx context.Context, sourceID string, name, url, method, description *string, headers map[string]string, body *string) (DataSource, error) {
	src, err := s.store.GetDataSource(ctx, sourceID)
	if err != nil {
		return DataSource{}, err
	}
	attrs := map[string]string{"account_id": src.AccountID, "source_id": sourceID, "resource": "oracle_source"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return DataSource{}, fmt.Errorf("name cannot be empty")
		}
		existing, err := s.store.ListDataSources(ctx, src.AccountID)
		if err != nil {
			return DataSource{}, err
		}
		for _, other := range existing {
			if other.ID != src.ID && strings.EqualFold(other.Name, trimmed) {
				return DataSource{}, fmt.Errorf("data source with name %q already exists", trimmed)
			}
		}
		src.Name = trimmed
	}
	if url != nil {
		trimmed := strings.TrimSpace(*url)
		if trimmed == "" {
			return DataSource{}, fmt.Errorf("url cannot be empty")
		}
		src.URL = trimmed
	}
	if method != nil {
		trimmed := strings.ToUpper(strings.TrimSpace(*method))
		if trimmed == "" {
			return DataSource{}, fmt.Errorf("method cannot be empty")
		}
		src.Method = trimmed
	}
	if description != nil {
		src.Description = strings.TrimSpace(*description)
	}
	if headers != nil {
		src.Headers = headers
	}
	if body != nil {
		src.Body = *body
	}

	src, err = s.store.UpdateDataSource(ctx, src)
	if err != nil {
		return DataSource{}, err
	}
	s.Logger().WithField("source_id", src.ID).
		WithField("account_id", src.AccountID).
		Info("oracle source updated")
	s.LogUpdated("oracle_source", src.ID, src.AccountID)
	s.IncrementCounter("oracle_sources_updated_total", map[string]string{"account_id": src.AccountID})
	return src, nil
}

// SetSourceEnabled toggles a data source.
func (s *Service) SetSourceEnabled(ctx context.Context, sourceID string, enabled bool) (DataSource, error) {
	src, err := s.store.GetDataSource(ctx, sourceID)
	if err != nil {
		return DataSource{}, err
	}
	if src.Enabled == enabled {
		return src, nil
	}
	src.Enabled = enabled
	attrs := map[string]string{"source_id": sourceID, "account_id": src.AccountID, "resource": "oracle_source_enabled"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	updated, err := s.store.UpdateDataSource(ctx, src)
	if err != nil {
		return DataSource{}, err
	}
	value := 0.0
	status := "disabled"
	if enabled {
		value = 1
		status = "enabled"
	}
	s.SetGauge("oracle_source_enabled", map[string]string{"source_id": updated.ID, "account_id": updated.AccountID}, value)
	s.LogAction("source_"+status, "oracle_source", updated.ID, updated.AccountID)
	s.IncrementCounter("oracle_sources_state_total", map[string]string{"account_id": updated.AccountID, "status": status})
	return updated, nil
}

// ListSources returns sources for an account.
func (s *Service) ListSources(ctx context.Context, accountID string) ([]DataSource, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListDataSources(ctx, accountID)
}

// GetSource fetches a source by identifier.
func (s *Service) GetSource(ctx context.Context, sourceID string) (DataSource, error) {
	return s.store.GetDataSource(ctx, sourceID)
}

// CreateRequestOptions configures oracle request creation.
type CreateRequestOptions struct {
	Fee *int64 // Custom fee; nil uses default
}

// CreateRequest enqueues a new oracle request with default fee.
func (s *Service) CreateRequest(ctx context.Context, accountID, sourceID, payload string) (Request, error) {
	return s.CreateRequestWithOptions(ctx, accountID, sourceID, payload, CreateRequestOptions{})
}

// CreateRequestWithOptions enqueues a new oracle request with custom options.
// Aligned with OracleHub.cs contract fee model.
func (s *Service) CreateRequestWithOptions(ctx context.Context, accountID, sourceID, payload string, opts CreateRequestOptions) (Request, error) {
	accountID = strings.TrimSpace(accountID)
	sourceID = strings.TrimSpace(sourceID)

	if accountID == "" {
		return Request{}, core.RequiredError("account_id")
	}
	if sourceID == "" {
		return Request{}, core.RequiredError("data_source_id")
	}
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return Request{}, fmt.Errorf("account validation failed: %w", err)
	}
	attrs := map[string]string{"account_id": accountID, "source_id": sourceID, "resource": "oracle_request"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)

	src, err := s.store.GetDataSource(ctx, sourceID)
	if err != nil {
		return Request{}, err
	}
	if err := core.EnsureOwnership(src.AccountID, accountID, "data source", sourceID); err != nil {
		return Request{}, err
	}
	if !src.Enabled {
		return Request{}, fmt.Errorf("data source %s is disabled", sourceID)
	}

	// Determine fee amount
	fee := s.defaultFee
	if opts.Fee != nil {
		fee = *opts.Fee
	}

	req := Request{
		AccountID:    accountID,
		DataSourceID: sourceID,
		Status:       StatusPending,
		Payload:      payload,
		Fee:          fee,
	}

	// Collect fee before creating request (if fee collector configured and fee > 0)
	if s.feeCollector != nil && fee > 0 {
		// Use a temporary reference; will update with actual request ID after creation
		if err := s.feeCollector.CollectFee(ctx, accountID, fee, "oracle-request-pending"); err != nil {
			return Request{}, fmt.Errorf("fee collection failed: %w", err)
		}
	}

	req, err = s.store.CreateRequest(ctx, req)
	if err != nil {
		return Request{}, err
	}
	s.Logger().WithField("request_id", req.ID).
		WithField("account_id", accountID).
		WithField("source_id", sourceID).
		Info("oracle request created")
	s.LogCreated("oracle_request", req.ID, req.AccountID)
	s.IncrementCounter("oracle_requests_created_total", map[string]string{"account_id": req.AccountID})
	return req, nil
}

// MarkRunning updates a request to running.
func (s *Service) MarkRunning(ctx context.Context, requestID string) (Request, error) {
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return Request{}, err
	}
	if req.Status != StatusPending {
		return Request{}, fmt.Errorf("cannot mark request in status %s as running", req.Status)
	}
	req.Status = StatusRunning
	req.Attempts++
	updated, err := s.store.UpdateRequest(ctx, req)
	if err != nil {
		return Request{}, err
	}
	s.LogAction("request_running", "oracle_request", updated.ID, updated.AccountID)
	s.IncrementCounter("oracle_requests_running_total", map[string]string{"account_id": updated.AccountID})
	return updated, nil
}

// IncrementAttempts increments the attempt counter without changing status.
func (s *Service) IncrementAttempts(ctx context.Context, requestID string) (Request, error) {
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return Request{}, err
	}
	req.Attempts++
	updated, err := s.store.UpdateRequest(ctx, req)
	if err != nil {
		return Request{}, err
	}
	s.IncrementCounter("oracle_request_attempts_total", map[string]string{"account_id": updated.AccountID})
	return updated, nil
}

// CompleteRequest records a successful result.
func (s *Service) CompleteRequest(ctx context.Context, requestID string, result string) (Request, error) {
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return Request{}, err
	}
	switch req.Status {
	case StatusRunning:
		// allowed transition
	case StatusPending:
		return Request{}, fmt.Errorf("cannot complete request in status %s", req.Status)
	case StatusSucceeded, StatusFailed:
		return Request{}, fmt.Errorf("request already %s", req.Status)
	default:
		return Request{}, fmt.Errorf("cannot complete request in status %s", req.Status)
	}
	req.Status = StatusSucceeded
	req.Result = result
	req.Error = ""
	req.CompletedAt = time.Now().UTC()
	updated, err := s.store.UpdateRequest(ctx, req)
	if err != nil {
		return Request{}, err
	}
	s.LogAction("request_completed", "oracle_request", updated.ID, updated.AccountID)
	s.IncrementCounter("oracle_requests_completed_total", map[string]string{"account_id": updated.AccountID})
	return updated, nil
}

// FailRequestOptions configures failure behavior.
type FailRequestOptions struct {
	RefundFee bool // If true, refund the collected fee
}

// FailRequest records a failure without fee refund.
func (s *Service) FailRequest(ctx context.Context, requestID string, errMsg string) (Request, error) {
	return s.FailRequestWithOptions(ctx, requestID, errMsg, FailRequestOptions{})
}

// FailRequestWithOptions records a failure with optional fee refund.
// Aligned with OracleHub.cs contract fee model.
func (s *Service) FailRequestWithOptions(ctx context.Context, requestID string, errMsg string, opts FailRequestOptions) (Request, error) {
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return Request{}, err
	}
	switch req.Status {
	case StatusRunning, StatusPending:
		// allowed transition
	case StatusSucceeded, StatusFailed:
		return Request{}, fmt.Errorf("request already %s", req.Status)
	default:
		return Request{}, fmt.Errorf("cannot fail request in status %s", req.Status)
	}

	// Refund fee if requested and fee was collected
	if opts.RefundFee && s.feeCollector != nil && req.Fee > 0 {
		if refundErr := s.feeCollector.RefundFee(ctx, req.AccountID, req.Fee, fmt.Sprintf("oracle-request-%s-refund", req.ID)); refundErr != nil {
			s.Logger().WithField("request_id", req.ID).
				WithField("fee", req.Fee).
				WithError(refundErr).
				Warn("failed to refund oracle request fee")
			// Continue with failure even if refund fails - log for manual resolution
		}
	}

	req.Status = StatusFailed
	req.Error = strings.TrimSpace(errMsg)
	req.CompletedAt = time.Now().UTC()
	updated, err := s.store.UpdateRequest(ctx, req)
	if err != nil {
		return Request{}, err
	}
	s.LogAction("request_failed", "oracle_request", updated.ID, updated.AccountID)
	s.IncrementCounter("oracle_requests_failed_total", map[string]string{"account_id": updated.AccountID})
	return updated, nil
}

// RetryRequest resets a failed request back to pending and clears error/result.
func (s *Service) RetryRequest(ctx context.Context, requestID string) (Request, error) {
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return Request{}, err
	}
	if req.Status != StatusFailed {
		return Request{}, fmt.Errorf("cannot retry request in status %s", req.Status)
	}
	req.Status = StatusPending
	req.Attempts = 0
	req.Error = ""
	req.Result = ""
	req.CompletedAt = time.Time{}
	return s.store.UpdateRequest(ctx, req)
}

// ListRequests returns requests for an account.
func (s *Service) ListRequests(ctx context.Context, accountID string, limit int, status string) ([]Request, error) {
	return s.store.ListRequests(ctx, accountID, limit, status)
}

// ListPending returns requests that are awaiting fulfilment.
func (s *Service) ListPending(ctx context.Context) ([]Request, error) {
	return s.store.ListPendingRequests(ctx)
}

// GetRequest returns an individual request.
func (s *Service) GetRequest(ctx context.Context, requestID string) (Request, error) {
	return s.store.GetRequest(ctx, requestID)
}

// HTTP API Methods
// These methods follow the HTTP{Method}{Path} naming convention for automatic route discovery.

// HTTPGetSources handles GET /sources - list all data sources for an account.
func (s *Service) HTTPGetSources(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListSources(ctx, req.AccountID)
}

// HTTPPostSources handles POST /sources - create a new data source.
func (s *Service) HTTPPostSources(ctx context.Context, req core.APIRequest) (any, error) {
	name, _ := req.Body["name"].(string)
	url, _ := req.Body["url"].(string)
	method, _ := req.Body["method"].(string)
	description, _ := req.Body["description"].(string)
	body, _ := req.Body["body"].(string)

	var headers map[string]string
	if rawHeaders, ok := req.Body["headers"].(map[string]any); ok {
		headers = make(map[string]string)
		for k, v := range rawHeaders {
			if str, ok := v.(string); ok {
				headers[k] = str
			}
		}
	}

	return s.CreateSource(ctx, req.AccountID, name, url, method, description, headers, body)
}

// HTTPGetSourcesById handles GET /sources/{id} - get a specific data source.
func (s *Service) HTTPGetSourcesById(ctx context.Context, req core.APIRequest) (any, error) {
	sourceID := req.PathParams["id"]
	src, err := s.GetSource(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	if err := core.EnsureOwnership(src.AccountID, req.AccountID, "data source", sourceID); err != nil {
		return nil, err
	}
	return src, nil
}

// HTTPPatchSourcesById handles PATCH /sources/{id} - update a data source.
func (s *Service) HTTPPatchSourcesById(ctx context.Context, req core.APIRequest) (any, error) {
	sourceID := req.PathParams["id"]

	// Verify ownership first
	src, err := s.GetSource(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	if err := core.EnsureOwnership(src.AccountID, req.AccountID, "data source", sourceID); err != nil {
		return nil, err
	}

	var name, url, method, description, body *string
	if v, ok := req.Body["name"].(string); ok {
		name = &v
	}
	if v, ok := req.Body["url"].(string); ok {
		url = &v
	}
	if v, ok := req.Body["method"].(string); ok {
		method = &v
	}
	if v, ok := req.Body["description"].(string); ok {
		description = &v
	}
	if v, ok := req.Body["body"].(string); ok {
		body = &v
	}

	var headers map[string]string
	if rawHeaders, ok := req.Body["headers"].(map[string]any); ok {
		headers = make(map[string]string)
		for k, v := range rawHeaders {
			if str, ok := v.(string); ok {
				headers[k] = str
			}
		}
	}

	// Handle enabled toggle
	if enabled, ok := req.Body["enabled"].(bool); ok {
		return s.SetSourceEnabled(ctx, sourceID, enabled)
	}

	return s.UpdateSource(ctx, sourceID, name, url, method, description, headers, body)
}

// HTTPGetRequests handles GET /requests - list oracle requests for an account.
func (s *Service) HTTPGetRequests(ctx context.Context, req core.APIRequest) (any, error) {
	limit := core.ParseLimitFromQuery(req.Query)
	status := req.Query["status"]
	return s.ListRequests(ctx, req.AccountID, limit, status)
}

// HTTPPostRequests handles POST /requests - create a new oracle request.
func (s *Service) HTTPPostRequests(ctx context.Context, req core.APIRequest) (any, error) {
	dataSourceID, _ := req.Body["data_source_id"].(string)
	payload, _ := req.Body["payload"].(string)
	return s.CreateRequest(ctx, req.AccountID, dataSourceID, payload)
}

// HTTPGetRequestsById handles GET /requests/{id} - get a specific oracle request.
func (s *Service) HTTPGetRequestsById(ctx context.Context, req core.APIRequest) (any, error) {
	requestID := req.PathParams["id"]
	oracleReq, err := s.GetRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if err := core.EnsureOwnership(oracleReq.AccountID, req.AccountID, "request", requestID); err != nil {
		return nil, err
	}
	return oracleReq, nil
}

// HTTPPatchRequestsById handles PATCH /requests/{id} - update request status.
func (s *Service) HTTPPatchRequestsById(ctx context.Context, req core.APIRequest) (any, error) {
	requestID := req.PathParams["id"]

	// Verify ownership first
	oracleReq, err := s.GetRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if err := core.EnsureOwnership(oracleReq.AccountID, req.AccountID, "request", requestID); err != nil {
		return nil, err
	}

	status, _ := req.Body["status"].(string)
	status = strings.ToLower(strings.TrimSpace(status))

	switch status {
	case "retry":
		return s.RetryRequest(ctx, requestID)
	default:
		return nil, fmt.Errorf("unsupported status update: %s", status)
	}
}
