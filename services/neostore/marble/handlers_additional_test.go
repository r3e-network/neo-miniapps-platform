package neostoremarble

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/marble"
	neostoresupabase "github.com/R3E-Network/service_layer/services/neostore/supabase"
)

type memoryStore struct {
	mu sync.Mutex

	secrets map[string]map[string]*neostoresupabase.Secret // user -> name -> secret
	allowed map[string]map[string][]string                 // user -> name -> allowed services

	auditLogs       map[string][]neostoresupabase.AuditLog            // user -> logs
	auditLogsByName map[string]map[string][]neostoresupabase.AuditLog // user -> name -> logs

	nextErr map[string]error

	auditSignal chan struct{}
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		secrets:         make(map[string]map[string]*neostoresupabase.Secret),
		allowed:         make(map[string]map[string][]string),
		auditLogs:       make(map[string][]neostoresupabase.AuditLog),
		auditLogsByName: make(map[string]map[string][]neostoresupabase.AuditLog),
		nextErr:         make(map[string]error),
	}
}

func (s *memoryStore) setErr(op string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nextErr == nil {
		s.nextErr = make(map[string]error)
	}
	s.nextErr[op] = err
}

func (s *memoryStore) takeErr(op string) error {
	if s.nextErr == nil {
		return nil
	}
	if err, ok := s.nextErr[op]; ok {
		delete(s.nextErr, op)
		return err
	}
	return nil
}

func (s *memoryStore) GetSecrets(_ context.Context, userID string) ([]neostoresupabase.Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.takeErr("GetSecrets"); err != nil {
		return nil, err
	}
	var out []neostoresupabase.Secret
	for _, rec := range s.secrets[userID] {
		out = append(out, *rec)
	}
	return out, nil
}

func (s *memoryStore) GetSecretByName(_ context.Context, userID, name string) (*neostoresupabase.Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.takeErr("GetSecretByName"); err != nil {
		return nil, err
	}
	rec := s.secrets[userID][name]
	if rec == nil {
		return nil, nil
	}
	copy := *rec
	return &copy, nil
}

func (s *memoryStore) CreateSecret(_ context.Context, secret *neostoresupabase.Secret) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.takeErr("CreateSecret"); err != nil {
		return err
	}
	if s.secrets[secret.UserID] == nil {
		s.secrets[secret.UserID] = make(map[string]*neostoresupabase.Secret)
	}
	copy := *secret
	s.secrets[secret.UserID][secret.Name] = &copy
	return nil
}

func (s *memoryStore) UpdateSecret(_ context.Context, secret *neostoresupabase.Secret) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.takeErr("UpdateSecret"); err != nil {
		return err
	}
	if s.secrets[secret.UserID] == nil {
		s.secrets[secret.UserID] = make(map[string]*neostoresupabase.Secret)
	}
	copy := *secret
	s.secrets[secret.UserID][secret.Name] = &copy
	return nil
}

func (s *memoryStore) DeleteSecret(_ context.Context, userID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.takeErr("DeleteSecret"); err != nil {
		return err
	}
	if s.secrets[userID] != nil {
		delete(s.secrets[userID], name)
	}
	return nil
}

func (s *memoryStore) GetAllowedServices(_ context.Context, userID, secretName string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.takeErr("GetAllowedServices"); err != nil {
		return nil, err
	}
	if s.allowed[userID] == nil {
		return nil, nil
	}
	out := s.allowed[userID][secretName]
	if out == nil {
		return nil, nil
	}
	cpy := make([]string, len(out))
	copy(cpy, out)
	return cpy, nil
}

func (s *memoryStore) SetAllowedServices(_ context.Context, userID, secretName string, services []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.takeErr("SetAllowedServices"); err != nil {
		return err
	}
	if s.allowed[userID] == nil {
		s.allowed[userID] = make(map[string][]string)
	}
	if services == nil {
		delete(s.allowed[userID], secretName)
		return nil
	}
	cpy := make([]string, len(services))
	copy(cpy, services)
	s.allowed[userID][secretName] = cpy
	return nil
}

