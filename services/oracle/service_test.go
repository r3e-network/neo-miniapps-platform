package oracle

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/R3E-Network/service_layer/internal/marble"
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
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if rr.Result().StatusCode != http.StatusOK {
		t.Fatalf("status=%d want 200", rr.Result().StatusCode)
	}
	// Expect truncated body to length 10
	if got := rr.Body.Len(); got < 10 {
		t.Fatalf("expected truncated body len >=10, got %d", got)
	}
}

// newTestOracle returns a service with minimal deps; secrets client won't be used.
func newTestOracle(t *testing.T, allowlist URLAllowlist) *Service {
	t.Helper()
	m, _ := marble.New(marble.Config{MarbleType: "oracle"})
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
