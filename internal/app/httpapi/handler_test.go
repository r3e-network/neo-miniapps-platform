package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/app/domain/automation"
	domainccip "github.com/R3E-Network/service_layer/internal/app/domain/ccip"
	domainconf "github.com/R3E-Network/service_layer/internal/app/domain/confidential"
	domaincre "github.com/R3E-Network/service_layer/internal/app/domain/cre"
	domainsds "github.com/R3E-Network/service_layer/internal/app/domain/datastreams"
	domaindta "github.com/R3E-Network/service_layer/internal/app/domain/dta"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	domainvrf "github.com/R3E-Network/service_layer/internal/app/domain/vrf"
	"github.com/R3E-Network/service_layer/internal/app/jam"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

const testAuthToken = "test-token"

const (
	testWalletABC123      = "0xabc123abc123abc123abc123abc123abc123abcd"
	testWalletABC123Upper = "0xABC123ABC123ABC123ABC123ABC123ABC123ABCD"
	testWalletDeadBeef    = "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	testWalletFeed        = "0xfeedfeedfeedfeedfeedfeedfeedfeedfeedfeed"
	testWalletDead        = "0xdead000000000000000000000000000000000000"
	testWalletVRF         = "0x1111111111111111111111111111111111111111"
	testWalletLink        = "0x2222222222222222222222222222222222222222"
	testWalletLane        = "0x3333333333333333333333333333333333333333"
	testWalletDTA         = "0x4444444444444444444444444444444444444444"
	testWalletFace        = "0x5555555555555555555555555555555555555555"
)

var (
	authTokens = []string{testAuthToken}
	testLogger = logger.NewDefault("test")
)

