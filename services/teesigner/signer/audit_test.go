package signer

import (
	"context"
	"testing"
	"time"
)

type auditRepoRecorder struct {
	calls chan AuditEvent
}

func (r *auditRepoRecorder) Request(_ context.Context, method, table string, body interface{}, _ string) ([]byte, error) {
	if method != "POST" {
		return nil, nil
	}
	if table == "" {
		return nil, nil
	}
	event := body.(AuditEvent)
	r.calls <- event
	return nil, nil
}

func TestAuditLogger_WritesAsync(t *testing.T) {
	repo := &auditRepoRecorder{calls: make(chan AuditEvent, 1)}
	logger := NewAuditLogger(repo, "tee_signer_audit", 10, 200*time.Millisecond)
	logger.Start()

	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	if ok := logger.Log(AuditEvent{Timestamp: ts, ClientCertCN: "cn", TxHash: "0x00", KeyVersion: "v1"}); !ok {
		t.Fatal("expected enqueue to succeed")
	}

	select {
	case got := <-repo.calls:
		if got.ClientCertCN != "cn" || got.KeyVersion != "v1" || !got.Timestamp.Equal(ts) {
			t.Fatalf("unexpected audit event: %+v", got)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for audit call")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := logger.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestNewAuditLogger_DefaultsAndNilSafety(t *testing.T) {
	logger := NewAuditLogger(nil, "tee_signer_audit", -1, -1)
	if logger == nil {
		t.Fatal("expected logger")
	}
	if cap(logger.queue) != DefaultAuditBuffer {
		t.Fatalf("expected default buffer %d, got %d", DefaultAuditBuffer, cap(logger.queue))
	}
	if logger.timeout != DefaultAuditTimeout {
		t.Fatalf("expected default timeout %s, got %s", DefaultAuditTimeout, logger.timeout)
	}

	var nilLogger *AuditLogger
	nilLogger.Start()
	if got := nilLogger.Dropped(); got != 0 {
		t.Fatalf("expected dropped=0 for nil logger, got %d", got)
	}
	if ok := nilLogger.Log(AuditEvent{}); ok {
		t.Fatal("expected Log to return false for nil logger")
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := nilLogger.Stop(stopCtx); err != nil {
		t.Fatalf("expected nil Stop to return nil, got %v", err)
	}
}

func TestAuditLogger_DropsWhenQueueFull(t *testing.T) {
	logger := NewAuditLogger(nil, "tee_signer_audit", 1, 100*time.Millisecond)

	if ok := logger.Log(AuditEvent{TxHash: "0x00"}); !ok {
		t.Fatal("expected first enqueue to succeed")
	}
	if ok := logger.Log(AuditEvent{TxHash: "0x01"}); ok {
		t.Fatal("expected second enqueue to be dropped")
	}
	if got := logger.Dropped(); got != 1 {
		t.Fatalf("expected dropped=1, got %d", got)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := logger.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestAuditLogger_LogAfterStopAndStartTwice(t *testing.T) {
	repo := &auditRepoRecorder{calls: make(chan AuditEvent, 1)}
	logger := NewAuditLogger(repo, "tee_signer_audit", 10, 100*time.Millisecond)
	logger.Start()
	logger.Start()

	stopCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := logger.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := logger.Stop(stopCtx); err != nil {
		t.Fatalf("Stop again: %v", err)
	}

	if ok := logger.Log(AuditEvent{TxHash: "0x00"}); ok {
		t.Fatal("expected Log to return false after Stop")
	}
}

func TestAuditLogger_TableEmptySkipsWrites(t *testing.T) {
	repo := &auditRepoRecorder{calls: make(chan AuditEvent, 1)}
	logger := NewAuditLogger(repo, "", 10, 100*time.Millisecond)
	logger.Start()
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = logger.Stop(stopCtx)
	}()

	_ = logger.Log(AuditEvent{TxHash: "0x00"})

	select {
	case <-repo.calls:
		t.Fatal("expected no writes when table is empty")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestAuditLogger_StopTimeout(t *testing.T) {
	repo := &auditRepoRecorder{calls: make(chan AuditEvent)} // unbuffered to block worker if it tries to write
	logger := NewAuditLogger(repo, "tee_signer_audit", 1, 250*time.Millisecond)
	logger.Start()

	_ = logger.Log(AuditEvent{TxHash: "0x00"})

	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := logger.Stop(stopCtx); err == nil {
		t.Fatal("expected Stop to time out")
	}
}
