package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	app "github.com/R3E-Network/service_layer/applications"
	"github.com/R3E-Network/service_layer/applications/auth"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.automation/service"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.ccip/service"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.confidential/service"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.cre/service"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.datastreams/service"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.dta/service"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.vrf/service"
	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/os/core"
)

const testAuthToken = "test-token"
const adminAuthToken = "admin-token"

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

// newTestApplication creates an application for testing.
// It skips the test if database is not available.
func newTestApplication(t *testing.T) *app.Application {
	t.Helper()
	t.Skipf("test requires database; run with integration test suite")
	return nil
}

type staticAdminValidator struct{}

func (staticAdminValidator) Validate(string) (*auth.Claims, error) {
	return &auth.Claims{Username: "admin", Role: "admin"}, nil
}

func TestSystemTenantEndpoint(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, nil), authTokens, testLogger, nil)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/system/tenant", nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken)
	req.Header.Set("X-Tenant-ID", "tenant-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("tenant endpoint: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["tenant"] != "tenant-123" {
		t.Fatalf("expected tenant echoed, got %v", payload["tenant"])
	}
	if reqHeader, ok := payload["require_tenant_header_on"].(bool); !ok {
		t.Fatalf("expected require_tenant_header_on boolean")
	} else if !reqHeader && requireTenantHeaderEnabled() {
		t.Fatalf("expected require_tenant_header_on to mirror env")
	}
}

func TestHandlerLifecycle(t *testing.T) {
	application := newTestApplication(t)

	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	audit := newAuditLog(50, nil)
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, nil), authTokens, testLogger, nil)

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
	handler = wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)
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
	handler = wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)
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
	application := newTestApplication(t)
	handler := NewHandler(application, []string{}, nil, newAuditLog(50, nil), nil, nil)

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
	application := newTestApplication(t)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)

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
	application := newTestApplication(t)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)
	accountID := createAccount(t, handler, "cre-owner")

	execResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/cre/executors", map[string]any{
		"name":     "runner-1",
		"type":     "http",
		"endpoint": "https://runner.example.com",
	})
	assertStatus(t, execResp, http.StatusCreated)
	exec := decodeResponse[cre.Executor](t, execResp)

	pbResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/cre/playbooks", map[string]any{
		"name":        "demo-playbook",
		"description": "test playbook",
		"steps": []map[string]any{
			{"name": "call-fn", "type": "function_call", "config": map[string]any{"function_id": "fn-1"}},
		},
	})
	assertStatus(t, pbResp, http.StatusCreated)
	playbook := decodeResponse[cre.Playbook](t, pbResp)

	listResp := doJSON(handler, http.MethodGet, "/accounts/"+accountID+"/cre/playbooks", nil)
	assertStatus(t, listResp, http.StatusOK)

	runResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/cre/runs", map[string]any{
		"playbook_id": playbook.ID,
		"executor_id": exec.ID,
		"params":      map[string]any{"foo": "bar"},
	})
	assertStatus(t, runResp, http.StatusCreated)
	run := decodeResponse[cre.Run](t, runResp)
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

func TestTenantIsolationEnforced(t *testing.T) {
	application := newTestApplication(t)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)

	accountA := createAccountWithTenant(t, handler, "alice", "tenant-a")
	accountB := createAccountWithTenant(t, handler, "bob", "tenant-b")

	// Cross-tenant account access should be forbidden
	assertStatus(t, doJSONWithTenant(handler, http.MethodGet, "/accounts/"+accountB, nil, "tenant-a"), http.StatusForbidden)
	assertStatus(t, doJSONWithTenant(handler, http.MethodGet, "/accounts/"+accountA, nil, "tenant-b"), http.StatusForbidden)

	// Create a function under tenant-a, then ensure tenant-b cannot access or create against that account.
	fnPayload := map[string]any{
		"name":   "hello",
		"source": "(p,s)=>({ok:true})",
	}
	assertStatus(t, doJSONWithTenant(handler, http.MethodPost, "/accounts/"+accountA+"/functions", fnPayload, "tenant-a"), http.StatusCreated)
	assertStatus(t, doJSONWithTenant(handler, http.MethodGet, "/accounts/"+accountA+"/functions", nil, "tenant-b"), http.StatusForbidden)
	assertStatus(t, doJSONWithTenant(handler, http.MethodPost, "/accounts/"+accountA+"/functions", fnPayload, "tenant-b"), http.StatusForbidden)

	// Likewise, tenant-a cannot list tenant-b resources.
	assertStatus(t, doJSONWithTenant(handler, http.MethodGet, "/accounts/"+accountB+"/functions", nil, "tenant-a"), http.StatusForbidden)
}

