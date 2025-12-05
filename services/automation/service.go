// Package automation provides task automation service.
package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

const (
	ServiceID   = "automation"
	ServiceName = "Automation Service"
	Version     = "1.0.0"
)

// Manifest returns the service manifest.
func Manifest() *os.LegacyManifest {
	return &os.LegacyManifest{
		ServiceID:   ServiceID,
		Version:     Version,
		Description: "Task automation and scheduling service",
		RequiredCapabilities: []os.Capability{
			os.CapStorage,
			os.CapCompute,
		},
		OptionalCapabilities: []os.Capability{
			os.CapNetwork,
			os.CapSecrets,
			os.CapDatabase,      // Database storage
			os.CapDatabaseWrite, // Database write
			os.CapMetrics,       // Metrics collection
			os.CapScheduler,     // Task scheduling
			os.CapQueue,         // Task queue
			os.CapLock,          // Distributed locking
			os.CapContract,      // Contract interaction
			os.CapContractWrite, // Contract trigger execution
		},
		ResourceLimits: os.ResourceLimits{
			MaxMemory:  128 * 1024 * 1024,
			MaxCPUTime: 60 * time.Second,
		},
	}
}

// Service implements the Automation service.
type Service struct {
	*base.BaseService
	mu      sync.RWMutex
	enclave *Enclave
	store   *Store
}

// New creates a new Automation service.
func New(serviceOS os.ServiceOS) (*Service, error) {
	s := &Service{
		BaseService: base.NewBaseService(ServiceID, ServiceName, Version, serviceOS),
		enclave:     NewEnclave(serviceOS),
		store:       NewStore(),
	}

	// Register components with BaseService for lifecycle management
	s.SetEnclave(s.enclave)
	s.SetStore(s.store)

	// Set lifecycle hooks
	s.SetHooks(base.LifecycleHooks{
		OnAfterStart: func(ctx context.Context) error {
			// Optional sealed config hydrate inside enclave
			var cfg map[string]any
			if loaded, err := s.enclave.LoadConfigJSON(ctx, "automation/config", &cfg); err != nil {
				s.Logger().Warn("automation storage hydrate failed", "err", err)
			} else if loaded {
				// TODO: apply cfg to enclave/runtime as needed
			}

			// Register metrics if available
			if s.OS().HasCapability(os.CapMetrics) {
				s.registerMetrics()
			}

			// Start task processor if scheduler available
			if s.OS().HasCapability(os.CapScheduler) {
				s.startTaskProcessor(ctx)
			}

			// Start contract trigger scheduler if contract capability available
			if s.OS().HasCapability(os.CapContract) {
				s.startContractTriggerScheduler(ctx)
			}

			s.Logger().Info("automation service started")
			return nil
		},
	})

	return s, nil
}

// registerMetrics registers service metrics.
func (s *Service) registerMetrics() {
	metrics := s.OS().Metrics()
	metrics.RegisterCounter("automation_tasks_created", "Total number of tasks created")
	metrics.RegisterCounter("automation_tasks_executed", "Total number of tasks executed")
	metrics.RegisterCounter("automation_tasks_failed", "Total number of failed tasks")
	metrics.RegisterGauge("automation_pending_tasks", "Number of pending tasks")
	metrics.RegisterHistogram("automation_task_duration_seconds", "Task execution duration", []float64{0.1, 0.5, 1, 5, 10, 30, 60})
}

// startTaskProcessor starts the background task processor.
func (s *Service) startTaskProcessor(ctx context.Context) {
	scheduler := s.OS().Scheduler()

	// Schedule periodic task processing (every 10 seconds)
	_, err := scheduler.ScheduleInterval(ctx, 10*time.Second, &os.ScheduledTask{
		Name:       "process_tasks",
		Handler:    "process_tasks",
		MaxRetries: 3,
		Timeout:    5 * time.Minute,
	})
	if err != nil {
		s.Logger().Warn("failed to schedule task processor", "error", err)
	}

	s.Logger().Info("task processor started")
}

// CreateTask creates a new automation task.
func (s *Service) CreateTask(ctx context.Context, task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}
	task.Status = TaskStatusPending
	task.SetTimestamps()
	return s.store.CreateTask(ctx, task)
}

// GetTask gets a task by ID.
func (s *Service) GetTask(ctx context.Context, id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.GetTask(ctx, id)
}

// ExecuteTask executes a task.
func (s *Service) ExecuteTask(ctx context.Context, taskID string) (*TaskResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}
	return s.enclave.ExecuteTask(ctx, taskID)
}

// =============================================================================
// Contract Integration (Trigger Model)
// =============================================================================

// startContractTriggerScheduler starts the contract trigger scheduler.
// Automation uses a trigger model - triggers are registered and executed based on schedule.
func (s *Service) startContractTriggerScheduler(ctx context.Context) {
	contract := s.OS().Contract()

	// Subscribe to new trigger creation requests
	_, err := contract.SubscribeRequests(ctx, s.handleTriggerRequest)
	if err != nil {
		s.Logger().Warn("failed to subscribe to trigger requests", "error", err)
		return
	}

	// Start the trigger execution loop
	go s.runTriggerExecutionLoop(ctx)

	s.Logger().Info("contract trigger scheduler started")
}

