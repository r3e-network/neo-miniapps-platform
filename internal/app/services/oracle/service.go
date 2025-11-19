package oracle

import (
	"context"
	"fmt"
	"strings"
	"time"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service manages oracle data sources and requests.
type Service struct {
	base  *core.Base
	store storage.OracleStore
	log   *logger.Logger
}

// New constructs a new oracle service.
func New(accounts storage.AccountStore, store storage.OracleStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("oracle")
	}
	return &Service{
		base:  core.NewBase(accounts),
		store: store,
		log:   log,
	}
}

// CreateSource registers a new data source.
func (s *Service) CreateSource(ctx context.Context, accountID, name, url, method, description string, headers map[string]string, body string) (oracle.DataSource, error) {
	accountID = strings.TrimSpace(accountID)
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	method = strings.ToUpper(strings.TrimSpace(method))
	description = strings.TrimSpace(description)

	if accountID == "" {
		return oracle.DataSource{}, fmt.Errorf("account_id is required")
	}
	if name == "" {
		return oracle.DataSource{}, fmt.Errorf("name is required")
	}
	if url == "" {
		return oracle.DataSource{}, fmt.Errorf("url is required")
	}
	if method == "" {
		method = "GET"
	}

	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return oracle.DataSource{}, fmt.Errorf("account validation failed: %w", err)
	}

	existing, err := s.store.ListDataSources(ctx, accountID)
	if err != nil {
		return oracle.DataSource{}, err
	}
	for _, src := range existing {
		if strings.EqualFold(src.Name, name) {
			return oracle.DataSource{}, fmt.Errorf("data source with name %q already exists", name)
		}
	}

	src := oracle.DataSource{
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
		return oracle.DataSource{}, err
	}
	s.log.WithField("source_id", src.ID).
		WithField("account_id", accountID).
		WithField("name", name).
		Info("oracle source created")
	return src, nil
}

// UpdateSource modifies mutable fields of a data source.
func (s *Service) UpdateSource(ctx context.Context, sourceID string, name, url, method, description *string, headers map[string]string, body *string) (oracle.DataSource, error) {
	src, err := s.store.GetDataSource(ctx, sourceID)
	if err != nil {
		return oracle.DataSource{}, err
	}

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return oracle.DataSource{}, fmt.Errorf("name cannot be empty")
		}
		existing, err := s.store.ListDataSources(ctx, src.AccountID)
		if err != nil {
			return oracle.DataSource{}, err
		}
		for _, other := range existing {
			if other.ID != src.ID && strings.EqualFold(other.Name, trimmed) {
				return oracle.DataSource{}, fmt.Errorf("data source with name %q already exists", trimmed)
			}
		}
		src.Name = trimmed
	}
	if url != nil {
		trimmed := strings.TrimSpace(*url)
		if trimmed == "" {
			return oracle.DataSource{}, fmt.Errorf("url cannot be empty")
		}
		src.URL = trimmed
	}
	if method != nil {
		trimmed := strings.ToUpper(strings.TrimSpace(*method))
		if trimmed == "" {
			return oracle.DataSource{}, fmt.Errorf("method cannot be empty")
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
		return oracle.DataSource{}, err
	}
	s.log.WithField("source_id", src.ID).
		WithField("account_id", src.AccountID).
		Info("oracle source updated")
	return src, nil
}

// SetSourceEnabled toggles a data source.
func (s *Service) SetSourceEnabled(ctx context.Context, sourceID string, enabled bool) (oracle.DataSource, error) {
	src, err := s.store.GetDataSource(ctx, sourceID)
	if err != nil {
		return oracle.DataSource{}, err
	}
	if src.Enabled == enabled {
		return src, nil
	}
	src.Enabled = enabled
	return s.store.UpdateDataSource(ctx, src)
}

// ListSources returns sources for an account.
func (s *Service) ListSources(ctx context.Context, accountID string) ([]oracle.DataSource, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListDataSources(ctx, accountID)
}

// GetSource fetches a source by identifier.
func (s *Service) GetSource(ctx context.Context, sourceID string) (oracle.DataSource, error) {
	return s.store.GetDataSource(ctx, sourceID)
}

