package lifecycle

import (
	"context"
	"errors"
	"testing"
)

func TestHooks_RunOrder(t *testing.T) {
	h := NewHooks()
	var order []string

	h.OnPreStart(func(ctx context.Context) error {
		order = append(order, "preStart")
		return nil
	})
	h.OnPostStart(func(ctx context.Context) error {
		order = append(order, "postStart")
		return nil
	})
	h.OnPreStop(func(ctx context.Context) error {
		order = append(order, "preStop")
		return nil
	})
	h.OnPostStop(func(ctx context.Context) error {
		order = append(order, "postStop")
		return nil
	})

	ctx := context.Background()

	if err := h.RunPreStart(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.RunPostStart(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.RunPreStop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.RunPostStop(ctx); err != nil {
		t.Fatal(err)
	}

	expected := []string{"preStart", "postStart", "preStop", "postStop"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d hooks, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected order[%d]=%q, got %q", i, v, order[i])
		}
	}
}

func TestHooks_PostStopReversesOrder(t *testing.T) {
	h := NewHooks()
	var order []string

	h.OnPostStop(func(ctx context.Context) error {
		order = append(order, "first")
		return nil
	})
	h.OnPostStop(func(ctx context.Context) error {
		order = append(order, "second")
		return nil
	})
	h.OnPostStop(func(ctx context.Context) error {
		order = append(order, "third")
		return nil
	})

	if err := h.RunPostStop(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Should run in LIFO order: third, second, first
	expected := []string{"third", "second", "first"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d hooks, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected order[%d]=%q, got %q", i, v, order[i])
		}
	}
}

func TestHooks_ErrorStopsExecution(t *testing.T) {
	h := NewHooks()
	expectedErr := errors.New("hook error")
	var count int

	h.OnPreStart(func(ctx context.Context) error {
		count++
		return nil
	})
	h.OnPreStart(func(ctx context.Context) error {
		count++
		return expectedErr
	})
	h.OnPreStart(func(ctx context.Context) error {
		count++
		return nil
	})

	err := h.RunPreStart(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error to wrap %v, got %v", expectedErr, err)
	}
	if count != 2 {
		t.Errorf("expected 2 hooks to run before error, got %d", count)
	}
}

func TestHooks_NamedHooks(t *testing.T) {
	h := NewHooks()
	expectedErr := errors.New("named hook error")

	h.OnPreStartNamed("init-db", func(ctx context.Context) error {
		return expectedErr
	})

	err := h.RunPreStart(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	// Error message should include hook name
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error message")
	}
	// The error should mention "init-db"
	if !containsSubstring(errStr, "init-db") {
		t.Errorf("expected error to mention hook name 'init-db', got: %s", errStr)
	}
}

func TestHooks_Counts(t *testing.T) {
	h := NewHooks()

	h.OnPreStart(func(ctx context.Context) error { return nil })
	h.OnPreStart(func(ctx context.Context) error { return nil })
	h.OnPostStart(func(ctx context.Context) error { return nil })
	h.OnPreStop(func(ctx context.Context) error { return nil })
	h.OnPreStop(func(ctx context.Context) error { return nil })
	h.OnPreStop(func(ctx context.Context) error { return nil })

	counts := h.Counts()
	if counts.PreStart != 2 {
		t.Errorf("expected PreStart=2, got %d", counts.PreStart)
	}
	if counts.PostStart != 1 {
		t.Errorf("expected PostStart=1, got %d", counts.PostStart)
	}
	if counts.PreStop != 3 {
		t.Errorf("expected PreStop=3, got %d", counts.PreStop)
	}
	if counts.PostStop != 0 {
		t.Errorf("expected PostStop=0, got %d", counts.PostStop)
	}
}

func TestHooks_Clear(t *testing.T) {
	h := NewHooks()

	h.OnPreStart(func(ctx context.Context) error { return nil })
	h.OnPostStart(func(ctx context.Context) error { return nil })
	h.OnPreStop(func(ctx context.Context) error { return nil })
	h.OnPostStop(func(ctx context.Context) error { return nil })

	h.Clear()

	counts := h.Counts()
	if counts.PreStart != 0 || counts.PostStart != 0 || counts.PreStop != 0 || counts.PostStop != 0 {
		t.Error("expected all counts to be 0 after clear")
	}
}

func TestHooks_NilFunction(t *testing.T) {
	h := NewHooks()

	// Should not panic
	h.OnPreStart(nil)
	h.OnPostStart(nil)
	h.OnPreStop(nil)
	h.OnPostStop(nil)

	// Should run without issues
	if err := h.RunPreStart(context.Background()); err != nil {
		t.Error(err)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
