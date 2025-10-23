package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
)

const testAuthToken = "test-token"

func TestHandlerLifecycle(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}

	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	defer application.Stop(context.Background())

	handler := wrapWithAuth(NewHandler(application), []string{testAuthToken}, nil)

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

	randomBody := marshal(map[string]any{"length": 16})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPost, "/accounts/"+id+"/random", randomBody))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 random, got %d", resp.Code)
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

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/gasbank/transactions?gas_account_id="+gasID, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 transactions, got %d", resp.Code)
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

	feedPatch := marshal(map[string]any{"active": false})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPatch, "/accounts/"+id+"/pricefeeds/"+feedID, feedPatch))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 patch feed, got %d", resp.Code)
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

	disableSource := marshal(map[string]any{"enabled": false})
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodPatch, "/accounts/"+id+"/oracle/sources/"+sourceID, disableSource))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 patch source, got %d", resp.Code)
	}

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

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, authedRequest(http.MethodGet, "/accounts/"+id+"/oracle/requests", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 list requests, got %d", resp.Code)
	}

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
}

func TestHandlerAuthRequired(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	handler := wrapWithAuth(NewHandler(application), []string{testAuthToken}, nil)

	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
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