func TestJAMEndpointsEnabled(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}

	audit := newAuditLog(50, nil)
	handler := wrapWithAuth(NewHandler(application, jam.Config{Enabled: true}, authTokens, nil, audit), authTokens, testLogger, nil)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Upload preimage
	content := []byte("hello-jam")
	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/jam/preimages/"+hash, bytes.NewReader(content))
	req.Header.Set("Authorization", "Bearer "+testAuthToken)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preimage put: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("preimage status %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Submit package
	body := marshal(map[string]any{
		"service_id": "svc-1",
		"items": []map[string]any{
			{"kind": "demo", "params_hash": "abc"},
		},
	})
	req, _ = http.NewRequest(http.MethodPost, server.URL+"/jam/packages", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testAuthToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("package post: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("package status %d", resp.StatusCode)
	}
	var pkg jam.WorkPackage
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		t.Fatalf("decode package: %v", err)
	}
	resp.Body.Close()

	// Process next
	req, _ = http.NewRequest(http.MethodPost, server.URL+"/jam/process", nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("process status %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Fetch report
	req, _ = http.NewRequest(http.MethodGet, server.URL+"/jam/packages/"+pkg.ID+"/report", nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get report: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("report status %d", resp.StatusCode)
	}
	var payload struct {
		Report       jam.WorkReport    `json:"report"`
		Attestations []jam.Attestation `json:"attestations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	resp.Body.Close()
	if payload.Report.PackageID != pkg.ID {
		t.Fatalf("report package mismatch")
	}
	if len(payload.Attestations) != 1 {
		t.Fatalf("expected 1 attestation, got %d", len(payload.Attestations))
	}
}

func TestHandlerLifecycle(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}

	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	audit := newAuditLog(50, nil)
	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, audit), authTokens, testLogger, nil)

	body := marshal(map[string]any{"owner": "alice"})
	req := authedRequest(http.MethodPost, "/accounts", body)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.Code)
	}

	var acct map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &acct); err != nil {
		t.Fatalf("unmarshal account: %v", err)
	}
	id := acct["ID"].(string)

	secretBody := marshal(map[string]any{"name": "apiKey", "value": "top-secret"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/secrets", secretBody))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create secret, got %d", resp.Code)
	}

	funcBody := marshal(map[string]any{
		"name":    "hello",
		"source":  "(params, secrets) => ({secret: secrets.apiKey})",
		"secrets": []string{"apiKey"},
	})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/functions", funcBody))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create function, got %d", resp.Code)
	}
	fnID := getFunctionID(resp.Body.Bytes())

	execBody := marshal(map[string]any{"input": "hello"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/functions/"+fnID+"/execute", execBody))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 execute, got %d", resp.Code)
	}
	var execResult map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &execResult); err != nil {
		t.Fatalf("unmarshal execution result: %v", err)
	}
	if execResult["status"] != "succeeded" {
		t.Fatalf("expected succeeded status, got %v", execResult["status"])
	}
	output, ok := execResult["output"].(map[string]any)
	if !ok || output["secret"] != "top-secret" {
		t.Fatalf("expected secret in execution result, got %v", execResult)
	}
	input, ok := execResult["input"].(map[string]any)
	if !ok || input["input"] != "hello" {
		t.Fatalf("expected input field recorded, got %v", execResult)
	}
	execID, ok := execResult["id"].(string)
	if !ok || execID == "" {
		t.Fatalf("expected execution id, got %v", execResult)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, fmt.Sprintf("/accounts/%s/functions/%s/executions", id, fnID), nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 list executions, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, fmt.Sprintf("/accounts/%s/functions/%s/executions/%s", id, fnID, execID), nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 get execution, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, fmt.Sprintf("/accounts/%s/functions/executions/%s", id, execID), nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 account execution lookup, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/metrics", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 metrics, got %d", resp.Code)
	}
	if resp.Body.Len() == 0 {
		t.Fatalf("expected metrics output to be non-empty")
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 health, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/system/version", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 version, got %d", resp.Code)
	}
	var versionPayload map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &versionPayload); err != nil {
		t.Fatalf("unmarshal version: %v", err)
	}
	if versionPayload["version"] == "" {
		t.Fatalf("expected version field")
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/system/status", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", resp.Code)
	}
	var statusPayload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &statusPayload); err != nil {
		t.Fatalf("unmarshal status payload: %v", err)
	}
	if statusPayload["status"] != "ok" {
		t.Fatalf("expected ok status, got %v", statusPayload["status"])
	}
	if jamStatus, ok := statusPayload["jam"].(map[string]any); ok {
		if jamStatus["enabled"] == nil {
			t.Fatalf("expected jam status")
		}
	}

	randomBody := marshal(map[string]any{"length": 16, "request_id": "req-http"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/random", randomBody))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 random, got %d", resp.Code)
	}
	var randomResp map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &randomResp); err != nil {
		t.Fatalf("unmarshal random: %v", err)
	}
	if randomResp["RequestID"] != "req-http" {
		t.Fatalf("expected request id echoed, got %v", randomResp["RequestID"])
	}

	listResp := httptest.NewRecorder()
	handler.ServeHTTP(listResp, authedRequest(http.MethodGet, "/accounts/"+id+"/random/requests?limit=5", nil))
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected 200 random list, got %d", listResp.Code)
	}
	var listPayload []map[string]any
	if err := json.Unmarshal(listResp.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("unmarshal random list: %v", err)
	}
	if len(listPayload) != 1 {
		t.Fatalf("expected single random record, got %d", len(listPayload))
	}
	if listPayload[0]["RequestID"] != "req-http" {
		t.Fatalf("expected list to include request, got %v", listPayload[0]["RequestID"])
	}

	trigBody := marshal(map[string]any{"function_id": fnID, "rule": "cron:@hourly"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/triggers", trigBody))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create trigger, got %d", resp.Code)
	}

	ensureBody := marshal(map[string]any{"wallet_address": "WALLET-1"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/gasbank", ensureBody))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 ensure gas account, got %d", resp.Code)
	}
	var gasAcct map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &gasAcct); err != nil {
		t.Fatalf("unmarshal gas account: %v", err)
	}
	gasID := gasAcct["ID"].(string)

	depositBody := marshal(map[string]any{"gas_account_id": gasID, "amount": 6.5, "tx_id": "tx1"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/gasbank/deposit", depositBody))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 deposit, got %d", resp.Code)
	}

	withdrawBody := marshal(map[string]any{"gas_account_id": gasID, "amount": 2.0, "to_address": "ADDR"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/gasbank/withdraw", withdrawBody))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 withdraw, got %d", resp.Code)
	}
	var withdrawPayload struct {
		Transaction map[string]any `json:"transaction"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &withdrawPayload); err != nil {
		t.Fatalf("unmarshal withdraw: %v", err)
	}
	txID := withdrawPayload.Transaction["ID"].(string)

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/gasbank/deposits?gas_account_id="+gasID, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 deposits list, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/gasbank/withdrawals?gas_account_id="+gasID, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 withdrawals list, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/gasbank/deadletters", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 deadletters list, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/gasbank/deadletters/nonexistent/retry", nil))
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 retry deadletter, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodDelete, "/accounts/"+id+"/gasbank/deadletters/nonexistent", nil))
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 delete deadletter, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/gasbank/transactions?gas_account_id="+gasID, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 transactions, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/gasbank/withdrawals/"+txID, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 get withdrawal, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/gasbank/withdrawals/"+txID+"/attempts", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 get attempts, got %d", resp.Code)
	}

	cancelBody := marshal(map[string]any{"action": "cancel", "reason": "test"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPatch, "/accounts/"+id+"/gasbank/withdrawals/"+txID, cancelBody))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 cancel withdrawal, got %d", resp.Code)
	}

	summaryResp := httptest.NewRecorder()
	handler.ServeHTTP(summaryResp, authedRequest(http.MethodGet, "/accounts/"+id+"/gasbank/summary", nil))
	if summaryResp.Code != http.StatusOK {
		t.Fatalf("expected 200 summary, got %d", summaryResp.Code)
	}
	var summary struct {
		PendingWithdrawals int `json:"pending_withdrawals"`
		Accounts           []map[string]any
	}
	if err := json.Unmarshal(summaryResp.Body.Bytes(), &summary); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	if len(summary.Accounts) != 1 {
		t.Fatalf("expected 1 account in summary, got %d", len(summary.Accounts))
	}

	jobBody := marshal(map[string]any{"function_id": fnID, "name": "daily", "schedule": "@daily"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/automation/jobs", jobBody))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create job, got %d", resp.Code)
	}
	var job map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &job); err != nil {
		t.Fatalf("unmarshal job: %v", err)
	}
	jobID := job["ID"].(string)

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/automation/jobs", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 list jobs, got %d", resp.Code)
	}

	disableJob := marshal(map[string]any{"enabled": false})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPatch, "/accounts/"+id+"/automation/jobs/"+jobID, disableJob))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 patch job, got %d", resp.Code)
	}

	devpackBody := marshal(map[string]any{
		"name":   "devpack",
		"source": "() => { Devpack.gasBank.ensureAccount({ wallet: 'wallet-2' }); return { ok: true }; }",
	})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/functions", devpackBody))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create devpack function, got %d", resp.Code)
	}
	devpackFnID := getFunctionID(resp.Body.Bytes())

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/functions/"+devpackFnID+"/execute", marshal(map[string]any{})))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 execute devpack function, got %d", resp.Code)
	}
	var devpackExec map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &devpackExec); err != nil {
		t.Fatalf("unmarshal devpack execution: %v", err)
	}
	actions, _ := devpackExec["actions"].([]any)
	if len(actions) != 1 {
		t.Fatalf("expected 1 devpack action, got %d", len(actions))
	}
	firstAction, _ := actions[0].(map[string]any)
	if firstAction["type"] != "gasbank.ensureAccount" || firstAction["status"] != "succeeded" {
		t.Fatalf("unexpected action payload: %#v", firstAction)
	}

	feedBody := marshal(map[string]any{
		"base_asset":         "NEO",
		"quote_asset":        "USD",
		"update_interval":    "@every 1m",
		"heartbeat_interval": "@every 1h",
		"deviation_percent":  0.5,
	})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/pricefeeds", feedBody))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create feed, got %d", resp.Code)
	}
	var feed map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &feed); err != nil {
		t.Fatalf("unmarshal feed: %v", err)
	}
	feedID := feed["ID"].(string)

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/pricefeeds/"+feedID, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 get feed, got %d", resp.Code)
	}

	snapshotBody := marshal(map[string]any{
		"price":        10.5,
		"source":       "oracle",
		"collected_at": time.Now().UTC().Format(time.RFC3339),
	})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/pricefeeds/"+feedID+"/snapshots", snapshotBody))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 snapshot, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/pricefeeds/"+feedID+"/snapshots", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 snapshots, got %d", resp.Code)
	}

	feedPatch := marshal(map[string]any{"active": false})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPatch, "/accounts/"+id+"/pricefeeds/"+feedID, feedPatch))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 patch feed, got %d", resp.Code)
	}

	sourceBody := marshal(map[string]any{
		"name":   "prices",
		"url":    "https://api.example.com",
		"method": "GET",
	})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/oracle/sources", sourceBody))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create source, got %d", resp.Code)
	}
	var source map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &source); err != nil {
		t.Fatalf("unmarshal source: %v", err)
	}
	sourceID := source["ID"].(string)

	requestBody := marshal(map[string]any{"data_source_id": sourceID, "payload": "{}"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/oracle/requests", requestBody))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create request, got %d", resp.Code)
	}
	var request map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &request); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	requestID := request["ID"].(string)

	running := marshal(map[string]any{"status": "running"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPatch, "/accounts/"+id+"/oracle/requests/"+requestID, running))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 mark running, got %d", resp.Code)
	}

	complete := marshal(map[string]any{"status": "succeeded", "result": `{"price":10}`})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPatch, "/accounts/"+id+"/oracle/requests/"+requestID, complete))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 complete request, got %d", resp.Code)
	}

	disableSource := marshal(map[string]any{"enabled": false})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPatch, "/accounts/"+id+"/oracle/sources/"+sourceID, disableSource))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 patch source, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/oracle/requests", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 list requests, got %d", resp.Code)
	}

	// create a failed request and exercise status filter + retry using a fresh source
	retrySourceBody := marshal(map[string]any{
		"name":   "retry-source",
		"url":    "https://api.example.com/retry",
		"method": "GET",
	})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/oracle/sources", retrySourceBody))
	assertStatus(t, resp, http.StatusCreated)
	var retrySource map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &retrySource)
	retrySourceID := retrySource["ID"].(string)
	failureBody := marshal(map[string]any{"data_source_id": retrySourceID, "payload": "{}"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/oracle/requests", failureBody))
	assertStatus(t, resp, http.StatusCreated)
	var failedReq map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &failedReq)
	failureID := failedReq["ID"].(string)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPatch, "/accounts/"+id+"/oracle/requests/"+failureID, marshal(map[string]any{"status": "failed", "error": "boom"})))
	assertStatus(t, resp, http.StatusOK)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/oracle/requests?status=failed", nil))
	assertStatus(t, resp, http.StatusOK)
	var filtered []map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &filtered)
	if len(filtered) != 1 || filtered[0]["ID"].(string) != failureID {
		t.Fatalf("expected filtered failed request list")
	}

	// pagination
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/oracle/requests?limit=1", nil))
	assertStatus(t, resp, http.StatusOK)
	var pageOne []map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &pageOne)
	nextCursor := resp.Header().Get("X-Next-Cursor")
	if len(pageOne) != 1 || nextCursor == "" {
		t.Fatalf("expected paginated response with next cursor")
	}
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/oracle/requests?limit=1&cursor="+nextCursor, nil))
	assertStatus(t, resp, http.StatusOK)
	var pageTwo []map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &pageTwo)
	if len(pageTwo) == 0 {
		t.Fatalf("expected second page of oracle requests")
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPatch, "/accounts/"+id+"/oracle/requests/"+failureID, marshal(map[string]any{"status": "retry"})))
	assertStatus(t, resp, http.StatusOK)

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/gasbank", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 list gas accounts, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodDelete, "/accounts/"+id, nil))
	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected 204 delete account, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id, nil))
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", resp.Code)
	}

	// Oracle runner auth + retry path
	application.OracleRunnerTokens = []string{"runner-secret"}
	handler = wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts", marshal(map[string]any{"owner": "runner-auth"})))
	assertStatus(t, resp, http.StatusCreated)
	runnerAccount := decodeResponse[map[string]any](t, resp)
	runnerAccountID := runnerAccount["ID"].(string)

	runnerSourceBody := marshal(map[string]any{"name": "prices", "url": "https://api.example.com", "method": "GET"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+runnerAccountID+"/oracle/sources", runnerSourceBody))
	assertStatus(t, resp, http.StatusCreated)
	runnerSource := decodeResponse[map[string]any](t, resp)
	runnerSourceID := runnerSource["ID"].(string)

	runnerReqBody := marshal(map[string]any{"data_source_id": runnerSourceID, "payload": "{}"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+runnerAccountID+"/oracle/requests", runnerReqBody))
	assertStatus(t, resp, http.StatusCreated)
	runnerRequest := decodeResponse[map[string]any](t, resp)
	runnerRequestID := runnerRequest["ID"].(string)

	runnerRunning := marshal(map[string]any{"status": "running"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPatch, "/accounts/"+runnerAccountID+"/oracle/requests/"+runnerRequestID, runnerRunning))
	assertStatus(t, resp, http.StatusUnauthorized)

	runnerReq := authedRequest(http.MethodPatch, "/accounts/"+runnerAccountID+"/oracle/requests/"+runnerRequestID, runnerRunning)
	runnerReq.Header.Set("X-Oracle-Runner-Token", "runner-secret")
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, runnerReq)
	assertStatus(t, resp, http.StatusOK)
	updated := decodeResponse[map[string]any](t, resp)
	if updated["Status"] != "running" {
		t.Fatalf("expected status running, got %v", updated["Status"])
	}

	failPayload := marshal(map[string]any{"status": "failed", "error": "boom"})
	failReq := authedRequest(http.MethodPatch, "/accounts/"+runnerAccountID+"/oracle/requests/"+runnerRequestID, failPayload)
	failReq.Header.Set("X-Oracle-Runner-Token", "runner-secret")
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, failReq)
	assertStatus(t, resp, http.StatusOK)

	retryPayload := marshal(map[string]any{"status": "retry"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPatch, "/accounts/"+runnerAccountID+"/oracle/requests/"+runnerRequestID, retryPayload))
	assertStatus(t, resp, http.StatusOK)
	retried := decodeResponse[map[string]any](t, resp)
	if retried["Status"] == "failed" {
		t.Fatalf("expected retried request not to be failed")
	}

	// Workspace wallet + DTA flow
	handler = wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts", marshal(map[string]any{"owner": "dta-owner"})))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 account for dta, got %d", resp.Code)
	}
	var dtaAcct map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &dtaAcct)
	dtaAcctID := dtaAcct["ID"].(string)

	walletPayload := marshal(map[string]any{"wallet_address": testWalletABC123, "label": "treasury", "status": "active"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+dtaAcctID+"/workspace-wallets", walletPayload))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 wallet, got %d", resp.Code)
	}

	productPayload := marshal(map[string]any{"name": "Fund", "symbol": "FND", "type": "open"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+dtaAcctID+"/dta/products", productPayload))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 dta product, got %d", resp.Code)
	}
	var product map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &product)
	productID := product["id"].(string)

	orderPayload := marshal(map[string]any{"type": "subscription", "amount": "100", "wallet_address": testWalletABC123Upper})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+dtaAcctID+"/dta/products/"+productID+"/orders", orderPayload))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 dta order, got %d", resp.Code)
	}

	// DataLink channel signer enforcement
	resp = httptest.NewRecorder()
	channelBody := marshal(map[string]any{
		"name":       "provider",
		"endpoint":   "https://api.example.com",
		"signer_set": []string{testWalletABC123},
	})
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+dtaAcctID+"/datalink/channels", channelBody))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 datalink channel, got %d", resp.Code)
	}

	// DataLink channel signer rejection for unknown wallet
	resp = httptest.NewRecorder()
	badChannel := marshal(map[string]any{
		"name":       "bad-provider",
		"endpoint":   "https://bad.example.com",
		"signer_set": []string{testWalletDeadBeef},
	})
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+dtaAcctID+"/datalink/channels", badChannel))
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected channel create to fail for unknown signer")
	}
}

