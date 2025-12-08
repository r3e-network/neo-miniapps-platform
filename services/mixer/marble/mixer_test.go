package mixermarble

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/marble"
	accountpool "github.com/R3E-Network/service_layer/services/accountpool/marble"
	mixersupabase "github.com/R3E-Network/service_layer/services/mixer/supabase"
)

// =============================================================================
// Service Tests
// =============================================================================

func TestNew(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "mixer"})

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
	if ServiceID != "mixer" {
		t.Errorf("ServiceID = %s, want mixer", ServiceID)
	}
	if ServiceName != "Mixer Service" {
		t.Errorf("ServiceName = %s, want Mixer Service", ServiceName)
	}
	if Version != "3.4.0" {
		t.Errorf("Version = %s, want 3.4.0", Version)
	}
}

// =============================================================================
// Status Constants Tests
// =============================================================================

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		status MixRequestStatus
		want   string
	}{
		{StatusPending, "pending"},
		{StatusDeposited, "deposited"},
		{StatusMixing, "mixing"},
		{StatusDelivered, "delivered"},
		{StatusFailed, "failed"},
		{StatusRefunded, "refunded"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("Status %v = %s, want %s", tt.status, string(tt.status), tt.want)
		}
	}
}

// =============================================================================
// Random Split Tests
// =============================================================================

func TestRandomSplitSumsToTotal(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "mixer"})
	svc, _ := New(Config{Marble: m})

	total := int64(1_000_000)
	parts := svc.randomSplit(total, 5)
	if len(parts) != 5 {
		t.Fatalf("len(parts)=%d, want 5", len(parts))
	}
	var sum int64
	for i, p := range parts {
		if p <= 0 {
			t.Fatalf("part[%d]=%d, want >0", i, p)
		}
		sum += p
	}
	if sum != total {
		t.Fatalf("sum=%d, want %d", sum, total)
	}
}

func TestRandomSplitSinglePart(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "mixer"})
	svc, _ := New(Config{Marble: m})

	total := int64(1_000_000)
	parts := svc.randomSplit(total, 1)
	if len(parts) != 1 {
		t.Fatalf("len(parts)=%d, want 1", len(parts))
	}
	if parts[0] != total {
		t.Fatalf("parts[0]=%d, want %d", parts[0], total)
	}
}

func TestRandomSplitManyParts(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "mixer"})
	svc, _ := New(Config{Marble: m})

	total := int64(10_000_000)
	parts := svc.randomSplit(total, 10)
	if len(parts) != 10 {
		t.Fatalf("len(parts)=%d, want 10", len(parts))
	}

	var sum int64
	for _, p := range parts {
		sum += p
	}
	if sum != total {
		t.Fatalf("sum=%d, want %d", sum, total)
	}
}

func TestRandomSplitSmallAmount(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "mixer"})
	svc, _ := New(Config{Marble: m})

	total := int64(100)
	parts := svc.randomSplit(total, 3)
	if len(parts) != 3 {
		t.Fatalf("len(parts)=%d, want 3", len(parts))
	}

	var sum int64
	for _, p := range parts {
		sum += p
	}
	if sum != total {
		t.Fatalf("sum=%d, want %d", sum, total)
	}
}

func TestRequestRoundTrip(t *testing.T) {
	req := &MixRequest{
		ID:             "req-1",
		UserID:         "user-1",
		Status:         StatusMixing,
		TotalAmount:    1000,
		ServiceFee:     10,
		NetAmount:      990,
		InitialSplits:  3,
		MixingDuration: 30 * time.Minute,
		DepositAddress: "addr1",
		PoolAccounts:   []string{"acc1", "acc2"},
		CreatedAt:      time.Now(),
	}

	rec := RequestToRecord(req)
	out := RequestFromRecord(rec)

	if out.ID != req.ID || out.Status != req.Status || out.NetAmount != req.NetAmount {
		t.Fatalf("round-trip mismatch: got %+v want %+v", out, req)
	}
	if out.MixingDuration != req.MixingDuration {
		t.Fatalf("mixing duration mismatch: got %v want %v", out.MixingDuration, req.MixingDuration)
	}
}

