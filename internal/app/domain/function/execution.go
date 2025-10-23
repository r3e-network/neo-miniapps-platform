package function

import "time"

// ExecutionStatus describes the lifecycle result of a function run.
type ExecutionStatus string

const (
	ExecutionStatusSucceeded ExecutionStatus = "succeeded"
	ExecutionStatusFailed    ExecutionStatus = "failed"
)

// ActionStatus captures the outcome of a devpack action.
type ActionStatus string

const (
	ActionStatusPending   ActionStatus = "pending"
	ActionStatusSucceeded ActionStatus = "succeeded"
	ActionStatusFailed    ActionStatus = "failed"
)

// Action defines an instruction emitted by the devpack runtime.
type Action struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Params map[string]any `json:"params,omitempty"`
}

const (
	ActionTypeGasBankEnsureAccount = "gasbank.ensureAccount"
	ActionTypeGasBankWithdraw      = "gasbank.withdraw"
	ActionTypeOracleCreateRequest  = "oracle.createRequest"
	ActionTypeTriggerRegister      = "triggers.register"
	ActionTypeAutomationSchedule   = "automation.schedule"
)

// ActionResult records the execution result for an action.
type ActionResult struct {
	Action
	Status ActionStatus   `json:"status"`
	Result map[string]any `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

// ExecutionResult captures the outcome returned by an executor.
type ExecutionResult struct {
	FunctionID    string          `json:"function_id"`
	Output        map[string]any  `json:"output"`
	Logs          []string        `json:"logs,omitempty"`
	Error         string          `json:"error,omitempty"`
	Status        ExecutionStatus `json:"status"`
	StartedAt     time.Time       `json:"started_at"`
	CompletedAt   time.Time       `json:"completed_at"`
	Duration      time.Duration   `json:"duration"`
	Actions       []Action        `json:"actions,omitempty"`
	ActionResults []ActionResult  `json:"action_results,omitempty"`
}

// Execution is the persisted record of a function run.
type Execution struct {
	ID          string          `json:"id"`
	AccountID   string          `json:"account_id"`
	FunctionID  string          `json:"function_id"`
	Input       map[string]any  `json:"input"`
	Output      map[string]any  `json:"output"`
	Logs        []string        `json:"logs,omitempty"`
	Error       string          `json:"error,omitempty"`
	Status      ExecutionStatus `json:"status"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt time.Time       `json:"completed_at"`
	Duration    time.Duration   `json:"duration"`
	Actions     []ActionResult  `json:"actions,omitempty"`
}