func TestSystemDescriptors(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	handler := NewHandler(application, jam.Config{}, []string{}, nil, newAuditLog(50, nil))

	req := httptest.NewRequest(http.MethodGet, "/system/descriptors", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var descr []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &descr); err != nil {
		t.Fatalf("unmarshal descriptors: %v", err)
	}
	if len(descr) == 0 {
		t.Fatalf("expected descriptors payload")
	}
}

func TestWorkspaceWallets(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)

	acctBody := marshal(map[string]any{"owner": "wallet-owner"})
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts", acctBody))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create account, got %d", resp.Code)
	}
	var acct map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &acct)
	accountID := acct["ID"].(string)

	createWallet := marshal(map[string]any{
		"wallet_address": testWalletABC123,
		"label":          "treasury",
		"status":         "active",
	})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+accountID+"/workspace-wallets", createWallet))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create wallet, got %d", resp.Code)
	}
	var wallet map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &wallet)
	walletID := wallet["ID"].(string)

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+accountID+"/workspace-wallets", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 list wallets, got %d", resp.Code)
	}
	var wallets []map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &wallets)
	if len(wallets) != 1 {
		t.Fatalf("expected 1 wallet, got %d", len(wallets))
	}

	invalidWallet := marshal(map[string]any{
		"wallet_address": "invalid-wallet",
	})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+accountID+"/workspace-wallets", invalidWallet))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid wallet, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+accountID+"/workspace-wallets/"+walletID, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 get wallet, got %d", resp.Code)
	}
}

