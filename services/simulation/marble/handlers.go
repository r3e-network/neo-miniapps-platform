package neosimulation

import (
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/infrastructure/httputil"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

// handleStart starts the simulation.
func (s *Service) handleStart(w http.ResponseWriter, r *http.Request) {
	var req StartSimulationRequest
	if !httputil.DecodeJSON(w, r, &req) {
		return
	}

	// Override configuration if provided
	if len(req.MiniApps) > 0 {
		s.mu.Lock()
		s.miniApps = req.MiniApps
		s.mu.Unlock()
	}

	if req.MinIntervalMS > 0 && req.MaxIntervalMS > 0 {
		s.mu.Lock()
		s.minInterval = time.Duration(req.MinIntervalMS) * time.Millisecond
		s.maxInterval = time.Duration(req.MaxIntervalMS) * time.Millisecond
		s.mu.Unlock()
	}

	err := s.Start(r.Context())
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, StartSimulationResponse{
			Success: false,
			Message: err.Error(),
			Running: s.running,
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, StartSimulationResponse{
		Success:  true,
		Message:  "Simulation started successfully",
		MiniApps: s.miniApps,
		Running:  true,
	})
}

// handleStop stops the simulation.
func (s *Service) handleStop(w http.ResponseWriter, r *http.Request) {
	err := s.Stop()
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, StopSimulationResponse{
			Success: false,
			Message: err.Error(),
			Running: s.running,
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, StopSimulationResponse{
		Success: true,
		Message: "Simulation stopped successfully",
		Running: false,
	})
}

// handleStatus returns the current simulation status.
func (s *Service) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := s.GetStatus()
	httputil.WriteJSON(w, http.StatusOK, status)
}
