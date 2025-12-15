package logging

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		service string
		level   string
		format  string
	}{
		{"json logger", "test-service", "info", "json"},
		{"text logger", "test-service", "debug", "text"},
		{"invalid level", "test-service", "invalid", "json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(tt.service, tt.level, tt.format)
			if logger == nil {
				t.Fatal("New() returned nil")
			}
			if logger.service != tt.service {
				t.Errorf("service = %v, want %v", logger.service, tt.service)
			}
		})
	}
}

func TestLogger_WithContext(t *testing.T) {
	logger := New("test", "info", "json")
	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-123")
	ctx = WithUserID(ctx, "user-456")

	entry := logger.WithContext(ctx)
	if entry == nil {
		t.Fatal("WithContext() returned nil")
	}

	// Check if fields are set
	if entry.Data["service"] != "test" {
		t.Errorf("service field = %v, want test", entry.Data["service"])
	}
	if entry.Data["trace_id"] != "trace-123" {
		t.Errorf("trace_id field = %v, want trace-123", entry.Data["trace_id"])
	}
	if entry.Data["user_id"] != "user-456" {
		t.Errorf("user_id field = %v, want user-456", entry.Data["user_id"])
	}
}

func TestLogger_WithTraceID(t *testing.T) {
	logger := New("test", "info", "json")
	entry := logger.WithTraceID("trace-123")

	if entry.Data["trace_id"] != "trace-123" {
		t.Errorf("trace_id = %v, want trace-123", entry.Data["trace_id"])
	}
	if entry.Data["service"] != "test" {
		t.Errorf("service = %v, want test", entry.Data["service"])
	}
}

func TestLogger_WithUserID(t *testing.T) {
	logger := New("test", "info", "json")
	entry := logger.WithUserID("user-456")

	if entry.Data["user_id"] != "user-456" {
		t.Errorf("user_id = %v, want user-456", entry.Data["user_id"])
	}
}

func TestLogger_WithFields(t *testing.T) {
	logger := New("test", "info", "json")
	fields := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}

	entry := logger.WithFields(fields)

	if entry.Data["key1"] != "value1" {
		t.Errorf("key1 = %v, want value1", entry.Data["key1"])
	}
	if entry.Data["key2"] != 123 {
		t.Errorf("key2 = %v, want 123", entry.Data["key2"])
	}
	if entry.Data["service"] != "test" {
		t.Errorf("service = %v, want test", entry.Data["service"])
	}
}

func TestLogger_WithError(t *testing.T) {
	logger := New("test", "info", "json")
	err := errors.New("test error")

	entry := logger.WithError(err)

	if entry.Data["error"] != "test error" {
		t.Errorf("error = %v, want test error", entry.Data["error"])
	}
}

func TestLogger_SetOutput(t *testing.T) {
	logger := New("test", "info", "json")
	buf := &bytes.Buffer{}

	logger.SetOutput(buf)
	logger.Logger.Info("test message")

	if buf.Len() == 0 {
		t.Error("SetOutput() did not redirect output")
	}
}

func TestNewTraceID(t *testing.T) {
	id1 := NewTraceID()
	id2 := NewTraceID()

	if id1 == "" {
		t.Error("NewTraceID() returned empty string")
	}
	if id1 == id2 {
		t.Error("NewTraceID() returned duplicate IDs")
	}
}

func TestWithTraceID(t *testing.T) {
	ctx := context.Background()
	traceID := "trace-123"

	ctx = WithTraceID(ctx, traceID)
	got := GetTraceID(ctx)

	if got != traceID {
		t.Errorf("GetTraceID() = %v, want %v", got, traceID)
	}
}

