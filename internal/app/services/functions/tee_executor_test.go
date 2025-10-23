package functions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/function"
)

type teeSecretResolver struct {
	values map[string]string
	calls  []string
}

func (r *teeSecretResolver) ResolveSecrets(ctx context.Context, accountID string, names []string) (map[string]string, error) {
	r.calls = append(r.calls, names...)
	out := make(map[string]string, len(names))
	for _, name := range names {
		value, ok := r.values[name]
		if !ok {
			return nil, errors.New("secret not found")
		}
		out[name] = value
	}
	return out, nil
}

func TestTEEExecutorExecutesJavaScript(t *testing.T) {
	resolver := &teeSecretResolver{values: map[string]string{"apiKey": "secret"}}
	exec := NewTEEExecutor(resolver)

	def := function.Definition{
		ID:        "fn-1",
		AccountID: "acct-1",
		Source:    `() => { console.log("running"); return {foo: params.foo, secret: secrets.apiKey}; }`,
		Secrets:   []string{"apiKey"},
	}

	payload := map[string]any{"foo": "bar"}
	result, err := exec.Execute(context.Background(), def, payload)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Status != function.ExecutionStatusSucceeded {
		t.Fatalf("unexpected status: %s", result.Status)
	}
	if result.Output["foo"] != "bar" {
		t.Fatalf("expected payload echo, got %#v", result.Output)
	}
	if result.Output["secret"] != "secret" {
		t.Fatalf("expected secret injection, got %#v", result.Output["secret"])
	}
	if len(result.Logs) == 0 || result.Logs[0] != "running" {
		t.Fatalf("expected console logs captured, got %#v", result.Logs)
	}
	if logs, ok := result.Output["logs"].([]string); !ok || len(logs) == 0 || logs[0] != "running" {
		t.Fatalf("expected logs propagated into output, got %#v", result.Output["logs"])
	}
	if len(resolver.calls) != 1 || resolver.calls[0] != "apiKey" {
		t.Fatalf("resolver not invoked with expected secret names: %#v", resolver.calls)
	}
}

func TestTEEExecutorMissingResolver(t *testing.T) {
	exec := NewTEEExecutor(nil)
	def := function.Definition{
		ID:        "fn-1",
		AccountID: "acct",
		Source:    `() => ({ ok: true })`,
		Secrets:   []string{"apiKey"},
	}
	if _, err := exec.Execute(context.Background(), def, nil); err == nil {
		t.Fatalf("expected error when secrets required and resolver missing")
	}
}

func TestTEEExecutorSetSecretResolver(t *testing.T) {
	exec := NewTEEExecutor(nil)
	resolver := &teeSecretResolver{values: map[string]string{"token": "value"}}
	exec.SetSecretResolver(resolver)

	def := function.Definition{
		ID:        "fn-1",
		AccountID: "acct",
		Source:    `() => secrets.token`,
		Secrets:   []string{"token"},
	}
	result, err := exec.Execute(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Output["result"] != "value" {
		t.Fatalf("expected return value wrapper, got %#v", result.Output)
	}
}

func TestTEEExecutorHandlesJavaScriptError(t *testing.T) {
	resolver := &teeSecretResolver{}
	exec := NewTEEExecutor(resolver)
	def := function.Definition{
		ID:        "fn-1",
		AccountID: "acct",
		Source:    `() => { throw new Error("boom"); }`,
	}
	if _, err := exec.Execute(context.Background(), def, nil); err == nil {
		t.Fatalf("expected error when JS throws")
	}
}

func TestTEEExecutorHandlesAsyncFunction(t *testing.T) {
	exec := NewTEEExecutor(nil)
	def := function.Definition{
		ID:        "fn-async",
		AccountID: "acct",
		Source: `async (params) => {
			console.log("start");
			const value = await Promise.resolve(params.foo);
			console.log("finish");
			return {foo: value};
		}`,
	}
	payload := map[string]any{"foo": "bar"}

	result, err := exec.Execute(context.Background(), def, payload)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Output["foo"] != "bar" {
		t.Fatalf("expected awaited value, got %#v", result.Output["foo"])
	}
	if len(result.Logs) != 2 || result.Logs[0] != "start" || result.Logs[1] != "finish" {
		t.Fatalf("expected async logs, got %#v", result.Logs)
	}
}

func TestTEEExecutorPropagatesPromiseRejection(t *testing.T) {
	exec := NewTEEExecutor(nil)
	def := function.Definition{
		ID:        "fn-reject",
		AccountID: "acct",
		Source: `async () => {
			await Promise.resolve();
			throw new Error("boom");
		}`,
	}

	if _, err := exec.Execute(context.Background(), def, nil); err == nil {
		t.Fatalf("expected rejection error")
	}
}

func TestTEEExecutorHonorsContextCancellation(t *testing.T) {
	exec := NewTEEExecutor(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	def := function.Definition{
		ID:        "fn-timeout",
		AccountID: "acct",
		Source: `() => {
			for (;;) {}
		}`,
	}

	_, err := exec.Execute(ctx, def, nil)
	if err == nil {
		t.Fatalf("expected cancellation error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation error, got %v", err)
	}
}

func TestTEEExecutorDevpackActions(t *testing.T) {
	exec := NewTEEExecutor(nil)
	def := function.Definition{
		ID:        "fn-devpack",
		AccountID: "acct",
		Source: `() => {
			Devpack.gasBank.ensureAccount({ wallet: "wallet" });
			return { ok: true };
		}`,
	}

	result, err := exec.Execute(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(result.Actions))
	}
	if result.Actions[0].Type != function.ActionTypeGasBankEnsureAccount {
		t.Fatalf("action type mismatch: %s", result.Actions[0].Type)
	}
}
