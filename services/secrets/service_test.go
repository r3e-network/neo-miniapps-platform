package secrets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
)

// mockRepo only satisfies the methods we call; no persistence needed for auth tests.
type mockRepo struct {
	secrets []database.Secret
}

func (m *mockRepo) GetSecrets(_ context.Context, _ string) ([]database.Secret, error) {
	return m.secrets, nil
}
func (m *mockRepo) CreateSecret(_ context.Context, _ *database.Secret) error { return nil }

func newTestService(t *testing.T) *Service {
	t.Helper()
	key := make([]byte, 32)
	m, _ := marble.New(marble.Config{MarbleType: "secrets"})
	svc, err := New(Config{Marble: m, DB: &mockRepo{}, EncryptKey: key})
	if err != nil {
		t.Fatalf("New() err = %v", err)
	}
	return svc
}

func TestAuthorizeServiceCaller_AllowsListed(t *testing.T) {
	svc := newTestService(t)
	req := httptest.NewRequest("GET", "/secrets", nil)
	req.Header.Set(ServiceIDHeader, "oracle")
	rr := httptest.NewRecorder()
	if !svc.authorizeServiceCaller(rr, req) {
		t.Fatalf("expected allowed service")
	}
}

func TestAuthorizeServiceCaller_BlocksUnlisted(t *testing.T) {
	svc := newTestService(t)
	req := httptest.NewRequest("GET", "/secrets", nil)
	req.Header.Set(ServiceIDHeader, "unknown")
	rr := httptest.NewRecorder()
	if svc.authorizeServiceCaller(rr, req) {
		t.Fatalf("expected block for unknown service")
	}
	if rr.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Result().StatusCode)
	}
}

func TestAuthorizeServiceCaller_UserCallWithoutServiceID(t *testing.T) {
	svc := newTestService(t)
	req := httptest.NewRequest("GET", "/secrets", nil)
	rr := httptest.NewRecorder()
	if !svc.authorizeServiceCaller(rr, req) {
		t.Fatalf("user call without service id should be allowed")
	}
}

func TestHandleListSecrets_RequiresUserID(t *testing.T) {
	svc := newTestService(t)
	req := httptest.NewRequest("GET", "/secrets", nil)
	rr := httptest.NewRecorder()
	svc.handleListSecrets(rr, req)
	if rr.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Result().StatusCode)
	}
}

func TestHandleCreateSecret_BlocksUnknownService(t *testing.T) {
	svc := newTestService(t)
	body := strings.NewReader(`{"name":"api","value":"abc"}`)
	req := httptest.NewRequest("POST", "/secrets", body)
	req.Header.Set("X-User-ID", "user1")
	req.Header.Set(ServiceIDHeader, "bad-service")
	rr := httptest.NewRecorder()
	svc.handleCreateSecret(rr, req)
	if rr.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Result().StatusCode)
	}
}
