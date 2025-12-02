package httpapi

import (
	"context"
	"strings"
	"time"

	engine "github.com/R3E-Network/service_layer/system/engine/core"
)

// ModuleStatus is exposed on /system/status to describe active modules and their domains.
type ModuleStatus struct {
	Name         string                 `json:"name"`
	Label        string                 `json:"label,omitempty"`
	Domain       string                 `json:"domain,omitempty"`
	Category     string                 `json:"category,omitempty"`
	Layer        string                 `json:"layer,omitempty"`
	Interfaces   []string               `json:"interfaces,omitempty"`
	APIs         []engine.APIDescriptor `json:"apis,omitempty"`
	Notes        []string               `json:"notes,omitempty"`
	DependsOn    []string               `json:"depends_on,omitempty"`
	Status       string                 `json:"status,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Ready        string                 `json:"ready_status,omitempty"`
	ReadyErr     string                 `json:"ready_error,omitempty"`
	StartedAt    *time.Time             `json:"started_at,omitempty"`
	StoppedAt    *time.Time             `json:"stopped_at,omitempty"`
	UpdatedAt    time.Time              `json:"updated_at,omitempty"`
	StartNanos   int64                  `json:"start_nanos,omitempty"`
	StopNanos    int64                  `json:"stop_nanos,omitempty"`
	Permissions  []string               `json:"permissions,omitempty"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Quotas       map[string]string      `json:"quotas,omitempty"`
	RequiresAPIs []string               `json:"requires_apis,omitempty"`
}

// ModuleProvider returns the current module statuses (used by /system/status).
type ModuleProvider func() []ModuleStatus

// BuildModuleStatuses converts engine module metadata into HTTP-friendly payloads.
func BuildModuleStatuses(infos []engine.ModuleInfo, health []engine.ModuleHealth) []ModuleStatus {
	statusByName := make(map[string]engine.ModuleHealth, len(health))
	for _, h := range health {
		statusByName[h.Name] = h
	}
	out := make([]ModuleStatus, 0, len(infos))
	for _, info := range infos {
		h := statusByName[info.Name]
		out = append(out, ModuleStatus{
			Name:         info.Name,
			Label:        firstNonEmpty(info.Label, info.Name),
			Domain:       info.Domain,
			Category:     info.Category,
			Layer:        info.Layer,
			Interfaces:   info.Interfaces,
			APIs:         info.APIs,
			Notes:        info.Notes,
			DependsOn:    info.DependsOn,
			Status:       h.Status,
			Error:        h.Error,
			Ready:        h.ReadyStatus,
			ReadyErr:     h.ReadyError,
			StartedAt:    h.StartedAt,
			StoppedAt:    h.StoppedAt,
			UpdatedAt:    h.UpdatedAt,
			StartNanos:   h.StartNanos,
			StopNanos:    h.StopNanos,
			Permissions:  info.Permissions,
			Capabilities: info.Capabilities,
			Quotas:       info.Quotas,
			RequiresAPIs: surfacesToStrings(info.RequiresAPIs),
		})
	}
	return out
}

func surfacesToStrings(surfaces []engine.APISurface) []string {
	var out []string
	for _, s := range surfaces {
		if str := strings.TrimSpace(string(s)); str != "" {
			out = append(out, str)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// EngineModuleProvider returns a ModuleProvider that probes readiness, fetches
// module info/health, and converts to HTTP statuses. Use this when wiring an
// engine into the HTTP layer to keep bus/status responses consistent.
func EngineModuleProvider(eng interface {
	ProbeReadiness(context.Context)
	ModulesInfo() []engine.ModuleInfo
	ModulesHealth() []engine.ModuleHealth
}) ModuleProvider {
	if eng == nil {
		return func() []ModuleStatus { return nil }
	}
	return func() []ModuleStatus {
		eng.ProbeReadiness(context.Background())
		return BuildModuleStatuses(eng.ModulesInfo(), eng.ModulesHealth())
	}
}
