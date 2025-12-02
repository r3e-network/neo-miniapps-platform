package gasbank

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// WithdrawalResolver decides whether a pending withdrawal has been settled.
type WithdrawalResolver interface {
	Resolve(ctx context.Context, tx Transaction) (done bool, success bool, message string, retryAfter time.Duration, err error)
}

// TimeoutResolver marks pending transactions as failed after a timeout.
type TimeoutResolver struct {
	timeout time.Duration
	seen    sync.Map // txID -> time.Time
}

func NewTimeoutResolver(timeout time.Duration) *TimeoutResolver {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &TimeoutResolver{timeout: timeout}
}

func (r *TimeoutResolver) Resolve(ctx context.Context, tx Transaction) (bool, bool, string, time.Duration, error) {
	if value, ok := r.seen.Load(tx.ID); ok {
		if time.Since(value.(time.Time)) >= r.timeout {
			return true, false, "timeout waiting for blockchain confirmation", 0, nil
		}
		return false, false, "", r.timeout / 4, nil
	}
	r.seen.Store(tx.ID, time.Now())
	return false, false, "", r.timeout / 4, nil
}

// SettlementPoller watches pending withdrawals and settles them using the resolver.
type SettlementPoller struct {
	framework.ServiceBase
	store       Store
	service     *Service
	resolver    WithdrawalResolver
	interval    time.Duration
	maxAttempts int
	log         *logger.Logger
	tracer      core.Tracer
	hooks       core.ObservationHooks

	mu          sync.Mutex
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	running     bool
	nextAttempt map[string]time.Time
}

var _ interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
	Ready(context.Context) error
} = (*SettlementPoller)(nil)

func NewSettlementPoller(store Store, service *Service, resolver WithdrawalResolver, log *logger.Logger) *SettlementPoller {
	if log == nil {
		log = logger.NewDefault("gasbank-settlement")
	}
	poller := &SettlementPoller{
		store:       store,
		service:     service,
		resolver:    resolver,
		interval:    15 * time.Second,
		maxAttempts: 5,
		log:         log,
		tracer:      core.NoopTracer,
		hooks:       core.NoopObservationHooks,
		nextAttempt: make(map[string]time.Time),
	}
	poller.SetName(poller.Name())
	return poller
}

// WithTracer configures span emission for per-transaction settlement attempts.
func (p *SettlementPoller) WithTracer(tracer core.Tracer) {
	p.mu.Lock()
	if tracer == nil {
		p.tracer = core.NoopTracer
	} else {
		p.tracer = tracer
	}
	p.mu.Unlock()
}

// WithObservationHooks configures callbacks for settlement attempts.
func (p *SettlementPoller) WithObservationHooks(hooks core.ObservationHooks) {
	p.mu.Lock()
	p.hooks = hooks
	p.mu.Unlock()
}

// WithRetryPolicy overrides the retry cadence and attempt budget.
func (p *SettlementPoller) WithRetryPolicy(maxAttempts int, interval time.Duration) {
	p.mu.Lock()
	if maxAttempts > 0 {
		p.maxAttempts = maxAttempts
	}
	if interval > 0 {
		p.interval = interval
	}
	p.mu.Unlock()
}

// Domain reports the service domain for engine grouping.
func (p *SettlementPoller) Domain() string { return "gasbank" }

func (p *SettlementPoller) Name() string { return "gasbank-settlement" }

func (p *SettlementPoller) Start(ctx context.Context) error {
	if p.resolver == nil {
		p.log.Warn("withdrawal resolver not configured; settlement poller disabled")
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.running = true

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				p.tick(runCtx)
			}
		}
	}()

	p.log.Info("gas bank settlement poller started")
	p.MarkReady(true)
	return nil
}

func (p *SettlementPoller) Stop(ctx context.Context) error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	cancel := p.cancel
	p.running = false
	p.cancel = nil
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		p.wg.Wait()
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	p.MarkReady(false)
	return nil
}

// Ready reports readiness based on running state and resolver presence.
func (p *SettlementPoller) Ready(ctx context.Context) error {
	if err := p.ServiceBase.Ready(ctx); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.resolver == nil {
		return fmt.Errorf("settlement resolver not configured")
	}
	if !p.running {
		return fmt.Errorf("gasbank settlement poller not running")
	}
	return nil
}

