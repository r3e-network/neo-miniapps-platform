// Package sandbox provides audit logging integration with the system logger.
//
// This file extends the SecurityAuditor to integrate with the existing
// logging infrastructure, providing structured audit logs for security events.
package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

// =============================================================================
// Audit Logger Interface
// =============================================================================

// AuditLogger is the interface for audit log output.
type AuditLogger interface {
	// LogAuditEvent logs a security audit event.
	LogAuditEvent(event AuditEvent)

	// LogSecurityAlert logs a high-priority security alert.
	LogSecurityAlert(alert SecurityAlert)
}

// SecurityAlert represents a high-priority security event.
type SecurityAlert struct {
	Timestamp   time.Time         `json:"timestamp"`
	Severity    AlertSeverity     `json:"severity"`
	AlertType   string            `json:"alert_type"`
	ServiceID   string            `json:"service_id"`
	Description string            `json:"description"`
	Details     map[string]string `json:"details,omitempty"`
}

// AlertSeverity indicates the severity of a security alert.
type AlertSeverity string

const (
	AlertSeverityLow      AlertSeverity = "low"
	AlertSeverityMedium   AlertSeverity = "medium"
	AlertSeverityHigh     AlertSeverity = "high"
	AlertSeverityCritical AlertSeverity = "critical"
)

// =============================================================================
// Standard Logger Adapter
// =============================================================================

// StdLoggerAdapter adapts the standard log.Logger to AuditLogger.
type StdLoggerAdapter struct {
	logger *log.Logger
	prefix string
}

// NewStdLoggerAdapter creates a new standard logger adapter.
func NewStdLoggerAdapter(logger *log.Logger, prefix string) *StdLoggerAdapter {
	if logger == nil {
		logger = log.Default()
	}
	return &StdLoggerAdapter{
		logger: logger,
		prefix: prefix,
	}
}

// LogAuditEvent logs an audit event using the standard logger.
func (a *StdLoggerAdapter) LogAuditEvent(event AuditEvent) {
	status := "ALLOWED"
	if !event.Allowed {
		status = "DENIED"
	}

	a.logger.Printf("%s[AUDIT] %s | %s | service=%s action=%s resource=%s",
		a.prefix,
		event.Timestamp.Format(time.RFC3339),
		status,
		event.ServiceID,
		event.Action,
		event.Resource,
	)
}

// LogSecurityAlert logs a security alert using the standard logger.
func (a *StdLoggerAdapter) LogSecurityAlert(alert SecurityAlert) {
	a.logger.Printf("%s[SECURITY ALERT] %s | %s | %s | service=%s | %s",
		a.prefix,
		alert.Timestamp.Format(time.RFC3339),
		alert.Severity,
		alert.AlertType,
		alert.ServiceID,
		alert.Description,
	)
}

// =============================================================================
// JSON Logger Adapter
// =============================================================================

// JSONLoggerAdapter outputs audit events as JSON.
type JSONLoggerAdapter struct {
	mu     sync.Mutex
	writer io.Writer
}

// NewJSONLoggerAdapter creates a new JSON logger adapter.
func NewJSONLoggerAdapter(writer io.Writer) *JSONLoggerAdapter {
	return &JSONLoggerAdapter{
		writer: writer,
	}
}

// LogAuditEvent logs an audit event as JSON.
func (a *JSONLoggerAdapter) LogAuditEvent(event AuditEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = a.writer.Write(append(data, '\n'))
}

// LogSecurityAlert logs a security alert as JSON.
func (a *JSONLoggerAdapter) LogSecurityAlert(alert SecurityAlert) {
	a.mu.Lock()
	defer a.mu.Unlock()

	data, err := json.Marshal(alert)
	if err != nil {
		return
	}
	_, _ = a.writer.Write(append(data, '\n'))
}

// =============================================================================
// Multi Logger (fan-out to multiple loggers)
// =============================================================================

// MultiLogger fans out audit events to multiple loggers.
type MultiLogger struct {
	loggers []AuditLogger
}

// NewMultiLogger creates a new multi-logger.
func NewMultiLogger(loggers ...AuditLogger) *MultiLogger {
	return &MultiLogger{
		loggers: loggers,
	}
}

// LogAuditEvent logs to all configured loggers.
func (m *MultiLogger) LogAuditEvent(event AuditEvent) {
	for _, logger := range m.loggers {
		logger.LogAuditEvent(event)
	}
}

// LogSecurityAlert logs to all configured loggers.
func (m *MultiLogger) LogSecurityAlert(alert SecurityAlert) {
	for _, logger := range m.loggers {
		logger.LogSecurityAlert(alert)
	}
}