func TestDataFeedsWalletGatedHTTP(t *testing.T) {
	application := newTestApplication(t)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)
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
	application := newTestApplication(t)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)
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
	key := decodeResponse[vrf.Key](t, keyResp)

	reqResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/vrf/keys/"+key.ID+"/requests", map[string]any{
		"consumer": "consumer-1",
		"seed":     "seed-1",
	})
	assertStatus(t, reqResp, http.StatusCreated)
	_ = decodeResponse[vrf.Request](t, reqResp)

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
	application := newTestApplication(t)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)
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
	application := newTestApplication(t)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)
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
	lane := decodeResponse[ccip.Lane](t, laneResp)

	msgResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/ccip/lanes/"+lane.ID+"/messages", map[string]any{
		"payload": map[string]any{"hello": "world"},
		"token_transfers": []map[string]any{
			{"token": "eth", "amount": "1", "recipient": "addr"},
		},
	})
	assertStatus(t, msgResp, http.StatusCreated)
	message := decodeResponse[ccip.Message](t, msgResp)
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
	application := newTestApplication(t)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)
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
	stream := decodeResponse[datastreams.Stream](t, streamResp)

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
	application := newTestApplication(t)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)
	accountID := createAccount(t, handler, "conf-owner")

	enclaveResp := doJSON(handler, http.MethodPost, "/accounts/"+accountID+"/confcompute/enclaves", map[string]any{
		"name":        "enclave-1",
		"provider":    "azure",
		"measurement": "MRSIGNER",
		"endpoint":    "https://enclave.example.com",
		"status":      "active",
	})
	assertStatus(t, enclaveResp, http.StatusCreated)
	enclave := decodeResponse[confidential.Enclave](t, enclaveResp)

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
	application := newTestApplication(t)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)
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
	product := decodeResponse[dta.Product](t, productResp)

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
	application := newTestApplication(t)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)

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
	application := newTestApplication(t)
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)

	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestHandler_PreventCrossAccountExecution(t *testing.T) {
	application := newTestApplication(t)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)

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
	application := newTestApplication(t)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { stopApplication(t, application) })

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, newAuditLog(50, nil), nil, nil), authTokens, testLogger, nil)

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
	manualExec := decodeResponse[map[string]any](t, execResp)
	if output, ok := manualExec["output"].(map[string]any); !ok || output["secret"] != "super-secret" {
		t.Fatalf("expected function to read secret, got %v", manualExec["output"])
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
		executions := decodeResponse[[]map[string]any](t, execsResp)
		if len(executions) == 0 {
			continue
		}
		latest := executions[0]
		input, _ := latest["input"].(map[string]any)
		output, _ := latest["output"].(map[string]any)
		if input != nil {
			if jobID, ok := input["automation_job"]; ok && jobID == job.ID {
				if output == nil || output["secret"] != "super-secret" {
					t.Fatalf("automation execution missing secret, got %v", output)
				}
				automationRun = true
				break
			}
		}
	}

	if !automationRun {
		t.Fatalf("automation job %s did not execute within timeout", job.ID)
	}
}

func authedRequest(method, url string, body []byte) *http.Request {
	return authedRequestWithTenant(method, url, "tenant-a", body)
}

func authedRequestWithTenant(method, url, tenant string, body []byte) *http.Request {
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, url, reader)
	req.Header.Set("Authorization", "Bearer "+testAuthToken)
	req.Header.Set("X-Tenant-ID", tenant)
	return req
}

func adminRequest(method, url string, body []byte) *http.Request {
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, url, reader)
	req.Header.Set("Authorization", "Bearer "+adminAuthToken)
	req.Header.Set("X-Tenant-ID", "tenant-a")
	return req
}

func marshal(v any) []byte {
	buf, _ := json.Marshal(v)
	return buf
}