func TestCREPlaybooksAndRunsHTTP(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)
	accountID := createAccount(t, handler, "cre-owner")

	execResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/cre/executors", map[string]any{
		"name":     "runner-1",
		"type":     "http",
		"endpoint": "https://runner.example.com",
	})
	assertStatus(t, execResp, http.StatusCreated)
	exec := decodeResponse[domaincre.Executor](t, execResp)

	pbResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/cre/playbooks", map[string]any{
		"name":        "demo-playbook",
		"description": "test playbook",
		"steps": []map[string]any{
			{"name": "call-fn", "type": "function_call", "config": map[string]any{"function_id": "fn-1"}},
		},
	})
	assertStatus(t, pbResp, http.StatusCreated)
	playbook := decodeResponse[domaincre.Playbook](t, pbResp)

	listResp := doJSON(handler, http.MethodGet, "/accounts/"+accountID+"/cre/playbooks", nil)
	assertStatus(t, listResp, http.StatusOK)

	runResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/cre/runs", map[string]any{
		"playbook_id": playbook.ID,
		"executor_id": exec.ID,
		"params":      map[string]any{"foo": "bar"},
	})
	assertStatus(t, runResp, http.StatusCreated)
	run := decodeResponse[domaincre.Run](t, runResp)
	if run.AccountID != accountID || run.ExecutorID != exec.ID {
		t.Fatalf("run not scoped to account/executor, got account %s executor %s", run.AccountID, run.ExecutorID)
	}

	fetchRunResp := doJSON(handler, http.MethodGet, "/accounts/"+accountID+"/cre/runs/"+run.ID, nil)
	assertStatus(t, fetchRunResp, http.StatusOK)

	otherAccount := createAccount(t, handler, "other-tenant")
	badResp := doJSON(handler, http.MethodPost, "/accounts/"+otherAccount+"/cre/runs", map[string]any{
		"playbook_id": playbook.ID,
		"executor_id": exec.ID,
	})
	if badResp.Code == http.StatusCreated {
		t.Fatalf("expected cross-tenant run creation to fail")
	}
}

