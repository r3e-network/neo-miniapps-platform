//go:build integration && postgres

package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/app/auth"
	"github.com/R3E-Network/service_layer/internal/app/jam"
	"github.com/R3E-Network/service_layer/internal/app/runtime"
	"github.com/R3E-Network/service_layer/internal/platform/database"
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

	cfg, err := runtime.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn

	appRuntime, err := runtime.NewApplication(cfg, true)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() { _ = appRuntime.Shutdown(ctx) })

	if err := appRuntime.Run(ctx); err != nil {
		t.Fatalf("run runtime: %v", err)
	}

	// Wire handler with JWT + tokens backed by persisted db
	appInstance := appRuntime.App()
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

	// Create account (persisted)
	acctResp := do(t, client, server.URL+"/accounts", http.MethodPost, marshalBody(t, map[string]any{"owner": "pg-integration"}), "dev-token")
	if acctResp.Code != http.StatusCreated {
		t.Fatalf("create account status: %d", acctResp.Code)
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
}
