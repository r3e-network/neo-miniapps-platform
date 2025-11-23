package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/app/auth"
	"github.com/R3E-Network/service_layer/internal/app/jam"
)

// Basic HTTP integration smoke test covering health, auth, accounts, wallets, datafeeds, secrets, oracle, audit.
func TestIntegrationHTTPAPI(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { _ = application.Stop(context.Background()) })

	tokens := []string{"dev-token"}
	authMgr := auth.NewManager("integration-secret", []auth.User{{Username: "admin", Password: "pass", Role: "admin"}})
	auditBuf := newAuditLog(100, nil)
	handler := NewHandler(application, jam.Config{}, tokens, authMgr, auditBuf)
	handler = wrapWithAuth(handler, tokens, nil, authMgr)
	handler = wrapWithAudit(handler, auditBuf)
	handler = wrapWithCORS(handler)

	server := httptest.NewServer(handler)
	defer server.Close()

	client := server.Client()

	// /healthz should be public
	if resp, err := client.Get(server.URL + "/healthz"); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz failed: %v status %d", err, resp.StatusCode)
	}

	// Unauthorized on protected endpoint
	if resp, err := client.Get(server.URL + "/accounts"); err != nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated accounts, got %v status %d", err, resp.StatusCode)
	}

	// Create account
	acctBody := map[string]any{"owner": "integration"}
	acctData := marshalBody(t, acctBody)
	acctResp := do(t, client, server.URL+"/accounts", http.MethodPost, acctData, "dev-token")
	if acctResp.Code != http.StatusCreated {
		t.Fatalf("create account status: %d", acctResp.Code)
	}
	var acct map[string]any
	_ = json.Unmarshal(acctResp.Body.Bytes(), &acct)
	accountID := acct["ID"].(string)

	// Register wallet for signer enforcement
	walletResp := do(t, client, server.URL+"/accounts/"+accountID+"/workspace-wallets", http.MethodPost, marshalBody(t, map[string]any{
		"wallet_address": testWalletFeed,
		"label":          "signer",
		"status":         "active",
	}), "dev-token")
	if walletResp.Code != http.StatusCreated {
		t.Fatalf("create wallet status: %d", walletResp.Code)
	}

	// Create datafeed (requires signer)
	dfResp := do(t, client, server.URL+"/accounts/"+accountID+"/datafeeds", http.MethodPost, marshalBody(t, map[string]any{
		"pair":              "neo/usd",
		"decimals":          8,
		"heartbeat_seconds": 30,
		"threshold_ppm":     0,
		"signer_set":        []string{testWalletFeed},
	}), "dev-token")
	if dfResp.Code != http.StatusCreated {
		t.Fatalf("create datafeed status: %d", dfResp.Code)
	}

	// List accounts
	listResp := do(t, client, server.URL+"/accounts", http.MethodGet, nil, "dev-token")
	if listResp.Code != http.StatusOK {
		t.Fatalf("list accounts status: %d", listResp.Code)
	}
	var accounts []map[string]any
	_ = json.Unmarshal(listResp.Body.Bytes(), &accounts)
	if len(accounts) == 0 {
		t.Fatalf("expected at least one account")
	}

	// Admin login to fetch JWT
	loginResp := do(t, client, server.URL+"/auth/login", http.MethodPost, marshalBody(t, map[string]any{
		"username": "admin",
		"password": "pass",
	}), "")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status: %d", loginResp.Code)
	}
	var login map[string]any
	_ = json.Unmarshal(loginResp.Body.Bytes(), &login)
	jwtToken := login["token"].(string)

	// Admin audit requires tenant
	auditNoTenant := doWithHeaders(t, client, server.URL+"/admin/audit", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer " + jwtToken,
	})
	if auditNoTenant.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without tenant, got %d", auditNoTenant.Code)
	}
	auditOK := doWithHeaders(t, client, server.URL+"/admin/audit?limit=10", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer " + jwtToken,
		"X-Tenant-ID":   "tenant-a",
	})
	if auditOK.Code != http.StatusOK {
		t.Fatalf("expected 200 with tenant, got %d", auditOK.Code)
	}
	var audits []auditEntry
	if err := json.Unmarshal(auditOK.Body.Bytes(), &audits); err != nil {
		t.Fatalf("decode audit: %v", err)
	}
	if len(audits) == 0 {
		t.Fatalf("expected audit entries")
	}
	filtered := doWithHeaders(t, client, server.URL+"/admin/audit?limit=5&contains=/accounts", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer " + jwtToken,
		"X-Tenant-ID":   "tenant-a",
	})
	if filtered.Code != http.StatusOK {
		t.Fatalf("expected 200 for filtered audit, got %d", filtered.Code)
	}

	// Secret create/list
	secretResp := do(t, client, server.URL+"/accounts/"+accountID+"/secrets", http.MethodPost, marshalBody(t, map[string]any{
		"name":  "apiKey",
		"value": "secret-value",
	}), "dev-token")
	if secretResp.Code != http.StatusCreated {
		t.Fatalf("create secret status: %d", secretResp.Code)
	}
	secretList := do(t, client, server.URL+"/accounts/"+accountID+"/secrets", http.MethodGet, nil, "dev-token")
	if secretList.Code != http.StatusOK {
		t.Fatalf("list secrets status: %d", secretList.Code)
	}

	// Oracle source + request
	srcResp := do(t, client, server.URL+"/accounts/"+accountID+"/oracle/sources", http.MethodPost, marshalBody(t, map[string]any{
		"name":   "prices",
		"url":    "https://example.com",
		"method": "GET",
	}), "dev-token")
	if srcResp.Code != http.StatusCreated {
		t.Fatalf("create oracle source status: %d", srcResp.Code)
	}
	var src map[string]any
	_ = json.Unmarshal(srcResp.Body.Bytes(), &src)
	srcID := src["ID"].(string)

	reqResp := do(t, client, server.URL+"/accounts/"+accountID+"/oracle/requests", http.MethodPost, marshalBody(t, map[string]any{
		"data_source_id": srcID,
		"payload":        "{}",
	}), "dev-token")
	if reqResp.Code != http.StatusCreated {
		t.Fatalf("oracle request status: %d", reqResp.Code)
	}

	// DataLink channel + delivery
	channelResp := do(t, client, server.URL+"/accounts/"+accountID+"/datalink/channels", http.MethodPost, marshalBody(t, map[string]any{
		"name":       "provider-1",
		"endpoint":   "https://api.provider.test",
		"signer_set": []string{testWalletFeed},
	}), "dev-token")
	if channelResp.Code != http.StatusCreated {
		t.Fatalf("create datalink channel status: %d", channelResp.Code)
	}
	var channel map[string]any
	_ = json.Unmarshal(channelResp.Body.Bytes(), &channel)
	channelID := channel["id"].(string)

	delResp := do(t, client, server.URL+"/accounts/"+accountID+"/datalink/channels/"+channelID+"/deliveries", http.MethodPost, marshalBody(t, map[string]any{
		"payload": map[string]any{"hello": "world"},
	}), "dev-token")
	if delResp.Code != http.StatusCreated {
		t.Fatalf("create datalink delivery status: %d", delResp.Code)
	}
}

func marshalBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewReader(b)
}

func do(t *testing.T, client *http.Client, url, method string, body io.Reader, token string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	rec.Code = resp.StatusCode
	b, _ := io.ReadAll(resp.Body)
	rec.Body.Write(b)
	return rec
}

func doWithHeaders(t *testing.T, client *http.Client, url, method string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	rec.Code = resp.StatusCode
	b, _ := io.ReadAll(resp.Body)
	rec.Body.Write(b)
	return rec
}
