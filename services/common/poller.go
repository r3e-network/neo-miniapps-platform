// Package common provides the Supabase request poller for Service Layer.
// The poller monitors Supabase for pending requests and processes them.
// This enables the Frontend ↔ Supabase ↔ Service Layer communication pattern.
package common

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/supabase/client"
)

// RequestPoller polls Supabase for pending requests and processes them.
type RequestPoller struct {
	mu sync.RWMutex

	supabase    *client.Client
	serviceType string
	processorID string

	// Handlers
	handlers map[string]RequestHandler

	// Polling config
	pollInterval time.Duration
	batchSize    int

	// State
	running bool
	done    chan struct{}
}

// RequestHandler processes a service request.
type RequestHandler func(ctx context.Context, req *ServiceRequest) (*ServiceResult, error)

// ServiceRequest represents a request from the queue.
type ServiceRequest struct {
	ID          string          `json:"id"`
	UserID      string          `json:"user_id"`
	ServiceType string          `json:"service_type"`
	Operation   string          `json:"operation"`
	Payload     json.RawMessage `json:"payload"`
	Priority    int             `json:"priority"`
	CreatedAt   time.Time       `json:"created_at"`
	Metadata    map[string]any  `json:"metadata"`
}

// ServiceResult is the result of processing a request.
type ServiceResult struct {
	Data         any    `json:"data,omitempty"`
	Error        string `json:"error,omitempty"`
	TEESignature []byte `json:"tee_signature,omitempty"`
}

// PollerConfig holds poller configuration.
type PollerConfig struct {
	SupabaseURL  string
	SupabaseKey  string
	ServiceType  string
	ProcessorID  string
	PollInterval time.Duration
	BatchSize    int
}

// NewRequestPoller creates a new request poller.
func NewRequestPoller(cfg PollerConfig) (*RequestPoller, error) {
	if cfg.SupabaseURL == "" || cfg.SupabaseKey == "" {
		return nil, fmt.Errorf("supabase credentials required")
	}
	if cfg.ServiceType == "" {
		return nil, fmt.Errorf("service type required")
	}

	supabase, err := client.New(client.Config{
		URL:    cfg.SupabaseURL,
		APIKey: cfg.SupabaseKey,
	})
	if err != nil {
		return nil, fmt.Errorf("create supabase client: %w", err)
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = 1 * time.Second
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 10
	}
	if cfg.ProcessorID == "" {
		cfg.ProcessorID = fmt.Sprintf("%s-%d", cfg.ServiceType, time.Now().UnixNano())
	}

	return &RequestPoller{
		supabase:     supabase,
		serviceType:  cfg.ServiceType,
		processorID:  cfg.ProcessorID,
		handlers:     make(map[string]RequestHandler),
		pollInterval: cfg.PollInterval,
		batchSize:    cfg.BatchSize,
		done:         make(chan struct{}),
	}, nil
}

// RegisterHandler registers a handler for an operation.
func (p *RequestPoller) RegisterHandler(operation string, handler RequestHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[operation] = handler
	log.Printf("[poller:%s] Registered handler for operation: %s", p.serviceType, operation)
}

// Start starts the poller.
func (p *RequestPoller) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("poller already running")
	}
	p.running = true
	p.done = make(chan struct{})
	p.mu.Unlock()

	log.Printf("[poller:%s] Starting request poller (interval: %v)", p.serviceType, p.pollInterval)

	go p.pollLoop(ctx)
	return nil
}

// Stop stops the poller.
func (p *RequestPoller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return
	}

	p.running = false
	close(p.done)
	log.Printf("[poller:%s] Stopped", p.serviceType)
}

// IsRunning returns true if the poller is running.
func (p *RequestPoller) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

func (p *RequestPoller) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.done:
			return
		case <-ticker.C:
			p.processPendingRequests(ctx)
		}
	}
}

func (p *RequestPoller) processPendingRequests(ctx context.Context) {
	// Fetch pending requests
	requests, err := p.fetchPendingRequests(ctx)
	if err != nil {
		log.Printf("[poller:%s] Error fetching requests: %v", p.serviceType, err)
		return
	}

	if len(requests) == 0 {
		return
	}

	log.Printf("[poller:%s] Processing %d pending requests", p.serviceType, len(requests))

	// Process each request
	for _, req := range requests {
		select {
		case <-ctx.Done():
			return
		case <-p.done:
			return
		default:
			p.processRequest(ctx, req)
		}
	}
}

func (p *RequestPoller) fetchPendingRequests(ctx context.Context) ([]*ServiceRequest, error) {
	// Query pending requests for this service type
	resp, err := p.supabase.From("service_requests").
		Select("*").
		Eq("service_type", p.serviceType).
		Eq("status", "pending").
		Order("priority", false). // DESC
		Order("created_at", true). // ASC
		Limit(p.batchSize).
		Execute(ctx)

	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	if err := resp.Error(); err != nil {
		return nil, err
	}

	var requests []*ServiceRequest
	if err := json.Unmarshal(resp.Body, &requests); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return requests, nil
}