func TestAccountPoolClientCreation(t *testing.T) {
	client := NewAccountPoolClient("http://localhost:8090", "mixer")
	if client.baseURL != "http://localhost:8090" {
		t.Fatalf("baseURL mismatch: got %s want http://localhost:8090", client.baseURL)
	}
	if client.serviceID != "mixer" {
		t.Fatalf("serviceID mismatch: got %s want mixer", client.serviceID)
	}
}

// =============================================================================
// Token Configuration Tests
// =============================================================================

func TestGetTokenConfig(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "mixer"})
	svc, _ := New(Config{Marble: m})

	// Test default token (GAS)
	cfg := svc.GetTokenConfig(DefaultToken)
	if cfg == nil {
		t.Fatal("GetTokenConfig(DefaultToken) returned nil")
	}
	if cfg.TokenType != "GAS" {
		t.Errorf("TokenType = %s, want GAS", cfg.TokenType)
	}
	if cfg.ServiceFeeRate <= 0 {
		t.Errorf("ServiceFeeRate = %f, want > 0", cfg.ServiceFeeRate)
	}
}

func TestGetSupportedTokens(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "mixer"})
	svc, _ := New(Config{Marble: m})

	tokens := svc.GetSupportedTokens()
	if len(tokens) == 0 {
		t.Error("GetSupportedTokens() returned empty list")
	}

	// Should contain at least GAS
	found := false
	for _, token := range tokens {
		if token == "GAS" {
			found = true
			break
		}
	}
	if !found {
		t.Error("GetSupportedTokens() should contain GAS")
	}
}

// =============================================================================
// Type Conversion Tests
// =============================================================================

func TestRequestToRecordWithCompletionProof(t *testing.T) {
	proof := &CompletionProof{
		RequestID:    "req-1",
		RequestHash:  "hash123",
		OutputsHash:  "outputs456",
		OutputTxIDs:  []string{"tx1", "tx2"},
		CompletedAt:  time.Now().Unix(),
		TEESignature: "sig789",
	}

	req := &MixRequest{
		ID:              "req-1",
		UserID:          "user-1",
		Status:          StatusDelivered,
		TotalAmount:     1000,
		ServiceFee:      10,
		NetAmount:       990,
		CompletionProof: proof,
		CreatedAt:       time.Now(),
	}

	rec := RequestToRecord(req)
	if rec.CompletionProofJSON == "" {
		t.Error("CompletionProofJSON should not be empty")
	}

	out := RequestFromRecord(rec)
	if out.CompletionProof == nil {
		t.Fatal("CompletionProof should not be nil after round-trip")
	}
	if out.CompletionProof.RequestID != proof.RequestID {
		t.Errorf("CompletionProof.RequestID = %s, want %s", out.CompletionProof.RequestID, proof.RequestID)
	}
}

func TestConvertTargetsFromDB(t *testing.T) {
	dbTargets := []mixersupabase.TargetAddress{
		{Address: "addr1", Amount: 100},
		{Address: "addr2", Amount: 200},
	}

	targets := convertTargetsFromDB(dbTargets)
	if len(targets) != 2 {
		t.Fatalf("len(targets) = %d, want 2", len(targets))
	}
	if targets[0].Address != "addr1" || targets[0].Amount != 100 {
		t.Errorf("targets[0] = %+v, want {addr1, 100}", targets[0])
	}
}

func TestConvertTargetsToDB(t *testing.T) {
	targets := []TargetAddress{
		{Address: "addr1", Amount: 100},
		{Address: "addr2", Amount: 200},
	}

	dbTargets := convertTargetsToDB(targets)
	if len(dbTargets) != 2 {
		t.Fatalf("len(dbTargets) = %d, want 2", len(dbTargets))
	}
	if dbTargets[0].Address != "addr1" || dbTargets[0].Amount != 100 {
		t.Errorf("dbTargets[0] = %+v, want {addr1, 100}", dbTargets[0])
	}
}

// =============================================================================
// AccountPoolClient Tests with Mock Server
// =============================================================================

