package gasbank

import (
	"context"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/internal/app/system"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// WithdrawalResolver decides whether a pending withdrawal has been settled.
type WithdrawalResolver interface {
	Resolve(ctx context.Context, tx gasbank.Transaction) (done bool, success bool, message string, retryAfter time.Duration, err error)
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

func (r *TimeoutResolver) Resolve(ctx context.Context, tx gasbank.Transaction) (bool, bool, string, time.Duration, error) {
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
	store    storage.GasBankStore
	service  *Service
	resolver WithdrawalResolver
	interval time.Duration
	log      *logger.Logger

	mu          sync.Mutex
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	running     bool
	nextAttempt map[string]time.Time
}

var _ system.Service = (*SettlementPoller)(nil)

func NewSettlementPoller(store storage.GasBankStore, service *Service, resolver WithdrawalResolver, log *logger.Logger) *SettlementPoller {
	if log == nil {
		log = logger.NewDefault("gasbank-settlement")
	}
	if resolver == nil {
		resolver = NewTimeoutResolver(2 * time.Minute)
	}
	return &SettlementPoller{
		store:       store,
		service:     service,
		resolver:    resolver,
		interval:    15 * time.Second,
		log:         log,
		nextAttempt: make(map[string]time.Time),
	}
}

func (p *SettlementPoller) Name() string { return "gasbank-settlement" }

func (p *SettlementPoller) Start(ctx context.Context) error {
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

	return nil
}

func (p *SettlementPoller) tick(ctx context.Context) {
	txs, err := p.store.ListPendingWithdrawals(ctx)
	if err != nil {
		p.log.WithError(err).Warn("list pending withdrawals failed")
		return
	}

	now := time.Now()
	for _, tx := range txs {
		if !p.shouldAttempt(tx.ID, now) {
			continue
		}

		done, success, message, retryAfter, err := p.resolver.Resolve(ctx, tx)
		if err != nil {
			p.log.WithError(err).Warnf("resolver error for withdrawal %s", tx.ID)
			p.scheduleNext(tx.ID, retryAfter)
			continue
		}

		if !done {
			p.scheduleNext(tx.ID, retryAfter)
			continue
		}

		if p.service == nil {
			p.log.Warnf("no gas bank service attached; cannot settle %s", tx.ID)
			continue
		}

		if _, _, err := p.service.CompleteWithdrawal(ctx, tx.ID, success, message); err != nil {
			p.log.WithError(err).Warnf("complete withdrawal %s failed", tx.ID)
			p.scheduleNext(tx.ID, retryAfter)
			continue
		}
		p.log.Infof("withdrawal %s settled (success=%t)", tx.ID, success)
		p.clearSchedule(tx.ID)
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