func getFunctionID(body []byte) string {
	var def map[string]any
	_ = json.Unmarshal(body, &def)
	if id, ok := def["id"].(string); ok {
		return id
	}
	if id, ok := def["ID"].(string); ok {
		return id
	}
	return ""
}

func createAccount(t *testing.T, handler http.Handler, owner string) string {
	return createAccountWithTenant(t, handler, owner, "tenant-a")
}

func createAccountWithTenant(t *testing.T, handler http.Handler, owner, tenant string) string {
	resp := doJSONWithTenant(handler, http.MethodPost, "/accounts", map[string]any{"owner": owner}, tenant)
	assertStatus(t, resp, http.StatusCreated)
	var acct map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &acct); err != nil {
		t.Fatalf("unmarshal account: %v", err)
	}
	id, _ := acct["ID"].(string)
	return id
}

func TestSystemStatusModules(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	modulesFn := func() []ModuleStatus {
		return []ModuleStatus{
			{Name: "store-postgres", Domain: "store", Category: "store", Status: "started"},
			{Name: "svc-automation", Domain: "automation", Category: "data", Status: "failed", Error: "boom"},
		}
	}
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, modulesFn), authTokens, testLogger, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/system/status", nil))
	assertStatus(t, resp, http.StatusOK)

	var payload struct {
		Modules []ModuleStatus `json:"modules"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal modules: %v", err)
	}
	if len(payload.Modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(payload.Modules))
	}
	if payload.Modules[1].Status != "failed" || payload.Modules[1].Error == "" {
		t.Fatalf("expected failed module with error, got %+v", payload.Modules[1])
	}
}

func TestSystemStatusModuleSummaryUsesInterfaces(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	modulesFn := func() []ModuleStatus {
		return []ModuleStatus{
			{
				Name:       "svc-multi",
				Domain:     "multi",
				Category:   "compute",
				Interfaces: []string{"compute", "data", "event"},
				Status:     "started",
			},
			{
				Name:     "svc-data",
				Domain:   "data",
				Category: "data",
				Status:   "started",
			},
			{
				Name:   "svc-phantom",
				Domain: "phantom",
				Status: "started",
			},
		}
	}
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, modulesFn), authTokens, testLogger, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/system/status", nil))
	assertStatus(t, resp, http.StatusOK)

	var payload struct {
		ModulesSummary map[string][]string       `json:"modules_summary"`
		ModulesAPIMeta map[string]map[string]int `json:"modules_api_meta"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal modules summary: %v", err)
	}

	if !slices.Equal(payload.ModulesSummary["data"], []string{"svc-multi", "svc-data"}) {
		t.Fatalf("expected data summary to include both modules, got %v", payload.ModulesSummary["data"])
	}
	if !slices.Equal(payload.ModulesSummary["event"], []string{"svc-multi"}) {
		t.Fatalf("expected event summary to include svc-multi, got %v", payload.ModulesSummary["event"])
	}
	if !slices.Equal(payload.ModulesSummary["compute"], []string{"svc-multi"}) {
		t.Fatalf("expected compute summary to include svc-multi, got %v", payload.ModulesSummary["compute"])
	}
	if _, ok := payload.ModulesSummary[""]; ok {
		t.Fatalf("expected modules without interfaces to be excluded from summary")
	}

	if len(payload.ModulesAPIMeta) > 0 {
		if meta := payload.ModulesAPIMeta["compute"]; meta["total"] != 1 {
			t.Fatalf("expected compute api meta total=1, got %+v", meta)
		}
	}
}

