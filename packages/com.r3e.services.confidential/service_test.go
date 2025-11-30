package confidential

import (
	"context"
	"testing"
)

func TestService_CreateEnclave(t *testing.T) {
	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	accounts.AddAccount("acct-1", "")
	svc := New(accounts, store, nil)

	enclave, err := svc.CreateEnclave(context.Background(), Enclave{AccountID: "acct-1", Name: "test-enclave", Endpoint: "http://localhost:8080"})
	if err != nil {
		t.Fatalf("create enclave: %v", err)
	}
	if enclave.Status != EnclaveStatusInactive {
		t.Fatalf("expected inactive status")
	}
}

func TestService_CreateSealedKey(t *testing.T) {
	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	accounts.AddAccount("acct-1", "")
	svc := New(accounts, store, nil)

	enclave, _ := svc.CreateEnclave(context.Background(), Enclave{AccountID: "acct-1", Name: "test-enclave", Endpoint: "http://localhost:8080"})
	key, err := svc.CreateSealedKey(context.Background(), SealedKey{AccountID: "acct-1", EnclaveID: enclave.ID, Name: "test-key", Blob: []byte("sealed")})
	if err != nil {
		t.Fatalf("create sealed key: %v", err)
	}
	if key.Name != "test-key" {
		t.Fatalf("expected key name test-key")
	}
}

func TestService_CreateAttestation(t *testing.T) {
	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	accounts.AddAccount("acct-1", "")
	svc := New(accounts, store, nil)

	enclave, _ := svc.CreateEnclave(context.Background(), Enclave{AccountID: "acct-1", Name: "test-enclave", Endpoint: "http://localhost:8080"})
	att, err := svc.CreateAttestation(context.Background(), Attestation{AccountID: "acct-1", EnclaveID: enclave.ID, Report: "test-report"})
	if err != nil {
		t.Fatalf("create attestation: %v", err)
	}
	if att.Report != "test-report" {
		t.Fatalf("expected report test-report")
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
	if m.Name != "confidential" {
		t.Fatalf("expected name confidential")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "confidential" {
		t.Fatalf("expected name confidential")
	}
}
