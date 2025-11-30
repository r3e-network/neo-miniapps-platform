package automation

import (
	"context"
	"testing"
)

func TestService_CreateAndUpdateJob(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_EnableDisableJob(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_DeleteJob(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_Lifecycle(t *testing.T) {
	svc := New(nil, nil, nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := svc.Ready(context.Background()); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if svc.Ready(context.Background()) == nil {
		t.Fatalf("expected not ready after stop")
	}
}

func TestService_Manifest(t *testing.T) {
	svc := New(nil, nil, nil)
	m := svc.Manifest()
	if m.Name != "automation" {
		t.Fatalf("expected name automation")
	}
	if m.Domain != "automation" {
		t.Fatalf("expected domain automation")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "automation" {
		t.Fatalf("expected name automation")
	}
}