// =============================================================================
// Enhanced Security Auditor with Logger Integration
// =============================================================================

// EnhancedAuditor extends SecurityAuditor with external logger integration.
type EnhancedAuditor struct {
	*SecurityAuditor

	mu     sync.RWMutex
	logger AuditLogger

	// Alert thresholds
	denialThreshold    int           // Number of denials before alert
	denialWindow       time.Duration // Time window for denial counting
	denialCounts       map[string][]time.Time
	alertCooldown      time.Duration
	lastAlerts         map[string]time.Time
}

// EnhancedAuditorConfig configures the enhanced auditor.
type EnhancedAuditorConfig struct {
	MaxEvents       int
	Logger          AuditLogger
	DenialThreshold int           // Denials before alert (default: 10)
	DenialWindow    time.Duration // Window for counting (default: 1 minute)
	AlertCooldown   time.Duration // Cooldown between alerts (default: 5 minutes)
}

// DefaultEnhancedAuditorConfig returns sensible defaults.
func DefaultEnhancedAuditorConfig() EnhancedAuditorConfig {
	return EnhancedAuditorConfig{
		MaxEvents:       1000,
		DenialThreshold: 10,
		DenialWindow:    time.Minute,
		AlertCooldown:   5 * time.Minute,
	}
}

// NewEnhancedAuditor creates a new enhanced auditor.
func NewEnhancedAuditor(config EnhancedAuditorConfig) *EnhancedAuditor {
	if config.DenialThreshold <= 0 {
		config.DenialThreshold = 10
	}
	if config.DenialWindow <= 0 {
		config.DenialWindow = time.Minute
	}
	if config.AlertCooldown <= 0 {
		config.AlertCooldown = 5 * time.Minute
	}

	return &EnhancedAuditor{
		SecurityAuditor: NewSecurityAuditor(config.MaxEvents),
		logger:          config.Logger,
		denialThreshold: config.DenialThreshold,
		denialWindow:    config.DenialWindow,
		denialCounts:    make(map[string][]time.Time),
		alertCooldown:   config.AlertCooldown,
		lastAlerts:      make(map[string]time.Time),
	}
}

// SetLogger sets the audit logger.
func (ea *EnhancedAuditor) SetLogger(logger AuditLogger) {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	ea.logger = logger
}

// LogCapabilityCheck logs a capability check with external logging.
func (ea *EnhancedAuditor) LogCapabilityCheck(ctx context.Context, identity *ServiceIdentity, cap Capability, allowed bool) {
	// Call parent implementation
	ea.SecurityAuditor.LogCapabilityCheck(ctx, identity, cap, allowed)

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "capability_check",
		ServiceID: identity.ServiceID,
		Action:    string(cap),
		Allowed:   allowed,
	}

	// Log to external logger
	ea.logEvent(event)

	// Check for denial patterns
	if !allowed {
		ea.trackDenial(identity.ServiceID, "capability", string(cap))
	}
}

// LogResourceAccess logs a resource access with external logging.
func (ea *EnhancedAuditor) LogResourceAccess(ctx context.Context, serviceID, resource, action string, allowed bool) {
	// Call parent implementation
	ea.SecurityAuditor.LogResourceAccess(ctx, serviceID, resource, action, allowed)

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "resource_access",
		ServiceID: serviceID,
		Action:    action,
		Resource:  resource,
		Allowed:   allowed,
	}

	// Log to external logger
	ea.logEvent(event)

	// Check for denial patterns
	if !allowed {
		ea.trackDenial(serviceID, "resource", resource)
	}
}

// LogIPCCall logs an IPC call with external logging.
func (ea *EnhancedAuditor) LogIPCCall(ctx context.Context, callerID, targetID, method string, allowed bool) {
	// Call parent implementation
	ea.SecurityAuditor.LogIPCCall(ctx, callerID, targetID, method, allowed)

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "ipc_call",
		ServiceID: callerID,
		Action:    method,
		Resource:  targetID,
		Allowed:   allowed,
	}

	// Log to external logger
	ea.logEvent(event)

	// Check for denial patterns
	if !allowed {
		ea.trackDenial(callerID, "ipc", targetID+"/"+method)
	}
}

// LogPolicyViolation logs a policy violation.
func (ea *EnhancedAuditor) LogPolicyViolation(ctx context.Context, serviceID, subject, object, action string) {
	_ = ctx

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "policy_violation",
		ServiceID: serviceID,
		Action:    action,
		Resource:  object,
		Allowed:   false,
		Details: map[string]string{
			"subject": subject,
			"object":  object,
		},
	}

	ea.logEvent(event)
	ea.trackDenial(serviceID, "policy", object)
}

