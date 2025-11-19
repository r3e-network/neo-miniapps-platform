package oracle

import (
	"context"
	"sync"
	"time"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	domain "github.com/R3E-Network/service_layer/internal/app/domain/oracle"
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
	tracer   core.Tracer

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
		tracer:      core.NoopTracer,
	}
}

// WithResolver overrides the default resolver.
func (d *Dispatcher) WithResolver(resolver RequestResolver) {
	d.mu.Lock()
	d.resolver = resolver
	d.mu.Unlock()
}

// WithTracer configures an optional tracer used for per-request spans.
func (d *Dispatcher) WithTracer(tracer core.Tracer) {
	d.mu.Lock()
	if tracer == nil {
		d.tracer = core.NoopTracer
	} else {
		d.tracer = tracer
	}
	d.mu.Unlock()
}

func (d *Dispatcher) Name() string { return "oracle-dispatcher" }

// Descriptor advertises the dispatcher's placement and capabilities.
func (d *Dispatcher) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "oracle-dispatcher",
		Domain:       "oracle",
		Layer:        core.LayerPlatform,
		Capabilities: []string{"dispatch", "resolve"},
	}
}

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
	tracer := d.tracer
	d.mu.Unlock()

	if resolver == nil {
		return
	}

	now := time.Now()
	for _, req := range reqs {
		if !d.shouldAttempt(req.ID, now) {
			continue
		}

		attrs := map[string]string{"request_id": req.ID}
		if req.AccountID != "" {
			attrs["account_id"] = req.AccountID
		}
		if req.DataSourceID != "" {
			attrs["data_source_id"] = req.DataSourceID
		}

		func(req domain.Request) {
			spanCtx, finishSpan := tracer.StartSpan(ctx, "oracle.dispatch", attrs)
			var spanErr error
			defer func() {
				finishSpan(spanErr)
			}()

			current := req
			if req.Status == domain.StatusPending {
				updated, err := d.service.MarkRunning(spanCtx, req.ID)
				if err != nil {
					spanErr = err
					d.log.WithError(err).
						WithField("request_id", req.ID).
						Warn("mark oracle request running failed")
					d.scheduleNext(req.ID, d.interval)
					return
				}
				current = updated
			}

			done, success, result, errMsg, retryAfter, err := resolver.Resolve(spanCtx, current)
			if err != nil {
				spanErr = err
				d.log.WithError(err).
					WithField("request_id", req.ID).
					Warn("oracle resolver error")
				d.scheduleNext(req.ID, retryAfter)
				return
			}

			if !done {
				d.scheduleNext(req.ID, retryAfter)
				return
			}

			if success {
				if _, err := d.service.CompleteRequest(spanCtx, req.ID, result); err != nil {
					spanErr = err
					d.log.WithError(err).
						WithField("request_id", req.ID).
						Warn("complete oracle request failed")
					d.scheduleNext(req.ID, retryAfter)
					return
				}
			} else {
				if _, err := d.service.FailRequest(spanCtx, req.ID, errMsg); err != nil {
					spanErr = err
					d.log.WithError(err).
						WithField("request_id", req.ID).
						Warn("mark oracle request failed")
					d.scheduleNext(req.ID, retryAfter)
					return
				}
			}

			d.clearSchedule(req.ID)
		}(req)
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
