package neovaultmarble

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/R3E-Network/service_layer/internal/testutil"
	neoaccounts "github.com/R3E-Network/service_layer/services/neoaccounts/marble"
	neovaultsupabase "github.com/R3E-Network/service_layer/services/neovault/supabase"
)

// =============================================================================
// Service Tests
// =============================================================================

func TestNew(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neovault"})

	svc, err := New(&Config{
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
	if ServiceID != "neovault" {
		t.Errorf("ServiceID = %s, want neovault", ServiceID)
	}
	if ServiceName != "NeoVault Service" {
		t.Errorf("ServiceName = %s, want NeoVault Service", ServiceName)
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
	m, _ := marble.New(marble.Config{MarbleType: "neovault"})
	svc, _ := New(&Config{Marble: m})

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
	m, _ := marble.New(marble.Config{MarbleType: "neovault"})
	svc, _ := New(&Config{Marble: m})

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
	m, _ := marble.New(marble.Config{MarbleType: "neovault"})
	svc, _ := New(&Config{Marble: m})

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
	m, _ := marble.New(marble.Config{MarbleType: "neovault"})
	svc, _ := New(&Config{Marble: m})

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

func TestNeoAccountsClientCreation(t *testing.T) {
	client := NewNeoAccountsClient("http://localhost:8090", "neovault")
	if client.baseURL != "http://localhost:8090" {
		t.Fatalf("baseURL mismatch: got %s want http://localhost:8090", client.baseURL)
	}
	if client.serviceID != "neovault" {
		t.Fatalf("serviceID mismatch: got %s want neovault", client.serviceID)
	}
}

// =============================================================================
// Token Configuration Tests
// =============================================================================

func TestGetTokenConfig(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neovault"})
	svc, _ := New(&Config{Marble: m})

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
	m, _ := marble.New(marble.Config{MarbleType: "neovault"})
	svc, _ := New(&Config{Marble: m})

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
	dbTargets := []neovaultsupabase.TargetAddress{
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
// NeoAccountsClient Tests with Mock Server
// =============================================================================

func TestNeoAccountsClientGetPoolInfo(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/info" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(neoaccounts.PoolInfoResponse{
			TotalAccounts:    10,
			ActiveAccounts:   8,
			LockedAccounts:   2,
			RetiringAccounts: 0,
			TokenStats: map[string]neoaccounts.TokenStats{
				"GAS": {TokenType: "GAS", TotalBalance: 1000000},
			},
		})
	}))
	defer mockServer.Close()

	client := NewNeoAccountsClient(mockServer.URL, "neovault")
	info, err := client.GetPoolInfo(context.Background())
	if err != nil {
		t.Fatalf("GetPoolInfo() error = %v", err)
	}
	if info.TotalAccounts != 10 {
		t.Errorf("TotalAccounts = %d, want 10", info.TotalAccounts)
	}
	if gasStats, ok := info.TokenStats["GAS"]; !ok || gasStats.TotalBalance != 1000000 {
		t.Errorf("TokenStats[GAS].TotalBalance = %v, want 1000000", info.TokenStats)
	}
}

func TestNeoAccountsClientRequestAccounts(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/request" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["service_id"] != "neovault" {
			t.Errorf("service_id = %v, want neovault", body["service_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(neoaccounts.RequestAccountsResponse{
			Accounts: []neoaccounts.AccountInfo{
				{ID: "acc-1", Address: "NAddr1", Balances: map[string]neoaccounts.TokenBalance{"GAS": {Amount: 1000}}},
				{ID: "acc-2", Address: "NAddr2", Balances: map[string]neoaccounts.TokenBalance{"GAS": {Amount: 2000}}},
			},
			LockID: "lock-123",
		})
	}))
	defer mockServer.Close()

	client := NewNeoAccountsClient(mockServer.URL, "neovault")
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

func TestNeoAccountsClientReleaseAccounts(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/release" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	client := NewNeoAccountsClient(mockServer.URL, "neovault")
	err := client.ReleaseAccounts(context.Background(), []string{"acc-1", "acc-2"})
	if err != nil {
		t.Fatalf("ReleaseAccounts() error = %v", err)
	}
}

func TestNeoAccountsClientUpdateBalance(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/balance" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	client := NewNeoAccountsClient(mockServer.URL, "neovault")
	err := client.UpdateBalance(context.Background(), "acc-1", 1000, nil)
	if err != nil {
		t.Fatalf("UpdateBalance() error = %v", err)
	}
}

