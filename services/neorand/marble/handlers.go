package neorand

import (
	"encoding/hex"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/httputil"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

func (s *Service) handleInfo(w http.ResponseWriter, r *http.Request) {
	pendingCount := 0
	fulfilledCount := 0

	if s.repo != nil {
		if pending, err := s.repo.ListByStatus(r.Context(), StatusPending); err == nil {
			pendingCount += len(pending)
		}
		if fulfilled, err := s.repo.ListByStatus(r.Context(), StatusFulfilled); err == nil {
			fulfilledCount += len(fulfilled)
		}
	}

	s.mu.RLock()
	for _, req := range s.requests {
		switch req.Status {
		case StatusPending:
			pendingCount++
		case StatusFulfilled:
			fulfilledCount++
		}
	}
	s.mu.RUnlock()

	pubKey := crypto.PublicKeyToBytes(&s.privateKey.PublicKey)

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":             "active",
		"public_key":         hex.EncodeToString(pubKey),
		"pending_requests":   pendingCount,
		"fulfilled_requests": fulfilledCount,
		"service_fee":        ServiceFeePerRequest,
	})
}

func (s *Service) handlePublicKey(w http.ResponseWriter, r *http.Request) {
	pubKey := crypto.PublicKeyToBytes(&s.privateKey.PublicKey)
	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"public_key": hex.EncodeToString(pubKey),
	})
}

func (s *Service) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	var input CreateRequestInput
	if !httputil.DecodeJSON(w, r, &input) {
		return
	}

	if input.Seed == "" {
		httputil.BadRequest(w, "seed is required")
		return
	}
	if input.NumWords <= 0 {
		input.NumWords = 1
	}
	if input.NumWords > MaxNumWords {
		input.NumWords = MaxNumWords
	}
	if input.CallbackGasLimit <= 0 {
		input.CallbackGasLimit = 100000
	}

	// Create request
	requestID := uuid.New().String()
	request := &Request{
		ID:               uuid.New().String(),
		RequestID:        requestID,
		UserID:           userID,
		RequesterAddress: input.CallbackContract,
		Seed:             input.Seed,
		NumWords:         input.NumWords,
		CallbackGasLimit: input.CallbackGasLimit,
		Status:           StatusPending,
		CreatedAt:        time.Now(),
	}

	if s.repo != nil {
		if err := s.repo.Create(r.Context(), neorandRecordFromReq(request)); err != nil {
			httputil.InternalError(w, "failed to persist request")
			return
		}
	}

	s.mu.Lock()
	s.requests[requestID] = request
	s.mu.Unlock()

	// Queue for fulfillment
	select {
	case s.pendingRequests <- request:
	default:
		httputil.ServiceUnavailable(w, "service busy")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]any{
		"request_id":  requestID,
		"status":      StatusPending,
		"service_fee": ServiceFeePerRequest,
	})
}

func (s *Service) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	requestID := mux.Vars(r)["id"]

	var request *Request
	if s.repo != nil {
		if rec, err := s.repo.GetByRequestID(r.Context(), requestID); err == nil {
			request = neorandReqFromRecord(rec)
		}
	}
	if request == nil {
		s.mu.RLock()
		request = cloneRequest(s.requests[requestID])
		s.mu.RUnlock()
	}
	if request == nil {
		httputil.NotFound(w, "request not found")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, request)
}

func (s *Service) handleListRequests(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	var requests []*Request
	if s.repo != nil {
		if rows, err := s.repo.ListByStatus(r.Context(), StatusPending); err == nil {
			for i := range rows {
				req := neorandReqFromRecord(&rows[i])
				if req.UserID == userID {
					requests = append(requests, req)
				}
			}
		}
	}
	s.mu.RLock()
	for _, req := range s.requests {
		if req.UserID == userID {
			requests = append(requests, cloneRequest(req))
		}
	}
	s.mu.RUnlock()

	httputil.WriteJSON(w, http.StatusOK, requests)
}

func (s *Service) handleDirectRandom(w http.ResponseWriter, r *http.Request) {
	var req DirectRandomRequest
	if !httputil.DecodeJSON(w, r, &req) {
		return
	}

	if req.Seed == "" {
		httputil.BadRequest(w, "seed is required")
		return
	}

	if req.NumWords <= 0 {
		req.NumWords = 1
	}
	if req.NumWords > MaxNumWords {
		req.NumWords = MaxNumWords
	}

	result, err := s.GenerateRandomness(r.Context(), req.Seed, req.NumWords)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}

// handleRandom is a backward-compatible alias for handleDirectRandom.
func (s *Service) handleRandom(w http.ResponseWriter, r *http.Request) {
	s.handleDirectRandom(w, r)
}

func (s *Service) handleVerify(w http.ResponseWriter, r *http.Request) {
	var req VerifyRequest
	if !httputil.DecodeJSON(w, r, &req) {
		return
	}

	valid, err := s.VerifyRandomness(&req)
	resp := VerifyResponse{Valid: valid}
	if err != nil {
		resp.Error = err.Error()
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

func cloneRequest(req *Request) *Request {
	if req == nil {
		return nil
	}
	cpy := *req
	if req.RandomWords != nil {
		cpy.RandomWords = append([]string(nil), req.RandomWords...)
	}
	return &cpy
}
