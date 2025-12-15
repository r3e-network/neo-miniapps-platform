// Package logging provides structured logging with trace ID support
package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ContextKey is the type for context keys
type ContextKey string

const (
	// TraceIDKey is the context key for trace ID
	TraceIDKey ContextKey = "trace_id"
	// UserIDKey is the context key for user ID
	UserIDKey ContextKey = "user_id"
	// RoleKey is the context key for user role
	RoleKey ContextKey = "role"
	// ServiceKey is the context key for service name
	ServiceKey ContextKey = "service"
)

// Logger wraps logrus.Logger with additional functionality
type Logger struct {
	*logrus.Logger
	service string
}

// New creates a new Logger instance
func New(service, level, format string) *Logger {
	logger := logrus.New()

	// Set log level
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)

	// Set formatter
	if format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: time.RFC3339,
			FullTimestamp:   true,
		})
	}

	logger.SetOutput(os.Stdout)

	return &Logger{
		Logger:  logger,
		service: service,
	}
}

// NewFromEnv constructs a logger using LOG_LEVEL and LOG_FORMAT environment variables.
// Defaults to "info" and "json" when unset.
func NewFromEnv(service string) *Logger {
	level := strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	if level == "" {
		level = "info"
	}
	format := strings.TrimSpace(os.Getenv("LOG_FORMAT"))
	if format == "" {
		format = "json"
	}
	return New(service, level, format)
}

// WithContext creates a new logger entry with context values
func (l *Logger) WithContext(ctx context.Context) *logrus.Entry {
	entry := l.Logger.WithField("service", l.service)

	// Add trace ID if present
	if traceID := ctx.Value(TraceIDKey); traceID != nil {
		entry = entry.WithField("trace_id", traceID)
	}

	// Add user ID if present
	if userID := ctx.Value(UserIDKey); userID != nil {
		entry = entry.WithField("user_id", userID)
	}

	// Add role if present
	if role := ctx.Value(RoleKey); role != nil {
		entry = entry.WithField("role", role)
	}

	return entry
}

// WithTraceID creates a new logger entry with trace ID
func (l *Logger) WithTraceID(traceID string) *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields{
		"service":  l.service,
		"trace_id": traceID,
	})
}

// WithUserID creates a new logger entry with user ID
func (l *Logger) WithUserID(userID string) *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields{
		"service": l.service,
		"user_id": userID,
	})
}

// WithFields creates a new logger entry with custom fields
func (l *Logger) WithFields(fields map[string]interface{}) *logrus.Entry {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["service"] = l.service
	return l.Logger.WithFields(fields)
}

// WithError creates a new logger entry with error
func (l *Logger) WithError(err error) *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields{
		"service": l.service,
		"error":   err.Error(),
	})
}

// SetOutput sets the logger output
func (l *Logger) SetOutput(output io.Writer) {
	l.Logger.SetOutput(output)
}

// Context helper functions

// NewTraceID generates a new trace ID
func NewTraceID() string {
	return uuid.New().String()
}

// WithTraceID adds a trace ID to the context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// GetTraceID retrieves the trace ID from context
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// WithUserID adds a user ID to the context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// GetUserID retrieves the user ID from context
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// WithRole adds a user role to the context
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, RoleKey, role)
}

// GetRole retrieves the user role from context
func GetRole(ctx context.Context) string {
	if role, ok := ctx.Value(RoleKey).(string); ok {
		return role
	}
	return ""
}

// WithService adds a service name to the context
func WithService(ctx context.Context, service string) context.Context {
	return context.WithValue(ctx, ServiceKey, service)
}

// GetService retrieves the service name from context
func GetService(ctx context.Context) string {
	if serviceName, ok := ctx.Value(ServiceKey).(string); ok {
		return serviceName
	}
	return ""
}

// Structured logging helpers

// LogRequest logs an HTTP request
func (l *Logger) LogRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration) {
	l.WithContext(ctx).WithFields(logrus.Fields{
		"method":      method,
		"path":        path,
		"status_code": statusCode,
		"duration_ms": duration.Milliseconds(),
	}).Info("HTTP request")
}

// LogDatabaseQuery logs a database query
func (l *Logger) LogDatabaseQuery(ctx context.Context, query string, duration time.Duration, err error) {
	entry := l.WithContext(ctx).WithFields(logrus.Fields{
		"query":       query,
		"duration_ms": duration.Milliseconds(),
	})

	if err != nil {
		entry.WithError(err).Error("Database query failed")
	} else {
		entry.Debug("Database query executed")
	}
}

// LogBlockchainTx logs a blockchain transaction
func (l *Logger) LogBlockchainTx(ctx context.Context, txHash, operation string, err error) {
	entry := l.WithContext(ctx).WithFields(logrus.Fields{
		"tx_hash":   txHash,
		"operation": operation,
	})

	if err != nil {
		entry.WithError(err).Error("Blockchain transaction failed")
	} else {
		entry.Info("Blockchain transaction successful")
	}
}