func TestGetTraceID(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{
			name: "with trace ID",
			ctx:  WithTraceID(context.Background(), "trace-123"),
			want: "trace-123",
		},
		{
			name: "without trace ID",
			ctx:  context.Background(),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetTraceID(tt.ctx); got != tt.want {
				t.Errorf("GetTraceID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithUserID(t *testing.T) {
	ctx := context.Background()
	userID := "user-456"

	ctx = WithUserID(ctx, userID)
	got := GetUserID(ctx)

	if got != userID {
		t.Errorf("GetUserID() = %v, want %v", got, userID)
	}
}

func TestGetUserID(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{
			name: "with user ID",
			ctx:  WithUserID(context.Background(), "user-456"),
			want: "user-456",
		},
		{
			name: "without user ID",
			ctx:  context.Background(),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetUserID(tt.ctx); got != tt.want {
				t.Errorf("GetUserID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithService(t *testing.T) {
	ctx := context.Background()
	service := "test-service"

	ctx = WithService(ctx, service)
	got := GetService(ctx)

	if got != service {
		t.Errorf("GetService() = %v, want %v", got, service)
	}
}

func TestGetService(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{
			name: "with service",
			ctx:  WithService(context.Background(), "test-service"),
			want: "test-service",
		},
		{
			name: "without service",
			ctx:  context.Background(),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetService(tt.ctx); got != tt.want {
				t.Errorf("GetService() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogger_LogRequest(t *testing.T) {
	logger := New("test", "info", "json")
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	ctx := WithTraceID(context.Background(), "trace-123")
	logger.LogRequest(ctx, "GET", "/api/test", 200, 100*time.Millisecond)

	if buf.Len() == 0 {
		t.Error("LogRequest() did not write log")
	}
}

func TestLogger_LogDatabaseQuery(t *testing.T) {
	logger := New("test", "debug", "json")
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	ctx := context.Background()

	// Test successful query
	logger.LogDatabaseQuery(ctx, "SELECT * FROM users", 50*time.Millisecond, nil)
	if buf.Len() == 0 {
		t.Error("LogDatabaseQuery() did not write log for success")
	}

	// Test failed query
	buf.Reset()
	logger.LogDatabaseQuery(ctx, "SELECT * FROM users", 50*time.Millisecond, errors.New("connection failed"))
	if buf.Len() == 0 {
		t.Error("LogDatabaseQuery() did not write log for error")
	}
}

func TestLogger_LogBlockchainTx(t *testing.T) {
	logger := New("test", "info", "json")
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	ctx := context.Background()

	// Test successful transaction
	logger.LogBlockchainTx(ctx, "0x123", "transfer", nil)
	if buf.Len() == 0 {
		t.Error("LogBlockchainTx() did not write log for success")
	}

	// Test failed transaction
	buf.Reset()
	logger.LogBlockchainTx(ctx, "0x123", "transfer", errors.New("insufficient gas"))
	if buf.Len() == 0 {
		t.Error("LogBlockchainTx() did not write log for error")
	}
}

func TestLogger_LogSecurityEvent(t *testing.T) {
	logger := New("test", "info", "json")
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	ctx := context.Background()
	details := map[string]interface{}{
		"ip":     "192.168.1.1",
		"action": "login_attempt",
	}

	logger.LogSecurityEvent(ctx, "suspicious_activity", details)

	if buf.Len() == 0 {
		t.Error("LogSecurityEvent() did not write log")
	}
}

func TestLogger_LogAudit(t *testing.T) {
	logger := New("test", "info", "json")
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	ctx := WithUserID(context.Background(), "user-123")
	logger.LogAudit(ctx, "delete", "user", "456", "success")

	if buf.Len() == 0 {
		t.Error("LogAudit() did not write log")
	}
}

func TestLogger_Info(t *testing.T) {
	logger := New("test", "info", "json")
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	ctx := context.Background()
	fields := map[string]interface{}{"key": "value"}

	logger.Info(ctx, "test message", fields)

	if buf.Len() == 0 {
		t.Error("Info() did not write log")
	}
}

func TestLogger_Error(t *testing.T) {
	logger := New("test", "info", "json")
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	ctx := context.Background()
	err := errors.New("test error")
	fields := map[string]interface{}{"key": "value"}

	logger.Error(ctx, "error occurred", err, fields)

	if buf.Len() == 0 {
		t.Error("Error() did not write log")
	}
}

func TestLogger_Warn(t *testing.T) {
	logger := New("test", "info", "json")
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	ctx := context.Background()
	fields := map[string]interface{}{"key": "value"}

	logger.Warn(ctx, "warning message", fields)

	if buf.Len() == 0 {
		t.Error("Warn() did not write log")
	}
}

func TestLogger_Debug(t *testing.T) {
	logger := New("test", "debug", "json")
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	ctx := context.Background()
	fields := map[string]interface{}{"key": "value"}

	logger.Debug(ctx, "debug message", fields)

	if buf.Len() == 0 {
		t.Error("Debug() did not write log")
	}
}

func TestInitDefault(t *testing.T) {
	InitDefault("test-service", "info", "json")

	logger := Default()
	if logger == nil {
		t.Fatal("Default() returned nil after InitDefault()")
	}
	if logger.service != "test-service" {
		t.Errorf("service = %v, want test-service", logger.service)
	}
}

func TestDefault(t *testing.T) {
	// Reset default logger
	defaultLogger = nil

	logger := Default()
	if logger == nil {
		t.Fatal("Default() returned nil")
	}
	if logger.service != "unknown" {
		t.Errorf("service = %v, want unknown", logger.service)
	}
}

func TestInfoDefault(t *testing.T) {
	InitDefault("test", "info", "json")
	buf := &bytes.Buffer{}
	Default().SetOutput(buf)

	ctx := context.Background()
	InfoDefault(ctx, "test message")

	if buf.Len() == 0 {
		t.Error("InfoDefault() did not write log")
	}
}

func TestErrorDefault(t *testing.T) {
	InitDefault("test", "info", "json")
	buf := &bytes.Buffer{}
	Default().SetOutput(buf)

	ctx := context.Background()
	err := errors.New("test error")
	ErrorDefault(ctx, "error message", err)

	if buf.Len() == 0 {
		t.Error("ErrorDefault() did not write log")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"1 millisecond", 1 * time.Millisecond, "1.00ms"},
		{"100 milliseconds", 100 * time.Millisecond, "100.00ms"},
		{"1 second", 1 * time.Second, "1000.00ms"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatDuration(tt.duration); got != tt.want {
				t.Errorf("FormatDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogger_LogLevels(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		logLevel logrus.Level
	}{
		{"debug level", "debug", logrus.DebugLevel},
		{"info level", "info", logrus.InfoLevel},
		{"warn level", "warn", logrus.WarnLevel},
		{"error level", "error", logrus.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New("test", tt.level, "json")
			if logger.Logger.Level != tt.logLevel {
				t.Errorf("Level = %v, want %v", logger.Logger.Level, tt.logLevel)
			}
		})
	}
}

func TestLogger_JSONFormatter(t *testing.T) {
	logger := New("test", "info", "json")
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	logger.Logger.Info("test")

	output := buf.String()
	if output == "" {
		t.Error("JSON formatter did not produce output")
	}
	// JSON output should contain quotes
	if !bytes.Contains(buf.Bytes(), []byte(`"`)) {
		t.Error("Output does not appear to be JSON")
	}
}

func TestLogger_TextFormatter(t *testing.T) {
	logger := New("test", "info", "text")
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	logger.Logger.Info("test")

	if buf.Len() == 0 {
		t.Error("Text formatter did not produce output")
	}
}
