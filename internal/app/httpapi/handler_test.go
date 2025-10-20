package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
)

func TestHandlerLifecycle(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}

	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	defer application.Stop(context.Background())

	h := NewHandler(application)

	body := marshal(map[string]any{"owner": "alice"})
	req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewReader(body))
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.Code)
	}

	var acct map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &acct); err != nil {
		t.Fatalf("unmarshal account: %v", err)
	}
	id := acct["ID"].(string)

	funcBody := marshal(map[string]any{"name": "hello", "source": "() => 1"})
	req = httptest.NewRequest(http.MethodPost, "/accounts/"+id+"/functions", bytes.NewReader(funcBody))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create function, got %d", resp.Code)
	}
	fnID := getFunctionID(resp.Body.Bytes())

	// Execute function
	execBody := marshal(map[string]any{"input": "hello"})
	req = httptest.NewRequest(http.MethodPost, "/accounts/"+id+"/functions/"+fnID+"/execute", bytes.NewReader(execBody))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 execute, got %d", resp.Code)
	}

	trigBody := marshal(map[string]any{"function_id": fnID, "rule": "cron:@hourly"})
	req = httptest.NewRequest(http.MethodPost, "/accounts/"+id+"/triggers", bytes.NewReader(trigBody))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create trigger, got %d", resp.Code)
	}

	ensureBody := marshal(map[string]any{"wallet_address": "WALLET-1"})
	req = httptest.NewRequest(http.MethodPost, "/accounts/"+id+"/gasbank", bytes.NewReader(ensureBody))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 ensure gas account, got %d", resp.Code)
	}
	var gasAcct map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &gasAcct); err != nil {
		t.Fatalf("unmarshal gas account: %v", err)
	}
	gasID := gasAcct["ID"].(string)

	depositBody := marshal(map[string]any{"gas_account_id": gasID, "amount": 6.5, "tx_id": "tx1"})
	req = httptest.NewRequest(http.MethodPost, "/accounts/"+id+"/gasbank/deposit", bytes.NewReader(depositBody))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 deposit, got %d", resp.Code)
	}

	withdrawBody := marshal(map[string]any{"gas_account_id": gasID, "amount": 2.0, "to_address": "ADDR"})
	req = httptest.NewRequest(http.MethodPost, "/accounts/"+id+"/gasbank/withdraw", bytes.NewReader(withdrawBody))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 withdraw, got %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/accounts/"+id+"/gasbank/transactions?gas_account_id="+gasID, nil)
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 transactions, got %d", resp.Code)
	}
	var txs []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &txs); err != nil {
		t.Fatalf("unmarshal transactions: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(txs))
	}

	jobBody := marshal(map[string]any{"function_id": fnID, "name": "daily", "schedule": "@daily"})
	req = httptest.NewRequest(http.MethodPost, "/accounts/"+id+"/automation/jobs", bytes.NewReader(jobBody))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create job, got %d", resp.Code)
	}
	var job map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &job); err != nil {
		t.Fatalf("unmarshal job: %v", err)
	}
	jobID := job["ID"].(string)

	req = httptest.NewRequest(http.MethodGet, "/accounts/"+id+"/automation/jobs", nil)
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 list jobs, got %d", resp.Code)
	}

	disableJob := marshal(map[string]any{"enabled": false})
	req = httptest.NewRequest(http.MethodPatch, "/accounts/"+id+"/automation/jobs/"+jobID, bytes.NewReader(disableJob))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 patch job, got %d", resp.Code)
	}

	feedBody := marshal(map[string]any{
		"base_asset":         "NEO",
		"quote_asset":        "USD",
		"update_interval":    "@every 1m",
		"heartbeat_interval": "@every 1h",
		"deviation_percent":  0.5,
	})
	req = httptest.NewRequest(http.MethodPost, "/accounts/"+id+"/pricefeeds", bytes.NewReader(feedBody))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create feed, got %d", resp.Code)
	}
	var feed map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &feed); err != nil {
		t.Fatalf("unmarshal feed: %v", err)
	}
	feedID := feed["ID"].(string)

	req = httptest.NewRequest(http.MethodGet, "/accounts/"+id+"/pricefeeds/"+feedID, nil)
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 get feed, got %d", resp.Code)
	}

	feedPatch := marshal(map[string]any{"active": false})
	req = httptest.NewRequest(http.MethodPatch, "/accounts/"+id+"/pricefeeds/"+feedID, bytes.NewReader(feedPatch))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 patch feed, got %d", resp.Code)
	}

	snapshotBody := marshal(map[string]any{
		"price":        10.5,
		"source":       "oracle",
		"collected_at": time.Now().UTC().Format(time.RFC3339),
	})
	req = httptest.NewRequest(http.MethodPost, "/accounts/"+id+"/pricefeeds/"+feedID+"/snapshots", bytes.NewReader(snapshotBody))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 snapshot, got %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/accounts/"+id+"/pricefeeds/"+feedID+"/snapshots", nil)
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 snapshots, got %d", resp.Code)
	}

	sourceBody := marshal(map[string]any{
		"name":   "prices",
		"url":    "https://api.example.com",
		"method": "GET",
	})
	req = httptest.NewRequest(http.MethodPost, "/accounts/"+id+"/oracle/sources", bytes.NewReader(sourceBody))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create source, got %d", resp.Code)
	}
	var source map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &source); err != nil {
		t.Fatalf("unmarshal source: %v", err)
	}
	sourceID := source["ID"].(string)

	req = httptest.NewRequest(http.MethodPatch, "/accounts/"+id+"/oracle/sources/"+sourceID, bytes.NewReader(marshal(map[string]any{"enabled": false})))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 patch source, got %d", resp.Code)
	}

	requestBody := marshal(map[string]any{"data_source_id": sourceID, "payload": "{}"})
	req = httptest.NewRequest(http.MethodPost, "/accounts/"+id+"/oracle/requests", bytes.NewReader(requestBody))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 create request, got %d", resp.Code)
	}
	var request map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &request); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	requestID := request["ID"].(string)

	req = httptest.NewRequest(http.MethodPatch, "/accounts/"+id+"/oracle/requests/"+requestID, bytes.NewReader(marshal(map[string]any{"status": "running"})))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 mark running, got %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodPatch, "/accounts/"+id+"/oracle/requests/"+requestID, bytes.NewReader(marshal(map[string]any{"status": "succeeded", "result": `{"price":10}`})))
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 complete request, got %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/accounts/"+id+"/oracle/requests", nil)
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 list requests, got %d", resp.Code)
	}
	var requestsList []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &requestsList); err != nil {
		t.Fatalf("unmarshal requests: %v", err)
	}
	if len(requestsList) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requestsList))
	}

	req = httptest.NewRequest(http.MethodGet, "/accounts/"+id+"/gasbank", nil)
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 list gas accounts, got %d", resp.Code)
	}
	var gasAccounts []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &gasAccounts); err != nil {
		t.Fatalf("unmarshal gas accounts: %v", err)
	}
	if len(gasAccounts) != 1 {
		t.Fatalf("expected 1 gas account, got %d", len(gasAccounts))
	}

	req = httptest.NewRequest(http.MethodDelete, "/accounts/"+id, nil)
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected 204 delete account, got %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/accounts/"+id, nil)
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", resp.Code)
	}
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
