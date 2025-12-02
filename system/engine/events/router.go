// Package events provides the request router for service layer.
// RequestRouter manages request lifecycle, ID generation, and routing to services.
package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
)

// RequestStatus represents the status of a service request.
type RequestStatus string

const (
	StatusPending   RequestStatus = "pending"
	StatusRunning   RequestStatus = "running"
	StatusSucceeded RequestStatus = "succeeded"
	StatusFailed    RequestStatus = "failed"
	StatusCancelled RequestStatus = "cancelled"
)

// ServiceType identifies the type of service.
type ServiceType string

const (
	ServiceOracle     ServiceType = "oracle"
	ServiceVRF        ServiceType = "vrf"
	ServiceDataFeeds  ServiceType = "datafeeds"
	ServiceAutomation ServiceType = "automation"
	ServiceSecrets    ServiceType = "secrets"
	ServiceFunctions  ServiceType = "functions"
	ServiceCCIP       ServiceType = "ccip"
)

// Request represents a service request in the system.
type Request struct {
	ID            string            `json:"id"`
	ExternalID    string            `json:"external_id,omitempty"` // On-chain request ID
	AccountID     string            `json:"account_id"`
	ServiceType   ServiceType       `json:"service_type"`
	ServiceID     string            `json:"service_id,omitempty"` // Specific service instance
	Status        RequestStatus     `json:"status"`
	Payload       map[string]any    `json:"payload,omitempty"`
	Result        map[string]any    `json:"result,omitempty"`
	Error         string            `json:"error,omitempty"`
	Fee           int64             `json:"fee"`
	FeeID         string            `json:"fee_id,omitempty"`
	TxHash        string            `json:"tx_hash,omitempty"`
	CallbackHash  string            `json:"callback_hash,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Attempts      int               `json:"attempts"`
	MaxAttempts   int               `json:"max_attempts"`
}

// RequestStore persists requests.
type RequestStore interface {
	Create(ctx context.Context, req *Request) error
	Get(ctx context.Context, id string) (*Request, error)
	GetByExternalID(ctx context.Context, externalID string) (*Request, error)
	Update(ctx context.Context, req *Request) error
	List(ctx context.Context, accountID string, serviceType ServiceType, status RequestStatus, limit int) ([]*Request, error)
	ListPending(ctx context.Context, serviceType ServiceType, limit int) ([]*Request, error)
}

// ServiceHandler processes requests for a specific service type.
type ServiceHandler interface {
	// ServiceType returns the type of service this handler supports.
	ServiceType() ServiceType

	// ProcessRequest handles a service request.
	ProcessRequest(ctx context.Context, req *Request) error

	// FulfillRequest submits the result back to the blockchain.
	FulfillRequest(ctx context.Context, req *Request, result map[string]any) error
}

// RequestRouter routes requests to appropriate service handlers.
type RequestRouter struct {
	handlers map[ServiceType]ServiceHandler
	store    RequestStore
	log      *logger.Logger

	// Request processing
	pendingQueue chan *Request
	workerCount  int

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// RouterConfig configures the request router.
type RouterConfig struct {
	Store       RequestStore
	Logger      *logger.Logger
	QueueSize   int
	WorkerCount int
}

// NewRequestRouter creates a new request router.
func NewRequestRouter(cfg RouterConfig) *RequestRouter {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 500
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 4
	}
	if cfg.Logger == nil {
		cfg.Logger = logger.NewDefault("router")
	}

	return &RequestRouter{
		handlers:     make(map[ServiceType]ServiceHandler),
		store:        cfg.Store,
		log:          cfg.Logger,
		pendingQueue: make(chan *Request, cfg.QueueSize),
		workerCount:  cfg.WorkerCount,
	}
}

// RegisterHandler registers a service handler.
func (r *RequestRouter) RegisterHandler(handler ServiceHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	svcType := handler.ServiceType()
	r.handlers[svcType] = handler
	r.log.WithField("service_type", svcType).Info("service handler registered")
}

// UnregisterHandler removes a service handler.
func (r *RequestRouter) UnregisterHandler(serviceType ServiceType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, serviceType)
}

// Start begins processing requests.
func (r *RequestRouter) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("router already running")
	}
	r.running = true
	r.stopCh = make(chan struct{})
	r.doneCh = make(chan struct{})
	r.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < r.workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			r.worker(ctx, workerID)
		}(i)
	}

	go func() {
		wg.Wait()
		close(r.doneCh)
	}()

	r.log.WithField("workers", r.workerCount).Info("request router started")
	return nil
}

// Stop halts request processing.
func (r *RequestRouter) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	r.running = false
	close(r.stopCh)
	r.mu.Unlock()

	<-r.doneCh
	r.log.Info("request router stopped")
}

// CreateRequest creates a new service request with a unique ID.
func (r *RequestRouter) CreateRequest(ctx context.Context, accountID string, serviceType ServiceType, payload map[string]any, opts ...RequestOption) (*Request, error) {
	req := &Request{
		ID:          generateRequestID(),
		AccountID:   accountID,
		ServiceType: serviceType,
		Status:      StatusPending,
		Payload:     payload,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		MaxAttempts: 3,
		Metadata:    make(map[string]string),
	}

	for _, opt := range opts {
		opt(req)
	}

	if r.store != nil {
		if err := r.store.Create(ctx, req); err != nil {
			return nil, fmt.Errorf("failed to store request: %w", err)
		}
	}

	r.log.WithField("request_id", req.ID).
		WithField("account_id", accountID).
		WithField("service_type", serviceType).
		Info("request created")

	return req, nil
}

// RequestOption configures a request.
type RequestOption func(*Request)

// WithExternalID sets the external (on-chain) request ID.
func WithExternalID(id string) RequestOption {
	return func(r *Request) { r.ExternalID = id }
}

// WithServiceID sets the specific service instance ID.
func WithServiceID(id string) RequestOption {
	return func(r *Request) { r.ServiceID = id }
}

// WithFee sets the request fee.
func WithFee(fee int64, feeID string) RequestOption {
	return func(r *Request) {
		r.Fee = fee
		r.FeeID = feeID
	}
}

// WithTxHash sets the originating transaction hash.
func WithTxHash(hash string) RequestOption {
	return func(r *Request) { r.TxHash = hash }
}

// WithCallback sets the callback contract hash.
func WithCallback(hash string) RequestOption {
	return func(r *Request) { r.CallbackHash = hash }
}

// WithMetadata adds metadata to the request.
func WithMetadata(key, value string) RequestOption {
	return func(r *Request) {
		if r.Metadata == nil {
			r.Metadata = make(map[string]string)
		}
		r.Metadata[key] = value
	}
}

// WithMaxAttempts sets the maximum retry attempts.
func WithMaxAttempts(n int) RequestOption {
	return func(r *Request) { r.MaxAttempts = n }
}

// SubmitRequest queues a request for processing.
func (r *RequestRouter) SubmitRequest(req *Request) error {
	r.mu.RLock()
	running := r.running
	r.mu.RUnlock()

	if !running {
		return fmt.Errorf("router not running")
	}

	select {
	case r.pendingQueue <- req:
		return nil
	default:
		return fmt.Errorf("request queue full")
	}
}

// ProcessRequestSync processes a request synchronously.
func (r *RequestRouter) ProcessRequestSync(ctx context.Context, req *Request) error {
	r.mu.RLock()
	handler, ok := r.handlers[req.ServiceType]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler for service type: %s", req.ServiceType)
	}

	// Update status to running
	req.Status = StatusRunning
	req.Attempts++
	req.UpdatedAt = time.Now().UTC()

	if r.store != nil {
		if err := r.store.Update(ctx, req); err != nil {
			r.log.WithError(err).Warn("failed to update request status")
		}
	}

	// Process the request
	if err := handler.ProcessRequest(ctx, req); err != nil {
		req.Status = StatusFailed
		req.Error = err.Error()
		req.UpdatedAt = time.Now().UTC()
		now := time.Now().UTC()
		req.CompletedAt = &now

		if r.store != nil {
			r.store.Update(ctx, req)
		}

		return err
	}

	return nil
}

// CompleteRequest marks a request as completed with result.
func (r *RequestRouter) CompleteRequest(ctx context.Context, requestID string, result map[string]any) error {
	if r.store == nil {
		return fmt.Errorf("no store configured")
	}

	req, err := r.store.Get(ctx, requestID)
	if err != nil {
		return fmt.Errorf("request not found: %w", err)
	}

	req.Status = StatusSucceeded
	req.Result = result
	req.UpdatedAt = time.Now().UTC()
	now := time.Now().UTC()
	req.CompletedAt = &now

	if err := r.store.Update(ctx, req); err != nil {
		return fmt.Errorf("failed to update request: %w", err)
	}

	r.log.WithField("request_id", requestID).Info("request completed")
	return nil
}

// FailRequest marks a request as failed.
func (r *RequestRouter) FailRequest(ctx context.Context, requestID string, errMsg string) error {
	if r.store == nil {
		return fmt.Errorf("no store configured")
	}

	req, err := r.store.Get(ctx, requestID)
	if err != nil {
		return fmt.Errorf("request not found: %w", err)
	}

	req.Status = StatusFailed
	req.Error = errMsg
	req.UpdatedAt = time.Now().UTC()
	now := time.Now().UTC()
	req.CompletedAt = &now

	if err := r.store.Update(ctx, req); err != nil {
		return fmt.Errorf("failed to update request: %w", err)
	}

	r.log.WithField("request_id", requestID).
		WithField("error", errMsg).
		Info("request failed")
	return nil
}

// GetRequest retrieves a request by ID.
func (r *RequestRouter) GetRequest(ctx context.Context, requestID string) (*Request, error) {
	if r.store == nil {
		return nil, fmt.Errorf("no store configured")
	}
	return r.store.Get(ctx, requestID)
}

// GetRequestByExternalID retrieves a request by external (on-chain) ID.
func (r *RequestRouter) GetRequestByExternalID(ctx context.Context, externalID string) (*Request, error) {
	if r.store == nil {
		return nil, fmt.Errorf("no store configured")
	}
	return r.store.GetByExternalID(ctx, externalID)
}

// ListRequests lists requests with filters.
func (r *RequestRouter) ListRequests(ctx context.Context, accountID string, serviceType ServiceType, status RequestStatus, limit int) ([]*Request, error) {
	if r.store == nil {
		return nil, fmt.Errorf("no store configured")
	}
	return r.store.List(ctx, accountID, serviceType, status, limit)
}

// worker processes requests from the queue.
func (r *RequestRouter) worker(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case req := <-r.pendingQueue:
			if err := r.ProcessRequestSync(ctx, req); err != nil {
				r.log.WithField("request_id", req.ID).
					WithError(err).
					Error("request processing failed")

				// Retry if attempts remaining
				if req.Attempts < req.MaxAttempts {
					req.Status = StatusPending
					r.SubmitRequest(req)
				}
			}
		}
	}
}

// generateRequestID generates a unique request ID.
func generateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("req_%s_%d", hex.EncodeToString(b[:8]), time.Now().UnixNano()%1000000)
}

// RouterStats holds router statistics.
type RouterStats struct {
	Running       bool `json:"running"`
	HandlersCount int  `json:"handlers_count"`
	QueueSize     int  `json:"queue_size"`
	QueueCapacity int  `json:"queue_capacity"`
}

// Stats returns router statistics.
func (r *RequestRouter) Stats() RouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return RouterStats{
		Running:       r.running,
		HandlersCount: len(r.handlers),
		QueueSize:     len(r.pendingQueue),
		QueueCapacity: cap(r.pendingQueue),
	}
}