func TestNeoAccountsClientWithHTTPClient(t *testing.T) {
	client := NewNeoAccountsClient("http://localhost:8090", "neovault")
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

func TestNeoAccountsClientErrorHandling(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer mockServer.Close()

	client := NewNeoAccountsClient(mockServer.URL, "neovault")

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
	m, _ := marble.New(marble.Config{MarbleType: "neovault"})
	svc, _ := New(&Config{Marble: m})

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
	m, _ := marble.New(marble.Config{MarbleType: "neovault"})
	svc, _ := New(&Config{Marble: m})

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

// =============================================================================
// Additional NeoAccountsClient Tests
// =============================================================================

func TestNeoAccountsClientGetLockedAccounts(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("service_id") != "neovault" {
			t.Errorf("service_id = %s, want neovault", r.URL.Query().Get("service_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(neoaccounts.ListAccountsResponse{
			Accounts: []neoaccounts.AccountInfo{
				{ID: "acc-1", Address: "NAddr1", Balances: map[string]neoaccounts.TokenBalance{"GAS": {Amount: 1000}}},
				{ID: "acc-2", Address: "NAddr2", Balances: map[string]neoaccounts.TokenBalance{"GAS": {Amount: 2000}}},
			},
		})
	}))
	defer mockServer.Close()

	client := NewNeoAccountsClient(mockServer.URL, "neovault")
	accounts, err := client.GetLockedAccounts(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetLockedAccounts() error = %v", err)
	}
	if len(accounts) != 2 {
		t.Errorf("len(accounts) = %d, want 2", len(accounts))
	}
}

func TestNeoAccountsClientGetLockedAccountsWithMinBalance(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("min_balance") != "1000" {
			t.Errorf("min_balance = %s, want 1000", r.URL.Query().Get("min_balance"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(neoaccounts.ListAccountsResponse{
			Accounts: []neoaccounts.AccountInfo{
				{ID: "acc-1", Address: "NAddr1", Balances: map[string]neoaccounts.TokenBalance{"GAS": {Amount: 2000}}},
			},
		})
	}))
	defer mockServer.Close()

	client := NewNeoAccountsClient(mockServer.URL, "neovault")
	minBalance := int64(1000)
	accounts, err := client.GetLockedAccounts(context.Background(), &minBalance)
	if err != nil {
		t.Fatalf("GetLockedAccounts() error = %v", err)
	}
	if len(accounts) != 1 {
		t.Errorf("len(accounts) = %d, want 1", len(accounts))
	}
}

func TestNeoAccountsClientSignTransaction(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sign" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SignTransactionResult{
			AccountID: "acc-1",
			Signature: "c2lnbmF0dXJl", // base64 "signature"
			PublicKey: "cHVia2V5",     // base64 "pubkey"
		})
	}))
	defer mockServer.Close()

	client := NewNeoAccountsClient(mockServer.URL, "neovault")
	result, err := client.SignTransaction(context.Background(), "acc-1", []byte("txhash"))
	if err != nil {
		t.Fatalf("SignTransaction() error = %v", err)
	}
	if result.AccountID != "acc-1" {
		t.Errorf("AccountID = %s, want acc-1", result.AccountID)
	}
	if result.Signature == "" {
		t.Error("Signature should not be empty")
	}
}

func TestNeoAccountsClientTransfer(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transfer" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var body TransferRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.ServiceID != "neovault" {
			t.Errorf("ServiceID = %s, want neovault", body.ServiceID)
		}
		if body.Amount != 1000 {
			t.Errorf("Amount = %d, want 1000", body.Amount)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TransferResult{
			TxHash:    "0x123abc",
			AccountID: "acc-1",
			Amount:    1000,
		})
	}))
	defer mockServer.Close()

	client := NewNeoAccountsClient(mockServer.URL, "neovault")
	result, err := client.Transfer(context.Background(), "acc-1", "NTargetAddr", 1000, "")
	if err != nil {
		t.Fatalf("Transfer() error = %v", err)
	}
	if result.TxHash != "0x123abc" {
		t.Errorf("TxHash = %s, want 0x123abc", result.TxHash)
	}
	if result.Amount != 1000 {
		t.Errorf("Amount = %d, want 1000", result.Amount)
	}
}

func TestNeoAccountsClientTransferError(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("insufficient balance"))
	}))
	defer mockServer.Close()

	client := NewNeoAccountsClient(mockServer.URL, "neovault")
	_, err := client.Transfer(context.Background(), "acc-1", "NTargetAddr", 1000, "")
	if err == nil {
		t.Error("Transfer() should return error on 400")
	}
}

func TestNeoAccountsClientSignTransactionError(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("account not locked by service"))
	}))
	defer mockServer.Close()

	client := NewNeoAccountsClient(mockServer.URL, "neovault")
	_, err := client.SignTransaction(context.Background(), "acc-1", []byte("txhash"))
	if err == nil {
		t.Error("SignTransaction() should return error on 403")
	}
}

