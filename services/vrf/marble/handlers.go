package vrfmarble

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/google/uuid"
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":             "active",
		"public_key":         hex.EncodeToString(pubKey),
		"pending_requests":   pendingCount,
		"fulfilled_requests": fulfilledCount,
		"service_fee":        ServiceFeePerRequest,
	})
}

func (s *Service) handlePublicKey(w http.ResponseWriter, r *http.Request) {
	pubKey := crypto.PublicKeyToBytes(&s.privateKey.PublicKey)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"public_key": hex.EncodeToString(pubKey),
	})
}

func (s *Service) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var input CreateRequestInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if input.Seed == "" {
		http.Error(w, "seed is required", http.StatusBadRequest)
		return
	}
	if input.NumWords <= 0 {
		input.NumWords = 1
	}
	if input.NumWords > 10 {
		input.NumWords = 10
	}
	if input.CallbackGasLimit <= 0 {
		input.CallbackGasLimit = 100000
	}

	// Create request
	requestID := uuid.New().String()
	request := &VRFRequest{
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
		_ = s.repo.Create(r.Context(), vrfRecordFromReq(request))
	}

	s.mu.Lock()
	s.requests[requestID] = request
	s.mu.Unlock()

	// Queue for fulfillment
	select {
	case s.pendingRequests <- request:
	default:
		http.Error(w, "service busy", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"request_id":  requestID,
		"status":      StatusPending,
		"service_fee": ServiceFeePerRequest,
	})
}

func (s *Service) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	requestID := r.URL.Path[len("/request/"):]

	var request *VRFRequest
	if s.repo != nil {
		if rec, err := s.repo.GetByRequestID(r.Context(), requestID); err == nil {
			request = vrfReqFromRecord(rec)
		}
	}
	if request == nil {
		s.mu.RLock()
		request = s.requests[requestID]
		s.mu.RUnlock()
	}
	if request == nil {
		http.Error(w, "request not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(request)
}

func (s *Service) handleListRequests(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var requests []*VRFRequest
	if s.repo != nil {
		if rows, err := s.repo.ListByStatus(r.Context(), StatusPending); err == nil {
			for i := range rows {
				req := vrfReqFromRecord(&rows[i])
				if req.UserID == userID {
					requests = append(requests, req)
				}
			}
		}
	}
	s.mu.RLock()
	for _, req := range s.requests {
		if req.UserID == userID {
			requests = append(requests, req)
		}
	}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

func (s *Service) handleDirectRandom(w http.ResponseWriter, r *http.Request) {
	var req DirectRandomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Seed == "" {
		http.Error(w, "seed is required", http.StatusBadRequest)
		return
	}

	if req.NumWords <= 0 {
		req.NumWords = 1
	}

	result, err := s.GenerateRandomness(r.Context(), req.Seed, req.NumWords)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleRandom is a backward-compatible alias for handleDirectRandom.
func (s *Service) handleRandom(w http.ResponseWriter, r *http.Request) {
	s.handleDirectRandom(w, r)
}

func (s *Service) handleVerify(w http.ResponseWriter, r *http.Request) {
	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	valid, err := s.VerifyRandomness(&req)
	resp := VerifyResponse{Valid: valid}
	if err != nil {
		resp.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
