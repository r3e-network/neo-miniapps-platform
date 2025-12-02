package service

import (
	"context"
	"time"
)

// HealthStatus represents the health state of a service.
type HealthStatus string

const (
	// HealthStatusHealthy indicates the service is operating normally.
	HealthStatusHealthy HealthStatus = "healthy"

	// HealthStatusDegraded indicates the service is running but with reduced capacity.
	HealthStatusDegraded HealthStatus = "degraded"

	// HealthStatusUnhealthy indicates the service is not operational.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck provides detailed health information for a service.
type HealthCheck struct {
	Status    HealthStatus      `json:"status"`
	Service   string            `json:"service"`
	Timestamp time.Time         `json:"timestamp"`
	Latency   time.Duration     `json:"latency_ms"`
	Details   map[string]string `json:"details,omitempty"`
	Checks    []ComponentCheck  `json:"checks,omitempty"`
}

// ComponentCheck represents health of a specific component/dependency.
type ComponentCheck struct {
	Name    string        `json:"name"`
	Status  HealthStatus  `json:"status"`
	Latency time.Duration `json:"latency_ms,omitempty"`
	Message string        `json:"message,omitempty"`
}

// HealthChecker is implemented by services that provide health checks.
type HealthChecker interface {
	// HealthCheck performs a deep health check including dependencies.
	HealthCheck(ctx context.Context) HealthCheck
}

// SimpleHealthChecker is implemented by services with basic liveness checks.
type SimpleHealthChecker interface {
	// Ping performs a quick liveness check.
	Ping(ctx context.Context) error
}

// NewHealthCheck creates a healthy HealthCheck for a service.
func NewHealthCheck(service string) HealthCheck {
	return HealthCheck{
		Status:    HealthStatusHealthy,
		Service:   service,
		Timestamp: time.Now().UTC(),
		Details:   make(map[string]string),
		Checks:    make([]ComponentCheck, 0),
	}
}

// WithStatus sets the overall status.
func (h HealthCheck) WithStatus(status HealthStatus) HealthCheck {
	h.Status = status
	return h
}

// WithLatency sets the check latency.
func (h HealthCheck) WithLatency(d time.Duration) HealthCheck {
	h.Latency = d
	return h
}

// WithDetail adds a detail key-value pair.
func (h HealthCheck) WithDetail(key, value string) HealthCheck {
	if h.Details == nil {
		h.Details = make(map[string]string)
	}
	h.Details[key] = value
	return h
}

// WithComponent adds a component check result.
func (h HealthCheck) WithComponent(check ComponentCheck) HealthCheck {
	h.Checks = append(h.Checks, check)
	// Update overall status based on component
	if check.Status == HealthStatusUnhealthy && h.Status == HealthStatusHealthy {
		h.Status = HealthStatusUnhealthy
	} else if check.Status == HealthStatusDegraded && h.Status == HealthStatusHealthy {
		h.Status = HealthStatusDegraded
	}
	return h
}

// IsHealthy returns true if the overall status is healthy.
func (h HealthCheck) IsHealthy() bool {
	return h.Status == HealthStatusHealthy
}

// CheckStore performs a health check on a store/database connection.
func CheckStore(ctx context.Context, name string, pingFn func(context.Context) error) ComponentCheck {
	start := time.Now()
	err := pingFn(ctx)
	latency := time.Since(start)

	if err != nil {
		return ComponentCheck{
			Name:    name,
			Status:  HealthStatusUnhealthy,
			Latency: latency,
			Message: err.Error(),
		}
	}

	status := HealthStatusHealthy
	if latency > 100*time.Millisecond {
		status = HealthStatusDegraded
	}

	return ComponentCheck{
		Name:    name,
		Status:  status,
		Latency: latency,
	}
}

// CheckDependency performs a health check on a service dependency.
func CheckDependency(ctx context.Context, name string, checker SimpleHealthChecker) ComponentCheck {
	if checker == nil {
		return ComponentCheck{
			Name:    name,
			Status:  HealthStatusUnhealthy,
			Message: "dependency not configured",
		}
	}

	start := time.Now()
	err := checker.Ping(ctx)
	latency := time.Since(start)

	if err != nil {
		return ComponentCheck{
			Name:    name,
			Status:  HealthStatusUnhealthy,
			Latency: latency,
			Message: err.Error(),
		}
	}

	return ComponentCheck{
		Name:    name,
		Status:  HealthStatusHealthy,
		Latency: latency,
	}
}

// AggregateStatus combines multiple statuses, returning the worst.
func AggregateStatus(statuses ...HealthStatus) HealthStatus {
	result := HealthStatusHealthy
	for _, s := range statuses {
		if s == HealthStatusUnhealthy {
			return HealthStatusUnhealthy
		}
		if s == HealthStatusDegraded {
			result = HealthStatusDegraded
		}
	}
	return result
}
