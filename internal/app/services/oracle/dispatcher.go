package oracle

import (
	"context"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/system"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

var _ system.Service = (*Dispatcher)(nil)

// Dispatcher periodically inspects pending oracle requests and forwards them
// to the configured resolver.
type Dispatcher struct {
	service  *Service
	log      *logger.Logger
	interval time.Duration
	resolver RequestResolver

	mu          sync.Mutex
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	running     bool
	nextAttempt map[string]time.Time
}

// NewDispatcher constructs a lifecycle-managed oracle dispatcher.
func NewDispatcher(service *Service, log *logger.Logger) *Dispatcher {
	if log == nil {
		log = logger.NewDefault("oracle-dispatcher")
	}
	return &Dispatcher{
		service:     service,
		log:         log,
		interval:    10 * time.Second,
		nextAttempt: make(map[string]time.Time),
	}
}

// WithResolver overrides the default resolver.
func (d *Dispatcher) WithResolver(resolver RequestResolver) {
	d.mu.Lock()
	d.resolver = resolver
	d.mu.Unlock()
}

func (d *Dispatcher) Name() string { return "oracle-dispatcher" }

func (d *Dispatcher) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.resolver == nil {
		d.mu.Unlock()
		d.log.Warn("oracle request resolver not configured; dispatcher disabled")
		return nil
	}
	if d.running {
		d.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel
	d.running = true
	d.mu.Unlock()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		ticker := time.NewTicker(d.interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				d.tick(runCtx)
			}
		}
	}()

	d.log.Info("oracle dispatcher started")
	return nil
}

func (d *Dispatcher) Stop(ctx context.Context) error {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return nil
	}
	cancel := d.cancel
	d.running = false
	d.cancel = nil
	d.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		d.wg.Wait()
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	d.log.Info("oracle dispatcher stopped")
	return nil
}

func (d *Dispatcher) tick(ctx context.Context) {
	if d.service == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	reqs, err := d.service.ListPending(ctx)
	if err != nil {
		d.log.WithError(err).Warn("oracle dispatcher tick failed")
		return
	}

	d.mu.Lock()
	resolver := d.resolver
	d.mu.Unlock()

	if resolver == nil {
		return
	}

	now := time.Now()
	for _, req := range reqs {
		if !d.shouldAttempt(req.ID, now) {
			continue
		}

		done, success, result, errMsg, retryAfter, err := resolver.Resolve(ctx, req)
		if err != nil {
			d.log.WithError(err).
				WithField("request_id", req.ID).
				Warn("oracle resolver error")
			d.scheduleNext(req.ID, retryAfter)
			continue
		}

		if !done {
			d.scheduleNext(req.ID, retryAfter)
			continue
		}

		if success {
			if _, err := d.service.CompleteRequest(ctx, req.ID, result); err != nil {
				d.log.WithError(err).
					WithField("request_id", req.ID).
					Warn("complete oracle request failed")
				d.scheduleNext(req.ID, retryAfter)
				continue
			}
		} else {
			if _, err := d.service.FailRequest(ctx, req.ID, errMsg); err != nil {
				d.log.WithError(err).
					WithField("request_id", req.ID).
					Warn("mark oracle request failed")
				d.scheduleNext(req.ID, retryAfter)
				continue
			}
		}

		d.clearSchedule(req.ID)
	}
}

func (d *Dispatcher) shouldAttempt(id string, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	next, ok := d.nextAttempt[id]
	if !ok || now.After(next) {
		return true
	}
	return false
}

func (d *Dispatcher) scheduleNext(id string, after time.Duration) {
	if after <= 0 {
		after = d.interval
	}
	d.mu.Lock()
	d.nextAttempt[id] = time.Now().Add(after)
	d.mu.Unlock()
}

func (d *Dispatcher) clearSchedule(id string) {
	d.mu.Lock()
	delete(d.nextAttempt, id)
	d.mu.Unlock()
}
