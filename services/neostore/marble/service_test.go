package neostoremarble

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/R3E-Network/service_layer/internal/marble"
	neostoresupabase "github.com/R3E-Network/service_layer/services/neostore/supabase"
)

// mockRepo only satisfies the methods we call; no persistence needed for auth tests.
type mockRepo struct {
	secrets  []neostoresupabase.Secret
	policies map[string][]string
}

func (m *mockRepo) GetSecrets(_ context.Context, _ string) ([]neostoresupabase.Secret, error) {
	return m.secrets, nil
}
func (m *mockRepo) CreateSecret(_ context.Context, _ *neostoresupabase.Secret) error { return nil }
func (m *mockRepo) GetSecretByName(_ context.Context, _ string, _ string) (*neostoresupabase.Secret, error) {
	return nil, nil
}
func (m *mockRepo) UpdateSecret(_ context.Context, _ *neostoresupabase.Secret) error { return nil }
func (m *mockRepo) DeleteSecret(_ context.Context, _ string, _ string) error         { return nil }
func (m *mockRepo) GetSecretPolicies(_ context.Context, _ string, name string) ([]string, error) {
	if m.policies == nil {
		return nil, nil
	}
	return m.policies[name], nil
}
func (m *mockRepo) SetSecretPolicies(_ context.Context, _ string, name string, services []string) error {
	if m.policies == nil {
		m.policies = map[string][]string{}
	}
	m.policies[name] = services
	return nil
}
func (m *mockRepo) GetAllowedServices(_ context.Context, _ string, _ string) ([]string, error) {
	return nil, nil
}
func (m *mockRepo) SetAllowedServices(_ context.Context, _ string, _ string, _ []string) error {
	return nil
}
func (m *mockRepo) CreateAuditLog(_ context.Context, _ *neostoresupabase.AuditLog) error {
	return nil
}
func (m *mockRepo) GetAuditLogs(_ context.Context, _ string, _ int) ([]neostoresupabase.AuditLog, error) {
	return nil, nil
}
func (m *mockRepo) GetAuditLogsForSecret(_ context.Context, _ string, _ string, _ int) ([]neostoresupabase.AuditLog, error) {
	return nil, nil
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	key := make([]byte, 32)
	m, _ := marble.New(marble.Config{MarbleType: "neostore"})
	svc, err := New(Config{Marble: m, DB: &mockRepo{}, EncryptKey: key})
	if err != nil {
		t.Fatalf("New() err = %v", err)
	}
	return svc
}

func TestAuthorizeServiceCaller_AllowsListed(t *testing.T) {
	svc := newTestService(t)
	req := httptest.NewRequest("GET", "/secrets", nil)
	req.Header.Set(ServiceIDHeader, "neooracle")
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

// =============================================================================
// Additional Handler Tests
// =============================================================================

func TestHandleListSecrets_WithUserID(t *testing.T) {
	svc := newTestService(t)
	req := httptest.NewRequest("GET", "/secrets", nil)
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleListSecrets(rr, req)
	if rr.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Result().StatusCode)
	}
}

func TestHandleCreateSecret_RequiresUserID(t *testing.T) {
	svc := newTestService(t)
	body := strings.NewReader(`{"name":"api","value":"abc"}`)
	req := httptest.NewRequest("POST", "/secrets", body)
	rr := httptest.NewRecorder()
	svc.handleCreateSecret(rr, req)
	if rr.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Result().StatusCode)
	}
}

func TestHandleCreateSecret_InvalidJSON(t *testing.T) {
	svc := newTestService(t)
	body := strings.NewReader(`{invalid json}`)
	req := httptest.NewRequest("POST", "/secrets", body)
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleCreateSecret(rr, req)
	if rr.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Result().StatusCode)
	}
}

func TestHandleCreateSecret_MissingName(t *testing.T) {
	svc := newTestService(t)
	body := strings.NewReader(`{"value":"abc"}`)
	req := httptest.NewRequest("POST", "/secrets", body)
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleCreateSecret(rr, req)
	if rr.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Result().StatusCode)
	}
}

func TestHandleCreateSecret_MissingValue(t *testing.T) {
	svc := newTestService(t)
	body := strings.NewReader(`{"name":"api"}`)
	req := httptest.NewRequest("POST", "/secrets", body)
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleCreateSecret(rr, req)
	if rr.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Result().StatusCode)
	}
}

func TestHandleCreateSecret_Success(t *testing.T) {
	svc := newTestService(t)
	body := strings.NewReader(`{"name":"api_key","value":"secret123"}`)
	req := httptest.NewRequest("POST", "/secrets", body)
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleCreateSecret(rr, req)
	if rr.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rr.Result().StatusCode)
	}
}

