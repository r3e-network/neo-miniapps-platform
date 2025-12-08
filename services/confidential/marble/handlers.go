// Package confidential provides HTTP handlers for the confidential compute service.
package confidentialmarble

import (
	"encoding/json"
	"net/http"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

func (s *Service) handleExecute(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Script == "" {
		http.Error(w, "script required", http.StatusBadRequest)
		return
	}

	if req.EntryPoint == "" {
		req.EntryPoint = "main"
	}

	result, err := s.Execute(r.Context(), userID, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Service) handleGetJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "not found"})
}

func (s *Service) handleListJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]interface{}{})
}