func TestDataFeedsWalletGatedHTTP(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)
	accountID := createAccount(t, handler, "datafeeds-owner")

	// register wallet for signer_set
	assertStatus(t, doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/workspace-wallets", map[string]any{
		"wallet_address": testWalletFeed,
		"label":          "signer",
		"status":         "active",
	}), http.StatusCreated)

	feedResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/datafeeds", map[string]any{
		"pair":              "eth/usd",
		"description":       "price feed",
		"decimals":          8,
		"heartbeat_seconds": 60,
		"threshold_ppm":     500,
		"signer_set":        []string{testWalletFeed},
		"aggregation":       "mean",
		"metadata":          map[string]any{"env": "test"},
	})
	assertStatus(t, feedResp, http.StatusCreated)
	var feed map[string]any
	_ = json.Unmarshal(feedResp.Body.Bytes(), &feed)
	feedID := feed["id"].(string)
	if agg := feed["aggregation"]; agg != "mean" {
		t.Fatalf("expected aggregation to round-trip, got %v", agg)
	}

	updateResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/datafeeds/"+feedID+"/updates", map[string]any{
		"round_id":  1,
		"price":     "123.45",
		"timestamp": time.Now().UTC(),
		"signer":    testWalletFeed,
		"signature": "sig",
	})
	assertStatus(t, updateResp, http.StatusCreated)

	latestResp := doJSON(handler, http.MethodGet, "/accounts/"+accountID+"/datafeeds/"+feedID+"/latest", nil)
	assertStatus(t, latestResp, http.StatusOK)

	// unknown signer should be rejected
	badResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/datafeeds", map[string]any{
		"pair":              "btc/usd",
		"decimals":          8,
		"heartbeat_seconds": 60,
		"threshold_ppm":     500,
		"signer_set":        []string{testWalletDead},
	})
	if badResp.Code == http.StatusCreated {
		t.Fatalf("expected feed creation to fail for unknown signer set")
	}
}

