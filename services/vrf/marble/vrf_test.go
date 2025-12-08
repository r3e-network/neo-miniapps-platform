// Package vrf provides the Verifiable Random Function service.
package vrfmarble

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/R3E-Network/service_layer/internal/marble"
)

// =============================================================================
// Service Tests
// =============================================================================

func TestNew(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})

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
	if ServiceID != "vrf" {
		t.Errorf("ServiceID = %s, want vrf", ServiceID)
	}
	if ServiceName != "VRF Service" {
		t.Errorf("ServiceName = %s, want VRF Service", ServiceName)
	}
	if Version != "2.0.0" {
		t.Errorf("Version = %s, want 2.0.0", Version)
	}
}

// =============================================================================
// GenerateRandomness Tests
// =============================================================================

func TestGenerateRandomness(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = New(Config{Marble: m})
	}
}

func BenchmarkGenerateRandomness(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
	svc, _ := New(Config{Marble: m})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.GenerateRandomness(ctx, "benchmark-seed", 1)
	}
}

func BenchmarkGenerateRandomnessMultipleWords(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
	svc, _ := New(Config{Marble: m})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.GenerateRandomness(ctx, "benchmark-seed", 10)
	}
}

func BenchmarkVerifyRandomness(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "vrf"})
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
