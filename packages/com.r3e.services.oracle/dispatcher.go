package oracle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/pkg/metrics"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

var _ interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
	Ready(context.Context) error
} = (*Dispatcher)(nil)

// Dispatcher periodically inspects pending oracle requests and forwards them
// to the configured resolver.
type Dispatcher struct {
	framework.ServiceBase
	service     *Service
	log         *logger.Logger
	interval    time.Duration
	resolver    RequestResolver
	tracer      core.Tracer
	maxAttempts int
	ttl         time.Duration
	deadLetter  bool

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
	d := &Dispatcher{
		service:     service,
		log:         log,
		interval:    10 * time.Second,
		maxAttempts: 0,
		ttl:         0,
		deadLetter:  true,
		nextAttempt: make(map[string]time.Time),
		tracer:      core.NoopTracer,
	}
	d.SetName(d.Name())
	return d
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

// WithRetryPolicy configures attempts/backoff/TTL.
func (d *Dispatcher) WithRetryPolicy(maxAttempts int, backoff, ttl time.Duration) {
	d.mu.Lock()
	if backoff > 0 {
		d.interval = backoff
	}
	if maxAttempts > 0 {
		d.maxAttempts = maxAttempts
	}
	if ttl > 0 {
		d.ttl = ttl
	}
	d.mu.Unlock()
}

// EnableDeadLetter toggles failing exhausted requests instead of retrying forever.
func (d *Dispatcher) EnableDeadLetter(enabled bool) {
	d.mu.Lock()
	d.deadLetter = enabled
	d.mu.Unlock()
}

func (d *Dispatcher) Name() string { return "oracle-dispatcher" }

// Descriptor advertises the dispatcher's placement and capabilities.
func (d *Dispatcher) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "oracle-dispatcher",
		Domain:       "oracle",
		Layer:        core.LayerRunner,
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
	d.MarkReady(true)
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
	d.MarkReady(false)
	return nil
}

// Ready reports readiness based on running state.
func (d *Dispatcher) Ready(ctx context.Context) error {
	if err := d.ServiceBase.Ready(ctx); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return fmt.Errorf("oracle dispatcher not running")
	}
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
	if len(reqs) > 0 {
		oldest := reqs[0].CreatedAt
		for _, r := range reqs {
			if r.CreatedAt.Before(oldest) {
				oldest = r.CreatedAt
			}
		}
		metrics.RecordOracleStaleness(reqs[0].AccountID, time.Since(oldest))
	}

	d.mu.Lock()
	resolver := d.resolver
	tracer := d.tracer
	maxAttempts := d.maxAttempts
	ttl := d.ttl
	deadLetter := d.deadLetter
	d.mu.Unlock()

	if resolver == nil {
		return
	}

	now := time.Now()
	for _, req := range reqs {
		if ttl > 0 && !req.CreatedAt.IsZero() && now.After(req.CreatedAt.Add(ttl)) {
			errMsg := "oracle request expired"
			if deadLetter {
				errMsg += " (dead-lettered)"
			}
			if _, err := d.service.FailRequest(ctx, req.ID, errMsg); err != nil {
				d.log.WithError(err).WithField("request_id", req.ID).Warn("expire oracle request failed")
			}
			d.clearSchedule(req.ID)
			continue
		}

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

		func(req Request) {
			spanCtx, finishSpan := tracer.StartSpan(ctx, "oracle.dispatch", attrs)
			var spanErr error
			defer func() {
				finishSpan(spanErr)
			}()

			var current Request
			nextAttempt := req.Attempts + 1
			if maxAttempts > 0 && nextAttempt > maxAttempts {
				errMsg := fmt.Sprintf("max attempts exceeded (%d)", maxAttempts)
				if deadLetter {
					errMsg += "; dead-lettered"
				}
				metrics.RecordOracleAttempt(req.AccountID, "exhausted")
				if _, err := d.service.FailRequest(spanCtx, req.ID, errMsg); err != nil {
					spanErr = err
					d.log.WithError(err).
						WithField("request_id", req.ID).
						Warn("fail oracle request after attempts")
				}
				d.clearSchedule(req.ID)
				return
			}
			if req.Status == StatusPending {
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
			} else {
				updated, err := d.service.IncrementAttempts(spanCtx, req.ID)
				if err != nil {
					spanErr = err
					d.log.WithError(err).
						WithField("request_id", req.ID).
						Warn("record oracle attempt failed")
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
				metrics.RecordOracleAttempt(req.AccountID, "success")
				if _, err := d.service.CompleteRequest(spanCtx, req.ID, result); err != nil {
					spanErr = err
					d.log.WithError(err).
						WithField("request_id", req.ID).
						Warn("complete oracle request failed")
					d.scheduleNext(req.ID, retryAfter)
					return
				}
			} else {
				metrics.RecordOracleAttempt(req.AccountID, "fail")
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
