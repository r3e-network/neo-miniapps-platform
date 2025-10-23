package secrets

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/secret"
)

type mockStore struct {
	items map[string]secret.Secret
}

func newMockStore() *mockStore { return &mockStore{items: make(map[string]secret.Secret)} }

func (m *mockStore) CreateSecret(_ context.Context, sec secret.Secret) (secret.Secret, error) {
	key := sec.AccountID + "|" + sec.Name
	if _, exists := m.items[key]; exists {
		return secret.Secret{}, fmt.Errorf("secret already exists")
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

func (m *mockStore) UpdateSecret(_ context.Context, sec secret.Secret) (secret.Secret, error) {
	key := sec.AccountID + "|" + sec.Name
	existing, ok := m.items[key]
	if !ok {
		return secret.Secret{}, fmt.Errorf("secret not found")
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

func (m *mockStore) GetSecret(_ context.Context, accountID, name string) (secret.Secret, error) {
	sec, ok := m.items[accountID+"|"+name]
	if !ok {
		return secret.Secret{}, fmt.Errorf("secret not found")
	}
	return sec, nil
}

func (m *mockStore) ListSecrets(_ context.Context, accountID string) ([]secret.Secret, error) {
	var out []secret.Secret
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
	store := newMockStore()
	svc := New(store, nil)

	meta, err := svc.Create(context.Background(), "acct1", "apiKey", "secret-value")
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}
	if meta.Name != "apiKey" {
		t.Fatalf("unexpected name: %s", meta.Name)
	}

	record, err := svc.Get(context.Background(), "acct1", "apiKey")
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if record.Value != "secret-value" {
		t.Fatalf("expected decrypted value, got %s", record.Value)
	}
}

func TestService_UpdateListDeleteResolve(t *testing.T) {
	store := newMockStore()
	svc := New(store, nil)

	if _, err := svc.Create(context.Background(), "acct", "token", "value1"); err != nil {
		t.Fatalf("create: %v", err)
	}

	meta, err := svc.Update(context.Background(), "acct", "token", "value2")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if meta.Version != 2 {
		t.Fatalf("expected version 2 after update, got %d", meta.Version)
	}

	list, err := svc.List(context.Background(), "acct")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "token" {
		t.Fatalf("unexpected list result: %#v", list)
	}

	resolved, err := svc.ResolveSecrets(context.Background(), "acct", []string{" token "})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved["token"] != "value2" {
		t.Fatalf("expected resolved value2, got %s", resolved["token"])
	}

	if err := svc.Delete(context.Background(), "acct", "token"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Get(context.Background(), "acct", "token"); err == nil {
		t.Fatalf("expected error retrieving deleted secret")
	}
}

func TestService_WithCipherEncryptsValues(t *testing.T) {
	store := newMockStore()
	key := make([]byte, 32)
	cipher, err := NewAESCipher(key)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	svc := New(store, nil, WithCipher(cipher))

	if _, err := svc.Create(context.Background(), "acct", "apiKey", "plaintext"); err != nil {
		t.Fatalf("create: %v", err)
	}

	raw := store.items["acct|apiKey"]
	if raw.Value == "plaintext" {
		t.Fatalf("expected stored value to be encrypted")
	}

	retrieved, err := svc.Get(context.Background(), "acct", "apiKey")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if retrieved.Value != "plaintext" {
		t.Fatalf("expected decrypted plaintext, got %s", retrieved.Value)
	}
}

func TestService_CreateValidation(t *testing.T) {
	store := newMockStore()
	svc := New(store, nil)

	if _, err := svc.Create(context.Background(), "acct", "", "value"); err == nil {
		t.Fatalf("expected validation error for empty name")
	}
	if _, err := svc.Create(context.Background(), "acct", "bad|name", "value"); err == nil {
		t.Fatalf("expected validation error for name with delimiter")
	}
	if _, err := svc.Create(context.Background(), "acct", "name", ""); err == nil {
		t.Fatalf("expected validation error for empty value")
	}
}

func ExampleService_Create() {
	store := newMockStore()
	svc := New(store, nil)
	meta, _ := svc.Create(context.Background(), "acct", "apiKey", "secret")
	resolved, _ := svc.ResolveSecrets(context.Background(), "acct", []string{"apiKey"})
	fmt.Println(meta.Name, len(resolved["apiKey"]))
	// Output:
	// apiKey 6
}