func TestAccountPoolClientGetPoolInfo(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/info" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(accountpool.PoolInfoResponse{
			TotalAccounts:    10,
			ActiveAccounts:   8,
			LockedAccounts:   2,
			RetiringAccounts: 0,
			TotalBalance:     1000000,
		})
	}))
	defer mockServer.Close()

	client := NewAccountPoolClient(mockServer.URL, "mixer")
	info, err := client.GetPoolInfo(context.Background())
	if err != nil {
		t.Fatalf("GetPoolInfo() error = %v", err)
	}
	if info.TotalAccounts != 10 {
		t.Errorf("TotalAccounts = %d, want 10", info.TotalAccounts)
	}
	if info.TotalBalance != 1000000 {
		t.Errorf("TotalBalance = %d, want 1000000", info.TotalBalance)
	}
}

func TestAccountPoolClientRequestAccounts(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/request" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["service_id"] != "mixer" {
			t.Errorf("service_id = %v, want mixer", body["service_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(accountpool.RequestAccountsResponse{
			Accounts: []accountpool.AccountInfo{
				{ID: "acc-1", Address: "NAddr1", Balance: 1000},
				{ID: "acc-2", Address: "NAddr2", Balance: 2000},
			},
			LockID: "lock-123",
		})
	}))
	defer mockServer.Close()

	client := NewAccountPoolClient(mockServer.URL, "mixer")
	resp, err := client.RequestAccounts(context.Background(), 2, "test")
	if err != nil {
		t.Fatalf("RequestAccounts() error = %v", err)
	}
	if len(resp.Accounts) != 2 {
		t.Errorf("len(Accounts) = %d, want 2", len(resp.Accounts))
	}
	if resp.LockID != "lock-123" {
		t.Errorf("LockID = %s, want lock-123", resp.LockID)
	}
}

func TestAccountPoolClientReleaseAccounts(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/release" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	client := NewAccountPoolClient(mockServer.URL, "mixer")
	err := client.ReleaseAccounts(context.Background(), []string{"acc-1", "acc-2"})
	if err != nil {
		t.Fatalf("ReleaseAccounts() error = %v", err)
	}
}

func TestAccountPoolClientUpdateBalance(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/balance" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	client := NewAccountPoolClient(mockServer.URL, "mixer")
	err := client.UpdateBalance(context.Background(), "acc-1", 1000, nil)
	if err != nil {
		t.Fatalf("UpdateBalance() error = %v", err)
	}
}

func TestAccountPoolClientWithHTTPClient(t *testing.T) {
	client := NewAccountPoolClient("http://localhost:8090", "mixer")
	customClient := &http.Client{Timeout: 60 * time.Second}

	client = client.WithHTTPClient(customClient)
	if client.httpClient != customClient {
		t.Error("WithHTTPClient did not set custom client")
	}

	// Test with nil client (should not change)
	originalClient := client.httpClient
	client = client.WithHTTPClient(nil)
	if client.httpClient != originalClient {
		t.Error("WithHTTPClient(nil) should not change client")
	}
}

func TestAccountPoolClientErrorHandling(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer mockServer.Close()

	client := NewAccountPoolClient(mockServer.URL, "mixer")

	_, err := client.GetPoolInfo(context.Background())
	if err == nil {
		t.Error("GetPoolInfo() should return error on 500")
	}

	_, err = client.RequestAccounts(context.Background(), 1, "test")
	if err == nil {
		t.Error("RequestAccounts() should return error on 500")
	}

	err = client.ReleaseAccounts(context.Background(), []string{"acc-1"})
	if err == nil {
		t.Error("ReleaseAccounts() should return error on 500")
	}
}

// =============================================================================
// JSON Serialization Tests
// =============================================================================

func TestMixRequestJSON(t *testing.T) {
	req := MixRequest{
		ID:          "req-123",
		UserID:      "user-456",
		UserAddress: "NAddr123",
		TokenType:   "GAS",
		Status:      StatusPending,
		TotalAmount: 1000000,
		ServiceFee:  5000,
		NetAmount:   995000,
		TargetAddresses: []TargetAddress{
			{Address: "target1", Amount: 500000},
			{Address: "target2", Amount: 495000},
		},
		InitialSplits:  3,
		MixingDuration: 30 * time.Minute,
		DepositAddress: "deposit-addr",
		RequestHash:    "hash123",
		TEESignature:   "sig456",
		Deadline:       time.Now().Add(24 * time.Hour).Unix(),
		CreatedAt:      time.Now(),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded MixRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != req.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, req.ID)
	}
	if decoded.Status != req.Status {
		t.Errorf("Status = %s, want %s", decoded.Status, req.Status)
	}
	if len(decoded.TargetAddresses) != len(req.TargetAddresses) {
		t.Errorf("len(TargetAddresses) = %d, want %d", len(decoded.TargetAddresses), len(req.TargetAddresses))
	}
}