func TestSystemStatusAPISummary(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	modulesFn := func() []ModuleStatus {
		return []ModuleStatus{
			{
				Name: "svc-one",
				APIs: []engine.APIDescriptor{
					{Name: "compute", Surface: engine.APISurfaceCompute},
					{Name: "telemetry", Surface: "telemetry"},
				},
			},
			{
				Name: "svc-two",
				APIs: []engine.APIDescriptor{
					{Name: "compute", Surface: engine.APISurfaceCompute},
				},
			},
		}
	}
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, modulesFn), authTokens, testLogger, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/system/status", nil))
	assertStatus(t, resp, http.StatusOK)

	var payload struct {
		APIs map[string][]string `json:"modules_api_summary"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal apis: %v", err)
	}
	if !slices.Equal(payload.APIs["compute"], []string{"svc-one", "svc-two"}) {
		t.Fatalf("expected compute api summary, got %v", payload.APIs["compute"])
	}
	if !slices.Equal(payload.APIs["telemetry"], []string{"svc-one"}) {
		t.Fatalf("expected telemetry api summary, got %v", payload.APIs["telemetry"])
	}
}

type eventCapableModule struct {
	name    string
	enabled bool
}

func (e eventCapableModule) Name() string   { return e.name }
func (e eventCapableModule) Domain() string { return "events" }
func (e eventCapableModule) Start(ctx context.Context) error {
	_ = ctx
	return nil
}
func (e eventCapableModule) Stop(ctx context.Context) error {
	_ = ctx
	return nil
}
func (e eventCapableModule) Publish(ctx context.Context, event string, payload any) error {
	_ = ctx
	_ = event
	_ = payload
	return nil
}
func (e eventCapableModule) Subscribe(ctx context.Context, event string, handler func(context.Context, any) error) error {
	_ = ctx
	_ = event
	if handler != nil {
		return handler(context.Background(), nil)
	}
	return nil
}
func (e eventCapableModule) HasEvent() bool { return e.enabled }

func TestSystemStatusRespectsEngineCapabilities(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)

	eng := engine.New(engine.WithOrder("events-off", "events-on"))
	if err := eng.Register(eventCapableModule{name: "events-off", enabled: false}); err != nil {
		t.Fatalf("register events-off: %v", err)
	}
	if err := eng.Register(eventCapableModule{name: "events-on", enabled: true}); err != nil {
		t.Fatalf("register events-on: %v", err)
	}

	modulesFn := EngineModuleProvider(eng)

	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, modulesFn), authTokens, testLogger, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/system/status", nil))
	assertStatus(t, resp, http.StatusOK)

	var payload struct {
		Modules        []ModuleStatus      `json:"modules"`
		ModulesSummary map[string][]string `json:"modules_summary"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal modules: %v", err)
	}
	if len(payload.Modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(payload.Modules))
	}
	if !slices.Equal(payload.ModulesSummary["event"], []string{"events-on"}) {
		t.Fatalf("expected only enabled event module in summary, got %v", payload.ModulesSummary["event"])
	}
}

func TestSystemStatusModuleMeta(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	start := time.Now().Add(-5 * time.Second)
	stop := start.Add(4 * time.Second)
	modulesFn := func() []ModuleStatus {
		return []ModuleStatus{
			{Name: "svc-ok", Status: "started", Ready: "ready", StartNanos: 2_500_000_000, StopNanos: 2_000_000, StartedAt: &start},
			{Name: "svc-fail", Status: "failed", Ready: "not-ready", StartNanos: 3_000_000, StartedAt: &start},
			{Name: "svc-stop", Status: "stop-error", Ready: "ready", StopNanos: 4_000_000, StartedAt: &start, StoppedAt: &stop},
		}
	}
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, modulesFn), authTokens, testLogger, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/system/status", nil))
	assertStatus(t, resp, http.StatusOK)

	var payload struct {
		ModulesMeta    map[string]int                `json:"modules_meta"`
		ModulesTimings map[string]map[string]float64 `json:"modules_timings"`
		ModulesUptime  map[string]float64            `json:"modules_uptime"`
		ModulesSlow    []string                      `json:"modules_slow"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal modules_meta: %v", err)
	}
	if payload.ModulesMeta["total"] != 3 || payload.ModulesMeta["started"] != 1 || payload.ModulesMeta["failed"] != 1 || payload.ModulesMeta["stop_error"] != 1 || payload.ModulesMeta["not_ready"] != 1 {
		t.Fatalf("unexpected modules_meta counts: %+v", payload.ModulesMeta)
	}
	if len(payload.ModulesTimings) != 3 {
		t.Fatalf("expected timings for 3 modules, got %d", len(payload.ModulesTimings))
	}
	if tt := payload.ModulesTimings["svc-ok"]; tt["start_ms"] < 1.0 || tt["stop_ms"] < 2.0 {
		t.Fatalf("unexpected timings for svc-ok: %+v", tt)
	}
	if tt := payload.ModulesTimings["svc-stop"]; tt["stop_ms"] < 4.0 {
		t.Fatalf("unexpected stop timing for svc-stop: %+v", tt)
	}
	if uptime := payload.ModulesUptime["svc-stop"]; uptime < 3.9 || uptime > 4.1 {
		t.Fatalf("unexpected uptime for svc-stop: %f", uptime)
	}
	if len(payload.ModulesSlow) == 0 || payload.ModulesSlow[0] != "svc-ok" {
		t.Fatalf("expected slow modules to include svc-ok, got %v", payload.ModulesSlow)
	}
}

// Ensures slow threshold can be overridden via env.
func TestSystemStatusSlowThresholdEnv(t *testing.T) {
	t.Setenv("MODULE_SLOW_MS", "5000")
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	modulesFn := func() []ModuleStatus {
		start := time.Now()
		return []ModuleStatus{
			{Name: "svc-ok", Status: "started", Ready: "ready", StartNanos: 2_500_000_000, StartedAt: &start},
		}
	}
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, modulesFn), authTokens, testLogger, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/system/status", nil))
	assertStatus(t, resp, http.StatusOK)

	var payload struct {
		ModulesSlow    []string                      `json:"modules_slow"`
		SlowThreshold  float64                       `json:"modules_slow_threshold_ms"`
		ModulesTimings map[string]map[string]float64 `json:"modules_timings"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if len(payload.ModulesSlow) != 0 {
		t.Fatalf("expected no slow modules with higher threshold, got %v", payload.ModulesSlow)
	}
	if payload.SlowThreshold != 5000 {
		t.Fatalf("expected threshold 5000, got %v", payload.SlowThreshold)
	}
	if tms := payload.ModulesTimings["svc-ok"]; tms["start_ms"] < 2500 {
		t.Fatalf("expected timings preserved, got %+v", tms)
	}
}

