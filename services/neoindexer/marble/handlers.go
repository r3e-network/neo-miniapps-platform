package neoindexer

import (
	"encoding/json"
	"net/http"
	"strconv"

	commonservice "github.com/R3E-Network/service_layer/services/common/service"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

// RegisterRoutes registers HTTP routes for the NeoIndexer service.
func (s *Service) RegisterRoutes() {
	// Standard routes from BaseService
	s.Router().HandleFunc("/health", commonservice.HealthHandler(s.BaseService)).Methods("GET")
	s.Router().HandleFunc("/ready", commonservice.ReadinessHandler(s.BaseService)).Methods("GET")
	s.Router().HandleFunc("/info", commonservice.InfoHandler(s.BaseService)).Methods("GET")

	// NeoIndexer-specific routes
	s.Router().HandleFunc("/status", s.handleStatus).Methods("GET")
	s.Router().HandleFunc("/replay", s.handleReplay).Methods("POST")
	s.Router().HandleFunc("/rpc/health", s.handleRPCHealth).Methods("GET")
}

// handleStatus returns the current indexer status.
func (s *Service) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	status := map[string]any{
		"service":              ServiceName,
		"version":              Version,
		"chain_id":             s.config.ChainID,
		"last_processed_block": s.progress.LastProcessedBlock,
		"last_block_hash":      s.progress.LastBlockHash,
		"blocks_processed":     s.blocksProcessed,
		"events_published":     s.eventsPublished,
		"confirmation_depth":   s.config.ConfirmationDepth,
		"poll_interval":        s.config.PollInterval.String(),
	}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleReplay triggers a replay from a specific block.
func (s *Service) handleReplay(w http.ResponseWriter, r *http.Request) {
	// Parse start block from query parameter
	startBlockStr := r.URL.Query().Get("start_block")
	if startBlockStr == "" {
		http.Error(w, "start_block parameter required", http.StatusBadRequest)
		return
	}

	startBlock, err := strconv.ParseInt(startBlockStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid start_block parameter", http.StatusBadRequest)
		return
	}

	// Update progress to trigger replay
	s.mu.Lock()
	s.progress.LastProcessedBlock = startBlock - 1
	s.mu.Unlock()

	response := map[string]any{
		"status":      "replay_initiated",
		"start_block": startBlock,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleRPCHealth returns the health status of RPC endpoints.
func (s *Service) handleRPCHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	endpoints := make([]map[string]any, len(s.rpcEndpoints))
	for i, ep := range s.rpcEndpoints {
		endpoints[i] = map[string]any{
			"url":        ep.URL,
			"priority":   ep.Priority,
			"healthy":    ep.Healthy,
			"latency_ms": ep.Latency,
			"active":     i == s.currentRPC,
		}
	}
	s.mu.RUnlock()

	response := map[string]any{
		"endpoints":   endpoints,
		"current_rpc": s.currentRPC,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
