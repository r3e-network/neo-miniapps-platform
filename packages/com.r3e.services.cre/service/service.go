package cre

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
	
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
	"github.com/R3E-Network/service_layer/system/sandbox"
)

// Runner dispatches CRE runs to the execution backend.
type Runner interface {
	Dispatch(ctx context.Context, run Run, playbook Playbook, exec *Executor) error
}

// RunnerFunc adapts a function to the Runner interface.
type RunnerFunc func(ctx context.Context, run Run, playbook Playbook, exec *Executor) error

// Dispatch calls f(ctx, run, playbook, exec).
func (f RunnerFunc) Dispatch(ctx context.Context, run Run, playbook Playbook, exec *Executor) error {
	return f(ctx, run, playbook, exec)
}

// Service manages CRE playbooks and runs.
type Service struct {
	*framework.SandboxedServiceEngine
	store  Store
	runner Runner
}

// New constructs a CRE service.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	return &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:         "cre",
				Domain:       "cre",
				Description:  "Composable Run Engine playbooks and executions",
				DependsOn:    []string{"store", "svc-accounts"},
				RequiresAPIs: []framework.APISurface{framework.APISurfaceStore, framework.APISurfaceEvent, framework.APISurfaceCompute},
				Capabilities: []string{"cre"},
				Accounts:     accounts,
				Logger:       log,
			},
			SecurityLevel: sandbox.SecurityLevelPrivileged,
			RequestedCapabilities: []sandbox.Capability{
				sandbox.CapStorageRead,
				sandbox.CapStorageWrite,
				sandbox.CapDatabaseRead,
				sandbox.CapDatabaseWrite,
				sandbox.CapBusPublish,
				sandbox.CapServiceCall,
			},
			StorageQuota: 10 * 1024 * 1024,
		}),
		store: store,
		runner: RunnerFunc(func(context.Context, Run, Playbook, *Executor) error {
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
func (s *Service) CreatePlaybook(ctx context.Context, pb Playbook) (Playbook, error) {
	if err := s.ValidateAccountExists(ctx, pb.AccountID); err != nil {
		return Playbook{}, err
	}
	if err := s.normalizePlaybook(&pb); err != nil {
		return Playbook{}, err
	}
	attrs := map[string]string{"account_id": pb.AccountID, "resource": "playbook"}
	ctx, finish := s.StartObservation(ctx, attrs)
	created, err := s.store.CreatePlaybook(ctx, pb)
	if err == nil && created.ID != "" {
		attrs["playbook_id"] = created.ID
	}
	finish(err)
	if err != nil {
		return Playbook{}, err
	}
	s.Logger().WithField("playbook_id", created.ID).WithField("account_id", created.AccountID).Info("cre playbook created")
	s.LogCreated("playbook", created.ID, created.AccountID)
	s.IncrementCounter("cre_playbooks_created_total", map[string]string{"account_id": created.AccountID})
	return created, nil
}

// UpdatePlaybook mutates a playbook owned by the account.
func (s *Service) UpdatePlaybook(ctx context.Context, pb Playbook) (Playbook, error) {
	stored, err := s.store.GetPlaybook(ctx, pb.ID)
	if err != nil {
		return Playbook{}, err
	}
	if err := core.EnsureOwnership(stored.AccountID, pb.AccountID, "playbook", pb.ID); err != nil {
		return Playbook{}, err
	}
	pb.AccountID = stored.AccountID
	if err := s.normalizePlaybook(&pb); err != nil {
		return Playbook{}, err
	}
	attrs := map[string]string{"account_id": pb.AccountID, "playbook_id": pb.ID, "resource": "playbook"}
	ctx, finish := s.StartObservation(ctx, attrs)
	updated, err := s.store.UpdatePlaybook(ctx, pb)
	finish(err)
	if err != nil {
		return Playbook{}, err
	}
	s.Logger().WithField("playbook_id", pb.ID).WithField("account_id", pb.AccountID).Info("cre playbook updated")
	s.LogUpdated("playbook", pb.ID, pb.AccountID)
	s.IncrementCounter("cre_playbooks_updated_total", map[string]string{"account_id": pb.AccountID})
	return updated, nil
}

// GetPlaybook returns a single playbook scoped to the account.
func (s *Service) GetPlaybook(ctx context.Context, accountID, playbookID string) (Playbook, error) {
	pb, err := s.store.GetPlaybook(ctx, playbookID)
	if err != nil {
		return Playbook{}, err
	}
	if err := core.EnsureOwnership(pb.AccountID, accountID, "playbook", playbookID); err != nil {
		return Playbook{}, err
	}
	return pb, nil
}

// ListPlaybooks lists account playbooks.
func (s *Service) ListPlaybooks(ctx context.Context, accountID string) ([]Playbook, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListPlaybooks(ctx, accountID)
}

// CreateRun creates a run for the given playbook.
func (s *Service) CreateRun(ctx context.Context, accountID, playbookID string, params map[string]any, tags []string, executorID string) (Run, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return Run{}, err
	}
	pb, err := s.store.GetPlaybook(ctx, playbookID)
	if err != nil {
		return Run{}, err
	}
	if err := core.EnsureOwnership(pb.AccountID, accountID, "playbook", playbookID); err != nil {
		return Run{}, err
	}

	var exec *Executor
	if executorID != "" {
		found, err := s.store.GetExecutor(ctx, executorID)
		if err != nil {
			return Run{}, err
		}
		if err := core.EnsureOwnership(found.AccountID, accountID, "executor", executorID); err != nil {
			return Run{}, err
		}
		exec = &found
	}

	run := Run{
		AccountID:  accountID,
		PlaybookID: playbookID,
		ExecutorID: strings.TrimSpace(executorID),
		Status:     RunStatusPending,
		Parameters: params,
		Tags:       core.NormalizeTags(tags),
	}
	attrs := map[string]string{"account_id": accountID, "playbook_id": playbookID, "resource": "run"}
	ctx, finish := s.StartObservation(ctx, attrs)

	created, err := s.store.CreateRun(ctx, run)
	if err != nil {
		finish(err)
		return Run{}, err
	}
	attrs["run_id"] = created.ID
	finish(nil)

	s.LogCreated("run", created.ID, created.AccountID)
	s.IncrementCounter("cre_runs_created_total", map[string]string{"account_id": created.AccountID})
	eventPayload := map[string]any{
		"run_id":      created.ID,
		"account_id":  created.AccountID,
		"playbook_id": created.PlaybookID,
	}
	if err := s.PublishEvent(ctx, "cre.run.created", eventPayload); err != nil {
		if errors.Is(err, core.ErrBusUnavailable) {
			s.Logger().WithError(err).Warn("bus unavailable for cre run event")
		} else {
			return Run{}, fmt.Errorf("publish run event: %w", err)
		}
	}
	ctx, dispatchFinish := s.StartObservation(ctx, map[string]string{"run_id": created.ID, "resource": "runner_dispatch"})
	if err := s.runner.Dispatch(ctx, created, pb, exec); err != nil {
		dispatchFinish(err)
		s.Logger().WithError(err).WithField("run_id", created.ID).Warn("cre runner dispatch failed")
		return created, nil
	}
	dispatchFinish(nil)

	return created, nil
}

// GetRun fetches a run ensuring account visibility.
func (s *Service) GetRun(ctx context.Context, accountID, runID string) (Run, error) {
	run, err := s.store.GetRun(ctx, runID)
	if err != nil {
		return Run{}, err
	}
	if err := core.EnsureOwnership(run.AccountID, accountID, "run", runID); err != nil {
		return Run{}, err
	}
	return run, nil
}

// ListRuns lists recent runs for the account.
func (s *Service) ListRuns(ctx context.Context, accountID string, limit int) ([]Run, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListRuns(ctx, accountID, clamped)
}

// CreateExecutor registers an executor for an account.
func (s *Service) CreateExecutor(ctx context.Context, exec Executor) (Executor, error) {
	if err := s.ValidateAccountExists(ctx, exec.AccountID); err != nil {
		return Executor{}, err
	}
	if err := s.normalizeExecutor(&exec); err != nil {
		return Executor{}, err
	}
	attrs := map[string]string{"account_id": exec.AccountID, "resource": "executor"}
	ctx, finish := s.StartObservation(ctx, attrs)
	created, err := s.store.CreateExecutor(ctx, exec)
	if err == nil && created.ID != "" {
		attrs["executor_id"] = created.ID
	}
	finish(err)
	if err != nil {
		return Executor{}, err
	}
	s.Logger().WithField("executor_id", created.ID).WithField("account_id", created.AccountID).Info("cre executor created")
	s.LogCreated("executor", created.ID, created.AccountID)
	s.IncrementCounter("cre_executors_created_total", map[string]string{"account_id": created.AccountID})
	return created, nil
}

// UpdateExecutor updates executor metadata.
func (s *Service) UpdateExecutor(ctx context.Context, exec Executor) (Executor, error) {
	stored, err := s.store.GetExecutor(ctx, exec.ID)
	if err != nil {
		return Executor{}, err
	}
	if err := core.EnsureOwnership(stored.AccountID, exec.AccountID, "executor", exec.ID); err != nil {
		return Executor{}, err
	}
	exec.AccountID = stored.AccountID
	if err := s.normalizeExecutor(&exec); err != nil {
		return Executor{}, err
	}
	attrs := map[string]string{"account_id": exec.AccountID, "executor_id": exec.ID, "resource": "executor"}
	ctx, finish := s.StartObservation(ctx, attrs)
	updated, err := s.store.UpdateExecutor(ctx, exec)
	finish(err)
	if err != nil {
		return Executor{}, err
	}
	s.Logger().WithField("executor_id", exec.ID).WithField("account_id", exec.AccountID).Info("cre executor updated")
	s.LogUpdated("executor", exec.ID, exec.AccountID)
	s.IncrementCounter("cre_executors_updated_total", map[string]string{"account_id": exec.AccountID})
	return updated, nil
}

// ListExecutors lists executors for an account.
func (s *Service) ListExecutors(ctx context.Context, accountID string) ([]Executor, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListExecutors(ctx, accountID)
}

// GetExecutor fetches a single executor.
func (s *Service) GetExecutor(ctx context.Context, accountID, executorID string) (Executor, error) {
	exec, err := s.store.GetExecutor(ctx, executorID)
	if err != nil {
		return Executor{}, err
	}
	if err := core.EnsureOwnership(exec.AccountID, accountID, "executor", executorID); err != nil {
		return Executor{}, err
	}
	return exec, nil
}

func (s *Service) normalizePlaybook(pb *Playbook) error {
	pb.Name = strings.TrimSpace(pb.Name)
	pb.Description = strings.TrimSpace(pb.Description)
	pb.Metadata = core.NormalizeMetadata(pb.Metadata)
	pb.Tags = core.NormalizeTags(pb.Tags)

	if pb.Name == "" {
		return core.RequiredError("name")
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

func normalizeStep(step *Step, idx int) error {
	step.Name = strings.TrimSpace(step.Name)
	if step.Name == "" {
		step.Name = fmt.Sprintf("step-%d", idx)
	}
	step.Type = StepType(strings.ToLower(string(step.Type)))
	if step.Type == "" {
		return fmt.Errorf("step %d type is required", idx)
	}
	switch step.Type {
	case StepTypeFunctionCall, StepTypeAutomation, StepTypeHTTPRequest:
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
		step.Config = core.CloneAnyMap(step.Config)
	}
	return nil
}

func (s *Service) normalizeExecutor(exec *Executor) error {
	exec.Name = strings.TrimSpace(exec.Name)
	exec.Type = strings.ToLower(strings.TrimSpace(exec.Type))
	exec.Endpoint = strings.TrimSpace(exec.Endpoint)
	exec.Metadata = core.NormalizeMetadata(exec.Metadata)
	exec.Tags = core.NormalizeTags(exec.Tags)
	if exec.Name == "" {
		return core.RequiredError("name")
	}
	if exec.Type == "" {
		exec.Type = "generic"
	}
	if exec.Endpoint == "" {
		return core.RequiredError("endpoint")
	}
	return nil
}

// HTTP API Methods
// These methods follow the HTTP{Method}{Path} naming convention for automatic route discovery.

// HTTPGetPlaybooks handles GET /playbooks - list all playbooks for an account.
func (s *Service) HTTPGetPlaybooks(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListPlaybooks(ctx, req.AccountID)
}

// HTTPPostPlaybooks handles POST /playbooks - create a new playbook.
func (s *Service) HTTPPostPlaybooks(ctx context.Context, req core.APIRequest) (any, error) {
	name, _ := req.Body["name"].(string)
	description, _ := req.Body["description"].(string)

	var steps []Step
	if rawSteps, ok := req.Body["steps"].([]any); ok {
		for _, rs := range rawSteps {
			if stepMap, ok := rs.(map[string]any); ok {
				step := Step{
					Name:           core.GetString(stepMap, "name"),
					Type:           StepType(core.GetString(stepMap, "type")),
					TimeoutSeconds: core.GetInt(stepMap, "timeout_seconds"),
					RetryLimit:     core.GetInt(stepMap, "retry_limit"),
				}
				if cfg, ok := stepMap["config"].(map[string]any); ok {
					step.Config = cfg
				}
				steps = append(steps, step)
			}
		}
	}

	var tags []string
	if rawTags, ok := req.Body["tags"].([]any); ok {
		for _, t := range rawTags {
			if str, ok := t.(string); ok {
				tags = append(tags, str)
			}
		}
	}

	metadata := core.ExtractMetadataRaw(req.Body, "")

	pb := Playbook{
		AccountID:   req.AccountID,
		Name:        name,
		Description: description,
		Steps:       steps,
		Tags:        tags,
		Metadata:    metadata,
	}

	return s.CreatePlaybook(ctx, pb)
}

// HTTPGetPlaybooksById handles GET /playbooks/{id} - get a specific playbook.
func (s *Service) HTTPGetPlaybooksById(ctx context.Context, req core.APIRequest) (any, error) {
	playbookID := req.PathParams["id"]
	return s.GetPlaybook(ctx, req.AccountID, playbookID)
}

// HTTPGetRuns handles GET /runs - list all runs for an account.
func (s *Service) HTTPGetRuns(ctx context.Context, req core.APIRequest) (any, error) {
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListRuns(ctx, req.AccountID, limit)
}

// HTTPPostRuns handles POST /runs - create a new run.
func (s *Service) HTTPPostRuns(ctx context.Context, req core.APIRequest) (any, error) {
	playbookID, _ := req.Body["playbook_id"].(string)
	executorID, _ := req.Body["executor_id"].(string)

	var params map[string]any
	if p, ok := req.Body["parameters"].(map[string]any); ok {
		params = p
	}

	var tags []string
	if rawTags, ok := req.Body["tags"].([]any); ok {
		for _, t := range rawTags {
			if str, ok := t.(string); ok {
				tags = append(tags, str)
			}
		}
	}

	return s.CreateRun(ctx, req.AccountID, playbookID, params, tags, executorID)
}

// HTTPGetRunsById handles GET /runs/{id} - get a specific run.
func (s *Service) HTTPGetRunsById(ctx context.Context, req core.APIRequest) (any, error) {
	runID := req.PathParams["id"]
	return s.GetRun(ctx, req.AccountID, runID)
}

// HTTPGetExecutors handles GET /executors - list all executors for an account.
func (s *Service) HTTPGetExecutors(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListExecutors(ctx, req.AccountID)
}

// HTTPPostExecutors handles POST /executors - create a new executor.
func (s *Service) HTTPPostExecutors(ctx context.Context, req core.APIRequest) (any, error) {
	name, _ := req.Body["name"].(string)
	typ, _ := req.Body["type"].(string)
	endpoint, _ := req.Body["endpoint"].(string)

	var tags []string
	if rawTags, ok := req.Body["tags"].([]any); ok {
		for _, t := range rawTags {
			if str, ok := t.(string); ok {
				tags = append(tags, str)
			}
		}
	}

	metadata := core.ExtractMetadataRaw(req.Body, "")

	exec := Executor{
		AccountID: req.AccountID,
		Name:      name,
		Type:      typ,
		Endpoint:  endpoint,
		Tags:      tags,
		Metadata:  metadata,
	}

	return s.CreateExecutor(ctx, exec)
}

// HTTPGetExecutorsById handles GET /executors/{id} - get a specific executor.
func (s *Service) HTTPGetExecutorsById(ctx context.Context, req core.APIRequest) (any, error) {
	executorID := req.PathParams["id"]
	return s.GetExecutor(ctx, req.AccountID, executorID)
}
