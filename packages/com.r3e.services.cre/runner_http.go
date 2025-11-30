package cre

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
)

// HTTPRunner dispatches CRE runs to executor endpoints via HTTP POST.
type HTTPRunner struct {
	client *http.Client
	log    *logger.Logger
}

// NewHTTPRunner constructs an HTTP-based runner.
func NewHTTPRunner(client *http.Client, log *logger.Logger) *HTTPRunner {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if log == nil {
		log = logger.NewDefault("cre-http-runner")
	}
	return &HTTPRunner{client: client, log: log}
}

// Dispatch POSTS the run + playbook to the executor endpoint if provided.
func (r *HTTPRunner) Dispatch(ctx context.Context, run Run, playbook Playbook, exec *Executor) error {
	if exec == nil {
		r.log.WithField("run_id", run.ID).Info("no executor provided; skipping dispatch")
		return nil
	}
	body := map[string]any{
		"run":      run,
		"playbook": playbook,
		"executor": exec,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, exec.Endpoint, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("dispatch http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("executor returned status %d", resp.StatusCode)
	}

	r.log.WithField("run_id", run.ID).WithField("executor_id", exec.ID).Info("cre run dispatched via http")
	return nil
}