// handleTriggerRequest handles incoming trigger creation/management requests.
func (s *Service) handleTriggerRequest(ctx context.Context, req *os.ServiceRequest) error {
	s.Logger().Info("received trigger request",
		"request_id", req.RequestID,
		"requester", req.Requester,
	)

	// Parse the trigger request payload
	var triggerReq TriggerContractRequest
	if err := json.Unmarshal(req.Payload, &triggerReq); err != nil {
		return s.sendTriggerError(ctx, req.RequestID, "invalid payload: "+err.Error())
	}

	switch triggerReq.Action {
	case "create":
		return s.handleCreateTrigger(ctx, req, &triggerReq)
	case "cancel":
		return s.handleCancelTrigger(ctx, req, &triggerReq)
	case "get":
		return s.handleGetTrigger(ctx, req, &triggerReq)
	default:
		return s.sendTriggerError(ctx, req.RequestID, "unknown action: "+triggerReq.Action)
	}
}

// TriggerContractRequest represents a trigger request from a contract.
type TriggerContractRequest struct {
	Action        string `json:"action"` // create, cancel, get
	TriggerID     string `json:"trigger_id,omitempty"`
	TriggerType   uint8  `json:"trigger_type,omitempty"`   // 0=cron, 1=interval, 2=event, 3=once
	Target        string `json:"target,omitempty"`         // Target contract script hash
	Method        string `json:"method,omitempty"`         // Method to call
	Args          []byte `json:"args,omitempty"`           // Arguments
	Schedule      string `json:"schedule,omitempty"`       // Cron expression or interval
	GasLimit      int64  `json:"gas_limit,omitempty"`      // Max gas per execution
	MaxExecutions uint64 `json:"max_executions,omitempty"` // Max number of executions (0=unlimited)
}

// handleCreateTrigger creates a new automation trigger.
func (s *Service) handleCreateTrigger(ctx context.Context, req *os.ServiceRequest, triggerReq *TriggerContractRequest) error {
	// Create trigger in store
	trigger := &ContractTrigger{
		TriggerID:      fmt.Sprintf("trigger-%d", time.Now().UnixNano()),
		Owner:          req.Requester,
		Type:           os.TriggerType(triggerReq.TriggerType),
		Target:         triggerReq.Target,
		Method:         triggerReq.Method,
		Args:           triggerReq.Args,
		Schedule:       triggerReq.Schedule,
		GasLimit:       triggerReq.GasLimit,
		MaxExecutions:  triggerReq.MaxExecutions,
		Active:         true,
		ExecutionCount: 0,
	}

	// Calculate next execution time
	trigger.NextExecution = s.calculateNextExecution(trigger)

	// Store the trigger
	if err := s.store.CreateContractTrigger(ctx, trigger); err != nil {
		return s.sendTriggerError(ctx, req.RequestID, "failed to create trigger: "+err.Error())
	}

	// Send success callback
	result, _ := json.Marshal(map[string]string{
		"trigger_id": trigger.TriggerID,
		"status":     "created",
	})

	return s.sendTriggerCallback(ctx, req.RequestID, result)
}

// handleCancelTrigger cancels an existing trigger.
func (s *Service) handleCancelTrigger(ctx context.Context, req *os.ServiceRequest, triggerReq *TriggerContractRequest) error {
	// Get the trigger
	trigger, err := s.store.GetContractTrigger(ctx, triggerReq.TriggerID)
	if err != nil {
		return s.sendTriggerError(ctx, req.RequestID, "trigger not found: "+err.Error())
	}

	// Verify ownership
	if trigger.Owner != req.Requester {
		return s.sendTriggerError(ctx, req.RequestID, "not authorized to cancel this trigger")
	}

	// Deactivate the trigger
	trigger.Active = false
	if err := s.store.UpdateContractTrigger(ctx, trigger); err != nil {
		return s.sendTriggerError(ctx, req.RequestID, "failed to cancel trigger: "+err.Error())
	}

	result, _ := json.Marshal(map[string]string{
		"trigger_id": trigger.TriggerID,
		"status":     "cancelled",
	})

	return s.sendTriggerCallback(ctx, req.RequestID, result)
}

// handleGetTrigger retrieves trigger information.
func (s *Service) handleGetTrigger(ctx context.Context, req *os.ServiceRequest, triggerReq *TriggerContractRequest) error {
	trigger, err := s.store.GetContractTrigger(ctx, triggerReq.TriggerID)
	if err != nil {
		return s.sendTriggerError(ctx, req.RequestID, "trigger not found: "+err.Error())
	}

	result, _ := json.Marshal(trigger)
	return s.sendTriggerCallback(ctx, req.RequestID, result)
}

