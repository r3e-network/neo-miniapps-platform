// Package neocompute provides HTTP handlers for the neocompute service.
package neocomputemarble

import (
	"net/http"

	"github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/gorilla/mux"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

func (s *Service) handleExecute(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	var req ExecuteRequest
	if !httputil.DecodeJSON(w, r, &req) {
		return
	}

	if req.Script == "" {
		httputil.BadRequest(w, "script required")
		return
	}

	if req.EntryPoint == "" {
		req.EntryPoint = "main"
	}

	result, err := s.Execute(r.Context(), userID, &req)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}

func (s *Service) handleGetJob(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	jobID := mux.Vars(r)["id"]
	if jobID == "" {
		httputil.BadRequest(w, "job id required")
		return
	}

	job := s.getJob(userID, jobID)
	if job == nil {
		httputil.NotFound(w, "job not found")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, job)
}

func (s *Service) handleListJobs(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	jobs := s.listJobs(userID)
	if jobs == nil {
		jobs = []*ExecuteResponse{}
	}

	httputil.WriteJSON(w, http.StatusOK, jobs)
}