func TestCreateRequestInputJSON(t *testing.T) {
	input := CreateRequestInput{
		Version:     1,
		TokenType:   "GAS",
		UserAddress: "NAddr123",
		InputTxs:    []string{"tx1", "tx2"},
		Targets: []TargetAddress{
			{Address: "target1", Amount: 1000},
		},
		MixOption: 1800000, // 30 minutes in ms
		Timestamp: time.Now().Unix(),
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded CreateRequestInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Version != input.Version {
		t.Errorf("Version = %d, want %d", decoded.Version, input.Version)
	}
	if decoded.TokenType != input.TokenType {
		t.Errorf("TokenType = %s, want %s", decoded.TokenType, input.TokenType)
	}
}

func TestCompletionProofJSON(t *testing.T) {
	proof := CompletionProof{
		RequestID:    "req-123",
		RequestHash:  "hash456",
		OutputsHash:  "outputs789",
		OutputTxIDs:  []string{"tx1", "tx2", "tx3"},
		CompletedAt:  time.Now().Unix(),
		TEESignature: "sig-abc",
	}

	data, err := json.Marshal(proof)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded CompletionProof
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.RequestID != proof.RequestID {
		t.Errorf("RequestID = %s, want %s", decoded.RequestID, proof.RequestID)
	}
	if len(decoded.OutputTxIDs) != len(proof.OutputTxIDs) {
		t.Errorf("len(OutputTxIDs) = %d, want %d", len(decoded.OutputTxIDs), len(proof.OutputTxIDs))
	}
}

func TestDisputeResponseJSON(t *testing.T) {
	resp := DisputeResponse{
		RequestID: "req-123",
		Status:    "delivered",
		CompletionProof: &CompletionProof{
			RequestID: "req-123",
		},
		OnChainTxHash: "0x123abc",
		Message:       "Dispute resolved",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded DisputeResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.RequestID != resp.RequestID {
		t.Errorf("RequestID = %s, want %s", decoded.RequestID, resp.RequestID)
	}
	if decoded.OnChainTxHash != resp.OnChainTxHash {
		t.Errorf("OnChainTxHash = %s, want %s", decoded.OnChainTxHash, resp.OnChainTxHash)
	}
}

// =============================================================================
// Handler Tests
// =============================================================================

func TestHandleHealthEndpoint(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "mixer"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["status"] != "healthy" {
		t.Errorf("status = %v, want healthy", resp["status"])
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkRandomSplit(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "mixer"})
	svc, _ := New(Config{Marble: m})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.randomSplit(1_000_000, 5)
	}
}

func BenchmarkRequestToRecord(b *testing.B) {
	req := &MixRequest{
		ID:             "req-1",
		UserID:         "user-1",
		Status:         StatusMixing,
		TotalAmount:    1000,
		ServiceFee:     10,
		NetAmount:      990,
		InitialSplits:  3,
		MixingDuration: 30 * time.Minute,
		DepositAddress: "addr1",
		PoolAccounts:   []string{"acc1", "acc2"},
		TargetAddresses: []TargetAddress{
			{Address: "target1", Amount: 500},
			{Address: "target2", Amount: 490},
		},
		CreatedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RequestToRecord(req)
	}
}

func BenchmarkMixRequestMarshal(b *testing.B) {
	req := MixRequest{
		ID:          "req-123",
		UserID:      "user-456",
		Status:      StatusPending,
		TotalAmount: 1000000,
		ServiceFee:  5000,
		NetAmount:   995000,
		TargetAddresses: []TargetAddress{
			{Address: "target1", Amount: 500000},
			{Address: "target2", Amount: 495000},
		},
		CreatedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(req)
	}
}

// Suppress unused import warnings
var (
	_ = bytes.NewReader
	_ = context.Background
)
