package httpapi

import (
	"testing"
	"time"

	engine "github.com/R3E-Network/service_layer/system/engine/core"
)

func TestBuildModuleStatuses_Labels(t *testing.T) {
	now := time.Now()
	infos := []engine.ModuleInfo{
		{Name: "svc-a", Label: "Service A"},
		{Name: "svc-b"},
	}
	health := []engine.ModuleHealth{
		{Name: "svc-a", Status: "started", UpdatedAt: now},
		{Name: "svc-b", Status: "started", UpdatedAt: now},
	}

	statuses := BuildModuleStatuses(infos, health)
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	if statuses[0].Label != "Service A" || statuses[0].Name != "svc-a" {
		t.Fatalf("expected label Service A for svc-a, got name=%s label=%s", statuses[0].Name, statuses[0].Label)
	}
	if statuses[1].Label != "svc-b" {
		t.Fatalf("expected fallback label to name svc-b, got %s", statuses[1].Label)
	}
}