// CreateRequest enqueues a new oracle request.
func (s *Service) CreateRequest(ctx context.Context, accountID, sourceID, payload string) (oracle.Request, error) {
	accountID = strings.TrimSpace(accountID)
	sourceID = strings.TrimSpace(sourceID)

	if accountID == "" {
		return oracle.Request{}, fmt.Errorf("account_id is required")
	}
	if sourceID == "" {
		return oracle.Request{}, fmt.Errorf("data_source_id is required")
	}
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return oracle.Request{}, fmt.Errorf("account validation failed: %w", err)
	}

	src, err := s.store.GetDataSource(ctx, sourceID)
	if err != nil {
		return oracle.Request{}, err
	}
	if src.AccountID != accountID {
		return oracle.Request{}, fmt.Errorf("data source %s does not belong to account %s", sourceID, accountID)
	}
	if !src.Enabled {
		return oracle.Request{}, fmt.Errorf("data source %s is disabled", sourceID)
	}

	req := oracle.Request{
		AccountID:    accountID,
		DataSourceID: sourceID,
		Status:       oracle.StatusPending,
		Payload:      payload,
	}
	req, err = s.store.CreateRequest(ctx, req)
	if err != nil {
		return oracle.Request{}, err
	}
	s.log.WithField("request_id", req.ID).
		WithField("account_id", accountID).
		WithField("source_id", sourceID).
		Info("oracle request created")
	return req, nil
}

// MarkRunning updates a request to running.
func (s *Service) MarkRunning(ctx context.Context, requestID string) (oracle.Request, error) {
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return oracle.Request{}, err
	}
	if req.Status != oracle.StatusPending {
		return oracle.Request{}, fmt.Errorf("cannot mark request in status %s as running", req.Status)
	}
	req.Status = oracle.StatusRunning
	return s.store.UpdateRequest(ctx, req)
}

// CompleteRequest records a successful result.
func (s *Service) CompleteRequest(ctx context.Context, requestID string, result string) (oracle.Request, error) {
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return oracle.Request{}, err
	}
	switch req.Status {
	case oracle.StatusRunning:
		// allowed transition
	case oracle.StatusPending:
		return oracle.Request{}, fmt.Errorf("cannot complete request in status %s", req.Status)
	case oracle.StatusSucceeded, oracle.StatusFailed:
		return oracle.Request{}, fmt.Errorf("request already %s", req.Status)
	default:
		return oracle.Request{}, fmt.Errorf("cannot complete request in status %s", req.Status)
	}
	req.Status = oracle.StatusSucceeded
	req.Result = result
	req.Error = ""
	req.CompletedAt = time.Now().UTC()
	return s.store.UpdateRequest(ctx, req)
}

// FailRequest records a failure.
func (s *Service) FailRequest(ctx context.Context, requestID string, errMsg string) (oracle.Request, error) {
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return oracle.Request{}, err
	}
	switch req.Status {
	case oracle.StatusRunning:
		// allowed transition
	case oracle.StatusPending:
		return oracle.Request{}, fmt.Errorf("cannot fail request in status %s", req.Status)
	case oracle.StatusSucceeded, oracle.StatusFailed:
		return oracle.Request{}, fmt.Errorf("request already %s", req.Status)
	default:
		return oracle.Request{}, fmt.Errorf("cannot fail request in status %s", req.Status)
	}
	req.Status = oracle.StatusFailed
	req.Error = strings.TrimSpace(errMsg)
	req.CompletedAt = time.Now().UTC()
	return s.store.UpdateRequest(ctx, req)
}

// ListRequests returns requests for an account.
func (s *Service) ListRequests(ctx context.Context, accountID string) ([]oracle.Request, error) {
	return s.store.ListRequests(ctx, accountID)
}

// ListPending returns requests that are awaiting fulfilment.
func (s *Service) ListPending(ctx context.Context) ([]oracle.Request, error) {
	return s.store.ListPendingRequests(ctx)
}

// GetRequest returns an individual request.
func (s *Service) GetRequest(ctx context.Context, requestID string) (oracle.Request, error) {
	return s.store.GetRequest(ctx, requestID)
}

// Descriptor advertises the service placement and capabilities.
func (s *Service) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "oracle",
		Domain:       "oracle",
		Layer:        core.LayerPlatform,
		Capabilities: []string{"requests", "resolve", "dispatch"},
	}
}