// logEvent logs an event to the external logger.
func (ea *EnhancedAuditor) logEvent(event AuditEvent) {
	ea.mu.RLock()
	logger := ea.logger
	ea.mu.RUnlock()

	if logger != nil {
		logger.LogAuditEvent(event)
	}
}

// trackDenial tracks denial events and triggers alerts if threshold exceeded.
func (ea *EnhancedAuditor) trackDenial(serviceID, denialType, target string) {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	key := fmt.Sprintf("%s:%s", serviceID, denialType)
	now := time.Now()
	cutoff := now.Add(-ea.denialWindow)

	// Clean old entries and add new one
	var recent []time.Time
	for _, t := range ea.denialCounts[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	recent = append(recent, now)
	ea.denialCounts[key] = recent

	// Check threshold
	if len(recent) >= ea.denialThreshold {
		ea.maybeAlert(serviceID, denialType, target, len(recent))
	}
}

// maybeAlert sends an alert if cooldown has passed.
func (ea *EnhancedAuditor) maybeAlert(serviceID, denialType, target string, count int) {
	alertKey := fmt.Sprintf("%s:%s", serviceID, denialType)
	now := time.Now()

	if lastAlert, ok := ea.lastAlerts[alertKey]; ok {
		if now.Sub(lastAlert) < ea.alertCooldown {
			return // Still in cooldown
		}
	}

	ea.lastAlerts[alertKey] = now

	if ea.logger != nil {
		ea.logger.LogSecurityAlert(SecurityAlert{
			Timestamp:   now,
			Severity:    AlertSeverityMedium,
			AlertType:   "excessive_denials",
			ServiceID:   serviceID,
			Description: fmt.Sprintf("Service %s has %d %s denials in the last %v", serviceID, count, denialType, ea.denialWindow),
			Details: map[string]string{
				"denial_type":  denialType,
				"target":       target,
				"denial_count": fmt.Sprintf("%d", count),
			},
		})
	}
}

// =============================================================================
// Audit Query Interface
// =============================================================================

// AuditQuery provides methods to query audit events.
type AuditQuery struct {
	auditor *EnhancedAuditor
}

// NewAuditQuery creates a new audit query interface.
func NewAuditQuery(auditor *EnhancedAuditor) *AuditQuery {
	return &AuditQuery{auditor: auditor}
}

// GetRecentEvents returns recent audit events.
func (q *AuditQuery) GetRecentEvents(limit int) []AuditEvent {
	return q.auditor.GetEvents(limit)
}

// GetEventsByService returns events for a specific service.
func (q *AuditQuery) GetEventsByService(serviceID string, limit int) []AuditEvent {
	all := q.auditor.GetEvents(limit * 10) // Get more to filter
	var result []AuditEvent
	for _, event := range all {
		if event.ServiceID == serviceID {
			result = append(result, event)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetDeniedEvents returns only denied events.
func (q *AuditQuery) GetDeniedEvents(limit int) []AuditEvent {
	all := q.auditor.GetEvents(limit * 10)
	var result []AuditEvent
	for _, event := range all {
		if !event.Allowed {
			result = append(result, event)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetEventsByType returns events of a specific type.
func (q *AuditQuery) GetEventsByType(eventType string, limit int) []AuditEvent {
	all := q.auditor.GetEvents(limit * 10)
	var result []AuditEvent
	for _, event := range all {
		if event.EventType == eventType {
			result = append(result, event)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetStatistics returns audit statistics.
func (q *AuditQuery) GetStatistics() AuditStatistics {
	events := q.auditor.GetEvents(1000)

	stats := AuditStatistics{
		TotalEvents:     len(events),
		EventsByType:    make(map[string]int),
		EventsByService: make(map[string]int),
	}

	for _, event := range events {
		stats.EventsByType[event.EventType]++
		stats.EventsByService[event.ServiceID]++
		if event.Allowed {
			stats.AllowedCount++
		} else {
			stats.DeniedCount++
		}
	}

	return stats
}

// AuditStatistics contains audit event statistics.
type AuditStatistics struct {
	TotalEvents     int            `json:"total_events"`
	AllowedCount    int            `json:"allowed_count"`
	DeniedCount     int            `json:"denied_count"`
	EventsByType    map[string]int `json:"events_by_type"`
	EventsByService map[string]int `json:"events_by_service"`
}
