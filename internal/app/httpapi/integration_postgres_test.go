//go:build integration && postgres

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/app/auth"
	"github.com/R3E-Network/service_layer/internal/app/jam"
	"github.com/R3E-Network/service_layer/internal/app/storage/postgres"
	"github.com/R3E-Network/service_layer/internal/platform/database"
	"github.com/R3E-Network/service_layer/internal/platform/migrations"
	"github.com/joho/godotenv"
)

// Integration test against Postgres to ensure migrations + core flows work with persistence.
func TestIntegrationPostgres(t *testing.T) {
	_ = godotenv.Load() // allow .env for local runs
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping Postgres integration")
	}

	ctx := context.Background()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := migrations.Apply(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	pgStore := postgres.New(db)
	stores := app.Stores{
		Accounts:         pgStore,
		Functions:        pgStore,
		Triggers:         pgStore,
		GasBank:          pgStore,
		Automation:       pgStore,
		PriceFeeds:       pgStore,
		DataFeeds:        pgStore,
		DataStreams:      pgStore,
		DataLink:         pgStore,
		DTA:              pgStore,
		Confidential:     pgStore,
		Oracle:           pgStore,
		Secrets:          pgStore,
		CRE:              pgStore,
		CCIP:             pgStore,
		VRF:              pgStore,
		WorkspaceWallets: pgStore,
	}
	appInstance, err := app.New(stores, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if err := appInstance.Start(ctx); err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { _ = appInstance.Stop(context.Background()) })

	// Wire handler with JWT + tokens backed by persisted db
	tokens := []string{"dev-token"}
	authMgr := auth.NewManager("integration-secret", []auth.User{{Username: "admin", Password: "pass", Role: "admin"}})
	auditBuf := newAuditLog(100, newPostgresAuditSink(db))
	handler := NewHandler(appInstance, jam.Config{}, tokens, authMgr, auditBuf)
	handler = wrapWithAuth(handler, tokens, nil, authMgr)
	handler = wrapWithAudit(handler, auditBuf)
	handler = wrapWithCORS(handler)

	server := httptest.NewServer(handler)
	defer server.Close()

	client := server.Client()

	tenant := "tenant-pg"
	owner := fmt.Sprintf("pg-integration-%d", time.Now().UnixNano())
	acctResp := doWithHeaders(t, client, server.URL+"/accounts", http.MethodPost, marshalBody(t, map[string]any{"owner": owner}), map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   tenant,
	})
	if acctResp.Code != http.StatusCreated {
		t.Fatalf("create account status: %d", acctResp.Code)
	}
	acctID := getID(decodeMap(t, acctResp.Body.Bytes()))

	// Workspace wallet to satisfy signer requirements
	walletResp := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/workspace-wallets", http.MethodPost, marshalBody(t, map[string]any{
		"wallet_address": "npg-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		"label":          "signer",
		"status":         "active",
	}), map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   tenant,
	})
	if walletResp.Code != http.StatusCreated {
		t.Fatalf("create wallet status: %d", walletResp.Code)
	}
	walletPayload := decodeMap(t, walletResp.Body.Bytes())
	walletAddress, _ := walletPayload["wallet_address"].(string)
	if walletAddress == "" {
		t.Fatalf("wallet address missing from response")
	}

	// Create datafeed (persists to Postgres)
	dfResp := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/datafeeds", http.MethodPost, marshalBody(t, map[string]any{
		"pair":              "pg/usd",
		"decimals":          8,
		"heartbeat_seconds": 30,
		"threshold_ppm":     0,
		"signer_set":        []string{walletAddress},
	}), map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   tenant,
	})
	if dfResp.Code != http.StatusCreated {
		t.Fatalf("create datafeed status: %d", dfResp.Code)
	}

	// Secret create/list
	secretResp := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/secrets", http.MethodPost, marshalBody(t, map[string]any{
		"name":  "apiKey",
		"value": "secret-value",
	}), map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   tenant,
	})
	if secretResp.Code != http.StatusCreated {
		t.Fatalf("create secret status: %d", secretResp.Code)
	}
	secretList := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/secrets", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   tenant,
	})
	if secretList.Code != http.StatusOK {
		t.Fatalf("list secrets status: %d", secretList.Code)
	}

	// Health & audit endpoints should work
	if resp, err := client.Get(server.URL + "/healthz"); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz failed: %v status %d", err, resp.StatusCode)
	}
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

	// Admin audit requires tenant header
	auditNoTenant := doWithHeaders(t, client, server.URL+"/admin/audit?limit=1", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer " + jwtToken,
	})
	if auditNoTenant.Code != http.StatusForbidden {
		t.Fatalf("expected admin audit to be forbidden without tenant, got %d", auditNoTenant.Code)
	}
	auditOK := doWithHeaders(t, client, server.URL+"/admin/audit?limit=5&contains="+acctID, http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer " + jwtToken,
		"X-Tenant-ID":   tenant,
	})
	if auditOK.Code != http.StatusOK {
		t.Fatalf("expected admin audit ok, got %d", auditOK.Code)
	}

	// Accessing a tenant-scoped account without tenant should be forbidden.
	noTenant := doWithHeaders(t, client, server.URL+"/accounts/"+acctID, http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenant.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden without tenant on tenant-scoped account, got %d", noTenant.Code)
	}

	// List without tenant should not leak tenant-tagged accounts.
	publicList := doWithHeaders(t, client, server.URL+"/accounts", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if publicList.Code != http.StatusForbidden {
		t.Fatalf("public list status: %d", publicList.Code)
	}

	// Tenant-scoped resources should reject missing tenant and accept correct tenant.
	noTenantSecret := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/secrets", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantSecret.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for tenant-scoped secret list without tenant, got %d", noTenantSecret.Code)
	}
	wrongTenantSecret := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/secrets", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "other-tenant",
	})
	if wrongTenantSecret.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for tenant-scoped secret list with wrong tenant, got %d", wrongTenantSecret.Code)
	}
	okTenantSecret := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/secrets", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   tenant,
	})
	if okTenantSecret.Code != http.StatusOK {
		t.Fatalf("expected ok for tenant-scoped secret list with correct tenant, got %d", okTenantSecret.Code)
	}

	// Other tenant-scoped resources should reject missing/wrong tenant.
	wrongTenantFeeds := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/datafeeds", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "other-tenant",
	})
	if wrongTenantFeeds.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for datafeeds with wrong tenant, got %d", wrongTenantFeeds.Code)
	}
	noTenantFeeds := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/datafeeds", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantFeeds.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for datafeeds without tenant, got %d", noTenantFeeds.Code)
	}
	wrongTenantPricefeeds := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/pricefeeds", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "other-tenant",
	})
	if wrongTenantPricefeeds.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for pricefeeds with wrong tenant, got %d", wrongTenantPricefeeds.Code)
	}
	noTenantPricefeeds := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/pricefeeds", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantPricefeeds.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for pricefeeds without tenant, got %d", noTenantPricefeeds.Code)
	}
	wrongTenantGasbank := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/gasbank", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "other-tenant",
	})
	if wrongTenantGasbank.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for gasbank with wrong tenant, got %d", wrongTenantGasbank.Code)
	}
	noTenantGasbank := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/gasbank", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantGasbank.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for gasbank without tenant, got %d", noTenantGasbank.Code)
	}
	wrongTenantDatalink := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/datalink/channels", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "other-tenant",
	})
	if wrongTenantDatalink.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for datalink with wrong tenant, got %d", wrongTenantDatalink.Code)
	}
	noTenantDatalink := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/datalink/channels", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantDatalink.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for datalink without tenant, got %d", noTenantDatalink.Code)
	}
	wrongTenantOracle := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/oracle/sources", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "other-tenant",
	})
	if wrongTenantOracle.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for oracle sources with wrong tenant, got %d", wrongTenantOracle.Code)
	}
	noTenantOracle := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/oracle/sources", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantOracle.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for oracle sources without tenant, got %d", noTenantOracle.Code)
	}
	wrongTenantOracleReqs := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/oracle/requests", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "other-tenant",
	})
	if wrongTenantOracleReqs.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for oracle requests with wrong tenant, got %d", wrongTenantOracleReqs.Code)
	}
	noTenantOracleReqs := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/oracle/requests", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantOracleReqs.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for oracle requests without tenant, got %d", noTenantOracleReqs.Code)
	}
	wrongTenantAutomation := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/automation/jobs", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "other-tenant",
	})
	if wrongTenantAutomation.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for automation with wrong tenant, got %d", wrongTenantAutomation.Code)
	}
	noTenantAutomation := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/automation/jobs", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantAutomation.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for automation without tenant, got %d", noTenantAutomation.Code)
	}
	wrongTenantRandom := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/random/requests", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "other-tenant",
	})
	if wrongTenantRandom.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for random with wrong tenant, got %d", wrongTenantRandom.Code)
	}
	noTenantRandom := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/random/requests", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantRandom.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for random without tenant, got %d", noTenantRandom.Code)
	}
	wrongTenantCCIP := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/ccip/lanes", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "other-tenant",
	})
	if wrongTenantCCIP.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for ccip with wrong tenant, got %d", wrongTenantCCIP.Code)
	}
	noTenantCCIP := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/ccip/lanes", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantCCIP.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for ccip without tenant, got %d", noTenantCCIP.Code)
	}
	wrongTenantVRF := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/vrf/keys", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "other-tenant",
	})
	if wrongTenantVRF.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for vrf with wrong tenant, got %d", wrongTenantVRF.Code)
	}
	noTenantVRF := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/vrf/keys", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantVRF.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for vrf without tenant, got %d", noTenantVRF.Code)
	}
	wrongTenantStreams := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/datastreams", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "other-tenant",
	})
	if wrongTenantStreams.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for datastreams with wrong tenant, got %d", wrongTenantStreams.Code)
	}
	noTenantStreams := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/datastreams", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantStreams.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for datastreams without tenant, got %d", noTenantStreams.Code)
	}
	wrongTenantDTA := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/dta/products", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
		"X-Tenant-ID":   "other-tenant",
	})
	if wrongTenantDTA.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for dta with wrong tenant, got %d", wrongTenantDTA.Code)
	}
	noTenantDTA := doWithHeaders(t, client, server.URL+"/accounts/"+acctID+"/dta/products", http.MethodGet, nil, map[string]string{
		"Authorization": "Bearer dev-token",
	})
	if noTenantDTA.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for dta without tenant, got %d", noTenantDTA.Code)
	}
}
