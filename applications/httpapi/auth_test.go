package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/applications/auth"
	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/golang-jwt/jwt/v5"
)

func TestWrapWithAuthRejectsWhenNoTokensConfigured(t *testing.T) {
	var called bool
	wrapped := wrapWithAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), nil, logger.NewDefault("test"), nil)

	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when tokens are missing, got %d", rec.Code)
	}
	if called {
		t.Fatalf("expected handler not to be invoked when unauthorised")
	}

	// Public endpoints should remain accessible.
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec = httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected healthz to be served, got %d", rec.Code)
	}
}

func TestSupabaseJWTValidator(t *testing.T) {
	secret := "supabase-secret"
	aud := "authenticated"
	claims := &auth.Claims{
		Username: "alice",
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Audience: jwt.ClaimStrings{aud},
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	validator := NewSupabaseJWTValidator(secret, aud, nil, "", "")
	got, err := validator.Validate(token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got.Username != "alice" || got.Role != "admin" {
		t.Fatalf("unexpected claims: %+v", got)
	}

	// Audience mismatch should fail.
	badValidator := NewSupabaseJWTValidator(secret, "other", nil, "", "")
	if _, err := badValidator.Validate(token); err == nil {
		t.Fatalf("expected audience mismatch to fail")
	}

	// Admin role mapping
	adminClaims := &auth.Claims{
		Username: "svc",
		Role:     "service_role",
		RegisteredClaims: jwt.RegisteredClaims{
			Audience: jwt.ClaimStrings{aud},
		},
	}
	adminToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, adminClaims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign admin: %v", err)
	}
	adminValidator := NewSupabaseJWTValidator(secret, aud, []string{"service_role"}, "", "")
	mapped, err := adminValidator.Validate(adminToken)
	if err != nil {
		t.Fatalf("validate admin: %v", err)
	}
	if mapped.Role != "admin" {
		t.Fatalf("expected role mapped to admin, got %s", mapped.Role)
	}

	// Role claim mapping
	roleClaims := jwt.MapClaims{
		"sub": "dave",
		"aud": aud,
		"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
		"app": map[string]any{"role": "service_role"},
	}
	roleToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, roleClaims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign role: %v", err)
	}
	roleValidator := NewSupabaseJWTValidator(secret, aud, []string{"service_role"}, "app.tenant", "app.role")
	roleMapped, err := roleValidator.Validate(roleToken)
	if err != nil {
		t.Fatalf("validate role: %v", err)
	}
	if roleMapped.Role != "admin" {
		t.Fatalf("expected role mapped via claim then admin map, got %s", roleMapped.Role)
	}

	// Tenant claim mapping
	tenantClaims := jwt.MapClaims{
		"sub":    "carol",
		"role":   "user",
		"tenant": "t-123",
		"aud":    aud,
		"exp":    jwt.NewNumericDate(time.Now().Add(time.Hour)),
		"app":    map[string]any{"tenant": "t-456"},
	}
	tenantToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, tenantClaims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign tenant: %v", err)
	}
	tenantValidator := NewSupabaseJWTValidator(secret, aud, nil, "app.tenant", "")
	withTenant, err := tenantValidator.Validate(tenantToken)
	if err != nil {
		t.Fatalf("validate tenant: %v", err)
	}
	if withTenant.Tenant != "t-456" {
		t.Fatalf("expected tenant mapped from claim, got %s", withTenant.Tenant)
	}
}

type stubValidator struct {
	claims *auth.Claims
	err    error
}

func (s stubValidator) Validate(string) (*auth.Claims, error) {
	return s.claims, s.err
}

func TestCompositeValidator(t *testing.T) {
	firstErr := stubValidator{err: jwt.ErrTokenInvalidClaims}
	secondOK := stubValidator{claims: &auth.Claims{Username: "bob", Role: "user"}}
	validator := NewCompositeValidator(firstErr, secondOK)

	got, err := validator.Validate("token")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got.Username != "bob" || got.Role != "user" {
		t.Fatalf("unexpected claims: %+v", got)
	}

	// All failing validators bubble the last error.
	allFail := NewCompositeValidator(firstErr, stubValidator{err: jwt.ErrTokenMalformed})
	if _, err := allFail.Validate("token"); err == nil {
		t.Fatalf("expected failure when all validators fail")
	}
}

func TestRefreshUsesConfiguredGoTrueURL(t *testing.T) {
	t.Setenv("SUPABASE_GOTRUE_URL", "")
	calls := make(chan *http.Request, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls <- r
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"new-token"}`))
	}))
	defer ts.Close()

	h := &handler{supabaseGoTrueURL: ts.URL}
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(`{"refresh_token":"rt"}`))
	rec := httptest.NewRecorder()

	h.refresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	select {
	case r := <-calls:
		if r.URL.Path != "/token" || !strings.Contains(r.URL.RawQuery, "grant_type=refresh_token") {
			t.Fatalf("unexpected refresh path/query: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
	default:
		t.Fatalf("expected refresh proxy to be invoked")
	}
	if !strings.Contains(rec.Body.String(), "new-token") {
		t.Fatalf("expected refresh response to be proxied, got %s", rec.Body.String())
	}
}

func TestRequireTenantHeaderEnforced(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestRefreshUsesTimeoutClient(t *testing.T) {
	t.Setenv("SUPABASE_GOTRUE_URL", "")
	called := false
	prev := refreshHTTPClient
	defer func() { refreshHTTPClient = prev }()
	refreshHTTPClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			called = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"x"}`)),
			}, nil
		}),
		Timeout: 100 * time.Millisecond,
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"ok"}`))
	}))
	defer ts.Close()

	h := &handler{supabaseGoTrueURL: ts.URL}
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(`{"refresh_token":"rt"}`))
	rec := httptest.NewRecorder()

	h.refresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !called {
		t.Fatalf("expected custom refresh client to be used")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
