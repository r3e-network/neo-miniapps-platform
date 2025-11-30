package secrets

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// mockAccountChecker implements AccountChecker for testing.
type mockAccountChecker struct {
	accounts map[string]bool
}

func newMockAccountChecker(accountIDs ...string) *mockAccountChecker {
	m := &mockAccountChecker{accounts: make(map[string]bool)}
	for _, id := range accountIDs {
		m.accounts[id] = true
	}
	return m
}

func (m *mockAccountChecker) AccountExists(_ context.Context, accountID string) error {
	if !m.accounts[accountID] {
		return fmt.Errorf("account not found: %s", accountID)
	}
	return nil
}

func (m *mockAccountChecker) AccountTenant(_ context.Context, _ string) string {
	return ""
}

type mockStore struct {
	items map[string]Secret
}

func newMockStore() *mockStore { return &mockStore{items: make(map[string]Secret)} }

func newTestService(t *testing.T, accountID string) (*Service, *mockStore) {
	t.Helper()
	if strings.TrimSpace(accountID) == "" {
		accountID = "acct"
	}
	acctChecker := newMockAccountChecker(accountID)
	secretStore := newMockStore()
	svc := New(acctChecker, secretStore, nil)
	return svc, secretStore
}

func (m *mockStore) CreateSecret(_ context.Context, sec Secret) (Secret, error) {
	key := sec.AccountID + "|" + sec.Name
	if _, exists := m.items[key]; exists {
		return Secret{}, fmt.Errorf("secret already exists")
	}
	if sec.Version == 0 {
		sec.Version = 1
	}
	now := time.Now().UTC()
	if sec.CreatedAt.IsZero() {
		sec.CreatedAt = now
	}
	sec.UpdatedAt = now
	m.items[key] = sec
	return sec, nil
}

func (m *mockStore) UpdateSecret(_ context.Context, sec Secret) (Secret, error) {
	key := sec.AccountID + "|" + sec.Name
	existing, ok := m.items[key]
	if !ok {
		return Secret{}, fmt.Errorf("secret not found")
	}
	sec.ID = existing.ID
	if existing.Version == 0 {
		existing.Version = 1
	}
	sec.Version = existing.Version + 1
	sec.CreatedAt = existing.CreatedAt
	sec.UpdatedAt = time.Now().UTC()
	m.items[key] = sec
	return sec, nil
}

func (m *mockStore) GetSecret(_ context.Context, accountID, name string) (Secret, error) {
	sec, ok := m.items[accountID+"|"+name]
	if !ok {
		return Secret{}, fmt.Errorf("secret not found")
	}
	return sec, nil
}

func (m *mockStore) ListSecrets(_ context.Context, accountID string) ([]Secret, error) {
	var out []Secret
	for key, item := range m.items {
		if strings.HasPrefix(key, accountID+"|") {
			out = append(out, item)
		}
	}
	return out, nil
}

func (m *mockStore) DeleteSecret(_ context.Context, accountID, name string) error {
	key := accountID + "|" + name
	if _, ok := m.items[key]; !ok {
		return fmt.Errorf("secret not found")
	}
	delete(m.items, key)
	return nil
}

func TestServiceCreateAndGet(t *testing.T) {
	acctID := "acct1"
	svc, _ := newTestService(t, acctID)

	meta, err := svc.Create(context.Background(), acctID, "apiKey", "secret-value")
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}
	if meta.Name != "apiKey" {
		t.Fatalf("unexpected name: %s", meta.Name)
	}

	record, err := svc.Get(context.Background(), acctID, "apiKey")
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if record.Value != "secret-value" {
		t.Fatalf("expected decrypted value, got %s", record.Value)
	}
}

