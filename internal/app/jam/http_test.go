package jam

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

type autoRefiner struct{}

func (autoRefiner) Refine(_ context.Context, pkg WorkPackage, _ PreimageStore) (WorkReport, error) {
	return WorkReport{
		ID:               uuid.NewString(),
		PackageID:        pkg.ID,
		ServiceID:        pkg.ServiceID,
		RefineOutputHash: "hash",
	}, nil
}

type autoAttestor struct{}

func (autoAttestor) Attest(_ context.Context, _ WorkReport) (Attestation, error) {
	return Attestation{WorkerID: "attestor-1"}, nil
}

type countingAccumulator struct {
	count int
}

func (c *countingAccumulator) Accumulate(_ context.Context, _ WorkReport, _ []Message) error {
	c.count++
	return nil
}

func TestHTTPHandlerEndToEnd(t *testing.T) {
	store := NewInMemoryStore()
	store.SetAccumulatorsEnabled(true)
	preimages := NewMemPreimageStore()
	accum := &countingAccumulator{}
	engine := Engine{
		Preimages:   preimages,
		Refiner:     autoRefiner{},
		Attestors:   []Attestor{autoAttestor{}},
		Accumulator: accum,
		Threshold:   1,
	}
	coord := Coordinator{Store: store, Engine: engine, AccumulatorsEnabled: true}
	handler := NewHTTPHandler(store, preimages, coord, Config{Enabled: true, AuthRequired: false, AccumulatorsEnabled: true}, nil)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Upload preimage.
	content := []byte("demo")
	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/jam/preimages/"+hash, bytes.NewReader(content))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preimage put err: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("preimage status %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Submit a package.
	payload := map[string]any{
		"service_id": "svc-1",
		"items": []map[string]any{
			{"kind": "demo", "params_hash": "abc123"},
		},
	}
	body, _ := json.Marshal(payload)
	resp, err = http.Post(server.URL+"/jam/packages?include_receipt=true", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post package err: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("package status %d", resp.StatusCode)
	}
	var pkgResp struct {
		Package WorkPackage `json:"package"`
		Receipt Receipt     `json:"receipt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pkgResp); err != nil {
		t.Fatalf("decode package: %v", err)
	}
	resp.Body.Close()
	if pkgResp.Package.ID == "" || len(pkgResp.Package.Items) != 1 {
		t.Fatalf("bad package response: %+v", pkgResp)
	}
	if pkgResp.Receipt.Hash == "" {
		t.Fatalf("expected receipt in create response")
	}

	// Process next package.
	resp, err = http.Post(server.URL+"/jam/process", "application/json", nil)
	if err != nil {
		t.Fatalf("process err: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("process status %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Fetch package and report.
	resp, err = http.Get(server.URL + "/jam/packages/" + pkgResp.Package.ID + "?include_receipt=true")
	if err != nil {
		t.Fatalf("get package: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get package status %d", resp.StatusCode)
	}
	var pkgFetched struct {
		Package WorkPackage `json:"package"`
		Receipt Receipt     `json:"receipt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pkgFetched); err != nil {
		t.Fatalf("decode pkg fetched: %v", err)
	}
	resp.Body.Close()
	if pkgFetched.Package.Status != PackageStatusApplied {
		t.Fatalf("expected applied status, got %s", pkgFetched.Package.Status)
	}
	if pkgFetched.Receipt.Hash == "" {
		t.Fatalf("expected receipt in get response")
	}

	resp, err = http.Get(server.URL + "/jam/packages/" + pkgResp.Package.ID + "/report")
	if err != nil {
		t.Fatalf("get report: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("report status %d", resp.StatusCode)
	}
	var reportPayload struct {
		Report       WorkReport    `json:"report"`
		Attestations []Attestation `json:"attestations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&reportPayload); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	resp.Body.Close()
	if reportPayload.Report.PackageID != pkgResp.Package.ID {
		t.Fatalf("report package mismatch")
	}
	if len(reportPayload.Attestations) != 1 {
		t.Fatalf("expected 1 attestation")
	}
	if accum.count != 1 {
		t.Fatalf("accumulator not called")
	}
}

func TestHTTPHandler_AuthRateLimitAndQuotas(t *testing.T) {
	store := NewInMemoryStore()
	preimages := NewMemPreimageStore()
	accum := &countingAccumulator{}
	engine := Engine{
		Preimages:   preimages,
		Refiner:     autoRefiner{},
		Attestors:   []Attestor{autoAttestor{}},
		Accumulator: accum,
		Threshold:   1,
	}
	coord := Coordinator{Store: store, Engine: engine}
	cfg := Config{
		Enabled:            true,
		AuthRequired:       true,
		MaxPreimageBytes:   4,
		MaxPendingPackages: 1,
		RateLimitPerMinute: 10,
	}
	handler := NewHTTPHandler(store, preimages, coord, cfg, []string{"jam-token"})
	server := httptest.NewServer(handler)
	defer server.Close()

	// Missing auth -> 401
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/jam/preimages/hash", bytes.NewReader([]byte("data")))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request err: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	authReq := func(method, path string, body []byte) (*http.Response, error) {
		r, _ := http.NewRequest(method, server.URL+path, bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer jam-token")
		return http.DefaultClient.Do(r)
	}

	// Preimage too large -> 413
	resp, err = authReq(http.MethodPut, "/jam/preimages/hash", []byte("longdata"))
	if err != nil {
		t.Fatalf("put err: %v", err)
	}
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Rate limit: use a low limit and hit it.
	cfg.RateLimitPerMinute = 1
	handler = NewHTTPHandler(store, preimages, coord, cfg, []string{"jam-token"})
	server.Config.Handler = handler
	resp, err = authReq(http.MethodHead, "/jam/preimages/hash", nil)
	if err != nil {
		t.Fatalf("head err: %v", err)
	}
	resp.Body.Close()
	resp, err = authReq(http.MethodHead, "/jam/preimages/hash", nil)
	if err != nil {
		t.Fatalf("head err 2: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	resp.Body.Close()
	// reset rate limit for rest of test
	cfg.RateLimitPerMinute = 10
	handler = NewHTTPHandler(store, preimages, coord, cfg, []string{"jam-token"})
	server.Config.Handler = handler

	// Allow rate window to advance before package submits
	time.Sleep(time.Second)

	// Pending cap: allow first package, second should 409
	pkg := map[string]any{
		"service_id": "svc-1",
		"items": []map[string]any{
			{"kind": "demo", "params_hash": "abc"},
		},
	}
	body, _ := json.Marshal(pkg)
	resp, err = authReq(http.MethodPost, "/jam/packages", body)
	if err != nil {
		t.Fatalf("post pkg: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()
	time.Sleep(time.Second)

	resp, err = authReq(http.MethodPost, "/jam/packages", body)
	if err != nil {
		t.Fatalf("post pkg 2: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Unauthorized token should 403 when allowed list set
	badReq, _ := http.NewRequest(http.MethodGet, server.URL+"/jam/packages", nil)
	badReq.Header.Set("Authorization", "Bearer nope")
	resp, err = http.DefaultClient.Do(badReq)
	if err != nil {
		t.Fatalf("bad token err: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestJAMStatusReportsAccumulatorRoot(t *testing.T) {
	store := NewInMemoryStore()
	store.SetAccumulatorsEnabled(true)
	store.SetAccumulatorHash("blake3-256")
	now := time.Now().UTC()
	if _, err := store.AppendReceipt(context.Background(), ReceiptInput{
		Hash:        "hash-1",
		ServiceID:   "svc-accum",
		EntryType:   ReceiptTypeReport,
		Status:      string(PackageStatusApplied),
		ProcessedAt: now,
	}); err != nil {
		t.Fatalf("append receipt: %v", err)
	}
	preimages := NewMemPreimageStore()
	coord := Coordinator{Store: store, Engine: Engine{
		Preimages:   preimages,
		Refiner:     autoRefiner{},
		Attestors:   []Attestor{autoAttestor{}},
		Accumulator: &countingAccumulator{},
		Threshold:   1,
	}}
	cfg := Config{
		Enabled:             true,
		AccumulatorsEnabled: true,
		AccumulatorHash:     "blake3-256",
	}
	handler := NewHTTPHandler(store, preimages, coord, cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/jam/status?service_id=svc-accum", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if enabled, _ := payload["accumulators_enabled"].(bool); !enabled {
		t.Fatalf("expected accumulators_enabled true")
	}
	if alg, _ := payload["accumulator_hash"].(string); alg != "blake3-256" {
		t.Fatalf("unexpected accumulator_hash %q", alg)
	}
	rootPayload, ok := payload["accumulator_root"].(map[string]any)
	if !ok {
		t.Fatalf("expected accumulator_root")
	}
	if svc, _ := rootPayload["service_id"].(string); svc != "svc-accum" {
		t.Fatalf("service_id mismatch: %q", svc)
	}
	if seq, _ := rootPayload["seq"].(float64); seq < 1 {
		t.Fatalf("expected seq >= 1, got %v", seq)
	}
}