func (p *RequestPoller) processRequest(ctx context.Context, req *ServiceRequest) {
	// Claim the request
	if err := p.claimRequest(ctx, req.ID); err != nil {
		log.Printf("[poller:%s] Failed to claim request %s: %v", p.serviceType, req.ID, err)
		return
	}

	log.Printf("[poller:%s] Processing request %s (operation: %s)", p.serviceType, req.ID, req.Operation)

	// Get handler
	p.mu.RLock()
	handler, ok := p.handlers[req.Operation]
	p.mu.RUnlock()

	if !ok {
		// Try default handler
		p.mu.RLock()
		handler, ok = p.handlers["*"]
		p.mu.RUnlock()
	}

	if !ok {
		p.failRequest(ctx, req.ID, fmt.Sprintf("no handler for operation: %s", req.Operation))
		return
	}

	// Execute handler
	result, err := handler(ctx, req)
	if err != nil {
		p.failRequest(ctx, req.ID, err.Error())
		return
	}

	// Complete request
	p.completeRequest(ctx, req.ID, result)
}

func (p *RequestPoller) claimRequest(ctx context.Context, requestID string) error {
	data := map[string]any{
		"status":     "processing",
		"started_at": time.Now().UTC().Format(time.RFC3339),
		"metadata": map[string]any{
			"processor_id": p.processorID,
		},
	}

	resp, err := p.supabase.From("service_requests").
		Eq("id", requestID).
		Eq("status", "pending").
		ExecuteUpdate(ctx, data)

	if err != nil {
		return err
	}

	return resp.Error()
}

func (p *RequestPoller) completeRequest(ctx context.Context, requestID string, result *ServiceResult) {
	data := map[string]any{
		"status":       "completed",
		"completed_at": time.Now().UTC().Format(time.RFC3339),
	}

	if result != nil {
		if result.Data != nil {
			resultJSON, _ := json.Marshal(result.Data)
			data["result"] = json.RawMessage(resultJSON)
		}
		if result.TEESignature != nil {
			data["tee_signature"] = result.TEESignature
		}
	}

	resp, err := p.supabase.From("service_requests").
		Eq("id", requestID).
		ExecuteUpdate(ctx, data)

	if err != nil {
		log.Printf("[poller:%s] Failed to complete request %s: %v", p.serviceType, requestID, err)
		return
	}

	if err := resp.Error(); err != nil {
		log.Printf("[poller:%s] Failed to complete request %s: %v", p.serviceType, requestID, err)
		return
	}

	// Create notification
	p.createNotification(ctx, requestID, "request_completed", result)

	log.Printf("[poller:%s] Completed request %s", p.serviceType, requestID)
}

func (p *RequestPoller) failRequest(ctx context.Context, requestID string, errorMsg string) {
	data := map[string]any{
		"status":        "failed",
		"error_message": errorMsg,
		"completed_at":  time.Now().UTC().Format(time.RFC3339),
	}

	resp, err := p.supabase.From("service_requests").
		Eq("id", requestID).
		ExecuteUpdate(ctx, data)

	if err != nil {
		log.Printf("[poller:%s] Failed to fail request %s: %v", p.serviceType, requestID, err)
		return
	}

	if err := resp.Error(); err != nil {
		log.Printf("[poller:%s] Failed to fail request %s: %v", p.serviceType, requestID, err)
		return
	}

	// Create notification
	p.createNotification(ctx, requestID, "request_failed", &ServiceResult{Error: errorMsg})

	log.Printf("[poller:%s] Failed request %s: %s", p.serviceType, requestID, errorMsg)
}

func (p *RequestPoller) createNotification(ctx context.Context, requestID, notifType string, result *ServiceResult) {
	// Get user_id from request
	resp, err := p.supabase.From("service_requests").
		Select("user_id").
		Eq("id", requestID).
		Single().
		Execute(ctx)

	if err != nil {
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(resp.Body, &req); err != nil {
		return
	}

	notification := map[string]any{
		"user_id":        req.UserID,
		"type":           notifType,
		"title":          fmt.Sprintf("%s %s", p.serviceType, notifType),
		"reference_type": "service_request",
		"reference_id":   requestID,
	}

	if result != nil {
		if result.Error != "" {
			notification["message"] = result.Error
		}
		if result.Data != nil {
			dataJSON, _ := json.Marshal(result.Data)
			notification["data"] = json.RawMessage(dataJSON)
		}
	}

	p.supabase.From("realtime_notifications").ExecuteInsert(ctx, notification)
}

// =============================================================================
// Specific Queue Pollers
// =============================================================================

// OraclePoller polls the oracle_request_queue table.
type OraclePoller struct {
	*RequestPoller
}

// NewOraclePoller creates a new Oracle poller.
func NewOraclePoller(supabaseURL, supabaseKey string) (*OraclePoller, error) {
	poller, err := NewRequestPoller(PollerConfig{
		SupabaseURL: supabaseURL,
		SupabaseKey: supabaseKey,
		ServiceType: "oracle",
	})
	if err != nil {
		return nil, err
	}

	return &OraclePoller{RequestPoller: poller}, nil
}

// VRFPoller polls the vrf_request_queue table.
type VRFPoller struct {
	*RequestPoller
}

// NewVRFPoller creates a new VRF poller.
func NewVRFPoller(supabaseURL, supabaseKey string) (*VRFPoller, error) {
	poller, err := NewRequestPoller(PollerConfig{
		SupabaseURL: supabaseURL,
		SupabaseKey: supabaseKey,
		ServiceType: "vrf",
	})
	if err != nil {
		return nil, err
	}

	return &VRFPoller{RequestPoller: poller}, nil
}