// LogCryptoOperation logs a cryptographic operation
func (l *Logger) LogCryptoOperation(ctx context.Context, operation string, success bool, err error) {
	entry := l.WithContext(ctx).WithFields(logrus.Fields{
		"operation": operation,
		"success":   success,
	})

	if err != nil {
		entry.WithError(err).Error("Cryptographic operation failed")
	} else {
		entry.Debug("Cryptographic operation completed")
	}
}

// LogServiceCall logs a service-to-service call
func (l *Logger) LogServiceCall(ctx context.Context, targetService, method string, duration time.Duration, err error) {
	entry := l.WithContext(ctx).WithFields(logrus.Fields{
		"target_service": targetService,
		"method":         method,
		"duration_ms":    duration.Milliseconds(),
	})

	if err != nil {
		entry.WithError(err).Error("Service call failed")
	} else {
		entry.Info("Service call successful")
	}
}

// LogSecurityEvent logs a security-related event
func (l *Logger) LogSecurityEvent(ctx context.Context, eventType string, details map[string]interface{}) {
	fields := logrus.Fields{
		"event_type": eventType,
		"severity":   "security",
	}
	for k, v := range details {
		fields[k] = v
	}

	l.WithContext(ctx).WithFields(fields).Warn("Security event")
}

// LogAudit logs an audit event
func (l *Logger) LogAudit(ctx context.Context, action, resource, resourceID, result string) {
	l.WithContext(ctx).WithFields(logrus.Fields{
		"action":      action,
		"resource":    resource,
		"resource_id": resourceID,
		"result":      result,
		"audit":       true,
	}).Info("Audit log")
}

// Performance logging

// LogPerformance logs performance metrics
func (l *Logger) LogPerformance(ctx context.Context, operation string, metrics map[string]interface{}) {
	fields := logrus.Fields{
		"operation": operation,
		"type":      "performance",
	}
	for k, v := range metrics {
		fields[k] = v
	}

	l.WithContext(ctx).WithFields(fields).Info("Performance metrics")
}

// Error logging with stack trace

// LogErrorWithStack logs an error with additional context
func (l *Logger) LogErrorWithStack(ctx context.Context, err error, message string, fields map[string]interface{}) {
	logFields := logrus.Fields{
		"error": err.Error(),
	}
	for k, v := range fields {
		logFields[k] = v
	}

	l.WithContext(ctx).WithFields(logFields).Error(message)
}

// Fatal logs a fatal error and exits
func (l *Logger) Fatal(ctx context.Context, message string, err error) {
	l.WithContext(ctx).WithError(err).Fatal(message)
}

// Panic logs a panic and panics
func (l *Logger) Panic(ctx context.Context, message string, err error) {
	l.WithContext(ctx).WithError(err).Panic(message)
}

// Development helpers

// Debug logs a debug message (only in development)
func (l *Logger) Debug(ctx context.Context, message string, fields map[string]interface{}) {
	if l.Logger.Level >= logrus.DebugLevel {
		l.WithContext(ctx).WithFields(fields).Debug(message)
	}
}

// Info logs an info message
func (l *Logger) Info(ctx context.Context, message string, fields map[string]interface{}) {
	l.WithContext(ctx).WithFields(fields).Info(message)
}

// Warn logs a warning message
func (l *Logger) Warn(ctx context.Context, message string, fields map[string]interface{}) {
	l.WithContext(ctx).WithFields(fields).Warn(message)
}

// Error logs an error message
func (l *Logger) Error(ctx context.Context, message string, err error, fields map[string]interface{}) {
	entry := l.WithContext(ctx)
	if err != nil {
		entry = entry.WithError(err)
	}
	entry.WithFields(fields).Error(message)
}

// Global logger instance (can be initialized once at startup)
var defaultLogger *Logger

// InitDefault initializes the default logger
func InitDefault(service, level, format string) {
	defaultLogger = New(service, level, format)
}

// Default returns the default logger
func Default() *Logger {
	if defaultLogger == nil {
		// Fallback to a basic logger if not initialized
		defaultLogger = New("unknown", "info", "json")
	}
	return defaultLogger
}

// Convenience functions using default logger

// InfoDefault logs an info message using the default logger
func InfoDefault(ctx context.Context, message string) {
	Default().WithContext(ctx).Info(message)
}

// ErrorDefault logs an error message using the default logger
func ErrorDefault(ctx context.Context, message string, err error) {
	Default().WithContext(ctx).WithError(err).Error(message)
}

// WarnDefault logs a warning message using the default logger
func WarnDefault(ctx context.Context, message string) {
	Default().WithContext(ctx).Warn(message)
}

// DebugDefault logs a debug message using the default logger
func DebugDefault(ctx context.Context, message string) {
	Default().WithContext(ctx).Debug(message)
}

// Helper to format duration in milliseconds
func FormatDuration(d time.Duration) string {
	return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1e6)
}
