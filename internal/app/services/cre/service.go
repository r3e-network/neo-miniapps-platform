package cre

import (
	"context"
	"fmt"
	"strings"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	"github.com/R3E-Network/service_layer/internal/app/domain/cre"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Runner dispatches CRE runs to the execution backend.
type Runner interface {
	Dispatch(ctx context.Context, run cre.Run, playbook cre.Playbook, exec *cre.Executor) error
}

// RunnerFunc adapts a function to the Runner interface.
type RunnerFunc func(ctx context.Context, run cre.Run, playbook cre.Playbook, exec *cre.Executor) error

// Dispatch calls f(ctx, run, playbook, exec).
func (f RunnerFunc) Dispatch(ctx context.Context, run cre.Run, playbook cre.Playbook, exec *cre.Executor) error {
	return f(ctx, run, playbook, exec)
}

// Service manages CRE playbooks and runs.
type Service struct {
	base   *core.Base
	store  storage.CREStore
	runner Runner
	log    *logger.Logger
}

// New constructs a CRE service.
func New(accounts storage.AccountStore, store storage.CREStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("cre")
	}
	return &Service{
		base:  core.NewBase(accounts),
		store: store,
		log:   log,
		runner: RunnerFunc(func(context.Context, cre.Run, cre.Playbook, *cre.Executor) error {
			return nil
		}),
	}
}

// WithRunner injects a runner dispatcher.
func (s *Service) WithRunner(r Runner) {
	if r != nil {
		s.runner = r
	}
}

// CreatePlaybook validates and stores a new playbook.
func (s *Service) CreatePlaybook(ctx context.Context, pb cre.Playbook) (cre.Playbook, error) {
	if err := s.base.EnsureAccount(ctx, pb.AccountID); err != nil {
		return cre.Playbook{}, err
	}
	if err := s.normalizePlaybook(&pb); err != nil {
		return cre.Playbook{}, err
	}
	created, err := s.store.CreatePlaybook(ctx, pb)
	if err != nil {
		return cre.Playbook{}, err
	}
	s.log.WithField("playbook_id", created.ID).WithField("account_id", created.AccountID).Info("cre playbook created")
	return created, nil
}

// UpdatePlaybook mutates a playbook owned by the account.
func (s *Service) UpdatePlaybook(ctx context.Context, pb cre.Playbook) (cre.Playbook, error) {
	stored, err := s.store.GetPlaybook(ctx, pb.ID)
	if err != nil {
		return cre.Playbook{}, err
	}
	if stored.AccountID != pb.AccountID {
		return cre.Playbook{}, fmt.Errorf("playbook %s does not belong to account %s", pb.ID, pb.AccountID)
	}
	pb.AccountID = stored.AccountID
	if err := s.normalizePlaybook(&pb); err != nil {
		return cre.Playbook{}, err
	}
	updated, err := s.store.UpdatePlaybook(ctx, pb)
	if err != nil {
		return cre.Playbook{}, err
	}
	s.log.WithField("playbook_id", pb.ID).WithField("account_id", pb.AccountID).Info("cre playbook updated")
	return updated, nil
}

// GetPlaybook returns a single playbook scoped to the account.
func (s *Service) GetPlaybook(ctx context.Context, accountID, playbookID string) (cre.Playbook, error) {
	pb, err := s.store.GetPlaybook(ctx, playbookID)
	if err != nil {
		return cre.Playbook{}, err
	}
	if pb.AccountID != accountID {
		return cre.Playbook{}, fmt.Errorf("playbook %s does not belong to account %s", playbookID, accountID)
	}
	return pb, nil
}

// ListPlaybooks lists account playbooks.
func (s *Service) ListPlaybooks(ctx context.Context, accountID string) ([]cre.Playbook, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListPlaybooks(ctx, accountID)
}

// Descriptor advertises the service placement and capabilities.
func (s *Service) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "cre",
		Domain:       "cre",
		Layer:        core.LayerPlatform,
		Capabilities: []string{"playbooks", "runs", "executors"},
	}
}

// CreateRun creates a run for the given playbook.
func (s *Service) CreateRun(ctx context.Context, accountID, playbookID string, params map[string]any, tags []string, executorID string) (cre.Run, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return cre.Run{}, err
	}
	pb, err := s.store.GetPlaybook(ctx, playbookID)
	if err != nil {
		return cre.Run{}, err
	}
	if pb.AccountID != accountID {
		return cre.Run{}, fmt.Errorf("playbook %s does not belong to account %s", playbookID, accountID)
	}

	var exec *cre.Executor
	if executorID != "" {
		found, err := s.store.GetExecutor(ctx, executorID)
		if err != nil {
			return cre.Run{}, err
		}
		if found.AccountID != accountID {
			return cre.Run{}, fmt.Errorf("executor %s does not belong to account %s", executorID, accountID)
		}
		exec = &found
	}

	run := cre.Run{
		AccountID:  accountID,
		PlaybookID: playbookID,
		ExecutorID: strings.TrimSpace(executorID),
		Status:     cre.RunStatusPending,
		Parameters: params,
		Tags:       core.NormalizeTags(tags),
	}

	created, err := s.store.CreateRun(ctx, run)
	if err != nil {
		return cre.Run{}, err
	}

	if err := s.runner.Dispatch(ctx, created, pb, exec); err != nil {
		s.log.WithError(err).WithField("run_id", created.ID).Warn("cre runner dispatch failed")
	}

	return created, nil
}