// ContractTrigger represents an automation trigger for contracts.
type ContractTrigger struct {
	base.BaseEntity
	TriggerID      string         `json:"trigger_id"`
	Owner          string         `json:"owner"`
	Type           os.TriggerType `json:"type"`
	Target         string         `json:"target"`
	Method         string         `json:"method"`
	Args           []byte         `json:"args,omitempty"`
	Schedule       string         `json:"schedule"`
	GasLimit       int64          `json:"gas_limit"`
	MaxExecutions  uint64         `json:"max_executions"`
	Active         bool           `json:"active"`
	ExecutionCount uint64         `json:"execution_count"`
	LastExecuted   *time.Time     `json:"last_executed,omitempty"`
	NextExecution  time.Time      `json:"next_execution"`
}

// runTriggerExecutionLoop runs the trigger execution loop.
func (s *Service) runTriggerExecutionLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.checkAndExecuteTriggers(ctx, now)
		}
	}
}

// checkAndExecuteTriggers checks for due triggers and executes them.
func (s *Service) checkAndExecuteTriggers(ctx context.Context, now time.Time) {
	// Get all active triggers that are due
	triggers, err := s.store.GetDueContractTriggers(ctx, now)
	if err != nil {
		s.Logger().Warn("failed to get due triggers", "error", err)
		return
	}

	for _, trigger := range triggers {
		go s.executeTrigger(ctx, trigger)
	}
}

// executeTrigger executes a single trigger.
func (s *Service) executeTrigger(ctx context.Context, trigger *ContractTrigger) {
	s.Logger().Info("executing trigger",
		"trigger_id", trigger.TriggerID,
		"target", trigger.Target,
		"method", trigger.Method,
	)

	// Check max executions
	if trigger.MaxExecutions > 0 && trigger.ExecutionCount >= trigger.MaxExecutions {
		trigger.Active = false
		s.store.UpdateContractTrigger(ctx, trigger)
		return
	}

	// Execute the trigger via contract API
	if s.OS().HasCapability(os.CapContractWrite) {
		if err := s.OS().Contract().ExecuteTrigger(ctx, trigger.TriggerID); err != nil {
			s.Logger().Warn("trigger execution failed", "trigger_id", trigger.TriggerID, "error", err)

			// Record metrics
			if s.OS().HasCapability(os.CapMetrics) {
				s.OS().Metrics().Counter("automation_trigger_failures", 1, "trigger_id", trigger.TriggerID)
			}
			return
		}
	}

	// Update trigger state
	now := time.Now()
	trigger.LastExecuted = &now
	trigger.ExecutionCount++
	trigger.NextExecution = s.calculateNextExecution(trigger)

	// Deactivate one-time triggers
	if trigger.Type == os.TriggerTypeOnce {
		trigger.Active = false
	}

	if err := s.store.UpdateContractTrigger(ctx, trigger); err != nil {
		s.Logger().Warn("failed to update trigger", "trigger_id", trigger.TriggerID, "error", err)
	}

	// Record metrics
	if s.OS().HasCapability(os.CapMetrics) {
		s.OS().Metrics().Counter("automation_trigger_executions", 1, "trigger_id", trigger.TriggerID)
	}

	s.Logger().Info("trigger executed successfully", "trigger_id", trigger.TriggerID)
}

// calculateNextExecution calculates the next execution time for a trigger.
func (s *Service) calculateNextExecution(trigger *ContractTrigger) time.Time {
	now := time.Now()

	switch trigger.Type {
	case os.TriggerTypeInterval:
		// Parse interval duration from schedule
		duration, err := time.ParseDuration(trigger.Schedule)
		if err != nil {
			duration = 1 * time.Minute // Default
		}
		return now.Add(duration)

	case os.TriggerTypeCron:
		// TODO: Implement cron parsing
		// For now, default to 1 hour
		return now.Add(1 * time.Hour)

	case os.TriggerTypeOnce:
		// Parse timestamp from schedule
		t, err := time.Parse(time.RFC3339, trigger.Schedule)
		if err != nil {
			return now.Add(24 * time.Hour * 365) // Far future
		}
		return t

	case os.TriggerTypeEvent:
		// Event triggers don't have scheduled execution
		return now.Add(24 * time.Hour * 365) // Far future

	default:
		return now.Add(1 * time.Hour)
	}
}

// sendTriggerCallback sends a successful callback for trigger operations.
func (s *Service) sendTriggerCallback(ctx context.Context, requestID string, result []byte) error {
	if !s.OS().HasCapability(os.CapContractWrite) {
		return fmt.Errorf("contract write capability not available")
	}

	response := &os.ServiceResponse{
		RequestID: requestID,
		Success:   true,
		Result:    result,
	}

	return s.OS().Contract().SendCallback(ctx, response)
}

// sendTriggerError sends an error callback for trigger operations.
func (s *Service) sendTriggerError(ctx context.Context, requestID, errMsg string) error {
	if !s.OS().HasCapability(os.CapContractWrite) {
		return fmt.Errorf("contract write capability not available")
	}

	response := &os.ServiceResponse{
		RequestID: requestID,
		Success:   false,
		Error:     errMsg,
	}

	return s.OS().Contract().SendCallback(ctx, response)
}
