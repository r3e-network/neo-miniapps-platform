package txsubmitter

import (
	"context"

	"net/http"
	"strconv"
	"time"

	"github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/services/txsubmitter/supabase"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

// RegisterRoutes registers the TxSubmitter HTTP routes.
func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	// Standard endpoints (from BaseService)
	s.BaseService.RegisterStandardRoutes()

	// TxSubmitter-specific endpoints
	mux.HandleFunc("/submit", s.handleSubmit)
	mux.HandleFunc("/tx/", s.handleGetTx)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/rpc/health", s.handleRPCHealth)
}

// handleSubmit handles POST /submit - submit a transaction.
func (s *Service) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get service ID from header
	serviceID := r.Header.Get("X-Service-ID")
	if serviceID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "X-Service-ID header required")
		return
	}

	var req TxRequest
	if !httputil.DecodeJSON(w, r, &req) {
		return
	}

	resp, err := s.Submit(r.Context(), serviceID, &req)
	if err != nil {
		// Return the response even on error (contains error details)
		if resp != nil {
			httputil.WriteJSON(w, http.StatusBadRequest, resp)
			return
		}
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// handleGetTx handles GET /tx/{id} - get transaction status.
func (s *Service) handleGetTx(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract ID from path: /tx/123
	path := r.URL.Path
	if len(path) <= 4 {
		httputil.WriteError(w, http.StatusBadRequest, "transaction ID required")
		return
	}

	idStr := path[4:] // Remove "/tx/"
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid transaction ID")
		return
	}

	record, err := s.repo.GetByID(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "transaction not found")
		return
	}

	resp := &TxResponse{
		ID:          record.ID,
		TxHash:      record.TxHash,
		Status:      string(record.Status),
		GasConsumed: record.GasConsumed,
		Error:       record.ErrorMessage,
		SubmittedAt: record.SubmittedAt,
		ConfirmedAt: record.ConfirmedAt,
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// handleStatus handles GET /status - detailed service status.
func (s *Service) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.mu.RLock()
	var totalEndpoints, healthyEndpoints int
	if s.rpcPool != nil {
		endpoints := s.rpcPool.GetEndpoints()
		totalEndpoints = len(endpoints)
		healthyEndpoints = s.rpcPool.HealthyCount()
	}
	status := ServiceStatus{
		Service:          ServiceName,
		Version:          Version,
		Healthy:          s.rpcPool != nil && healthyEndpoints > 0,
		RPCEndpoints:     totalEndpoints,
		HealthyEndpoints: healthyEndpoints,
		PendingTxs:       len(s.pendingTxs),
		TxsSubmitted:     s.txsSubmitted,
		TxsConfirmed:     s.txsConfirmed,
		TxsFailed:        s.txsFailed,
		RateLimitStatus:  s.rateLimiter.Status(),
		Uptime:           time.Since(s.startTime),
	}
	s.mu.RUnlock()

	httputil.WriteJSON(w, http.StatusOK, status)
}

// handleRPCHealth handles GET /rpc/health - RPC endpoint health.
func (s *Service) handleRPCHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if s.rpcPool == nil {
		httputil.WriteError(w, http.StatusServiceUnavailable, "RPC pool not configured")
		return
	}

	endpoints := s.rpcPool.GetEndpoints()
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"total":     len(endpoints),
		"healthy":   s.rpcPool.HealthyCount(),
		"endpoints": endpoints,
	})
}

// =============================================================================
// Internal API (for service-to-service calls)
// =============================================================================

// SubmitInternal is for internal service-to-service transaction submission.
// This bypasses HTTP and calls the service directly.
func (s *Service) SubmitInternal(ctx context.Context, fromService string, req *TxRequest) (*TxResponse, error) {
	return s.Submit(ctx, fromService, req)
}

// GetTxStatus returns the status of a transaction by ID.
func (s *Service) GetTxStatus(ctx context.Context, id int64) (*supabase.ChainTxRecord, error) {
	return s.repo.GetByID(ctx, id)
}

// GetTxByHash returns a transaction by its hash.
func (s *Service) GetTxByHash(ctx context.Context, txHash string) (*supabase.ChainTxRecord, error) {
	return s.repo.GetByTxHash(ctx, txHash)
}