// GetRun fetches a run ensuring account visibility.
func (s *Service) GetRun(ctx context.Context, accountID, runID string) (cre.Run, error) {
	run, err := s.store.GetRun(ctx, runID)
	if err != nil {
		return cre.Run{}, err
	}
	if run.AccountID != accountID {
		return cre.Run{}, fmt.Errorf("run %s does not belong to account %s", runID, accountID)
	}
	return run, nil
}

// ListRuns lists recent runs for the account.
func (s *Service) ListRuns(ctx context.Context, accountID string, limit int) ([]cre.Run, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListRuns(ctx, accountID, clamped)
}

// CreateExecutor registers an executor for an account.
func (s *Service) CreateExecutor(ctx context.Context, exec cre.Executor) (cre.Executor, error) {
	if err := s.base.EnsureAccount(ctx, exec.AccountID); err != nil {
		return cre.Executor{}, err
	}
	if err := s.normalizeExecutor(&exec); err != nil {
		return cre.Executor{}, err
	}
	created, err := s.store.CreateExecutor(ctx, exec)
	if err != nil {
		return cre.Executor{}, err
	}
	s.log.WithField("executor_id", created.ID).WithField("account_id", created.AccountID).Info("cre executor created")
	return created, nil
}

// UpdateExecutor updates executor metadata.
func (s *Service) UpdateExecutor(ctx context.Context, exec cre.Executor) (cre.Executor, error) {
	stored, err := s.store.GetExecutor(ctx, exec.ID)
	if err != nil {
		return cre.Executor{}, err
	}
	if stored.AccountID != exec.AccountID {
		return cre.Executor{}, fmt.Errorf("executor %s does not belong to account %s", exec.ID, exec.AccountID)
	}
	exec.AccountID = stored.AccountID
	if err := s.normalizeExecutor(&exec); err != nil {
		return cre.Executor{}, err
	}
	updated, err := s.store.UpdateExecutor(ctx, exec)
	if err != nil {
		return cre.Executor{}, err
	}
	s.log.WithField("executor_id", exec.ID).WithField("account_id", exec.AccountID).Info("cre executor updated")
	return updated, nil
}

// ListExecutors lists executors for an account.
func (s *Service) ListExecutors(ctx context.Context, accountID string) ([]cre.Executor, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListExecutors(ctx, accountID)
}

// GetExecutor fetches a single executor.
func (s *Service) GetExecutor(ctx context.Context, accountID, executorID string) (cre.Executor, error) {
	exec, err := s.store.GetExecutor(ctx, executorID)
	if err != nil {
		return cre.Executor{}, err
	}
	if exec.AccountID != accountID {
		return cre.Executor{}, fmt.Errorf("executor %s does not belong to account %s", executorID, accountID)
	}
	return exec, nil
}

func (s *Service) normalizePlaybook(pb *cre.Playbook) error {
	pb.Name = strings.TrimSpace(pb.Name)
	pb.Description = strings.TrimSpace(pb.Description)
	pb.Metadata = core.NormalizeMetadata(pb.Metadata)
	pb.Tags = core.NormalizeTags(pb.Tags)

	if pb.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(pb.Steps) == 0 {
		return fmt.Errorf("playbook must contain at least one step")
	}

	for i := range pb.Steps {
		if err := normalizeStep(&pb.Steps[i], i); err != nil {
			return err
		}
	}
	return nil
}

func normalizeStep(step *cre.Step, idx int) error {
	step.Name = strings.TrimSpace(step.Name)
	if step.Name == "" {
		step.Name = fmt.Sprintf("step-%d", idx)
	}
	step.Type = cre.StepType(strings.ToLower(string(step.Type)))
	if step.Type == "" {
		return fmt.Errorf("step %d type is required", idx)
	}
	switch step.Type {
	case cre.StepTypeFunctionCall, cre.StepTypeAutomation, cre.StepTypeHTTPRequest:
	default:
		return fmt.Errorf("step %d has unsupported type %s", idx, step.Type)
	}
	if step.TimeoutSeconds < 0 {
		step.TimeoutSeconds = 0
	}
	if step.RetryLimit < 0 {
		step.RetryLimit = 0
	}
	step.Metadata = core.NormalizeMetadata(step.Metadata)
	step.Tags = core.NormalizeTags(step.Tags)
	if step.Config == nil {
		step.Config = map[string]any{}
	} else {
		step.Config = cloneAnyMap(step.Config)
	}
	return nil
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func (s *Service) normalizeExecutor(exec *cre.Executor) error {
	exec.Name = strings.TrimSpace(exec.Name)
	exec.Type = strings.ToLower(strings.TrimSpace(exec.Type))
	exec.Endpoint = strings.TrimSpace(exec.Endpoint)
	exec.Metadata = core.NormalizeMetadata(exec.Metadata)
	exec.Tags = core.NormalizeTags(exec.Tags)
	if exec.Name == "" {
		return fmt.Errorf("name is required")
	}
	if exec.Type == "" {
		exec.Type = "generic"
	}
	if exec.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	return nil
}
