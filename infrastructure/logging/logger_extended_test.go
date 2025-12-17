package logging

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewFromEnv(t *testing.T) {
	// Save and restore environment
	savedLevel := os.Getenv("LOG_LEVEL")
	savedFormat := os.Getenv("LOG_FORMAT")
	defer func() {
		if savedLevel != "" {
			os.Setenv("LOG_LEVEL", savedLevel)
		} else {
			os.Unsetenv("LOG_LEVEL")
		}
		if savedFormat != "" {
			os.Setenv("LOG_FORMAT", savedFormat)
		} else {
			os.Unsetenv("LOG_FORMAT")
		}
	}()

	t.Run("defaults when env not set", func(t *testing.T) {
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("LOG_FORMAT")

		logger := NewFromEnv("test-service")
		if logger == nil {
			t.Fatal("NewFromEnv() returned nil")
		}
	})

	t.Run("custom level and format", func(t *testing.T) {
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("LOG_FORMAT", "text")

		logger := NewFromEnv("test-service")
		if logger == nil {
			t.Fatal("NewFromEnv() returned nil")
		}
	})

	t.Run("whitespace trimmed", func(t *testing.T) {
		os.Setenv("LOG_LEVEL", "  warn  ")
		os.Setenv("LOG_FORMAT", "  json  ")

		logger := NewFromEnv("test-service")
		if logger == nil {
			t.Fatal("NewFromEnv() returned nil")
		}
	})
}

func TestWithRoleAndGetRole(t *testing.T) {
	ctx := context.Background()

	t.Run("set and get role", func(t *testing.T) {
		ctx = WithRole(ctx, "admin")
		role := GetRole(ctx)
		if role != "admin" {
			t.Errorf("GetRole() = %s, want admin", role)
		}
	})

	t.Run("empty context returns empty string", func(t *testing.T) {
		emptyCtx := context.Background()
		role := GetRole(emptyCtx)
		if role != "" {
			t.Errorf("GetRole() = %s, want empty", role)
		}
	})
}

func TestLogCryptoOperation(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test-service", "debug", "json")
	logger.SetOutput(&buf)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		buf.Reset()
		logger.LogCryptoOperation(ctx, "encrypt", true, nil)
		output := buf.String()
		if !strings.Contains(output, "encrypt") {
			t.Error("output should contain operation name")
		}
	})

	t.Run("failure", func(t *testing.T) {
		buf.Reset()
		logger.LogCryptoOperation(ctx, "decrypt", false, errors.New("key error"))
		output := buf.String()
		if !strings.Contains(output, "key error") {
			t.Error("output should contain error message")
		}
	})
}

func TestLogServiceCall(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test-service", "debug", "json")
	logger.SetOutput(&buf)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		buf.Reset()
		logger.LogServiceCall(ctx, "auth-service", "validate", 100*time.Millisecond, nil)
		output := buf.String()
		if !strings.Contains(output, "auth-service") {
			t.Error("output should contain target service")
		}
	})

	t.Run("failure", func(t *testing.T) {
		buf.Reset()
		logger.LogServiceCall(ctx, "auth-service", "validate", 100*time.Millisecond, errors.New("timeout"))
		output := buf.String()
		if !strings.Contains(output, "timeout") {
			t.Error("output should contain error message")
		}
	})
}

func TestLogPerformance(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test-service", "info", "json")
	logger.SetOutput(&buf)

	ctx := context.Background()

	logger.LogPerformance(ctx, "database_query", map[string]interface{}{
		"duration_ms": 50,
		"rows":        100,
	})

	output := buf.String()
	if !strings.Contains(output, "database_query") {
		t.Error("output should contain operation name")
	}
	if !strings.Contains(output, "performance") {
		t.Error("output should contain performance type")
	}
}

func TestLogErrorWithStack(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test-service", "error", "json")
	logger.SetOutput(&buf)

	ctx := context.Background()
	err := errors.New("test error")

	logger.LogErrorWithStack(ctx, err, "operation failed", map[string]interface{}{
		"key": "value",
	})

	output := buf.String()
	if !strings.Contains(output, "test error") {
		t.Error("output should contain error message")
	}
	if !strings.Contains(output, "operation failed") {
		t.Error("output should contain message")
	}
}

func TestLogErrorWithStackNilFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test-service", "error", "json")
	logger.SetOutput(&buf)

	ctx := context.Background()
	err := errors.New("test error")

	// Should not panic with nil fields
	logger.LogErrorWithStack(ctx, err, "operation failed", nil)

	output := buf.String()
	if !strings.Contains(output, "test error") {
		t.Error("output should contain error message")
	}
}

func TestWarnDefault(t *testing.T) {
	// WarnDefault uses the default logger
	// Just verify it doesn't panic
	ctx := context.Background()
	WarnDefault(ctx, "test warning message")
}

func TestDebugDefault(t *testing.T) {
	// DebugDefault uses the default logger
	// Just verify it doesn't panic
	ctx := context.Background()
	DebugDefault(ctx, "test debug message")
}

func TestLoggerWithContextRole(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test-service", "info", "json")
	logger.SetOutput(&buf)

	ctx := context.Background()
	ctx = WithRole(ctx, "admin")
	ctx = WithTraceID(ctx, "trace-123")
	ctx = WithUserID(ctx, "user-456")

	logger.WithContext(ctx).Info("test message")

	output := buf.String()
	if !strings.Contains(output, "admin") {
		t.Error("output should contain role")
	}
	if !strings.Contains(output, "trace-123") {
		t.Error("output should contain trace ID")
	}
	if !strings.Contains(output, "user-456") {
		t.Error("output should contain user ID")
	}
}

func TestWithFieldsNil(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test-service", "info", "json")
	logger.SetOutput(&buf)

	// Should not panic with nil fields
	entry := logger.WithFields(nil)
	entry.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test-service") {
		t.Error("output should contain service name")
	}
}