func TestService_UpdateListDeleteResolve(t *testing.T) {
	acctID := "acct"
	svc, _ := newTestService(t, acctID)

	if _, err := svc.Create(context.Background(), acctID, "token", "value1"); err != nil {
		t.Fatalf("create: %v", err)
	}

	meta, err := svc.Update(context.Background(), acctID, "token", "value2")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if meta.Version != 2 {
		t.Fatalf("expected version 2 after update, got %d", meta.Version)
	}

	list, err := svc.List(context.Background(), acctID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "token" {
		t.Fatalf("unexpected list result: %#v", list)
	}

	resolved, err := svc.ResolveSecrets(context.Background(), acctID, []string{" token "})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved["token"] != "value2" {
		t.Fatalf("expected resolved value2, got %s", resolved["token"])
	}

	if err := svc.Delete(context.Background(), acctID, "token"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Get(context.Background(), acctID, "token"); err == nil {
		t.Fatalf("expected error retrieving deleted secret")
	}
}

func TestService_WithCipherEncryptsValues(t *testing.T) {
	acctID := "acct"
	svc, store := newTestService(t, acctID)
	key := make([]byte, 32)
	cipher, err := NewAESCipher(key)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	svc.SetCipher(cipher)

	if _, err := svc.Create(context.Background(), acctID, "apiKey", "plaintext"); err != nil {
		t.Fatalf("create: %v", err)
	}

	raw := store.items[acctID+"|apiKey"]
	if raw.Value == "plaintext" {
		t.Fatalf("expected stored value to be encrypted")
	}

	retrieved, err := svc.Get(context.Background(), acctID, "apiKey")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if retrieved.Value != "plaintext" {
		t.Fatalf("expected decrypted plaintext, got %s", retrieved.Value)
	}
}

func TestService_CreateValidation(t *testing.T) {
	acctID := "acct"
	svc, _ := newTestService(t, acctID)

	if _, err := svc.Create(context.Background(), acctID, "", "value"); err == nil {
		t.Fatalf("expected validation error for empty name")
	}
	if _, err := svc.Create(context.Background(), acctID, "bad|name", "value"); err == nil {
		t.Fatalf("expected validation error for name with delimiter")
	}
	if _, err := svc.Create(context.Background(), acctID, "name", ""); err == nil {
		t.Fatalf("expected validation error for empty value")
	}
}

func TestService_AccountIDValidation(t *testing.T) {
	acctID := "acct"
	svc, store := newTestService(t, acctID)

	if _, err := svc.Create(context.Background(), "  ", "name", "value"); err == nil {
		t.Fatalf("expected error for blank account id on create")
	}
	if _, err := svc.Update(context.Background(), "\t", "name", "value"); err == nil {
		t.Fatalf("expected error for blank account id on update")
	}
	if _, err := svc.Get(context.Background(), "", "name"); err == nil {
		t.Fatalf("expected error for blank account id on get")
	}
	if _, err := svc.ResolveSecrets(context.Background(), "\n", []string{"name"}); err == nil {
		t.Fatalf("expected error for blank account id on resolve")
	}
	if err := svc.Delete(context.Background(), " ", "name"); err == nil {
		t.Fatalf("expected error for blank account id on delete")
	}
	if _, err := svc.List(context.Background(), ""); err == nil {
		t.Fatalf("expected error for blank account id on list")
	}

	meta, err := svc.Create(context.Background(), " "+acctID+" ", "apiKey", "secret")
	if err != nil {
		t.Fatalf("create with padded account id: %v", err)
	}
	if meta.AccountID != acctID {
		t.Fatalf("expected trimmed account id, got %q", meta.AccountID)
	}
	if _, ok := store.items[acctID+"|apiKey"]; !ok {
		t.Fatalf("expected secret stored with trimmed account id")
	}

	record, err := svc.Get(context.Background(), " "+acctID+" ", "apiKey")
	if err != nil {
		t.Fatalf("get with padded account id: %v", err)
	}
	if record.AccountID != acctID {
		t.Fatalf("expected trimmed account id on retrieval, got %q", record.AccountID)
	}
}

func ExampleService_Create() {
	acctChecker := newMockAccountChecker("acct")
	store := newMockStore()
	svc := New(acctChecker, store, nil)
	meta, _ := svc.Create(context.Background(), "acct", "apiKey", "secret")
	resolved, _ := svc.ResolveSecrets(context.Background(), "acct", []string{"apiKey"})
	fmt.Println(meta.Name, len(resolved["apiKey"]))
	// Output:
	// apiKey 6
}

func TestService_Lifecycle(t *testing.T) {
	svc := New(nil, nil, nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := svc.Ready(context.Background()); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if svc.Ready(context.Background()) == nil {
		t.Fatalf("expected not ready after stop")
	}
}

func TestService_Manifest(t *testing.T) {
	svc := New(nil, nil, nil)
	m := svc.Manifest()
	if m.Name != "secrets" {
		t.Fatalf("expected name secrets")
	}
}

func TestService_Domain(t *testing.T) {
	svc := New(nil, nil, nil)
	if svc.Domain() != "secrets" {
		t.Fatalf("expected domain secrets")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "secrets" {
		t.Fatalf("expected name secrets")
	}
}

func TestService_WithCipher(t *testing.T) {
	key := make([]byte, 32)
	cipher, _ := NewAESCipher(key)
	svc := New(nil, nil, nil, WithCipher(cipher))
	if svc == nil {
		t.Fatalf("expected service")
	}
}

func TestService_UpdateWithOptions(t *testing.T) {
	acctID := "acct"
	svc, _ := newTestService(t, acctID)

	svc.Create(context.Background(), acctID, "apiKey", "original")
	newValue := "updated"
	meta, err := svc.UpdateWithOptions(context.Background(), acctID, "apiKey", UpdateOptions{
		Value: &newValue,
	})
	if err != nil {
		t.Fatalf("update with options: %v", err)
	}
	if meta.Name != "apiKey" {
		t.Fatalf("expected name apiKey")
	}
}

func TestService_CreateWithOptions(t *testing.T) {
	acctID := "acct2"
	svc, _ := newTestService(t, acctID)

	meta, err := svc.CreateWithOptions(context.Background(), acctID, "apiKey", "value", CreateOptions{})
	if err != nil {
		t.Fatalf("create with options: %v", err)
	}
	if meta.Name != "apiKey" {
		t.Fatalf("expected name apiKey")
	}
}

func TestService_ResolveSecretsWithACL(t *testing.T) {
	acctID := "acct"
	svc, _ := newTestService(t, acctID)
	svc.CreateWithOptions(context.Background(), acctID, "mykey", "myvalue", CreateOptions{
		ACL: ACLFunctionAccess,
	})

	resolved, err := svc.ResolveSecretsWithACL(context.Background(), acctID, []string{"mykey"}, CallerFunctions)
	if err != nil {
		t.Fatalf("resolve with acl: %v", err)
	}
	if resolved["mykey"] != "myvalue" {
		t.Fatalf("expected resolved value")
	}
}

func TestService_ResolveSecretsWithACL_AutomationCaller(t *testing.T) {
	acctID := "acct3"
	svc, _ := newTestService(t, acctID)
	svc.CreateWithOptions(context.Background(), acctID, "autokey", "autovalue", CreateOptions{
		ACL: ACLAutomationAccess,
	})

	resolved, err := svc.ResolveSecretsWithACL(context.Background(), acctID, []string{"autokey"}, CallerAutomation)
	if err != nil {
		t.Fatalf("resolve with acl: %v", err)
	}
	if resolved["autokey"] != "autovalue" {
		t.Fatalf("expected resolved value")
	}
}

func TestService_ResolveSecretsWithACL_OracleCaller(t *testing.T) {
	acctID := "acct4"
	svc, _ := newTestService(t, acctID)
	svc.CreateWithOptions(context.Background(), acctID, "oraclekey", "oraclevalue", CreateOptions{
		ACL: ACLOracleAccess,
	})

	resolved, err := svc.ResolveSecretsWithACL(context.Background(), acctID, []string{"oraclekey"}, CallerOracle)
	if err != nil {
		t.Fatalf("resolve with acl: %v", err)
	}
	if resolved["oraclekey"] != "oraclevalue" {
		t.Fatalf("expected resolved value")
	}
}

func TestService_Delete_NotFound(t *testing.T) {
	acctID := "acct"
	svc, _ := newTestService(t, acctID)
	err := svc.Delete(context.Background(), acctID, "nonexistent")
	if err == nil {
		t.Fatalf("expected not found error")
	}
}

func TestService_Get_NotFound(t *testing.T) {
	acctID := "acct"
	svc, _ := newTestService(t, acctID)
	_, err := svc.Get(context.Background(), acctID, "nonexistent")
	if err == nil {
		t.Fatalf("expected not found error")
	}
}

func TestAESCipher_InvalidKey(t *testing.T) {
	// Too short key
	_, err := NewAESCipher([]byte("short"))
	if err == nil {
		t.Fatalf("expected error for short key")
	}
}