func TestSystemStatusIncludesListenAddr(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, nil,
		WithListenAddrProvider(func() string { return "127.0.0.1:1234" })),
		authTokens, testLogger, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/system/status", nil))
	assertStatus(t, resp, http.StatusOK)

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if addr, _ := payload["listen_addr"].(string); addr != "127.0.0.1:1234" {
		t.Fatalf("expected listen_addr in status payload, got %v", payload["listen_addr"])
	}
}

func TestSystemStatusMethodGuard(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, nil), authTokens, testLogger, nil)

	okResp := httptest.NewRecorder()
	handler.ServeHTTP(okResp, authedRequest(http.MethodGet, "/system/status", nil))
	assertStatus(t, okResp, http.StatusOK)

	postResp := httptest.NewRecorder()
	handler.ServeHTTP(postResp, authedRequest(http.MethodPost, "/system/status", nil))
	assertStatus(t, postResp, http.StatusMethodNotAllowed)
}

func TestSystemBusEvents(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	var gotEvent string
	var gotPayload any
	publish := func(ctx context.Context, event string, payload any) error {
		gotEvent = event
		gotPayload = payload
		return nil
	}
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, nil, WithBusEndpoints(publish, nil, nil)), nil, testLogger, staticAdminValidator{})

	resp := httptest.NewRecorder()
	body := marshal(map[string]any{
		"event":   "observation",
		"payload": map[string]any{"price": "1.23"},
	})
	handler.ServeHTTP(resp, adminRequest(http.MethodPost, "/system/events", body))
	assertStatus(t, resp, http.StatusOK)
	if gotEvent != "observation" {
		t.Fatalf("expected event propagated, got %s", gotEvent)
	}
	if gotPayload == nil {
		t.Fatalf("expected payload propagated")
	}
}

func TestSystemBusEventsMethodGuard(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, nil), nil, testLogger, staticAdminValidator{})

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, adminRequest(http.MethodGet, "/system/events", nil))
	assertStatus(t, resp, http.StatusMethodNotAllowed)
	if allow := resp.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("expected Allow header %s, got %q", http.MethodPost, allow)
	}
}

