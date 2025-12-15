package neoaccountsmarble

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleRequestAccounts_ProductionRequiresServiceAuthContext(t *testing.T) {
	t.Setenv("MARBLE_ENV", "production")

	svc, _ := newTestServiceWithMock(t)

	body := []byte(`{"service_id":"neovault","count":1}`)
	req := httptest.NewRequest(http.MethodPost, "/request", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleRequestAccounts(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}