func TestVRFWalletGatedHTTP(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)
	accountID := createAccount(t, handler, "vrf-owner")

	// register wallet required for keys
	assertStatus(t, doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/workspace-wallets", map[string]any{
		"wallet_address": testWalletVRF,
		"label":          "vrf-signer",
		"status":         "active",
	}), http.StatusCreated)

	keyResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/vrf/keys", map[string]any{
		"public_key":     "pub-1",
		"label":          "key-1",
		"status":         "active",
		"wallet_address": testWalletVRF,
	})
	assertStatus(t, keyResp, http.StatusCreated)
	key := decodeResponse[domainvrf.Key](t, keyResp)

	reqResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/vrf/keys/"+key.ID+"/requests", map[string]any{
		"consumer": "consumer-1",
		"seed":     "seed-1",
	})
	assertStatus(t, reqResp, http.StatusCreated)
	_ = decodeResponse[domainvrf.Request](t, reqResp)

	listResp := doJSON(handler, http.MethodGet, "/accounts/"+accountID+"/vrf/requests?limit=10", nil)
	assertStatus(t, listResp, http.StatusOK)

	// creating key with unknown wallet should fail
	badKeyResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/vrf/keys", map[string]any{
		"public_key":     "pub-2",
		"label":          "bad",
		"status":         "active",
		"wallet_address": testWalletDead,
	})
	if badKeyResp.Code == http.StatusCreated {
		t.Fatalf("expected vrf key creation to fail for unknown wallet")
	}
}

func TestDataLinkChannelAndDeliveryHTTP(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)
	accountID := createAccount(t, handler, "datalink-owner")

	assertStatus(t, doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/workspace-wallets", map[string]any{
		"wallet_address": testWalletLink,
		"label":          "signer",
		"status":         "active",
	}), http.StatusCreated)

	channelResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/datalink/channels", map[string]any{
		"name":       "provider-1",
		"endpoint":   "https://api.provider.test",
		"signer_set": []string{testWalletLink},
	})
	assertStatus(t, channelResp, http.StatusCreated)
	var channel map[string]any
	_ = json.Unmarshal(channelResp.Body.Bytes(), &channel)
	channelID := channel["id"].(string)

	// Missing signer set should be rejected
	missingSigner := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/datalink/channels", map[string]any{
		"name":     "provider-2",
		"endpoint": "https://api.provider2.test",
	})
	assertStatus(t, missingSigner, http.StatusBadRequest)

	deliveryResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/datalink/channels/"+channelID+"/deliveries", map[string]any{
		"payload": map[string]any{"data": "hello"},
	})
	assertStatus(t, deliveryResp, http.StatusCreated)

	listResp := doJSON(handler, http.MethodGet, "/accounts/"+accountID+"/datalink/deliveries?limit=5", nil)
	assertStatus(t, listResp, http.StatusOK)

	otherAccount := createAccount(t, handler, "other-dl")
	foreignResp := doJSON(handler, http.MethodPost, "/accounts/"+otherAccount+"/datalink/channels/"+channelID+"/deliveries", map[string]any{
		"payload": map[string]any{"data": "bad"},
	})
	if foreignResp.Code == http.StatusCreated {
		t.Fatalf("expected delivery creation to fail for foreign account")
	}
}

func TestCCIPLaneAndMessageHTTP(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)
	accountID := createAccount(t, handler, "ccip-owner")

	assertStatus(t, doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/workspace-wallets", map[string]any{
		"wallet_address": testWalletLane,
		"label":          "signer",
		"status":         "active",
	}), http.StatusCreated)

	laneResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/ccip/lanes", map[string]any{
		"name":           "lane-1",
		"source_chain":   "eth",
		"dest_chain":     "neo",
		"signer_set":     []string{testWalletLane},
		"allowed_tokens": []string{"eth"},
	})
	assertStatus(t, laneResp, http.StatusCreated)
	lane := decodeResponse[domainccip.Lane](t, laneResp)

	msgResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/ccip/lanes/"+lane.ID+"/messages", map[string]any{
		"payload": map[string]any{"hello": "world"},
		"token_transfers": []map[string]any{
			{"token": "eth", "amount": "1", "recipient": "addr"},
		},
	})
	assertStatus(t, msgResp, http.StatusCreated)
	message := decodeResponse[domainccip.Message](t, msgResp)
	if message.AccountID != accountID || message.LaneID != lane.ID {
		t.Fatalf("message not scoped properly to account/lane")
	}

	listResp := doJSON(handler, http.MethodGet, "/accounts/"+accountID+"/ccip/messages?limit=5", nil)
	assertStatus(t, listResp, http.StatusOK)

	other := createAccount(t, handler, "other-ccip")
	badResp := doJSON(handler, http.MethodPost, "/accounts/"+other+"/ccip/lanes/"+lane.ID+"/messages", map[string]any{
		"payload": map[string]any{"bad": true},
	})
	if badResp.Code == http.StatusCreated {
		t.Fatalf("expected foreign account to be rejected for CCIP message")
	}
}

