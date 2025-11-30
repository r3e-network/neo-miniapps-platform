package cre

import (
	"context"
	"testing"
)

func TestService_CreatePlaybook(t *testing.T) {
	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	accounts.AddAccount("acct-1", "")
	svc := New(accounts, store, nil)

	pb, err := svc.CreatePlaybook(context.Background(), Playbook{
		AccountID: "acct-1",
		Name:      "test-playbook",
		Steps:     []Step{{Name: "step1", Type: StepTypeFunctionCall}},
	})
	if err != nil {
		t.Fatalf("create playbook: %v", err)
	}
	if pb.Name != "test-playbook" {
		t.Fatalf("expected name test-playbook")
	}
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
}

func TestService_Manifest(t *testing.T) {
	svc := New(nil, nil, nil)
	m := svc.Manifest()
	if m.Name != "cre" {
		t.Fatalf("expected name cre")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "cre" {
		t.Fatalf("expected name cre")
	}
}