func (s *memoryStore) CreateAuditLog(_ context.Context, logEntry *neostoresupabase.AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.auditSignal != nil {
		select {
		case s.auditSignal <- struct{}{}:
		default:
		}
	}
	if err := s.takeErr("CreateAuditLog"); err != nil {
		return err
	}
	s.auditLogs[logEntry.UserID] = append(s.auditLogs[logEntry.UserID], *logEntry)
	if s.auditLogsByName[logEntry.UserID] == nil {
		s.auditLogsByName[logEntry.UserID] = make(map[string][]neostoresupabase.AuditLog)
	}
	s.auditLogsByName[logEntry.UserID][logEntry.SecretName] = append(s.auditLogsByName[logEntry.UserID][logEntry.SecretName], *logEntry)
	return nil
}

func (s *memoryStore) GetAuditLogs(_ context.Context, userID string, limit int) ([]neostoresupabase.AuditLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.takeErr("GetAuditLogs"); err != nil {
		return nil, err
	}
	logs := s.auditLogs[userID]
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}
	out := make([]neostoresupabase.AuditLog, len(logs))
	copy(out, logs)
	return out, nil
}

func (s *memoryStore) GetAuditLogsForSecret(_ context.Context, userID, secretName string, limit int) ([]neostoresupabase.AuditLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.takeErr("GetAuditLogsForSecret"); err != nil {
		return nil, err
	}
	logs := s.auditLogsByName[userID][secretName]
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}
	out := make([]neostoresupabase.AuditLog, len(logs))
	copy(out, logs)
	return out, nil
}

func newServiceWithStore(t *testing.T, store Store) *Service {
	t.Helper()
	key := bytes.Repeat([]byte{0x01}, 32)
	m, err := marble.New(marble.Config{MarbleType: "neostore"})
	if err != nil {
		t.Fatalf("marble.New: %v", err)
	}
	svc, err := New(Config{Marble: m, DB: store, EncryptKey: key})
	if err != nil {
		t.Fatalf("New() err = %v", err)
	}
	return svc
}

