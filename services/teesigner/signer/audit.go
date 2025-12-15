package signer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultAuditBuffer  = 1024
	DefaultAuditTimeout = 5 * time.Second
)

type AuditRepository interface {
	Request(ctx context.Context, method, table string, body interface{}, query string) ([]byte, error)
}

type AuditEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	ClientCertCN string    `json:"client_cert_cn"`
	TxHash       string    `json:"tx_hash"`
	KeyVersion   string    `json:"key_version"`
}

type AuditLogger struct {
	repo    AuditRepository
	table   string
	timeout time.Duration

	queue   chan AuditEvent
	once    sync.Once
	stopped atomic.Bool
	dropped atomic.Uint64

	wg sync.WaitGroup
}

func NewAuditLogger(repo AuditRepository, table string, buffer int, timeout time.Duration) *AuditLogger {
	if buffer <= 0 {
		buffer = DefaultAuditBuffer
	}
	if timeout <= 0 {
		timeout = DefaultAuditTimeout
	}
	return &AuditLogger{
		repo:    repo,
		table:   table,
		timeout: timeout,
		queue:   make(chan AuditEvent, buffer),
	}
}

func (l *AuditLogger) Start() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		l.wg.Add(1)
		go l.run()
	})
}

func (l *AuditLogger) Stop(ctx context.Context) error {
	if l == nil {
		return nil
	}
	if !l.stopped.CompareAndSwap(false, true) {
		return nil
	}

	close(l.queue)

	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("audit stop: %w", ctx.Err())
	}
}

func (l *AuditLogger) Dropped() uint64 {
	if l == nil {
		return 0
	}
	return l.dropped.Load()
}

// Log enqueues an audit record. It never blocks; records may be dropped when the queue is full.
func (l *AuditLogger) Log(event AuditEvent) bool {
	if l == nil || l.stopped.Load() {
		return false
	}

	select {
	case l.queue <- event:
		return true
	default:
		l.dropped.Add(1)
		return false
	}
}

func (l *AuditLogger) run() {
	defer l.wg.Done()

	for event := range l.queue {
		if l.repo == nil || l.table == "" {
			continue
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), l.timeout)
		_, _ = l.repo.Request(reqCtx, "POST", l.table, event, "")
		cancel()
	}
}
