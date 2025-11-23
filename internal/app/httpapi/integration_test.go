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

// Basic HTTP integration smoke test covering health, auth, accounts, wallets, datafeeds, secrets, oracle, audit, random, datalink.
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
	// Token-only (non-admin) should be forbidden on admin paths even with tenant header.
	tokenAudit := doWithHeaders(t, client, server.URL+"/admin/audit?limit=1", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "tenant-a",
	})
	if tokenAudit.Code != http.StatusForbidden && tokenAudit.Code != http.StatusUnauthorized {
		t.Fatalf("expected forbidden/unauthorized for token auth on admin, got %d", tokenAudit.Code)
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
	// Tenant scoping on accounts list: create an account under tenant-b and ensure tenant-a list filters it out.
	tenantBResp := doWithHeaders(t, client, server.URL+"/accounts", http.MethodPost, marshalBody(t, map[string]any{
		"owner": "tenant-b-owner",
	}), map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "tenant-b",
	})
	if tenantBResp.Code != http.StatusCreated {
		t.Fatalf("create account tenant-b status: %d", tenantBResp.Code)
	}
	tenantAList := doWithHeaders(t, client, server.URL+"/accounts", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "tenant-a",
	})
	if tenantAList.Code != http.StatusOK {
		t.Fatalf("tenant-a list status: %d", tenantAList.Code)
	}
	var tenantAAccounts []map[string]any
	_ = json.Unmarshal(tenantAList.Body.Bytes(), &tenantAAccounts)
	for _, acc := range tenantAAccounts {
		if acc["Owner"] == "tenant-b-owner" {
			t.Fatalf("tenant-a list should not include tenant-b accounts")
		}
	}
	// Account fetch across tenant should be forbidden.
	tenantBAccount := getID(decodeMap(t, tenantBResp.Body.Bytes()))
	crossTenant := doWithHeaders(t, client, server.URL+"/accounts/"+tenantBAccount, http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "tenant-a",
	})
	if crossTenant.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden when accessing other tenant account, got %d", crossTenant.Code)
	}
	noTenant := doWithHeaders(t, client, server.URL+"/accounts/"+tenantBAccount, http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenant.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden without tenant when account is tenant-scoped, got %d", noTenant.Code)
	}
	// List without tenant should be forbidden.
	publicList := doWithHeaders(t, client, server.URL+"/accounts", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if publicList.Code != http.StatusForbidden {
		t.Fatalf("public list status: %d", publicList.Code)
	}
	// Access tenant-scoped resources should fail without or with mismatched tenant, succeed with correct tenant.
	noTenantSecret := doWithHeaders(t, client, server.URL+"/accounts/"+tenantBAccount+"/secrets", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantSecret.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for tenant-scoped secret list without tenant, got %d", noTenantSecret.Code)
	}
	wrongTenantSecret := doWithHeaders(t, client, server.URL+"/accounts/"+tenantBAccount+"/secrets", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "tenant-a",
	})
	if wrongTenantSecret.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for tenant-scoped secret list with wrong tenant, got %d", wrongTenantSecret.Code)
	}
	okTenantSecret := doWithHeaders(t, client, server.URL+"/accounts/"+tenantBAccount+"/secrets", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "tenant-b",
	})
	if okTenantSecret.Code != http.StatusOK {
		t.Fatalf("expected ok for tenant-scoped secret list with correct tenant, got %d", okTenantSecret.Code)
	}
	// Other tenant-scoped resources should reject missing/wrong tenant.
	wrongTenantFeeds := doWithHeaders(t, client, server.URL+"/accounts/"+accountID+"/datafeeds", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "tenant-b",
	})
	if wrongTenantFeeds.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for datafeeds with wrong tenant, got %d", wrongTenantFeeds.Code)
	}
	noTenantFeeds := doWithHeaders(t, client, server.URL+"/accounts/"+accountID+"/datafeeds", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantFeeds.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for datafeeds without tenant, got %d", noTenantFeeds.Code)
	}
	wrongTenantPricefeeds := doWithHeaders(t, client, server.URL+"/accounts/"+accountID+"/pricefeeds", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "tenant-b",
	})
	if wrongTenantPricefeeds.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for pricefeeds with wrong tenant, got %d", wrongTenantPricefeeds.Code)
	}
	noTenantPricefeeds := doWithHeaders(t, client, server.URL+"/accounts/"+accountID+"/pricefeeds", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantPricefeeds.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for pricefeeds without tenant, got %d", noTenantPricefeeds.Code)
	}
	// Gasbank must also enforce tenant.
	wrongTenantGasbank := doWithHeaders(t, client, server.URL+"/accounts/"+accountID+"/gasbank", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "tenant-b",
	})
	if wrongTenantGasbank.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for gasbank with wrong tenant, got %d", wrongTenantGasbank.Code)
	}
	noTenantGasbank := doWithHeaders(t, client, server.URL+"/accounts/"+accountID+"/gasbank", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantGasbank.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for gasbank without tenant, got %d", noTenantGasbank.Code)
	}
	// Datalink must enforce tenant.
	wrongTenantDatalink := doWithHeaders(t, client, server.URL+"/accounts/"+accountID+"/datalink/channels", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "tenant-b",
	})
	if wrongTenantDatalink.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for datalink with wrong tenant, got %d", wrongTenantDatalink.Code)
	}
	noTenantDatalink := doWithHeaders(t, client, server.URL+"/accounts/"+accountID+"/datalink/channels", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantDatalink.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for datalink without tenant, got %d", noTenantDatalink.Code)
	}
	// Oracle must enforce tenant.
	wrongTenantOracle := doWithHeaders(t, client, server.URL+"/accounts/"+accountID+"/oracle/sources", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "tenant-b",
	})
	if wrongTenantOracle.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for oracle sources with wrong tenant, got %d", wrongTenantOracle.Code)
	}
	noTenantOracle := doWithHeaders(t, client, server.URL+"/accounts/"+accountID+"/oracle/sources", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantOracle.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for oracle sources without tenant, got %d", noTenantOracle.Code)
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
	channelID := getID(channel)

	delResp := do(t, client, server.URL+"/accounts/"+accountID+"/datalink/channels/"+channelID+"/deliveries", http.MethodPost, marshalBody(t, map[string]any{
		"payload": map[string]any{"hello": "world"},
	}), "dev-token")
	if delResp.Code != http.StatusCreated {
		t.Fatalf("create datalink delivery status: %d", delResp.Code)
	}
	delList := do(t, client, server.URL+"/accounts/"+accountID+"/datalink/deliveries?limit=5", http.MethodGet, nil, "dev-token")
	if delList.Code != http.StatusOK {
		t.Fatalf("list datalink deliveries status: %d", delList.Code)
	}

	// Random generation
	randResp := do(t, client, server.URL+"/accounts/"+accountID+"/random", http.MethodPost, marshalBody(t, map[string]any{
		"length": 8,
	}), "dev-token")
	if randResp.Code != http.StatusOK {
		t.Fatalf("random generate status: %d", randResp.Code)
	}
	randList := do(t, client, server.URL+"/accounts/"+accountID+"/random/requests?limit=5", http.MethodGet, nil, "dev-token")
	if randList.Code != http.StatusOK {
		t.Fatalf("random list status: %d", randList.Code)
	}

	// Function + automation job
	fnResp := do(t, client, server.URL+"/accounts/"+accountID+"/functions", http.MethodPost, marshalBody(t, map[string]any{
		"name":   "hello",
		"source": "() => ({ ok: true })",
	}), "dev-token")
	if fnResp.Code != http.StatusCreated {
		t.Fatalf("create function status: %d", fnResp.Code)
	}
	var fn map[string]any
	_ = json.Unmarshal(fnResp.Body.Bytes(), &fn)
	fnID := getID(fn)

	jobResp := do(t, client, server.URL+"/accounts/"+accountID+"/automation/jobs", http.MethodPost, marshalBody(t, map[string]any{
		"function_id": fnID,
		"name":        "job-1",
		"schedule":    "@every 1m",
	}), "dev-token")
	if jobResp.Code != http.StatusCreated {
		t.Fatalf("create automation job status: %d", jobResp.Code)
	}

	// Pricefeed create/list
	pfResp := do(t, client, server.URL+"/accounts/"+accountID+"/pricefeeds", http.MethodPost, marshalBody(t, map[string]any{
		"base_asset":         "NEO",
		"quote_asset":        "USD",
		"update_interval":    "1m",
		"heartbeat_interval": "2m",
		"deviation_percent":  1.0,
	}), "dev-token")
	if pfResp.Code != http.StatusCreated {
		t.Fatalf("create pricefeed status: %d", pfResp.Code)
	}
	pfList := do(t, client, server.URL+"/accounts/"+accountID+"/pricefeeds", http.MethodGet, nil, "dev-token")
	if pfList.Code != http.StatusOK {
		t.Fatalf("list pricefeeds status: %d", pfList.Code)
	}

	// System status/descriptors
	statusResp := do(t, client, server.URL+"/system/status", http.MethodGet, nil, "dev-token")
	if statusResp.Code != http.StatusOK {
		t.Fatalf("system status status: %d", statusResp.Code)
	}
	descrResp := do(t, client, server.URL+"/system/descriptors", http.MethodGet, nil, "dev-token")
	if descrResp.Code != http.StatusOK {
		t.Fatalf("system descriptors status: %d", descrResp.Code)
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
		req.Header.Set("X-Tenant-ID", "tenant-a")
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

func getID(m map[string]any) string {
	if v, ok := m["id"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	if v, ok := m["ID"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func decodeMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode map: %v", err)
	}
	return m
}