func TestHandleCreateSecret_ServiceCallerDenied(t *testing.T) {
	svc := newTestService(t)
	body := strings.NewReader(`{"name":"api_key","value":"secret123"}`)
	req := httptest.NewRequest("POST", "/secrets", body)
	req.Header.Set("X-User-ID", "user1")
	req.Header.Set(ServiceIDHeader, "neooracle") // oracle is in AllowedServices
	rr := httptest.NewRecorder()
	svc.handleCreateSecret(rr, req)
	if rr.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Result().StatusCode)
	}
}

// =============================================================================
// Service Constants Tests
// =============================================================================

func TestServiceConstants(t *testing.T) {
	if ServiceID != "neostore" {
		t.Errorf("ServiceID = %s, want neostore", ServiceID)
	}
	if ServiceName != "NeoStore Service" {
		t.Errorf("ServiceName = %s, want NeoStore Service", ServiceName)
	}
	if Version != "1.0.0" {
		t.Errorf("Version = %s, want 1.0.0", Version)
	}
}

func TestServiceIDHeader(t *testing.T) {
	if ServiceIDHeader != "X-Service-ID" {
		t.Errorf("ServiceIDHeader = %s, want X-Service-ID", ServiceIDHeader)
	}
}

// Note: AllowedServices test removed - constant is internal to authorizeServiceCaller

// =============================================================================
// Service Creation Tests
// =============================================================================

func TestNew(t *testing.T) {
	key := make([]byte, 32)
	m, _ := marble.New(marble.Config{MarbleType: "neostore"})
	svc, err := New(Config{Marble: m, DB: &mockRepo{}, EncryptKey: key})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if svc.ID() != ServiceID {
		t.Errorf("ID() = %s, want %s", svc.ID(), ServiceID)
	}
	if svc.Name() != ServiceName {
		t.Errorf("Name() = %s, want %s", svc.Name(), ServiceName)
	}
	if svc.Version() != Version {
		t.Errorf("Version() = %s, want %s", svc.Version(), Version)
	}
}

func TestNew_WithNilDB(t *testing.T) {
	key := make([]byte, 32)
	m, _ := marble.New(marble.Config{MarbleType: "neostore"})
	svc, err := New(Config{Marble: m, DB: nil, EncryptKey: key})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if svc == nil {
		t.Error("service should not be nil")
	}
}

// =============================================================================
// Health Endpoint Test
// =============================================================================

func TestHandleHealthEndpoint(t *testing.T) {
	svc := newTestService(t)
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	svc.Router().ServeHTTP(rr, req)
	if rr.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Result().StatusCode)
	}
}

// =============================================================================
// Mock Repository Tests
// =============================================================================

func TestMockRepo_GetSecrets(t *testing.T) {
	repo := &mockRepo{
		secrets: []neostoresupabase.Secret{
			{Name: "secret1", EncryptedValue: []byte("enc1")},
			{Name: "secret2", EncryptedValue: []byte("enc2")},
		},
	}
	secrets, err := repo.GetSecrets(context.Background(), "user1")
	if err != nil {
		t.Fatalf("GetSecrets() error = %v", err)
	}
	if len(secrets) != 2 {
		t.Errorf("len(secrets) = %d, want 2", len(secrets))
	}
}

func TestMockRepo_CreateSecret(t *testing.T) {
	repo := &mockRepo{}
	err := repo.CreateSecret(context.Background(), &neostoresupabase.Secret{
		Name:           "test",
		EncryptedValue: []byte("encrypted"),
	})
	if err != nil {
		t.Fatalf("CreateSecret() error = %v", err)
	}
}

func TestMockRepo_GetSetPolicies(t *testing.T) {
	repo := &mockRepo{}

	// Initially nil
	policies, err := repo.GetSecretPolicies(context.Background(), "user1", "secret1")
	if err != nil {
		t.Fatalf("GetSecretPolicies() error = %v", err)
	}
	if policies != nil {
		t.Errorf("policies should be nil initially")
	}

	// Set policies
	err = repo.SetSecretPolicies(context.Background(), "user1", "secret1", []string{"neooracle", "neorand"})
	if err != nil {
		t.Fatalf("SetSecretPolicies() error = %v", err)
	}

	// Get policies
	policies, err = repo.GetSecretPolicies(context.Background(), "user1", "secret1")
	if err != nil {
		t.Fatalf("GetSecretPolicies() error = %v", err)
	}
	if len(policies) != 2 {
		t.Errorf("len(policies) = %d, want 2", len(policies))
	}
}
