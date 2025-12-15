package signer

import (
	"context"
	"errors"
	"testing"
	"time"

	signerpb "github.com/R3E-Network/service_layer/api/gen/signer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestKeyManager_Rotate_OverlapAndExpiry(t *testing.T) {
	current := time.Unix(1, 0).UTC()
	nowFn := func() time.Time { return current }

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed:     []byte("seed"),
		Now:               nowFn,
		InitialKeyVersion: KeyVersionV1,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	if got := mgr.ActiveVersion(); got != "v1" {
		t.Fatalf("expected active=v1, got %q", got)
	}

	current = time.Unix(2, 0).UTC()
	res, err := mgr.Rotate(context.Background())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if !res.Rotated {
		t.Fatal("expected rotated=true")
	}
	if res.OldVersion != "v1" {
		t.Fatalf("expected old_version=v1, got %q", res.OldVersion)
	}
	if res.NewVersion != "v2" {
		t.Fatalf("expected new_version=v2, got %q", res.NewVersion)
	}
	if res.DeprecatedUntil == nil {
		t.Fatal("expected deprecated_until to be set")
	}

	withinOverlap := time.Unix(2, 0).UTC().Add(6 * 24 * time.Hour)
	if _, used, err := mgr.SigningKeyAt(context.Background(), "v1", withinOverlap); err != nil {
		t.Fatalf("SigningKeyAt(v1 within overlap): %v", err)
	} else if used != "v1" {
		t.Fatalf("expected used=v1, got %q", used)
	}

	afterExpiry := time.Unix(2, 0).UTC().Add(8 * 24 * time.Hour)
	if _, _, err := mgr.SigningKeyAt(context.Background(), "v1", afterExpiry); err == nil {
		t.Fatal("expected expired key error")
	} else if !isKeyExpired(err) {
		t.Fatalf("expected ErrKeyVersionExpired, got %v", err)
	}

	if _, used, err := mgr.SigningKeyAt(context.Background(), "", afterExpiry); err != nil {
		t.Fatalf("SigningKeyAt(active): %v", err)
	} else if used != "v2" {
		t.Fatalf("expected used=v2, got %q", used)
	}
}

func TestService_RotateKey_AndSignWithDeprecatedVersion(t *testing.T) {
	current := time.Unix(1, 0).UTC()
	nowFn := func() time.Time { return current }

	repo := &auditRepoRecorder{calls: make(chan AuditEvent, 16)}
	audit := NewAuditLogger(repo, "tee_signer_audit", 10, 200*time.Millisecond)
	audit.Start()
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = audit.Stop(stopCtx)
	})

	service, err := NewService(ServiceConfig{
		MasterKeySeed: []byte("seed"),
		RateLimiter:   NewClientRateLimiter(1000, 1000, nowFn),
		AuditLogger:   audit,
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	current = time.Unix(2, 0).UTC()
	res, err := service.RotateKey(context.Background())
	if err != nil {
		t.Fatalf("RotateKey: %v", err)
	}
	if res.NewVersion != "v2" {
		t.Fatalf("expected new_version=v2, got %q", res.NewVersion)
	}

	select {
	case evt := <-repo.calls:
		if evt.ClientCertCN != "internal" {
			t.Fatalf("expected internal CN, got %q", evt.ClientCertCN)
		}
		if evt.KeyVersion != "v2" {
			t.Fatalf("expected key_version=v2, got %q", evt.KeyVersion)
		}
		if evt.TxHash == "" {
			t.Fatal("expected rotation tx hash to be set")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for rotation audit event")
	}

	ctx := contextWithClientCN(context.Background(), "client-1")

	withinOverlap := time.Unix(2, 0).UTC().Add(6 * 24 * time.Hour)
	current = withinOverlap
	resp, err := service.Sign(ctx, &signerpb.SignRequest{
		TxHash:     "0x" + repeatHex("aa", 32),
		KeyVersion: "v1",
	})
	if err != nil {
		t.Fatalf("Sign(deprecated v1 within overlap): %v", err)
	}
	if resp.GetKeyVersion() != "v1" {
		t.Fatalf("expected response key_version=v1, got %q", resp.GetKeyVersion())
	}

	afterExpiry := time.Unix(2, 0).UTC().Add(8 * 24 * time.Hour)
	current = afterExpiry
	if _, err := service.Sign(ctx, &signerpb.SignRequest{
		TxHash:     "0x" + repeatHex("bb", 32),
		KeyVersion: "v1",
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for expired key, got %v", err)
	}
}

func isKeyExpired(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrKeyVersionExpired)
}
