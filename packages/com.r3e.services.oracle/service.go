package oracle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Compile-time check: Service exposes Publish for the core engine adapter.
type eventPublisher interface {
	Publish(context.Context, string, any) error
}

var _ eventPublisher = (*Service)(nil)

// FeeCollector handles fee collection for oracle requests.
// Aligned with OracleHub.cs contract fee model.
type FeeCollector interface {
	// CollectFee deducts a fee from the account's gas bank.
	// Returns error if insufficient funds.
	CollectFee(ctx context.Context, accountID string, amount int64, reference string) error
	// RefundFee returns a previously collected fee (e.g., on request failure).
	RefundFee(ctx context.Context, accountID string, amount int64, reference string) error
}

// Service manages oracle data sources and requests.
type Service struct {
	framework.ServiceBase
	accounts     AccountChecker
	store        Store
	log          *logger.Logger
	feeCollector FeeCollector
	defaultFee   int64 // default fee per request in smallest unit
}

// Name returns the stable engine module name.
func (s *Service) Name() string { return "oracle" }

// Domain reports the service domain for engine grouping.
func (s *Service) Domain() string { return "oracle" }

// Manifest describes the service contract for the engine OS.
func (s *Service) Manifest() *framework.Manifest {
	return &framework.Manifest{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Description:  "Oracle sources and request lifecycle",
		Layer:        "service",
		DependsOn:    []string{"store", "svc-accounts"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceData, engine.APISurfaceEvent},
		Capabilities: []string{"oracle"},
		Quotas:       map[string]string{"rpc": "oracle-callbacks"},
	}
}

// Descriptor advertises the service for system discovery.
func (s *Service) Descriptor() core.Descriptor { return s.Manifest().ToDescriptor() }

// Option configures the oracle service.
type Option func(*Service)

// WithFeeCollector sets a fee collector for charging oracle request fees.
// Aligned with OracleHub.cs contract fee model.
func WithFeeCollector(fc FeeCollector) Option {
	return func(s *Service) { s.feeCollector = fc }
}

// WithDefaultFee sets the default fee per request in smallest unit.
func WithDefaultFee(fee int64) Option {
	return func(s *Service) { s.defaultFee = fee }
}

// New constructs a new oracle service.
func New(accounts AccountChecker, store Store, log *logger.Logger, opts ...Option) *Service {
	if log == nil {
		log = logger.NewDefault("oracle")
	}
	svc := &Service{
		accounts:   accounts,
		store:      store,
		log:        log,
		defaultFee: 0, // free by default
	}
	for _, opt := range opts {
		opt(svc)
	}
	svc.SetName(svc.Name())
	return svc
}

// Start/Stop/Ready are inherited from framework.ServiceBase.

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

	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return DataSource{}, fmt.Errorf("account validation failed: %w", err)
	}

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
	s.log.WithField("source_id", src.ID).
		WithField("account_id", accountID).
		WithField("name", name).
		Info("oracle source created")
	return src, nil
}

// UpdateSource modifies mutable fields of a data source.
func (s *Service) UpdateSource(ctx context.Context, sourceID string, name, url, method, description *string, headers map[string]string, body *string) (DataSource, error) {
	src, err := s.store.GetDataSource(ctx, sourceID)
	if err != nil {
		return DataSource{}, err
	}

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
	s.log.WithField("source_id", src.ID).
		WithField("account_id", src.AccountID).
		Info("oracle source updated")
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
	return s.store.UpdateDataSource(ctx, src)
}

// ListSources returns sources for an account.
func (s *Service) ListSources(ctx context.Context, accountID string) ([]DataSource, error) {
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
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
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return Request{}, fmt.Errorf("account validation failed: %w", err)
	}

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
	s.log.WithField("request_id", req.ID).
		WithField("account_id", accountID).
		WithField("source_id", sourceID).
		Info("oracle request created")
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
	return s.store.UpdateRequest(ctx, req)
}

// IncrementAttempts increments the attempt counter without changing status.
func (s *Service) IncrementAttempts(ctx context.Context, requestID string) (Request, error) {
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return Request{}, err
	}
	req.Attempts++
	return s.store.UpdateRequest(ctx, req)
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
	return s.store.UpdateRequest(ctx, req)
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
			s.log.WithField("request_id", req.ID).
				WithField("fee", req.Fee).
				WithError(refundErr).
				Warn("failed to refund oracle request fee")
			// Continue with failure even if refund fails - log for manual resolution
		}
	}

	req.Status = StatusFailed
	req.Error = strings.TrimSpace(errMsg)
	req.CompletedAt = time.Now().UTC()
	return s.store.UpdateRequest(ctx, req)
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
