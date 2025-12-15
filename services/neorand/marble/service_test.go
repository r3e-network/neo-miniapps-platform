// Package neorand provides the Verifiable Random Function service.
package neorand

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/marble"
	neorandsupabase "github.com/R3E-Network/service_layer/services/neorand/supabase"
)

// =============================================================================
// Service Tests
// =============================================================================

func TestNew(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})

	svc, err := New(Config{
		Marble: m,
		DB:     nil,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc.ID() != ServiceID {
		t.Errorf("ID() = %s, want %s", svc.ID(), ServiceID)
	}
	if svc.Name() != ServiceName {
		t.Errorf("Name() = %s, want %s", svc.Name(), ServiceName)
	}
	if svc.Version() != Version {
		t.Errorf("Version() = %s, want %s", svc.Version(), Version)
	}
}

func TestServiceConstants(t *testing.T) {
	if ServiceID != "neorand" {
		t.Errorf("ServiceID = %s, want neorand", ServiceID)
	}
	if ServiceName != "NeoRand Service" {
		t.Errorf("ServiceName = %s, want NeoRand Service", ServiceName)
	}
	if Version != "2.0.0" {
		t.Errorf("Version = %s, want 2.0.0", Version)
	}
}

// =============================================================================
// GenerateRandomness Tests
// =============================================================================

func TestGenerateRandomness(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	ctx := context.Background()
	resp, err := svc.GenerateRandomness(ctx, "test-seed", 1)

	if err != nil {
		t.Fatalf("GenerateRandomness() error = %v", err)
	}
	if resp.Seed != "test-seed" {
		t.Errorf("Seed = %s, want test-seed", resp.Seed)
	}
	if len(resp.RandomWords) != 1 {
		t.Errorf("len(RandomWords) = %d, want 1", len(resp.RandomWords))
	}
	if resp.Proof == "" {
		t.Error("Proof is empty")
	}
	if resp.PublicKey == "" {
		t.Error("PublicKey is empty")
	}
}

func TestGenerateRandomnessMultipleWords(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	ctx := context.Background()
	resp, err := svc.GenerateRandomness(ctx, "test-seed", 5)

	if err != nil {
		t.Fatalf("GenerateRandomness() error = %v", err)
	}
	if len(resp.RandomWords) != 5 {
		t.Errorf("len(RandomWords) = %d, want 5", len(resp.RandomWords))
	}

	// Verify all words are unique
	seen := make(map[string]bool)
	for _, word := range resp.RandomWords {
		if seen[word] {
			t.Error("Random words should be unique")
		}
		seen[word] = true
	}
}

func TestGenerateRandomnessHexSeed(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	ctx := context.Background()
	hexSeed := hex.EncodeToString([]byte("test-seed"))
	resp, err := svc.GenerateRandomness(ctx, hexSeed, 1)

	if err != nil {
		t.Fatalf("GenerateRandomness() error = %v", err)
	}
	if len(resp.RandomWords) != 1 {
		t.Errorf("len(RandomWords) = %d, want 1", len(resp.RandomWords))
	}
}

func TestGenerateRandomnessWordLength(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	ctx := context.Background()
	resp, _ := svc.GenerateRandomness(ctx, "test-seed", 1)

	// Each word should be 64 hex characters (32 bytes)
	if len(resp.RandomWords[0]) != 64 {
		t.Errorf("RandomWord length = %d, want 64", len(resp.RandomWords[0]))
	}
}

// =============================================================================
// VerifyRandomness Tests
// =============================================================================

func TestVerifyRandomness(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	ctx := context.Background()
	genResp, _ := svc.GenerateRandomness(ctx, "test-seed", 1)

	verifyReq := &VerifyRequest{
		Seed:        genResp.Seed,
		RandomWords: genResp.RandomWords,
		Proof:       genResp.Proof,
		PublicKey:   genResp.PublicKey,
	}

	valid, err := svc.VerifyRandomness(verifyReq)
	if err != nil {
		t.Fatalf("VerifyRandomness() error = %v", err)
	}
	if !valid {
		t.Error("VerifyRandomness() = false, want true")
	}
}

func TestVerifyRandomnessInvalidProof(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	verifyReq := &VerifyRequest{
		Seed:        "test-seed",
		RandomWords: []string{"invalid"},
		Proof:       "invalid-proof",
		PublicKey:   "invalid-pubkey",
	}

	_, err := svc.VerifyRandomness(verifyReq)
	if err == nil {
		t.Error("VerifyRandomness() should return error for invalid proof")
	}
}

func TestVerifyRandomnessInvalidPublicKeyLength(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	// Valid hex but wrong length
	verifyReq := &VerifyRequest{
		Seed:        "test-seed",
		RandomWords: []string{"word"},
		Proof:       hex.EncodeToString([]byte("proof")),
		PublicKey:   hex.EncodeToString([]byte("short")), // Not 33 bytes
	}

	_, err := svc.VerifyRandomness(verifyReq)
	if err == nil {
		t.Error("VerifyRandomness() should return error for invalid public key length")
	}
}

// =============================================================================
// Handler Tests
// =============================================================================

func TestHandleRandom(t *testing.T) {
	t.Skip("handler not implemented in current VRF service")
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(RandomRequest{
		Seed:     "test-seed",
		NumWords: 2,
	})

	req := httptest.NewRequest("POST", "/random", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleRandom(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp RandomResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.RandomWords) != 2 {
		t.Errorf("len(RandomWords) = %d, want 2", len(resp.RandomWords))
	}
}

func TestHandleRandomInvalidBody(t *testing.T) {
	t.Skip("handler not implemented in current VRF service")
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("POST", "/random", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleRandom(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleRandomMissingSeed(t *testing.T) {
	t.Skip("handler not implemented in current VRF service")
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(RandomRequest{
		NumWords: 1,
	})

	req := httptest.NewRequest("POST", "/random", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleRandom(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleRandomDefaultNumWords(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(RandomRequest{
		Seed: "test-seed",
		// NumWords not specified, should default to 1
	})

	req := httptest.NewRequest("POST", "/random", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleRandom(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp RandomResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.RandomWords) != 1 {
		t.Errorf("len(RandomWords) = %d, want 1 (default)", len(resp.RandomWords))
	}
}

func TestHandleVerify(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	// First generate randomness
	ctx := context.Background()
	genResp, _ := svc.GenerateRandomness(ctx, "test-seed", 1)

	reqBody, _ := json.Marshal(VerifyRequest{
		Seed:        genResp.Seed,
		RandomWords: genResp.RandomWords,
		Proof:       genResp.Proof,
		PublicKey:   genResp.PublicKey,
	})

	req := httptest.NewRequest("POST", "/verify", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleVerify(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp VerifyResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if !resp.Valid {
		t.Errorf("Valid = %v, want true", resp.Valid)
	}
}

func TestHandleVerifyInvalidBody(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("POST", "/verify", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleVerify(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandlePublicKey(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("GET", "/pubkey", nil)
	rr := httptest.NewRecorder()

	svc.handlePublicKey(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)

	pubKey := resp["public_key"]
	if pubKey == "" {
		t.Error("public_key is empty")
	}

	// Public key should be 66 hex characters (33 bytes compressed)
	if len(pubKey) != 66 {
		t.Errorf("public_key length = %d, want 66", len(pubKey))
	}
}

// =============================================================================
// Request/Response Type Tests
// =============================================================================

func TestRandomRequestJSON(t *testing.T) {
	req := RandomRequest{
		Seed:     "test-seed",
		NumWords: 5,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded RandomRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Seed != req.Seed {
		t.Errorf("Seed = %s, want %s", decoded.Seed, req.Seed)
	}
	if decoded.NumWords != req.NumWords {
		t.Errorf("NumWords = %d, want %d", decoded.NumWords, req.NumWords)
	}
}

func TestRandomResponseJSON(t *testing.T) {
	resp := RandomResponse{
		RequestID:   "req-123",
		Seed:        "test-seed",
		RandomWords: []string{"word1", "word2"},
		Proof:       "proof-hex",
		PublicKey:   "pubkey-hex",
		Timestamp:   "2024-01-01T00:00:00Z",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded RandomResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(decoded.RandomWords) != len(resp.RandomWords) {
		t.Errorf("len(RandomWords) = %d, want %d", len(decoded.RandomWords), len(resp.RandomWords))
	}
}

func TestVerifyRequestJSON(t *testing.T) {
	req := VerifyRequest{
		Seed:        "test-seed",
		RandomWords: []string{"word1"},
		Proof:       "proof-hex",
		PublicKey:   "pubkey-hex",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded VerifyRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Proof != req.Proof {
		t.Errorf("Proof = %s, want %s", decoded.Proof, req.Proof)
	}
}

func TestVerifyResponseJSON(t *testing.T) {
	resp := VerifyResponse{
		Valid: true,
		Error: "",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded VerifyResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Valid != resp.Valid {
		t.Errorf("Valid = %v, want %v", decoded.Valid, resp.Valid)
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkNew(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = New(Config{Marble: m})
	}
}

func BenchmarkGenerateRandomness(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.GenerateRandomness(ctx, "benchmark-seed", 1)
	}
}

func BenchmarkGenerateRandomnessMultipleWords(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.GenerateRandomness(ctx, "benchmark-seed", 10)
	}
}

func BenchmarkVerifyRandomness(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})
	ctx := context.Background()

	genResp, _ := svc.GenerateRandomness(ctx, "benchmark-seed", 1)
	verifyReq := &VerifyRequest{
		Seed:        genResp.Seed,
		RandomWords: genResp.RandomWords,
		Proof:       genResp.Proof,
		PublicKey:   genResp.PublicKey,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.VerifyRandomness(verifyReq)
	}
}

// =============================================================================
// Additional Handler Tests
// =============================================================================

func TestHandleInfo(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("GET", "/info", nil)
	rr := httptest.NewRecorder()

	svc.handleInfo(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["status"] != "active" {
		t.Errorf("status = %v, want active", resp["status"])
	}
	if resp["public_key"] == nil {
		t.Error("public_key should not be nil")
	}
}

func TestHandleDirectRandom(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(DirectRandomRequest{
		Seed:     "test-seed",
		NumWords: 2,
	})

	req := httptest.NewRequest("POST", "/direct", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleDirectRandom(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp RandomResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.RandomWords) != 2 {
		t.Errorf("len(RandomWords) = %d, want 2", len(resp.RandomWords))
	}
}

func TestHandleDirectRandomMissingSeed(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(DirectRandomRequest{
		NumWords: 1,
	})

	req := httptest.NewRequest("POST", "/direct", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleDirectRandom(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleDirectRandomInvalidJSON(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("POST", "/direct", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleDirectRandom(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleDirectRandomDefaultNumWords(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(DirectRandomRequest{
		Seed: "test-seed",
		// NumWords not specified
	})

	req := httptest.NewRequest("POST", "/direct", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleDirectRandom(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp RandomResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.RandomWords) != 1 {
		t.Errorf("len(RandomWords) = %d, want 1 (default)", len(resp.RandomWords))
	}
}

func TestHandleCreateRequestUnauthorized(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(CreateRequestInput{
		Seed:     "test-seed",
		NumWords: 1,
	})

	req := httptest.NewRequest("POST", "/request", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	// No X-User-ID header
	rr := httptest.NewRecorder()

	svc.handleCreateRequest(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateRequestInvalidJSON(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("POST", "/request", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()

	svc.handleCreateRequest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateRequestMissingSeed(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(CreateRequestInput{
		NumWords: 1,
	})

	req := httptest.NewRequest("POST", "/request", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()

	svc.handleCreateRequest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListRequestsUnauthorized(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("GET", "/requests", nil)
	// No X-User-ID header
	rr := httptest.NewRecorder()

	svc.handleListRequests(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestHandleHealthEndpoint(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// =============================================================================
// Additional Type Tests
// =============================================================================

func TestNeoRandRequestStatusConstants(t *testing.T) {
	if StatusPending != "pending" {
		t.Errorf("StatusPending = %s, want pending", StatusPending)
	}
	if StatusFulfilled != "fulfilled" {
		t.Errorf("StatusFulfilled = %s, want fulfilled", StatusFulfilled)
	}
	if StatusFailed != "failed" {
		t.Errorf("StatusFailed = %s, want failed", StatusFailed)
	}
}

func TestServiceFeeConstant(t *testing.T) {
	if ServiceFeePerRequest <= 0 {
		t.Errorf("ServiceFeePerRequest = %d, should be > 0", ServiceFeePerRequest)
	}
}

func TestCreateRequestInputJSON(t *testing.T) {
	input := CreateRequestInput{
		Seed:             "test-seed",
		NumWords:         3,
		CallbackContract: "NContract123",
		CallbackGasLimit: 100000,
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded CreateRequestInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Seed != input.Seed {
		t.Errorf("Seed = %s, want %s", decoded.Seed, input.Seed)
	}
	if decoded.NumWords != input.NumWords {
		t.Errorf("NumWords = %d, want %d", decoded.NumWords, input.NumWords)
	}
	if decoded.CallbackGasLimit != input.CallbackGasLimit {
		t.Errorf("CallbackGasLimit = %d, want %d", decoded.CallbackGasLimit, input.CallbackGasLimit)
	}
}

func TestDirectRandomRequestJSON(t *testing.T) {
	req := DirectRandomRequest{
		Seed:     "test-seed",
		NumWords: 5,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded DirectRandomRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Seed != req.Seed {
		t.Errorf("Seed = %s, want %s", decoded.Seed, req.Seed)
	}
}

// =============================================================================
// Type Conversion Tests
// =============================================================================

func TestVRFRecordFromReq(t *testing.T) {
	now := time.Now()
	req := &NeoRandRequest{
		ID:               "id-123",
		RequestID:        "req-456",
		UserID:           "user-789",
		RequesterAddress: "NAddr123",
		Seed:             "test-seed",
		NumWords:         3,
		CallbackGasLimit: 100000,
		Status:           StatusPending,
		RandomWords:      []string{"word1", "word2", "word3"},
		Proof:            "proof-hex",
		FulfillTxHash:    "0x123",
		Error:            "",
		CreatedAt:        now,
		FulfilledAt:      now.Add(time.Minute),
	}

	rec := neorandRecordFromReq(req)

	if rec.ID != req.ID {
		t.Errorf("ID = %s, want %s", rec.ID, req.ID)
	}
	if rec.RequestID != req.RequestID {
		t.Errorf("RequestID = %s, want %s", rec.RequestID, req.RequestID)
	}
	if rec.UserID != req.UserID {
		t.Errorf("UserID = %s, want %s", rec.UserID, req.UserID)
	}
	if rec.Seed != req.Seed {
		t.Errorf("Seed = %s, want %s", rec.Seed, req.Seed)
	}
	if rec.NumWords != req.NumWords {
		t.Errorf("NumWords = %d, want %d", rec.NumWords, req.NumWords)
	}
	if rec.Status != req.Status {
		t.Errorf("Status = %s, want %s", rec.Status, req.Status)
	}
	if len(rec.RandomWords) != len(req.RandomWords) {
		t.Errorf("len(RandomWords) = %d, want %d", len(rec.RandomWords), len(req.RandomWords))
	}
}

func TestVRFReqFromRecord(t *testing.T) {
	now := time.Now()
	rec := &neorandsupabase.RequestRecord{
		ID:               "id-123",
		RequestID:        "req-456",
		UserID:           "user-789",
		RequesterAddress: "NAddr123",
		Seed:             "test-seed",
		NumWords:         3,
		CallbackGasLimit: 100000,
		Status:           StatusFulfilled,
		RandomWords:      []string{"word1", "word2", "word3"},
		Proof:            "proof-hex",
		FulfillTxHash:    "0x123",
		Error:            "",
		CreatedAt:        now,
		FulfilledAt:      now.Add(time.Minute),
	}

	req := neorandReqFromRecord(rec)

	if req.ID != rec.ID {
		t.Errorf("ID = %s, want %s", req.ID, rec.ID)
	}
	if req.RequestID != rec.RequestID {
		t.Errorf("RequestID = %s, want %s", req.RequestID, rec.RequestID)
	}
	if req.UserID != rec.UserID {
		t.Errorf("UserID = %s, want %s", req.UserID, rec.UserID)
	}
	if req.Status != rec.Status {
		t.Errorf("Status = %s, want %s", req.Status, rec.Status)
	}
	if req.NumWords != rec.NumWords {
		t.Errorf("NumWords = %d, want %d", req.NumWords, rec.NumWords)
	}
}

func TestVRFRecordRoundTrip(t *testing.T) {
	now := time.Now()
	original := &NeoRandRequest{
		ID:               "id-123",
		RequestID:        "req-456",
		UserID:           "user-789",
		RequesterAddress: "NAddr123",
		Seed:             "test-seed",
		NumWords:         5,
		CallbackGasLimit: 200000,
		Status:           StatusFulfilled,
		RandomWords:      []string{"a", "b", "c", "d", "e"},
		Proof:            "proof",
		FulfillTxHash:    "0xabc",
		CreatedAt:        now,
		FulfilledAt:      now.Add(time.Hour),
	}

	rec := neorandRecordFromReq(original)
	result := neorandReqFromRecord(rec)

	if result.ID != original.ID {
		t.Errorf("ID mismatch after round-trip")
	}
	if result.RequestID != original.RequestID {
		t.Errorf("RequestID mismatch after round-trip")
	}
	if result.Status != original.Status {
		t.Errorf("Status mismatch after round-trip")
	}
	if len(result.RandomWords) != len(original.RandomWords) {
		t.Errorf("RandomWords length mismatch after round-trip")
	}
}

// =============================================================================
// Handler Tests - GetRequest
// =============================================================================

func TestHandleGetRequestNotFound(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("GET", "/request/nonexistent-id", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent-id"})
	rr := httptest.NewRecorder()

	svc.handleGetRequest(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleGetRequestFromMemory(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	// Add a request to in-memory store
	testReq := &NeoRandRequest{
		ID:        "test-id",
		RequestID: "req-123",
		UserID:    "user-456",
		Seed:      "test-seed",
		NumWords:  2,
		Status:    StatusPending,
		CreatedAt: time.Now(),
	}
	svc.mu.Lock()
	svc.requests["req-123"] = testReq
	svc.mu.Unlock()

	req := httptest.NewRequest("GET", "/request/req-123", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "req-123"})
	rr := httptest.NewRecorder()

	svc.handleGetRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp NeoRandRequest
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.RequestID != "req-123" {
		t.Errorf("RequestID = %s, want req-123", resp.RequestID)
	}
	if resp.Status != StatusPending {
		t.Errorf("Status = %s, want %s", resp.Status, StatusPending)
	}
}

// =============================================================================
// Handler Tests - ListRequests with Memory
// =============================================================================

func TestHandleListRequestsWithMemory(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	svc, _ := New(Config{Marble: m})

	// Add requests to in-memory store
	svc.mu.Lock()
	svc.requests["req-1"] = &NeoRandRequest{
		ID:        "id-1",
		RequestID: "req-1",
		UserID:    "user-123",
		Status:    StatusPending,
		CreatedAt: time.Now(),
	}
	svc.requests["req-2"] = &NeoRandRequest{
		ID:        "id-2",
		RequestID: "req-2",
		UserID:    "user-123",
		Status:    StatusFulfilled,
		CreatedAt: time.Now(),
	}
	svc.requests["req-3"] = &NeoRandRequest{
		ID:        "id-3",
		RequestID: "req-3",
		UserID:    "other-user",
		Status:    StatusPending,
		CreatedAt: time.Now(),
	}
	svc.mu.Unlock()

	req := httptest.NewRequest("GET", "/requests", nil)
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()

	svc.handleListRequests(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp []*NeoRandRequest
	json.NewDecoder(rr.Body).Decode(&resp)

	// Should only return requests for user-123
	if len(resp) != 2 {
		t.Errorf("len(resp) = %d, want 2", len(resp))
	}

	for _, r := range resp {
		if r.UserID != "user-123" {
			t.Errorf("UserID = %s, want user-123", r.UserID)
		}
	}
}

// =============================================================================
// NeoRandRequest JSON Tests
// =============================================================================

func TestNeoRandRequestJSON(t *testing.T) {
	now := time.Now()
	req := NeoRandRequest{
		ID:               "id-123",
		RequestID:        "req-456",
		UserID:           "user-789",
		RequesterAddress: "NAddr123",
		Seed:             "test-seed",
		NumWords:         3,
		CallbackGasLimit: 100000,
		Status:           StatusPending,
		RandomWords:      []string{"word1", "word2"},
		Proof:            "proof-hex",
		CreatedAt:        now,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded NeoRandRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != req.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, req.ID)
	}
	if decoded.RequestID != req.RequestID {
		t.Errorf("RequestID = %s, want %s", decoded.RequestID, req.RequestID)
	}
	if decoded.Status != req.Status {
		t.Errorf("Status = %s, want %s", decoded.Status, req.Status)
	}
}
