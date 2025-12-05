// Package os provides the ServiceOS abstraction layer.
package os

import (
	"fmt"
)

// defaultLogger provides a simple default logger implementation.
type defaultLogger struct {
	serviceID string
}

func newDefaultLogger(serviceID string) *defaultLogger {
	return &defaultLogger{serviceID: serviceID}
}

func (l *defaultLogger) Debug(msg string, args ...any) {
	fmt.Printf("[DEBUG][%s] %s %v\n", l.serviceID, msg, args)
}

func (l *defaultLogger) Info(msg string, args ...any) {
	fmt.Printf("[INFO][%s] %s %v\n", l.serviceID, msg, args)
}

func (l *defaultLogger) Warn(msg string, args ...any) {
	fmt.Printf("[WARN][%s] %s %v\n", l.serviceID, msg, args)
}

func (l *defaultLogger) Error(msg string, args ...any) {
	fmt.Printf("[ERROR][%s] %s %v\n", l.serviceID, msg, args)
}
