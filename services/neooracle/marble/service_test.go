package neooracle

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	internalhttputil "github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/R3E-Network/service_layer/internal/testutil"
)

func TestAllowlistBlocksURL(t *testing.T) {
	svc := newTestOracle(t, URLAllowlist{Prefixes: []string{"https://allowed.example"}})
	body := `{"url":"https://forbidden.example/data"}`
	req := httptest.NewRequest("POST", "/query", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleQuery(rr, req)
	if rr.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rr.Result().StatusCode)
	}
}

func TestBodyLimitApplied(t *testing.T) {
	// Mock upstream returning large body.
	up := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("A", 1024)))
	}))
	defer up.Close()

	svc := newTestOracle(t, URLAllowlist{Prefixes: []string{up.URL}})
	svc.maxBodyBytes = 10 // very small for test

	body := `{"url":"` + up.URL + `"}`
	req := httptest.NewRequest("POST", "/query", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleQuery(rr, req)
	if rr.Result().StatusCode != http.StatusBadGateway {
		t.Fatalf("status=%d want 502", rr.Result().StatusCode)
	}

	var resp internalhttputil.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Message != "upstream response too large" {
		t.Fatalf("message=%q want %q", resp.Message, "upstream response too large")
	}
}

// newTestOracle returns a service with minimal deps; secrets client won't be used.
func newTestOracle(t *testing.T, allowlist URLAllowlist) *Service {
	t.Helper()
	m, _ := marble.New(marble.Config{MarbleType: "neooracle"})
	svc, err := New(Config{
		Marble:         m,
		SecretsBaseURL: "http://127.0.0.1:0",
		URLAllowlist:   allowlist,
	})
	if err != nil {
		t.Fatalf("New() err = %v", err)
	}
	return svc
}