func TestNeoAccountsClientUpdateBalanceWithAbsolute(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["absolute"] == nil {
			t.Error("absolute should be set")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	client := NewNeoAccountsClient(mockServer.URL, "neovault")
	absolute := int64(5000)
	err := client.UpdateBalance(context.Background(), "acc-1", 0, &absolute)
	if err != nil {
		t.Fatalf("UpdateBalance() error = %v", err)
	}
}

// =============================================================================
// Handler Tests (Basic validation only - handlers require full service setup)
// =============================================================================

func TestHandleCreateRequestUnauthorized(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neovault"})
	svc, _ := New(&Config{Marble: m})

	reqBody, _ := json.Marshal(CreateRequestInput{
		TokenType:   "GAS",
		UserAddress: "NAddr123",
		Targets:     []TargetAddress{{Address: "target1", Amount: 1000}},
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
	m, _ := marble.New(marble.Config{MarbleType: "neovault"})
	svc, _ := New(&Config{Marble: m})

	req := httptest.NewRequest("POST", "/request", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()

	svc.handleCreateRequest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateRequestMissingTargets(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neovault"})
	svc, _ := New(&Config{Marble: m})

	reqBody, _ := json.Marshal(CreateRequestInput{
		TokenType:   "GAS",
		UserAddress: "NAddr123",
		Targets:     []TargetAddress{}, // Empty targets
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
	m, _ := marble.New(marble.Config{MarbleType: "neovault"})
	svc, _ := New(&Config{Marble: m})

	req := httptest.NewRequest("GET", "/requests", nil)
	// No X-User-ID header
	rr := httptest.NewRecorder()

	svc.handleListRequests(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// Note: TestHandleDisputeUnauthorized removed - handler requires full service setup with repo

// =============================================================================
// Pool Account Conversion Tests
// =============================================================================

func TestPoolAccountConversion(t *testing.T) {
	now := time.Now()
	acc := neoaccounts.AccountInfo{
		ID:      "acc-1",
		Address: "NAddr123",
		Balances: map[string]neoaccounts.TokenBalance{
			"GAS": {TokenType: "GAS", Amount: 1000},
		},
		CreatedAt:  now,
		LastUsedAt: now,
		TxCount:    5,
		IsRetiring: false,
	}

	poolAcc := accountInfoToPoolAccount(&acc)

	if poolAcc.ID != "acc-1" {
		t.Errorf("ID = %s, want acc-1", poolAcc.ID)
	}
	if poolAcc.GetBalance("GAS") != 1000 {
		t.Errorf("GetBalance(GAS) = %d, want 1000", poolAcc.GetBalance("GAS"))
	}
}

// =============================================================================
// Additional Type Tests
// =============================================================================

func TestTokenConfigJSON(t *testing.T) {
	cfg := TokenConfig{
		TokenType:      "GAS",
		ServiceFeeRate: 0.005,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded TokenConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.TokenType != cfg.TokenType {
		t.Errorf("TokenType = %s, want %s", decoded.TokenType, cfg.TokenType)
	}
	if decoded.ServiceFeeRate != cfg.ServiceFeeRate {
		t.Errorf("ServiceFeeRate = %f, want %f", decoded.ServiceFeeRate, cfg.ServiceFeeRate)
	}
}

func TestMixOptionDurations(t *testing.T) {
	// Test common mixing duration options in milliseconds
	tests := []struct {
		name     string
		optionMs int64
		expected time.Duration
	}{
		{"30 minutes", 1800000, 30 * time.Minute},
		{"1 hour", 3600000, 1 * time.Hour},
		{"6 hours", 21600000, 6 * time.Hour},
		{"24 hours", 86400000, 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration := time.Duration(tt.optionMs) * time.Millisecond
			if duration != tt.expected {
				t.Errorf("MixOption %d = %v, want %v", tt.optionMs, duration, tt.expected)
			}
		})
	}
}

func TestSignTransactionRequestJSON(t *testing.T) {
	req := SignTransactionRequest{
		ServiceID: "neovault",
		AccountID: "acc-1",
		TxHash:    "dHhoYXNo", // base64 "txhash"
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded SignTransactionRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ServiceID != req.ServiceID {
		t.Errorf("ServiceID = %s, want %s", decoded.ServiceID, req.ServiceID)
	}
	if decoded.TxHash != req.TxHash {
		t.Errorf("TxHash = %s, want %s", decoded.TxHash, req.TxHash)
	}
}

func TestTransferRequestJSON(t *testing.T) {
	req := TransferRequest{
		ServiceID: "neovault",
		AccountID: "acc-1",
		ToAddress: "NTargetAddr",
		Amount:    1000,
		TokenHash: "0xd2a4cff31913016155e38e474a2c06d08be276cf",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded TransferRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Amount != req.Amount {
		t.Errorf("Amount = %d, want %d", decoded.Amount, req.Amount)
	}
	if decoded.TokenHash != req.TokenHash {
		t.Errorf("TokenHash = %s, want %s", decoded.TokenHash, req.TokenHash)
	}
}

// =============================================================================
// Additional Benchmarks
// =============================================================================

func BenchmarkNeoAccountsClientRequestAccounts(b *testing.B) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(neoaccounts.RequestAccountsResponse{
			Accounts: []neoaccounts.AccountInfo{{ID: "acc-1", Address: "NAddr1", Balances: map[string]neoaccounts.TokenBalance{"GAS": {Amount: 1000}}}},
			LockID:   "lock-123",
		})
	}))
	defer mockServer.Close()

	client := NewNeoAccountsClient(mockServer.URL, "neovault")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.RequestAccounts(context.Background(), 1, "test")
	}
}

func BenchmarkConvertTargetsToDB(b *testing.B) {
	targets := []TargetAddress{
		{Address: "addr1", Amount: 100},
		{Address: "addr2", Amount: 200},
		{Address: "addr3", Amount: 300},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = convertTargetsToDB(targets)
	}
}