func (p *SettlementPoller) tick(ctx context.Context) {
	if p.resolver == nil {
		return
	}
	if p.service != nil {
		if err := p.service.ActivateDueSchedules(ctx, 100); err != nil {
			p.log.WithError(err).Warn("activate due schedules failed")
		}
	}
	txs, err := p.store.ListPendingWithdrawals(ctx)
	if err != nil {
		p.log.WithError(err).Warn("list pending withdrawals failed")
		return
	}

	p.mu.Lock()
	tracer := p.tracer
	hooks := p.hooks
	p.mu.Unlock()

	now := time.Now()
	for _, tx := range txs {
		if !tx.NextAttemptAt.IsZero() && tx.NextAttemptAt.After(now) {
			p.seedNextAttempt(tx.ID, tx.NextAttemptAt)
		}
		if !p.shouldAttempt(tx.ID, now) {
			continue
		}
		txCopy := tx
		attrs := map[string]string{"transaction_id": tx.ID}
		if tx.AccountID != "" {
			attrs["account_id"] = tx.AccountID
		}
		spanCtx, finishSpan := tracer.StartSpan(ctx, "gasbank.settlement", attrs)
		finishObs := core.StartObservation(spanCtx, hooks, attrs)
		start := time.Now()
		done, success, message, retryAfter, err := p.resolver.Resolve(spanCtx, tx)
		completed := time.Now()
		if err != nil {
			p.log.WithError(err).
				WithField("transaction_id", tx.ID).
				Warn("withdrawal resolver error")
			txUpdated, recErr := p.recordAttempt(spanCtx, txCopy, "error", err.Error(), start, completed, retryAfter)
			if recErr != nil {
				p.log.WithError(recErr).WithField("transaction_id", tx.ID).Warn("record attempt failed")
			} else {
				txCopy = txUpdated
			}
			if p.shouldDeadLetter(txCopy) {
				p.promoteDeadLetter(spanCtx, txCopy, "resolver error", err.Error())
				p.clearSchedule(tx.ID)
			} else {
				p.scheduleNext(tx.ID, retryAfter)
			}
			finishObs(err)
			finishSpan(err)
			continue
		}

		status := "retry"
		if done {
			if success {
				status = "succeeded"
			} else {
				status = "failed"
			}
		}
		txUpdated, recErr := p.recordAttempt(spanCtx, txCopy, status, message, start, completed, retryAfter)
		if recErr != nil {
			p.log.WithError(recErr).WithField("transaction_id", tx.ID).Warn("record attempt failed")
		} else {
			txCopy = txUpdated
		}

		if !done {
			if p.shouldDeadLetter(txCopy) {
				p.promoteDeadLetter(spanCtx, txCopy, "max attempts exceeded", message)
				p.clearSchedule(tx.ID)
			} else {
				p.scheduleNext(tx.ID, retryAfter)
			}
			finishObs(nil)
			finishSpan(nil)
			continue
		}

		if p.service == nil {
			p.log.WithField("transaction_id", tx.ID).
				Warn("no gas bank service attached; cannot settle withdrawal")
			finishObs(nil)
			finishSpan(nil)
			continue
		}

		if _, _, err := p.service.CompleteWithdrawal(spanCtx, tx.ID, success, message); err != nil {
			p.log.WithError(err).
				WithField("transaction_id", tx.ID).
				Warn("complete withdrawal failed")
			p.scheduleNext(tx.ID, retryAfter)
			finishObs(err)
			finishSpan(err)
			continue
		}
		p.log.WithField("transaction_id", tx.ID).
			WithField("account_id", tx.AccountID).
			WithField("success", success).
			Info("settlement poller completed withdrawal")
		p.clearSchedule(tx.ID)
		finishObs(nil)
		finishSpan(nil)
	}
}

func (p *SettlementPoller) shouldAttempt(id string, now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	next, ok := p.nextAttempt[id]
	if !ok || now.After(next) {
		return true
	}
	return false
}

func (p *SettlementPoller) seedNextAttempt(id string, at time.Time) {
	p.mu.Lock()
	if existing, ok := p.nextAttempt[id]; !ok || at.Before(existing) {
		p.nextAttempt[id] = at
	}
	p.mu.Unlock()
}

func (p *SettlementPoller) scheduleNext(id string, after time.Duration) {
	if after <= 0 {
		after = p.interval
	}
	p.mu.Lock()
	p.nextAttempt[id] = time.Now().Add(after)
	p.mu.Unlock()
}

func (p *SettlementPoller) clearSchedule(id string) {
	p.mu.Lock()
	delete(p.nextAttempt, id)
	p.mu.Unlock()
}

func (p *SettlementPoller) recordAttempt(ctx context.Context, tx Transaction, status string, message string, started, completed time.Time, retryAfter time.Duration) (Transaction, error) {
	attempt := tx.ResolverAttempt + 1
	record := SettlementAttempt{
		TransactionID: tx.ID,
		Attempt:       attempt,
		StartedAt:     started.UTC(),
		CompletedAt:   completed.UTC(),
		Latency:       completed.Sub(started),
		Status:        status,
		Error:         message,
	}
	if _, err := p.store.RecordSettlementAttempt(ctx, record); err != nil {
		p.log.WithError(err).
			WithField("transaction_id", tx.ID).
			Warn("record settlement attempt failed")
	}
	tx.ResolverAttempt = attempt
	tx.ResolverError = message
	tx.LastAttemptAt = completed.UTC()
	if status == "succeeded" || status == "failed" {
		tx.NextAttemptAt = time.Time{}
	} else if retryAfter > 0 {
		tx.NextAttemptAt = completed.Add(retryAfter)
	}
	tx.UpdatedAt = completed.UTC()
	updated, err := p.store.UpdateGasTransaction(ctx, tx)
	if err != nil {
		return tx, err
	}
	return updated, nil
}

func (p *SettlementPoller) shouldDeadLetter(tx Transaction) bool {
	p.mu.Lock()
	max := p.maxAttempts
	p.mu.Unlock()
	return max > 0 && tx.ResolverAttempt >= max
}

func (p *SettlementPoller) promoteDeadLetter(ctx context.Context, tx Transaction, reason, message string) {
	if p.service == nil {
		return
	}
	if err := p.service.MarkDeadLetter(ctx, tx, reason, message); err != nil {
		p.log.WithError(err).
			WithField("transaction_id", tx.ID).
			Warn("failed to mark dead letter")
	} else {
		p.log.WithField("transaction_id", tx.ID).
			WithField("account_id", tx.AccountID).
			Warn("withdrawal moved to dead letter queue")
	}
}