func TestDataStreamsHTTP(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)
	accountID := createAccount(t, handler, "streams-owner")

	streamResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/datastreams", map[string]any{
		"name":        "ticker",
		"symbol":      "TCKR",
		"description": "demo stream",
		"frequency":   "1s",
		"sla_ms":      50,
		"status":      "active",
	})
	assertStatus(t, streamResp, http.StatusCreated)
	stream := decodeResponse[domainsds.Stream](t, streamResp)

	frameResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/datastreams/"+stream.ID+"/frames", map[string]any{
		"sequence":   1,
		"payload":    map[string]any{"price": 100},
		"latency_ms": 10,
		"status":     "delivered",
	})
	assertStatus(t, frameResp, http.StatusCreated)

	listResp := doJSON(handler, http.MethodGet, "/accounts/"+accountID+"/datastreams/"+stream.ID+"/frames?limit=5", nil)
	assertStatus(t, listResp, http.StatusOK)
}

func TestConfidentialComputeHTTP(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)
	accountID := createAccount(t, handler, "conf-owner")

	enclaveResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/confcompute/enclaves", map[string]any{
		"name":        "enclave-1",
		"provider":    "azure",
		"measurement": "MRSIGNER",
		"endpoint":    "https://enclave.example.com",
		"status":      "active",
	})
	assertStatus(t, enclaveResp, http.StatusCreated)
	enclave := decodeResponse[domainconf.Enclave](t, enclaveResp)

	sealedResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/confcompute/sealed_keys", map[string]any{
		"enclave_id": enclave.ID,
		"name":       "key-1",
		"blob":       "c2VhbGVk",
	})
	assertStatus(t, sealedResp, http.StatusCreated)

	attResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/confcompute/attestations", map[string]any{
		"enclave_id": enclave.ID,
		"report":     "attestation-report",
		"metadata":   map[string]any{"env": "test"},
	})
	assertStatus(t, attResp, http.StatusCreated)

	listResp := doJSON(handler, http.MethodGet, "/accounts/"+accountID+"/confcompute/attestations", nil)
	assertStatus(t, listResp, http.StatusOK)
}

func TestDTAWalletGatedHTTP(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)
	accountID := createAccount(t, handler, "dta-owner-http")

	assertStatus(t, doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/workspace-wallets", map[string]any{
		"wallet_address": testWalletDTA,
		"label":          "investor",
		"status":         "active",
	}), http.StatusCreated)

	productResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/dta/products", map[string]any{
		"name":             "Fund A",
		"symbol":           "FNDA",
		"type":             "open",
		"settlement_terms": "T+1",
	})
	assertStatus(t, productResp, http.StatusCreated)
	product := decodeResponse[domaindta.Product](t, productResp)

	orderResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/dta/products/"+product.ID+"/orders", map[string]any{
		"type":           "subscription",
		"amount":         "1000",
		"wallet_address": testWalletDTA,
	})
	assertStatus(t, orderResp, http.StatusCreated)

	badOrder := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/dta/products/"+product.ID+"/orders", map[string]any{
		"type":           "subscription",
		"amount":         "1000",
		"wallet_address": testWalletDead,
	})
	if badOrder.Code == http.StatusCreated {
		t.Fatalf("expected DTA order to fail for unknown wallet")
	}
}

func TestCCIPLanesRequireRegisteredSigner(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)

	// Create account
	acctBody := marshal(map[string]any{"owner": "lane-owner"})
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts", acctBody))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 account, got %d", resp.Code)
	}
	var acct map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &acct)
	accountID := acct["ID"].(string)

	// Register wallet
	walletPayload := marshal(map[string]any{"wallet_address": testWalletFace, "label": "signer", "status": "active"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+accountID+"/workspace-wallets", walletPayload))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 wallet, got %d", resp.Code)
	}

	// Create lane with registered signer
	lanePayload := marshal(map[string]any{
		"name":         "lane-1",
		"source_chain": "eth",
		"dest_chain":   "arb",
		"signer_set":   []string{testWalletFace},
	})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+accountID+"/ccip/lanes", lanePayload))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 lane, got %d", resp.Code)
	}

	// Create lane with unknown signer should fail
	badLane := marshal(map[string]any{
		"name":         "lane-2",
		"source_chain": "eth",
		"dest_chain":   "arb",
		"signer_set":   []string{testWalletDead},
	})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+accountID+"/ccip/lanes", badLane))
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected failure for unknown signer_set")
	}
}