func TestNew_Requires32ByteKey(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neostore"})
	_, err := New(Config{Marble: m, DB: newMemoryStore(), EncryptKey: []byte("short")})
	if err == nil {
		t.Fatalf("expected error for short key")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	svc := newServiceWithStore(t, newMemoryStore())
	cipher, err := svc.encrypt([]byte("plain"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	plain, err := svc.decrypt(cipher)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(plain) != "plain" {
		t.Fatalf("decrypt = %q, want plain", string(plain))
	}
}

func TestLifecycle_StartStop(t *testing.T) {
	svc := newServiceWithStore(t, newMemoryStore())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start() err = %v", err)
	}
	if err := svc.Stop(); err != nil {
		t.Fatalf("Stop() err = %v", err)
	}
}

func TestHandleGetSecret_SuccessAsGateway(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)

	// Seed secret.
	cipher, err := svc.encrypt([]byte("value-1"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	now := time.Now()
	_ = store.CreateSecret(context.Background(), &neostoresupabase.Secret{
		ID:             "id1",
		UserID:         "user1",
		Name:           "api",
		EncryptedValue: cipher,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	})

	req := httptest.NewRequest(http.MethodGet, "/secrets/api", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "api"})
	req.Header.Set("X-User-ID", "user1")

	rr := httptest.NewRecorder()
	svc.handleGetSecret(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp GetSecretResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Value != "value-1" {
		t.Fatalf("value = %q, want value-1", resp.Value)
	}
}

func TestHandleGetSecret_AllowedServiceCanReadWhenPermitted(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)

	cipher, _ := svc.encrypt([]byte("value-2"))
	now := time.Now()
	_ = store.CreateSecret(context.Background(), &neostoresupabase.Secret{
		ID:             "id2",
		UserID:         "user1",
		Name:           "db",
		EncryptedValue: cipher,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	_ = store.SetAllowedServices(context.Background(), "user1", "db", []string{"neooracle"})

	req := httptest.NewRequest(http.MethodGet, "/secrets/db", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "db"})
	req.Header.Set("X-User-ID", "user1")
	req.Header.Set("X-Service-ID", "neooracle")

	rr := httptest.NewRecorder()
	svc.handleGetSecret(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestHandleCreateSecret_UpdatesExistingSecret(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)

	cipher, _ := svc.encrypt([]byte("old"))
	now := time.Now()
	_ = store.CreateSecret(context.Background(), &neostoresupabase.Secret{
		ID:             "id3",
		UserID:         "user1",
		Name:           "api",
		EncryptedValue: cipher,
		Version:        2,
		CreatedAt:      now.Add(-time.Hour),
		UpdatedAt:      now.Add(-time.Hour),
	})

	body := bytes.NewReader([]byte(`{"name":"api","value":"new"}`))
	req := httptest.NewRequest(http.MethodPost, "/secrets", body)
	req.Header.Set("X-User-ID", "user1")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	svc.handleCreateSecret(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	updated, err := store.GetSecretByName(context.Background(), "user1", "api")
	if err != nil {
		t.Fatalf("GetSecretByName: %v", err)
	}
	if updated == nil || updated.Version != 3 {
		t.Fatalf("expected version to increment to 3")
	}
}

func TestHandleGetSecretPermissions_AndSetPermissions(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)
	_ = store.SetAllowedServices(context.Background(), "user1", "api", []string{"neooracle"})

	req := httptest.NewRequest(http.MethodGet, "/secrets/api/permissions", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "api"})
	req.Header.Set("X-User-ID", "user1")

	rr := httptest.NewRecorder()
	svc.handleGetSecretPermissions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	body := bytes.NewReader([]byte(`{"services":["neorand"]}`))
	req = httptest.NewRequest(http.MethodPut, "/secrets/api/permissions", body)
	req = mux.SetURLVars(req, map[string]string{"name": "api"})
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleSetSecretPermissions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	allowed, _ := store.GetAllowedServices(context.Background(), "user1", "api")
	if len(allowed) != 1 || allowed[0] != "neorand" {
		t.Fatalf("allowed services = %v, want [neorand]", allowed)
	}
}

func TestHandleDeleteSecret_Success(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)
	cipher, _ := svc.encrypt([]byte("value"))
	now := time.Now()
	_ = store.CreateSecret(context.Background(), &neostoresupabase.Secret{
		ID:             "id4",
		UserID:         "user1",
		Name:           "tmp",
		EncryptedValue: cipher,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	})

	req := httptest.NewRequest(http.MethodDelete, "/secrets/tmp", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "tmp"})
	req.Header.Set("X-User-ID", "user1")

	rr := httptest.NewRecorder()
	svc.handleDeleteSecret(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	secret, _ := store.GetSecretByName(context.Background(), "user1", "tmp")
	if secret != nil {
		t.Fatalf("secret should be deleted")
	}
}

func TestHandleAuditLogEndpoints(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)

	// Seed a log entry.
	_ = store.CreateAuditLog(context.Background(), &neostoresupabase.AuditLog{
		ID:         "log1",
		UserID:     "user1",
		SecretName: "api",
		Action:     "read",
		CreatedAt:  time.Now(),
	})

	req := httptest.NewRequest(http.MethodGet, "/audit?limit=1", nil)
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleGetAuditLogs(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/secrets/api/audit?limit=1", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "api"})
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleGetSecretAuditLogs(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestLogAudit_NonBlockingAndCapturesIP(t *testing.T) {
	store := newMemoryStore()
	store.setErr("CreateAuditLog", fmt.Errorf("boom"))
	store.auditSignal = make(chan struct{}, 1)
	svc := newServiceWithStore(t, store)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:1234"

	svc.logAudit(context.Background(), "user1", "api", "read", "gateway", true, "", req)
	select {
	case <-store.auditSignal:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected audit log goroutine to finish")
	}
}

func TestGetClientIP_HeaderPrecedence(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	if got := getClientIP(req); got != "1.2.3.4" {
		t.Fatalf("getClientIP(xff) = %q, want 1.2.3.4", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	req.Header.Set("X-Real-IP", "9.8.7.6")
	if got := getClientIP(req); got != "9.8.7.6" {
		t.Fatalf("getClientIP(xri) = %q, want 9.8.7.6", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	if got := getClientIP(req); got != "10.0.0.1" {
		t.Fatalf("getClientIP(remote) = %q, want 10.0.0.1", got)
	}
}

func TestHandleListSecrets_DBUnavailable(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 32)
	m, _ := marble.New(marble.Config{MarbleType: "neostore"})
	svc, err := New(Config{Marble: m, DB: nil, EncryptKey: key})
	if err != nil {
		t.Fatalf("New() err = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/secrets", nil)
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleListSecrets(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestHandleListSecrets_DBError(t *testing.T) {
	store := newMemoryStore()
	store.setErr("GetSecrets", fmt.Errorf("boom"))
	svc := newServiceWithStore(t, store)

	req := httptest.NewRequest(http.MethodGet, "/secrets", nil)
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleListSecrets(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestHandleListSecrets_RejectsUnknownService(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)

	req := httptest.NewRequest(http.MethodGet, "/secrets", nil)
	req.Header.Set("X-User-ID", "user1")
	req.Header.Set("X-Service-ID", "unknown")
	rr := httptest.NewRecorder()
	svc.handleListSecrets(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestHandleCreateSecret_DBErrorPaths(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)

	body := bytes.NewReader([]byte(`{"name":"api","value":"x"}`))
	req := httptest.NewRequest(http.MethodPost, "/secrets", body)
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()

	store.setErr("GetSecretByName", fmt.Errorf("boom"))
	svc.handleCreateSecret(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}

	// Create failure.
	body = bytes.NewReader([]byte(`{"name":"api","value":"x"}`))
	req = httptest.NewRequest(http.MethodPost, "/secrets", body)
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	store.setErr("CreateSecret", fmt.Errorf("boom"))
	svc.handleCreateSecret(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}

	// Update failure.
	cipher, _ := svc.encrypt([]byte("old"))
	now := time.Now()
	_ = store.CreateSecret(context.Background(), &neostoresupabase.Secret{
		ID:             "id5",
		UserID:         "user1",
		Name:           "api",
		EncryptedValue: cipher,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	body = bytes.NewReader([]byte(`{"name":"api","value":"x"}`))
	req = httptest.NewRequest(http.MethodPost, "/secrets", body)
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	store.setErr("UpdateSecret", fmt.Errorf("boom"))
	svc.handleCreateSecret(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestHandleGetSecret_ErrorPaths(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)

	// Missing name.
	req := httptest.NewRequest(http.MethodGet, "/secrets/", nil)
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleGetSecret(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}

	// Secret not found.
	cipher, _ := svc.encrypt([]byte("value"))
	now := time.Now()
	_ = store.CreateSecret(context.Background(), &neostoresupabase.Secret{
		ID:             "id6",
		UserID:         "user1",
		Name:           "present",
		EncryptedValue: cipher,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	req = httptest.NewRequest(http.MethodGet, "/secrets/missing", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "missing"})
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleGetSecret(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}

	// DB GetSecretByName error.
	store.setErr("GetSecretByName", fmt.Errorf("boom"))
	req = httptest.NewRequest(http.MethodGet, "/secrets/present", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "present"})
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleGetSecret(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestHandleGetSecret_PermissionCheckError(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)

	cipher, _ := svc.encrypt([]byte("value"))
	now := time.Now()
	_ = store.CreateSecret(context.Background(), &neostoresupabase.Secret{
		ID:             "id7",
		UserID:         "user1",
		Name:           "db",
		EncryptedValue: cipher,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	})

	store.setErr("GetAllowedServices", fmt.Errorf("boom"))
	req := httptest.NewRequest(http.MethodGet, "/secrets/db", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "db"})
	req.Header.Set("X-User-ID", "user1")
	req.Header.Set("X-Service-ID", "neooracle")
	rr := httptest.NewRecorder()
	svc.handleGetSecret(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestHandleGetSecret_DecryptionFailure(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)

	now := time.Now()
	_ = store.CreateSecret(context.Background(), &neostoresupabase.Secret{
		ID:             "id8",
		UserID:         "user1",
		Name:           "broken",
		EncryptedValue: []byte("not-ciphertext"),
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	})

	req := httptest.NewRequest(http.MethodGet, "/secrets/broken", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "broken"})
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleGetSecret(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestHandleSecretPermissions_ErrorPaths(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)

	// Non-production direct user call without service id is allowed.
	req := httptest.NewRequest(http.MethodGet, "/secrets/api/permissions", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "api"})
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleGetSecretPermissions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	// Unauthorized when not gateway.
	req = httptest.NewRequest(http.MethodGet, "/secrets/api/permissions", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "api"})
	req.Header.Set("X-User-ID", "user1")
	req.Header.Set("X-Service-ID", "neooracle")
	rr = httptest.NewRecorder()
	svc.handleGetSecretPermissions(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}

	store.setErr("GetAllowedServices", fmt.Errorf("boom"))
	req = httptest.NewRequest(http.MethodGet, "/secrets/api/permissions", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "api"})
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleGetSecretPermissions(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}

	// handleSetSecretPermissions bad JSON.
	req = httptest.NewRequest(http.MethodPut, "/secrets/api/permissions", bytes.NewReader([]byte("{bad")))
	req = mux.SetURLVars(req, map[string]string{"name": "api"})
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleSetSecretPermissions(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}

	// handleSetSecretPermissions SetAllowedServices error.
	store.setErr("SetAllowedServices", fmt.Errorf("boom"))
	req = httptest.NewRequest(http.MethodPut, "/secrets/api/permissions", bytes.NewReader([]byte(`{"services":["neorand"]}`)))
	req = mux.SetURLVars(req, map[string]string{"name": "api"})
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleSetSecretPermissions(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestHandleDeleteSecret_ErrorPaths(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)

	// Missing name.
	req := httptest.NewRequest(http.MethodDelete, "/secrets/", nil)
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleDeleteSecret(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}

	// Secret not found.
	req = httptest.NewRequest(http.MethodDelete, "/secrets/missing", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "missing"})
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleDeleteSecret(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}

	// DB error during lookup.
	store.setErr("GetSecretByName", fmt.Errorf("boom"))
	req = httptest.NewRequest(http.MethodDelete, "/secrets/missing", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "missing"})
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleDeleteSecret(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}

	// Delete error.
	cipher, _ := svc.encrypt([]byte("value"))
	now := time.Now()
	_ = store.CreateSecret(context.Background(), &neostoresupabase.Secret{
		ID:             "id9",
		UserID:         "user1",
		Name:           "tmp",
		EncryptedValue: cipher,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	store.setErr("DeleteSecret", fmt.Errorf("boom"))
	req = httptest.NewRequest(http.MethodDelete, "/secrets/tmp", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "tmp"})
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleDeleteSecret(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestHandleAuditLog_ErrorPaths(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)

	// Unauthorized when not gateway.
	req := httptest.NewRequest(http.MethodGet, "/audit", nil)
	req.Header.Set("X-User-ID", "user1")
	req.Header.Set("X-Service-ID", "neooracle")
	rr := httptest.NewRecorder()
	svc.handleGetAuditLogs(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}

	store.setErr("GetAuditLogs", fmt.Errorf("boom"))
	req = httptest.NewRequest(http.MethodGet, "/audit", nil)
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleGetAuditLogs(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}

	// Secret audit missing name.
	req = httptest.NewRequest(http.MethodGet, "/secrets//audit", nil)
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleGetSecretAuditLogs(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}

	store.setErr("GetAuditLogsForSecret", fmt.Errorf("boom"))
	req = httptest.NewRequest(http.MethodGet, "/secrets/api/audit", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "api"})
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleGetSecretAuditLogs(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestNew_LoadsEncryptKeyFromMarbleSecret(t *testing.T) {
	key := bytes.Repeat([]byte{0x02}, 32)
	m, _ := marble.New(marble.Config{MarbleType: "neostore"})
	m.SetTestSecret(SecretKeyEnv, key)

	svc, err := New(Config{Marble: m, DB: newMemoryStore()})
	if err != nil {
		t.Fatalf("New() err = %v", err)
	}
	if svc == nil {
		t.Fatalf("service should not be nil")
	}
}

func TestHandlePermissionHandlers_BasicValidation(t *testing.T) {
	store := newMemoryStore()
	svc := newServiceWithStore(t, store)

	// Missing user id.
	req := httptest.NewRequest(http.MethodGet, "/secrets/api/permissions", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "api"})
	rr := httptest.NewRecorder()
	svc.handleGetSecretPermissions(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}

	// Missing name.
	req = httptest.NewRequest(http.MethodGet, "/secrets//permissions", nil)
	req.Header.Set("X-User-ID", "user1")
	rr = httptest.NewRecorder()
	svc.handleGetSecretPermissions(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}

	// Set permissions without gateway should be rejected.
	req = httptest.NewRequest(http.MethodPut, "/secrets/api/permissions", bytes.NewReader([]byte(`{"services":[]}`)))
	req = mux.SetURLVars(req, map[string]string{"name": "api"})
	req.Header.Set("X-User-ID", "user1")
	req.Header.Set("X-Service-ID", "neooracle")
	rr = httptest.NewRecorder()
	svc.handleSetSecretPermissions(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestHandlers_DBUnavailable(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 32)
	m, _ := marble.New(marble.Config{MarbleType: "neostore"})
	svc, err := New(Config{Marble: m, DB: nil, EncryptKey: key})
	if err != nil {
		t.Fatalf("New() err = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/secrets", bytes.NewReader([]byte(`{"name":"a","value":"b"}`)))
	req.Header.Set("X-User-ID", "user1")
	rr := httptest.NewRecorder()
	svc.handleCreateSecret(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/secrets/a", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "a"})
	req.Header.Set("X-User-ID", "user1")
	req.Header.Set("X-Service-ID", "gateway")
	rr = httptest.NewRecorder()
	svc.handleGetSecret(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/audit", nil)
	req.Header.Set("X-User-ID", "user1")
	req.Header.Set("X-Service-ID", "gateway")
	rr = httptest.NewRecorder()
	svc.handleGetAuditLogs(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestIsServiceAllowedForSecret_DBUnavailable(t *testing.T) {
	svc := newServiceWithStore(t, newMemoryStore())
	svc.db = nil
	if _, err := svc.isServiceAllowedForSecret(context.Background(), "user1", "api", "neooracle"); err == nil {
		t.Fatalf("expected error when db is nil")
	}
}

func TestLogAudit_NoDBDoesNothing(t *testing.T) {
	svc := newServiceWithStore(t, newMemoryStore())
	svc.db = nil
	svc.logAudit(context.Background(), "user1", "api", "read", "gateway", true, "", nil)
}

func TestGetClientIP_AdditionalBranches(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := getClientIP(req); got != "1.2.3.4" {
		t.Fatalf("getClientIP(single xff) = %q, want 1.2.3.4", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := getClientIP(req); got != "203.0.113.10" {
		t.Fatalf("getClientIP(untrusted xff) = %q, want 203.0.113.10", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.9"
	if got := getClientIP(req); got != "10.0.0.9" {
		t.Fatalf("getClientIP(remote no port) = %q, want 10.0.0.9", got)
	}
}