func TestSystemBusCompute(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	var seenPayload any
	invoker := func(ctx context.Context, payload any) ([]ComputeResult, error) {
		seenPayload = payload
		return []ComputeResult{
			{Module: "compute-ok", Result: "ok"},
			{Module: "compute-fail", Error: "boom"},
		}, fmt.Errorf("aggregate failure")
	}
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, nil, WithBusEndpoints(nil, nil, invoker)), nil, testLogger, staticAdminValidator{})

	resp := httptest.NewRecorder()
	body := marshal(map[string]any{"payload": map[string]any{"job": 1}})
	handler.ServeHTTP(resp, adminRequest(http.MethodPost, "/system/compute", body))
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected compute fan-out to bubble error as 500, got %d", resp.Code)
	}
	var payload struct {
		Results []ComputeResult `json:"results"`
		Error   string          `json:"error"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(payload.Results) != 2 || payload.Error == "" {
		t.Fatalf("expected results and aggregate error, got %+v", payload)
	}
	if seenPayload == nil {
		t.Fatalf("expected payload passed to invoker")
	}
}

func TestAuthLoginMethodGuard(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, nil), authTokens, testLogger, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/auth/login", nil))
	assertStatus(t, resp, http.StatusMethodNotAllowed)
	if allow := resp.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("expected Allow header %s, got %q", http.MethodPost, allow)
	}
}

func TestSystemBusData(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	var gotTopic string
	var gotPayload any
	push := func(ctx context.Context, topic string, payload any) error {
		gotTopic = topic
		gotPayload = payload
		return nil
	}
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, nil, WithBusEndpoints(nil, push, nil)), nil, testLogger, staticAdminValidator{})

	resp := httptest.NewRecorder()
	body := marshal(map[string]any{
		"topic":   "stream-1",
		"payload": map[string]any{"value": 123},
	})
	handler.ServeHTTP(resp, adminRequest(http.MethodPost, "/system/data", body))
	assertStatus(t, resp, http.StatusOK)
	if gotTopic != "stream-1" || gotPayload == nil {
		t.Fatalf("expected data bus to receive topic and payload, got %s %+v", gotTopic, gotPayload)
	}
}

func TestSystemBusPayloadLimit(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, nil, WithBusEndpoints(func(ctx context.Context, event string, payload any) error {
		return nil
	}, nil, nil), WithBusMaxBytes(32)), nil, testLogger, staticAdminValidator{})

	req := adminRequest(http.MethodPost, "/system/events", marshal(map[string]any{
		"event":   "big",
		"payload": strings.Repeat("a", 1024),
	}))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest && resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected payload limit rejection, got %d (%s)", resp.Code, resp.Body.String())
	}
}

func TestSystemBusRequiresAdminRole(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, nil, WithBusEndpoints(func(ctx context.Context, event string, payload any) error {
		return nil
	}, nil, nil)), authTokens, testLogger, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/system/events", marshal(map[string]any{
		"event":   "test",
		"payload": map[string]any{"value": 1},
	})))
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected admin-only bus to forbid non-admin, got %d", resp.Code)
	}
}

func TestReadyzAndLivez(t *testing.T) {
	application := newTestApplication(t)
	audit := newAuditLog(10, nil)
	modulesFn := func() []ModuleStatus {
		return []ModuleStatus{
			{Name: "svc-ready", Ready: "ready"},
			{Name: "svc-bad", Ready: "not-ready", Error: "boom"},
		}
	}
	handler := wrapWithAuth(NewHandler(application, authTokens, nil, audit, nil, modulesFn), authTokens, testLogger, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/livez", nil))
	assertStatus(t, resp, http.StatusOK)

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/readyz", nil))
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 readyz, got %d", resp.Code)
	}
	var payload struct {
		Modules []ModuleStatus `json:"modules"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.Modules) != 1 || payload.Modules[0].Name != "svc-bad" {
		t.Fatalf("expected not-ready module returned, got %+v", payload.Modules)
	}
}

func doJSON(handler http.Handler, method, path string, payload any) *httptest.ResponseRecorder {
	return doJSONWithTenant(handler, method, path, payload, "tenant-a")
}

func doJSONWithTenant(handler http.Handler, method, path string, payload any, tenant string) *httptest.ResponseRecorder {
	var body []byte
	switch v := payload.(type) {
	case nil:
	case []byte:
		body = v
	default:
		body = marshal(v)
	}
	req := authedRequestWithTenant(method, path, tenant, body)
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
