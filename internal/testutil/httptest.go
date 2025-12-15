package testutil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// NewHTTPTestServer creates an httptest.Server and skips the test if the sandbox
// blocks opening a local listener (common in restricted CI environments).
func NewHTTPTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprint(r)
			if strings.Contains(msg, "operation not permitted") || strings.Contains(msg, "permission denied") {
				t.Skipf("skipping HTTP server test due to sandbox restrictions: %v", r)
			}
			panic(r)
		}
	}()
	return httptest.NewServer(handler)
}
