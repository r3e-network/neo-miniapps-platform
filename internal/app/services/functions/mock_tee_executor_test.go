package functions

import (
	"context"
	"errors"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/function"
)

type mockSecretResolver struct {
	allow bool
	calls int
}

func (m *mockSecretResolver) ResolveSecrets(ctx context.Context, accountID string, names []string) (map[string]string, error) {
	m.calls++
	if !m.allow {
		return nil, errors.New("secrets unavailable")
	}
	return map[string]string{}, nil
}

func TestMockTEEExecutorExecutes(t *testing.T) {
	exec := NewMockTEEExecutor()
	def := function.Definition{
		ID:        "fn",
		Name:      "test-fn",
		AccountID: "acct",
	}

	payload := map[string]any{"foo": "bar"}
	result, err := exec.Execute(context.Background(), def, payload)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Output["foo"] != "bar" {
		t.Fatalf("expected payload echoed, got %#v", result.Output)
	}
	if result.Output["message"] != "mock execution (TEE disabled)" {
		t.Fatalf("unexpected message: %#v", result.Output["message"])
	}
}

func TestMockTEEExecutorValidatesSecrets(t *testing.T) {
	exec := NewMockTEEExecutor()
	resolver := &mockSecretResolver{allow: true}
	exec.SetSecretResolver(resolver)

	def := function.Definition{
		ID:        "fn",
		Name:      "secret-fn",
		AccountID: "acct",
		Secrets:   []string{"apiKey"},
	}

	if _, err := exec.Execute(context.Background(), def, nil); err != nil {
		t.Fatalf("expected secrets to resolve, got %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolver to be called once, got %d", resolver.calls)
	}
}

func TestMockTEEExecutorFailsWithoutResolver(t *testing.T) {
	exec := NewMockTEEExecutor()
	def := function.Definition{
		ID:        "fn",
		Name:      "secret-fn",
		AccountID: "acct",
		Secrets:   []string{"apiKey"},
	}

	if _, err := exec.Execute(context.Background(), def, nil); err == nil {
		t.Fatalf("expected error when secrets configured without resolver")
	}
}