func TestHandlerAuthRequired(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)

	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestHandler_PreventCrossAccountExecution(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)

	createAccount := func(owner string) string {
		reqBody := marshal(map[string]any{"owner": owner})
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts", reqBody))
		if resp.Code != http.StatusCreated {
			t.Fatalf("create account %s: status %d", owner, resp.Code)
		}
		var acct map[string]any
		if err := json.Unmarshal(resp.Body.Bytes(), &acct); err != nil {
			t.Fatalf("unmarshal account %s: %v", owner, err)
		}
		id, _ := acct["ID"].(string)
		return id
	}

	accountA := createAccount("tenant-a")
	accountB := createAccount("tenant-b")

	fnBody := marshal(map[string]any{
		"name":   "hello",
		"source": "() => ({ ok: true })",
	})
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+accountA+"/functions", fnBody))
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create function, got %d", resp.Code)
	}
	functionID := getFunctionID(resp.Body.Bytes())

	execBody := marshal(map[string]any{"input": "data"})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+accountB+"/functions/"+functionID+"/execute", execBody))
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-account execution, got %d", resp.Code)
	}
}

func TestIntegration_AutomationExecutesFunction(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, jam.Config{}, authTokens, nil, newAuditLog(50, nil)), authTokens, testLogger, nil)

	accountID := createAccount(t, handler, "integration-owner")

	assertStatus(t, doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/secrets", map[string]any{
		"name":  "apiKey",
		"value": "super-secret",
	}), http.StatusCreated)

	functionSource := `(params, secrets) => {
		const job = params.automation_job || null;
		return { secret: secrets.apiKey, job };
	}`

	fnResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/functions", map[string]any{
		"name":        "uses-secret",
		"source":      functionSource,
		"secrets":     []string{"apiKey"},
		"description": "returns stored secret",
	})
	assertStatus(t, fnResp, http.StatusCreated)
	functionID := getFunctionID(fnResp.Body.Bytes())

	execResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/functions/"+functionID+"/execute", map[string]any{"input": "manual"})
	assertStatus(t, execResp, http.StatusOK)
	manualExec := decodeResponse[function.Execution](t, execResp)
	if output := manualExec.Output["secret"]; output != "super-secret" {
		t.Fatalf("expected function to read secret, got %v", output)
	}

	jobResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/automation/jobs", map[string]any{
		"function_id": functionID,
		"name":        "integration-job",
		"schedule":    "@every 1s",
		"description": "integration flow",
	})
	assertStatus(t, jobResp, http.StatusCreated)
	job := decodeResponse[automation.Job](t, jobResp)

	deadline := time.Now().Add(12 * time.Second)
	var automationRun bool
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)

		jobStatusResp := doJSON(handler, http.MethodGet, "/accounts/"+accountID+"/automation/jobs/"+job.ID, nil)
		if jobStatusResp.Code != http.StatusOK {
			continue
		}
		currentJob := decodeResponse[automation.Job](t, jobStatusResp)
		if currentJob.LastRun.IsZero() {
			continue
		}

		execsResp := doJSON(handler, http.MethodGet, fmt.Sprintf("/accounts/%s/functions/%s/executions?limit=2", accountID, functionID), nil)
		if execsResp.Code != http.StatusOK {
			continue
		}
		executions := decodeResponse[[]function.Execution](t, execsResp)
		if len(executions) == 0 {
			continue
		}
		latest := executions[0]
		if jobID, ok := latest.Input["automation_job"]; ok && jobID == job.ID {
			if secret := latest.Output["secret"]; secret != "super-secret" {
				t.Fatalf("automation execution missing secret, got %v", secret)
			}
			automationRun = true
			break
		}
	}

	if !automationRun {
		t.Fatalf("automation job %s did not execute within timeout", job.ID)
	}
}

func authedRequest(method, url string, body []byte) *http.Request {
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, url, reader)
	req.Header.Set("Authorization", "Bearer "+testAuthToken)
	return req
}

func marshal(v any) []byte {
	buf, _ := json.Marshal(v)
	return buf
}

func getFunctionID(body []byte) string {
	var def function.Definition
	_ = json.Unmarshal(body, &def)
	return def.ID
}

func createAccount(t *testing.T, handler http.Handler, owner string) string {
	resp := doJSON(handler, http.MethodPost, "/accounts", map[string]any{"owner": owner})
	assertStatus(t, resp, http.StatusCreated)
	var acct map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &acct); err != nil {
		t.Fatalf("unmarshal account: %v", err)
	}
	id, _ := acct["ID"].(string)
	return id
}

func doJSON(handler http.Handler, method, path string, payload any) *httptest.ResponseRecorder {
	var body []byte
	switch v := payload.(type) {
	case nil:
	case []byte:
		body = v
	default:
		body = marshal(v)
	}
	req := authedRequest(method, path, body)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func assertStatus(t *testing.T, resp *httptest.ResponseRecorder, expected int) {
	if resp.Code != expected {
		t.Fatalf("expected status %d, got %d (body: %s)", expected, resp.Code, resp.Body.String())
	}
}

func decodeResponse[T any](t *testing.T, resp *httptest.ResponseRecorder) T {
	var val T
	if err := json.Unmarshal(resp.Body.Bytes(), &val); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, resp.Body.String())
	}
	return val
}

func stopApplication(t *testing.T, application *app.Application) {
	t.Helper()
	if err := application.Stop(context.Background()); err != nil {
		t.Fatalf("stop application: %v", err)
	}
}
