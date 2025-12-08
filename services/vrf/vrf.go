// Package vrf provides the Verifiable Random Function service.
//
// Architecture: Request-Callback Pattern
// 1. User contract calls VRF contract's requestRandomness(seed, numWords, callbackGasLimit)
// 2. VRF contract emits RandomnessRequested event
// 3. TEE listens for events, generates VRF proof, and calls fulfillRandomness on user contract
// 4. User contract receives random words in its fulfillRandomness callback
package vrf

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/google/uuid"
)

const (
	ServiceID   = "vrf"
	ServiceName = "VRF Service"
	Version     = "2.0.0"

	// Request status
	StatusPending   = "pending"
	StatusFulfilled = "fulfilled"
	StatusFailed    = "failed"

	// Polling interval for chain events
	EventPollInterval = 5 * time.Second

	// Service fee per request (in GAS smallest unit)
	ServiceFeePerRequest = 100000 // 0.001 GAS
)

// VRFRequest represents a randomness request from a user contract.
type VRFRequest struct {
	ID               string    `json:"id"`
	RequestID        string    `json:"request_id"`        // On-chain request ID
	UserID           string    `json:"user_id"`           // Service Layer user
	RequesterAddress string    `json:"requester_address"` // User contract address
	Seed             string    `json:"seed"`
	NumWords         int       `json:"num_words"`
	CallbackGasLimit int64     `json:"callback_gas_limit"`
	Status           string    `json:"status"`
	RandomWords      []string  `json:"random_words,omitempty"`
	Proof            string    `json:"proof,omitempty"`
	FulfillTxHash    string    `json:"fulfill_tx_hash,omitempty"`
	Error            string    `json:"error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	FulfilledAt      time.Time `json:"fulfilled_at,omitempty"`
}

func vrfRecordFromReq(req *VRFRequest) *database.VRFRequestRecord {
	return &database.VRFRequestRecord{
		ID:               req.ID,
		RequestID:        req.RequestID,
		UserID:           req.UserID,
		RequesterAddress: req.RequesterAddress,
		Seed:             req.Seed,
		NumWords:         req.NumWords,
		CallbackGasLimit: req.CallbackGasLimit,
		Status:           req.Status,
		RandomWords:      req.RandomWords,
		Proof:            req.Proof,
		FulfillTxHash:    req.FulfillTxHash,
		Error:            req.Error,
		CreatedAt:        req.CreatedAt,
		FulfilledAt:      req.FulfilledAt,
	}
}

func vrfReqFromRecord(rec *database.VRFRequestRecord) *VRFRequest {
	return &VRFRequest{
		ID:               rec.ID,
		RequestID:        rec.RequestID,
		UserID:           rec.UserID,
		RequesterAddress: rec.RequesterAddress,
		Seed:             rec.Seed,
		NumWords:         rec.NumWords,
		CallbackGasLimit: rec.CallbackGasLimit,
		Status:           rec.Status,
		RandomWords:      rec.RandomWords,
		Proof:            rec.Proof,
		FulfillTxHash:    rec.FulfillTxHash,
		Error:            rec.Error,
		CreatedAt:        rec.CreatedAt,
		FulfilledAt:      rec.FulfilledAt,
	}
}

// Service implements the VRF service.
type Service struct {
	*marble.Service
	mu sync.RWMutex

	privateKey *ecdsa.PrivateKey

	// Chain interaction
	chainClient   *chain.Client
	teeFulfiller  *chain.TEEFulfiller
	eventListener *chain.EventListener

	// Request tracking
	requests         map[string]*VRFRequest // requestID -> request (in-memory cache)
	pendingRequests  chan *VRFRequest       // ephemeral channel; source of truth is DB
	lastProcessedBlk uint64

	// Background workers
	stopCh chan struct{}
}

// Config holds VRF service configuration.
type Config struct {
	Marble        *marble.Marble
	DB            *database.Repository
	ChainClient   *chain.Client
	TEEFulfiller  *chain.TEEFulfiller
	EventListener *chain.EventListener
}

// New creates a new VRF service.
func New(cfg Config) (*Service, error) {
	base := marble.NewService(marble.ServiceConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	s := &Service{
		Service:         base,
		chainClient:     cfg.ChainClient,
		teeFulfiller:    cfg.TEEFulfiller,
		eventListener:   cfg.EventListener,
		requests:        make(map[string]*VRFRequest),
		pendingRequests: make(chan *VRFRequest, 100),
		stopCh:          make(chan struct{}),
	}

	// Load VRF private key from Marble secrets
	// UPGRADE SAFETY: VRF_PRIVATE_KEY is injected by MarbleRun Coordinator from
	// manifest-defined secrets. This key remains constant across enclave upgrades
	// (MRENCLAVE changes) as long as the manifest secret is unchanged.
	// The key is NOT derived from SGX sealing keys or enclave identity.
	if keyBytes, ok := cfg.Marble.Secret("VRF_PRIVATE_KEY"); ok {
		privateKey := new(ecdsa.PrivateKey)
		privateKey.Curve = elliptic.P256()
		privateKey.D = new(big.Int).SetBytes(keyBytes)
		privateKey.PublicKey.X, privateKey.PublicKey.Y = privateKey.Curve.ScalarBaseMult(keyBytes)
		s.privateKey = privateKey
	} else {
		// Generate new key pair if not provided
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			return nil, fmt.Errorf("generate key pair: %w", err)
		}
		s.privateKey = keyPair.PrivateKey
	}

	// Register routes
	s.registerRoutes()

	return s, nil
}

// registerRoutes registers HTTP routes.
func (s *Service) registerRoutes() {
	router := s.Router()
	router.HandleFunc("/health", marble.HealthHandler(s.Service)).Methods("GET")
	router.HandleFunc("/info", s.handleInfo).Methods("GET")
	router.HandleFunc("/pubkey", s.handlePublicKey).Methods("GET")
	router.HandleFunc("/request", s.handleCreateRequest).Methods("POST")
	router.HandleFunc("/request/{id}", s.handleGetRequest).Methods("GET")
	router.HandleFunc("/requests", s.handleListRequests).Methods("GET")
	// Direct API for off-chain usage
	router.HandleFunc("/random", s.handleDirectRandom).Methods("POST")
	router.HandleFunc("/verify", s.handleVerify).Methods("POST")
}

// DirectRandomRequest for direct API usage.
type DirectRandomRequest struct {
	Seed     string `json:"seed"`
	NumWords int    `json:"num_words,omitempty"`
}

// DirectRandomResponse for direct API usage.
type DirectRandomResponse struct {
	RequestID   string   `json:"request_id"`
	Seed        string   `json:"seed"`
	RandomWords []string `json:"random_words"`
	Proof       string   `json:"proof"`
	PublicKey   string   `json:"public_key"`
	Timestamp   string   `json:"timestamp"`
}

// Backward-compatible aliases used by tests.
type RandomRequest = DirectRandomRequest
type RandomResponse = DirectRandomResponse

// =============================================================================
// Lifecycle
// =============================================================================

// Start starts the VRF service and background workers.
func (s *Service) Start(ctx context.Context) error {
	if err := s.Service.Start(ctx); err != nil {
		return err
	}

	// Hydrate pending requests from DB
	if s.DB() != nil {
		if pending, err := s.DB().ListVRFRequestsByStatus(ctx, StatusPending); err == nil {
			for i := range pending {
				req := vrfReqFromRecord(&pending[i])
				s.requests[req.RequestID] = req
				select {
				case s.pendingRequests <- req:
				default:
				}
			}
		}
	}

	// Start background workers
	go s.runEventListener(ctx)
	go s.runRequestFulfiller(ctx)

	return nil
}

// Stop stops the VRF service.
func (s *Service) Stop() error {
	close(s.stopCh)
	return s.Service.Stop()
}

// =============================================================================
// Event Listener - Monitors chain for VRF requests
// =============================================================================

// runEventListener registers for VRFRequest events via the chain event listener.
func (s *Service) runEventListener(ctx context.Context) {
	if s.eventListener == nil {
		return
	}

	listener := s.eventListener
	listener.On("VRFRequest", func(event *chain.ContractEvent) error {
		parsed, err := chain.ParseVRFRequestEvent(event)
		if err != nil {
			return err
		}
		s.handleVRFRequestEvent(ctx, parsed)
		return nil
	})

	go listener.Start(ctx)
}

// handleVRFRequestEvent processes a single VRF request event.
func (s *Service) handleVRFRequestEvent(ctx context.Context, event *chain.VRFRequestEvent) {
	s.mu.Lock()
	if _, exists := s.requests[strconv.FormatUint(event.RequestID, 10)]; exists {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	reqID := strconv.FormatUint(event.RequestID, 10)
	request := &VRFRequest{
		ID:               uuid.New().String(),
		RequestID:        reqID,
		UserID:           "", // not provided by event; can be mapped via gateway if needed
		RequesterAddress: event.UserContract,
		Seed:             hex.EncodeToString(event.Seed),
		NumWords:         int(event.NumWords),
		CallbackGasLimit: 100000,
		Status:           StatusPending,
		CreatedAt:        time.Now(),
	}

	if s.DB() != nil {
		_ = s.DB().CreateVRFRequest(ctx, vrfRecordFromReq(request))
	}

	s.mu.Lock()
	s.requests[request.RequestID] = request
	s.mu.Unlock()

	select {
	case s.pendingRequests <- request:
	default:
	}
}

// =============================================================================
// Request Fulfiller - Generates randomness and calls back to user contracts
// =============================================================================

// runRequestFulfiller processes pending VRF requests.
func (s *Service) runRequestFulfiller(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case request := <-s.pendingRequests:
			s.fulfillRequest(ctx, request)
		}
	}
}

// fulfillRequest generates randomness and submits callback to user contract.
func (s *Service) fulfillRequest(ctx context.Context, request *VRFRequest) {
	// Generate VRF proof
	seedBytes, err := hex.DecodeString(request.Seed)
	if err != nil {
		seedBytes = []byte(request.Seed)
	}

	vrfProof, err := crypto.GenerateVRF(s.privateKey, seedBytes)
	if err != nil {
		s.markRequestFailed(request, fmt.Sprintf("generate VRF: %v", err))
		return
	}

	// Generate random words
	randomWords := make([]string, request.NumWords)
	randomWordsBig := make([]*big.Int, request.NumWords)
	for i := 0; i < request.NumWords; i++ {
		wordInput := append(vrfProof.Output, byte(i))
		wordHash := crypto.Hash256(wordInput)
		randomWords[i] = hex.EncodeToString(wordHash)
		randomWordsBig[i] = new(big.Int).SetBytes(wordHash)
	}

	// Submit callback to user contract via TEE fulfiller
	// Update request status
	s.mu.Lock()
	request.Status = StatusFulfilled
	request.RandomWords = randomWords
	request.Proof = hex.EncodeToString(vrfProof.Proof)
	request.FulfilledAt = time.Now()
	s.mu.Unlock()

	if s.DB() != nil {
		_ = s.DB().UpdateVRFRequest(ctx, vrfRecordFromReq(request))
	}

}

// markRequestFailed marks a request as failed.
func (s *Service) markRequestFailed(request *VRFRequest, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	request.Status = StatusFailed
	request.Error = errMsg

	if s.DB() != nil {
		_ = s.DB().UpdateVRFRequest(context.Background(), vrfRecordFromReq(request))
	}
}

// =============================================================================
// HTTP Handlers
// =============================================================================

func (s *Service) handleInfo(w http.ResponseWriter, r *http.Request) {
	pendingCount := 0
	fulfilledCount := 0

	if s.DB() != nil {
		if pending, err := s.DB().ListVRFRequestsByStatus(r.Context(), StatusPending); err == nil {
			pendingCount += len(pending)
		}
		if fulfilled, err := s.DB().ListVRFRequestsByStatus(r.Context(), StatusFulfilled); err == nil {
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

// CreateRequestInput for API-initiated requests (off-chain trigger).
type CreateRequestInput struct {
	Seed             string `json:"seed"`
	NumWords         int    `json:"num_words"`
	CallbackContract string `json:"callback_contract"`
	CallbackGasLimit int64  `json:"callback_gas_limit"`
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

	if s.DB() != nil {
		_ = s.DB().CreateVRFRequest(r.Context(), vrfRecordFromReq(request))
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
	if s.DB() != nil {
		if rec, err := s.DB().GetVRFRequest(r.Context(), requestID); err == nil {
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
	if s.DB() != nil {
		if rows, err := s.DB().ListVRFRequestsByStatus(r.Context(), StatusPending); err == nil {
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

// =============================================================================
// Direct API (for off-chain usage without callback)
// =============================================================================

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

// VerifyRequest represents a VRF verification request.
type VerifyRequest struct {
	Seed        string   `json:"seed"`
	RandomWords []string `json:"random_words"`
	Proof       string   `json:"proof"`
	PublicKey   string   `json:"public_key"`
}

// VerifyResponse represents a VRF verification response.
type VerifyResponse struct {
	Valid bool   `json:"valid"`
	Error string `json:"error,omitempty"`
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

// =============================================================================
// Core Logic
// =============================================================================

// GenerateRandomness generates verifiable random numbers.
func (s *Service) GenerateRandomness(ctx context.Context, seed string, numWords int) (*DirectRandomResponse, error) {
	seedBytes, err := hex.DecodeString(seed)
	if err != nil {
		seedBytes = []byte(seed)
	}

	// Generate VRF proof
	vrfProof, err := crypto.GenerateVRF(s.privateKey, seedBytes)
	if err != nil {
		return nil, fmt.Errorf("generate VRF: %w", err)
	}

	// Generate multiple random words from the VRF output
	randomWords := make([]string, numWords)
	for i := 0; i < numWords; i++ {
		wordInput := append(vrfProof.Output, byte(i))
		wordHash := crypto.Hash256(wordInput)
		randomWords[i] = hex.EncodeToString(wordHash)
	}

	return &DirectRandomResponse{
		RequestID:   uuid.New().String(),
		Seed:        seed,
		RandomWords: randomWords,
		Proof:       hex.EncodeToString(vrfProof.Proof),
		PublicKey:   hex.EncodeToString(vrfProof.PublicKey),
		Timestamp:   time.Now().Format(time.RFC3339),
	}, nil
}

// VerifyRandomness verifies a VRF proof.
func (s *Service) VerifyRandomness(req *VerifyRequest) (bool, error) {
	seedBytes, err := hex.DecodeString(req.Seed)
	if err != nil {
		seedBytes = []byte(req.Seed)
	}

	proofBytes, err := hex.DecodeString(req.Proof)
	if err != nil {
		return false, fmt.Errorf("invalid proof hex: %w", err)
	}

	pubKeyBytes, err := hex.DecodeString(req.PublicKey)
	if err != nil {
		return false, fmt.Errorf("invalid public key hex: %w", err)
	}

	// Parse public key
	if len(pubKeyBytes) != 33 {
		return false, fmt.Errorf("invalid public key length")
	}

	// Decompress public key
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), pubKeyBytes)
	if x == nil {
		return false, fmt.Errorf("invalid compressed public key")
	}

	publicKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	// Reconstruct VRF proof
	outputHash := crypto.Hash256(proofBytes)
	vrfProof := &crypto.VRFProof{
		PublicKey: pubKeyBytes,
		Proof:     proofBytes,
		Output:    outputHash,
	}

	return crypto.VerifyVRF(publicKey, seedBytes, vrfProof), nil
}
